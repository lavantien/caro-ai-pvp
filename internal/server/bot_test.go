package server

import (
	"errors"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var (
	botWinMoves = []string{"D4", "D5", "D6", "D7", "D3"}
	humanFiller = []string{"P16", "H8", "P12", "P8", "N16"}
)

type scriptedBot struct {
	mu       sync.Mutex
	script   []rules.Move
	stats    engine.SearchStats
	searches int
	closed   bool
}

func fakeStats() engine.SearchStats {
	st := engine.SearchStats{
		Depth: 9, Nodes: 45_000, Nps: 1_200_000, EBFMilli: 1400,
		TTHitPermille: 180, HashFullPermille: 600, FirstMoveFailHighPermille: 880,
		Score: config.EvalMateMax - 9*config.EvalMateScoreStep, Threads: 1,
		ElapsedNs: int64(30 * time.Millisecond), AllocNs: int64(3 * time.Second), PVLen: 9,
	}
	for i, name := range []string{"G7", "H7", "G8", "G6", "G9", "G10", "F8", "E9", "I8"} {
		cell, err := rules.ParseCell(name)
		if err != nil {
			panic("bot test: bad pv cell " + name)
		}
		st.PV[i] = rules.Move(cell)
	}
	return st
}
func (b *scriptedBot) Search(_ *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	mv := b.script[0]
	b.script = b.script[1:]
	b.searches++
	return mv, b.stats, ""
}
func (b *scriptedBot) StartPonder(*rules.Board) {}

func (b *scriptedBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *scriptedBot) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}
func (b *scriptedBot) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
func (b *scriptedBot) searchCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.searches
}
func injectScriptedBot(t *testing.T, r *Room, script []rules.Move) func() []*scriptedBot {
	t.Helper()
	var mu sync.Mutex
	var bots []*scriptedBot
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher {
		b := &scriptedBot{script: append([]rules.Move(nil), script...), stats: fakeStats()}
		mu.Lock()
		bots = append(bots, b)
		mu.Unlock()
		return b
	}
	r.mu.Unlock()
	return func() []*scriptedBot {
		mu.Lock()
		defer mu.Unlock()
		return append([]*scriptedBot(nil), bots...)
	}
}

type gatedBot struct {
	mu       sync.Mutex
	searches int
	started  sync.Once
	seen     chan struct{}
	release  chan struct{}
	closed   bool
}

func (g *gatedBot) Search(b *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	g.mu.Lock()
	g.searches++
	g.mu.Unlock()
	g.started.Do(func() { close(g.seen) })
	<-g.release
	var buf [config.BoardCells]rules.Move
	if b.LegalMoves(buf[:]) == 0 {
		panic("bot test: no legal move for the gated bot")
	}
	return buf[0], engine.SearchStats{}, ""
}
func (g *gatedBot) StartPonder(*rules.Board) {}

func (g *gatedBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (g *gatedBot) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}
func (g *gatedBot) isClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

type contractBot struct {
	mu     sync.Mutex
	closed bool
}

func (b *contractBot) Search(bd *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		panic("engine contract: Search on a closed instance")
	}
	var buf [config.BoardCells]rules.Move
	if bd.LegalMoves(buf[:]) == 0 {
		panic("bot test: no legal move for the contract bot")
	}
	return buf[0], engine.SearchStats{}, ""
}
func (b *contractBot) StartPonder(*rules.Board) {}

func (b *contractBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *contractBot) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}
func (b *contractBot) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
func blockAtSearchEntry(t *testing.T, r *Room) (entered, release chan struct{}) {
	t.Helper()
	entered, release = make(chan struct{}, 1), make(chan struct{})
	r.mu.Lock()
	r.searchEntry = func() {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}
	r.mu.Unlock()
	return entered, release
}
func TestForfeitInSearchEntryGapSurvives(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &contractBot{}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	entered, release := blockAtSearchEntry(t, r)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-entered
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit inside the search entry gap: %v", err)
	}
	close(release)
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	waitFor(t, bot.isClosed)
	r.mu.Lock()
	stones := r.board.MoveCount
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after the discarded gap search = %d, want only the human move", stones)
	}
	if err := r.Forfeit(alice.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("double forfeit = %v, want ErrRoomClosed", err)
	}
}
func TestShutdownInSearchEntryGapSurvives(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &contractBot{}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	entered, release := blockAtSearchEntry(t, r)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-entered
	shut := make(chan struct{})
	go func() {
		defer close(shut)
		s.rm.Shutdown()
	}()
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	close(release)
	select {
	case <-shut:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown did not join the bot worker within the bound")
	}
	waitFor(t, bot.isClosed)
	r.mu.Lock()
	stones := r.board.MoveCount
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after shutdown in the gap = %d, want only the human move", stones)
	}
}
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not reached within timeout")
}
func humanTurn(r *Room, userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over || r.board == nil {
		return false
	}
	seat := r.seatByColorLocked(r.board.Side)
	return seat.bot == nil && seat.userID == userID
}
func botRoom(t *testing.T, s *stack, tier config.Tier) (User, *Room) {
	t.Helper()
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier)
	if err != nil {
		t.Fatalf("create vs bot: %v", err)
	}
	return alice, r
}
func drainEvents(sub *Subscription) []string {
	var out []string
	for {
		select {
		case ev := <-sub.Events():
			out = append(out, ev.Kind+" "+ev.Payload)
		default:
			return out
		}
	}
}
func TestBotSeriesScriptedThroughWorker(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bots := injectScriptedBot(t, r, movesOf(t, botWinMoves))
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for game := 0; game < 2; game++ {
		for i, name := range humanFiller {
			waitFor(t, func() bool { return humanTurn(r, alice.ID) })
			if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
				t.Fatalf("game %d human move %d: %v", game+1, i+1, err)
			}
		}
	}
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	instances := bots()
	if len(instances) != 2 {
		t.Fatalf("engine instances = %d, want one per game", len(instances))
	}
	for i, b := range instances {
		waitFor(t, b.isClosed)
		if b.searchCount() != len(botWinMoves) {
			t.Errorf("engine %d searches = %d, want %d", i, b.searchCount(), len(botWinMoves))
		}
	}
	seq := drainEvents(sub)
	roundLen := len(humanFiller)*2 + len(botWinMoves)
	if len(seq) != 2*(roundLen+1)+1 {
		t.Fatalf("stream length = %d, want %d: %v", len(seq), 2*(roundLen+1)+1, seq)
	}
	for game := 0; game < 2; game++ {
		base := game * (roundLen + 1)
		for j := range humanFiller {
			k := base + 3*j
			if !strings.HasPrefix(seq[k], EventKindMove+" ") ||
				!strings.HasPrefix(seq[k+1], EventKindMove+" ") ||
				!strings.HasPrefix(seq[k+2], EventKindMLine+" M") {
				t.Errorf("game %d round %d = %q/%q/%q, want move, move, mline", game+1, j, seq[k], seq[k+1], seq[k+2])
			}
		}
		if got := seq[base+roundLen]; got != EventKindGameEnd+" "+OutcomeBlue {
			t.Errorf("game %d end = %q, want blue", game+1, got)
		}
	}
	if got := seq[len(seq)-1]; got != EventKindSeries+" "+SideGuest.String() {
		t.Errorf("series end = %q, want guest", got)
	}
	r.mu.Lock()
	rec := r.lastM
	redCommits, blueCommits := r.clock[rules.Red].Moves(), r.clock[rules.Blue].Moves()
	r.mu.Unlock()
	var lastMLine string
	for _, e := range seq {
		if strings.HasPrefix(e, EventKindMLine+" ") {
			lastMLine = strings.TrimPrefix(e, EventKindMLine+" ")
		}
	}
	if want := MLine(rec.moveNumber, rec.side, rec.move, &rec.stats, ""); lastMLine != want {
		t.Errorf("mline payload = %q, want renderer output %q", lastMLine, want)
	}
	if blueCommits != len(botWinMoves) || redCommits != len(humanFiller) {
		t.Errorf("final game clock commits = red %d blue %d, want 5 and 5", redCommits, blueCommits)
	}
	if id := r.SeriesID(); id == 0 {
		t.Error("bot room series id = 0, want the persisted pairing row")
	}
	if rows, err := s.store.MatchHistory(alice.ID); err != nil || len(rows) != 2 {
		t.Errorf("bot room history = %d rows err %v, want the two played games", len(rows), err)
	}
	if hist, err := s.store.RatingHistoryByUser(alice.ID); err != nil || len(hist) != 0 {
		t.Errorf("bot room rating events = %d err %v, want none", len(hist), err)
	}
}
func TestForfeitDuringBotSearchDiscardsAnswer(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-bot.seen
	if err := r.PlayMove(alice.ID, mustCellT(t, "P16")); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("human moves for the bot = %v, want ErrNotYourTurn", err)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit during search: %v", err)
	}
	close(bot.release)
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	waitFor(t, bot.isClosed)
	r.mu.Lock()
	stones := r.board.MoveCount
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after discarded search = %d, want only the human move", stones)
	}
	if err := r.Forfeit(alice.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("double forfeit = %v, want ErrRoomClosed", err)
	}
}
func TestBotRoomMediumBeatsRandomMover(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierMedium)
	r.mu.Lock()
	r.budgetCap = 3 * time.Millisecond
	r.mu.Unlock()
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	var mu sync.Mutex
	var moves, mlines int
	var lastMLine string
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range sub.Events() {
			mu.Lock()
			switch ev.Kind {
			case EventKindMove:
				moves++
			case EventKindMLine:
				mlines++
				lastMLine = ev.Payload
			}
			mu.Unlock()
		}
	}()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	rng := rand.New(rand.NewPCG(20261002, 42))
	deadline := time.Now().Add(90 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("medium bot did not close the series within the time box")
		}
		if _, ok := r.Info(); !ok {
			break
		}
		if !humanTurn(r, alice.ID) {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		r.mu.Lock()
		var buf [config.BoardCells]rules.Move
		n := r.board.LegalMoves(buf[:])
		cell := rules.Cell(buf[rng.IntN(n)])
		blueRem, redRem := r.clock[rules.Blue].Remaining(), r.clock[rules.Red].Remaining()
		r.mu.Unlock()
		if blueRem < 0 || redRem < 0 {
			t.Fatalf("clock below zero: blue %s red %s", blueRem, redRem)
		}
		if err := r.PlayMove(alice.ID, cell); err != nil {
			t.Fatalf("random human move: %v", err)
		}
	}
	sub.Unsubscribe()
	<-drained
	r.mu.Lock()
	hostWins, guestWins := r.series.Score()
	played := r.series.GamesPlayed()
	rec := r.lastM
	blueCommits := r.clock[rules.Blue].Moves()
	r.mu.Unlock()
	if guestWins != config.SeriesBO3/2+1 || hostWins != 0 {
		t.Errorf("final line = %d-%d after %d games, want the bot at majority", hostWins, guestWins, played)
	}
	if played > config.SeriesBO3 {
		t.Errorf("games played = %d, want at most the bo3 schedule", played)
	}
	mu.Lock()
	gotMoves, gotMLines, gotLast := moves, mlines, lastMLine
	mu.Unlock()
	if gotMLines == 0 || gotMoves < gotMLines {
		t.Errorf("stream counts = %d moves vs %d mlines", gotMoves, gotMLines)
	}
	if want := MLine(rec.moveNumber, rec.side, rec.move, &rec.stats, rec.tag); gotLast != want {
		t.Errorf("last mline = %q, want renderer output %q", gotLast, want)
	}
	if blueCommits == 0 {
		t.Error("bot clock never committed")
	}
}
func TestNoGoroutineLeakAcrossBotRooms(t *testing.T) {
	s := newStack(t)
	base := runtime.NumGoroutine()
	alice := seedUser(t, s.store, "alice")
	newRoom := func() *Room {
		r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return r
	}
	scripted := newRoom()
	injectScriptedBot(t, scripted, movesOf(t, botWinMoves))
	if err := scripted.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for _, name := range humanFiller {
		waitFor(t, func() bool { return humanTurn(scripted, alice.ID) })
		if err := scripted.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("scripted room move %s: %v", name, err)
		}
	}
	live := newRoom()
	gbot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	live.mu.Lock()
	live.makeSearcher = func(config.Tier) searcher { return gbot }
	live.mu.Unlock()
	if err := live.Ready(alice.ID); err != nil {
		t.Fatalf("ready live: %v", err)
	}
	if err := live.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("live move: %v", err)
	}
	<-gbot.seen
	close(gbot.release)
	s.rm.Shutdown()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("goroutines = %d after shutdown, want at most %d", runtime.NumGoroutine(), base+1)
}

type budgetProbeBot struct {
	mu      sync.Mutex
	seen    chan struct{}
	answer  rules.Move
	budgets []time.Duration
}

func (b *budgetProbeBot) Search(_ *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	if bg, ok := dl.(engine.Budgeter); ok {
		b.mu.Lock()
		b.budgets = append(b.budgets, bg.Budget())
		b.mu.Unlock()
	}
	select {
	case b.seen <- struct{}{}:
	default:
	}
	return b.answer, fakeStats(), ""
}
func (b *budgetProbeBot) StartPonder(*rules.Board) {}

func (b *budgetProbeBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *budgetProbeBot) Close() {}

func (b *budgetProbeBot) firstBudget(t *testing.T) time.Duration {
	t.Helper()
	<-b.seen
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.budgets) != 1 {
		t.Fatalf("bot searches = %d, want the single granted turn", len(b.budgets))
	}
	return b.budgets[0]
}

func TestBotTurnBudgetCapClampsOnlyWhenSet(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	run := func(limit time.Duration) time.Duration {
		r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy)
		if err != nil {
			t.Fatalf("create vs bot: %v", err)
		}
		bot := &budgetProbeBot{seen: make(chan struct{}, 1), answer: rules.Move(mustCellT(t, "D5"))}
		r.mu.Lock()
		r.makeSearcher = func(config.Tier) searcher { return bot }
		r.budgetCap = limit
		r.mu.Unlock()
		if err := r.Ready(alice.ID); err != nil {
			t.Fatalf("ready: %v", err)
		}
		if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
			t.Fatalf("human move: %v", err)
		}
		return bot.firstBudget(t)
	}
	if got := run(0); got <= 0 {
		t.Errorf("uncapped default grant = %s, want the clock budget with cap 0 meaning uncapped", got)
	}
	if got := run(1); got != 1 {
		t.Errorf("capped grant = %s, want the exact 1ns clamp", got)
	}
}

func TestBotTurnLockedRefusesFinishedWindow(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-bot.seen
	r.mu.Lock()
	r.finished = true
	side, ok := r.botTurnLocked()
	r.mu.Unlock()
	if ok || side != 0 {
		t.Errorf("botTurnLocked in the finished window = (%d, %t), want (0, false)", side, ok)
	}
	close(bot.release)
	r.Close()
	r.mu.Lock()
	stones := r.board.MoveCount
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after the finished-window turn = %d, want the discarded answer to leave the human move", stones)
	}
}

func TestBotTurnLockedRefusesRetireWindow(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-bot.seen
	r.mu.Lock()
	close(r.quit)
	side, ok := r.botTurnLocked()
	r.mu.Unlock()
	if ok || side != 0 {
		t.Errorf("botTurnLocked in the retire window = (%d, %t), want (0, false)", side, ok)
	}
	close(bot.release)
	waitFor(t, bot.isClosed)
	r.mu.Lock()
	stones := r.board.MoveCount
	r.quit = make(chan struct{}, 1)
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after the retire-window turn = %d, want the post-Close answer discarded on the closed quit", stones)
	}
}

type finishingBot struct {
	room   *Room
	answer rules.Move
}

func (b *finishingBot) Search(_ *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.room.mu.Lock()
	b.room.finished = true
	b.room.mu.Unlock()
	return b.answer, fakeStats(), ""
}
func (b *finishingBot) StartPonder(*rules.Board) {}

func (b *finishingBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *finishingBot) Close() {}

func TestBotTurnDiscardsAnswerWhenGameFinishesMidSearch(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher {
		return &finishingBot{room: r, answer: rules.Move(mustCellT(t, "D5"))}
	}
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.finished
	})
	time.Sleep(100 * time.Millisecond)
	r.mu.Lock()
	stones, moves := r.board.MoveCount, len(r.moves)
	r.mu.Unlock()
	if stones != 1 || moves != 1 {
		t.Errorf("stones %d moves %d after the mid-search finish, want the stale answer discarded", stones, moves)
	}
	if humanTurn(r, alice.ID) {
		t.Error("human holds the turn after the mid-search finish, want the bot's unfinished turn preserved")
	}
}

func TestBotTurnDiscardsAnswerAfterCloseMidSearch(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-bot.seen
	r.mu.Lock()
	close(r.quit)
	r.mu.Unlock()
	close(bot.release)
	waitFor(t, bot.isClosed)
	r.mu.Lock()
	stones := r.board.MoveCount
	r.quit = make(chan struct{}, 1)
	r.mu.Unlock()
	if stones != 1 {
		t.Errorf("stones after a Close during the search = %d, want the stale answer discarded", stones)
	}
}

type interleavedSearchBot struct {
	room     *Room
	first    rules.Move
	second   rules.Move
	answer   rules.Move
	injected bool
	parked   chan struct{}
	release  chan struct{}
}

func (b *interleavedSearchBot) Search(_ *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.room.mu.Lock()
	if !b.injected {
		b.injected = true
		b.room.applyMoveLocked(rules.Blue, rules.Cell(b.first))
		b.room.applyMoveLocked(rules.Red, rules.Cell(b.second))
		b.room.mu.Unlock()
		return b.answer, fakeStats(), ""
	}
	b.room.mu.Unlock()
	close(b.parked)
	<-b.release
	return b.answer, fakeStats(), ""
}
func (b *interleavedSearchBot) StartPonder(*rules.Board) {}

func (b *interleavedSearchBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *interleavedSearchBot) Close() {}

func TestBotTurnDiscardsStaleAnswerAfterInterleavedMoves(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot2 := &interleavedSearchBot{
		room: r, first: rules.Move(mustCellT(t, "D5")), second: rules.Move(mustCellT(t, "P16")), answer: rules.Move(mustCellT(t, "D6")),
		parked: make(chan struct{}), release: make(chan struct{}),
	}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot2 }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.board.MoveCount == 3
	})
	waitFor(t, func() bool {
		select {
		case <-bot2.parked:
			return true
		default:
			return false
		}
	})
	r.mu.Lock()
	stones, last := r.board.MoveCount, r.moves[len(r.moves)-1]
	r.mu.Unlock()
	if stones != 3 || last != rules.Move(mustCellT(t, "P16")) {
		t.Errorf("stones %d last %s after the interleaved writer, want the stale answer discarded", stones, cellName(rules.Cell(last)))
	}
	close(bot2.release)
	if humanTurn(r, alice.ID) {
		t.Error("human holds the turn after the interleaved writer, want the bot's unfinished turn preserved")
	}
}

func TestBotIllegalAnswerOnTheLastCellFillsTheBoard(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	script := drawMoves(t)
	var human, bot []string
	for i, name := range script {
		if i%2 == 0 {
			human = append(human, name)
		} else {
			bot = append(bot, name)
		}
	}
	bot[len(bot)-1] = "D4"
	bots := injectScriptedBot(t, r, movesOf(t, bot))
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for _, name := range human {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("human move %s: %v", name, err)
		}
	}
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.series.GamesPlayed() == 1 && r.board != nil && r.board.MoveCount == 0
	})
	r.mu.Lock()
	drawn := append([]rules.Move(nil), r.lastMoves...)
	r.mu.Unlock()
	if len(drawn) != config.BoardCells || !slices.Equal(drawn, movesOf(t, script)) {
		t.Errorf("drawn game = %d moves, want the full %d-move fill through the single remaining cell", len(drawn), config.BoardCells)
	}
	instances := bots()
	if len(instances) != 2 {
		t.Fatalf("engine instances = %d, want a fresh engine for game 2 after the drawn opener", len(instances))
	}
	if got := instances[0].searchCount(); got != len(bot) {
		t.Errorf("bot searches = %d, want %d answers with the last answered through the fallback", got, len(bot))
	}
}

func TestFirstLegalPanicsOnAFullBoard(t *testing.T) {
	s := newStack(t)
	_, r := botRoom(t, s, config.TierEasy)
	full := rules.NewBoard()
	for _, name := range drawMoves(t) {
		full.Make(mustCellT(t, name))
	}
	r.mu.Lock()
	r.board = full
	panicked := true
	func() {
		defer func() {
			if recover() == nil {
				panicked = false
			}
		}()
		r.firstLegalLocked()
	}()
	r.board = nil
	r.mu.Unlock()
	if !panicked {
		t.Error("firstLegalLocked on a full board returned, want the no-legal-move invariant panic")
	}
}

func TestWakeBotCoalescesToASingleBufferedToken(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	<-bot.seen
	r.mu.Lock()
	r.wakeBotLocked()
	r.wakeBotLocked()
	tokens := 0
drain:
	for {
		select {
		case <-r.wake:
			tokens++
		default:
			break drain
		}
	}
	r.mu.Unlock()
	if tokens != 1 {
		t.Errorf("buffered wake tokens = %d, want exactly one coalesced signal", tokens)
	}
	close(bot.release)
	r.Close()
}
