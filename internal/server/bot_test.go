package server

import (
	"errors"
	"math/rand/v2"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The scripted loss line: the human scatters red stones far apart while
// the bot closes an open four on the D file in five blue moves.
var (
	botWinMoves = []string{"D4", "D5", "D6", "D7", "D3"}
	humanFiller = []string{"P16", "H8", "P12", "P8", "N16"}
)

// scriptedBot is a deterministic fake engine: it answers each turn with the
// next scripted move and fixed stats, recording its lifecycle for asserts.
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

func (b *scriptedBot) Search(_ *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats) {
	b.mu.Lock()
	defer b.mu.Unlock()
	mv := b.script[0]
	b.script = b.script[1:]
	b.searches++
	return mv, b.stats
}

func (b *scriptedBot) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}

// injectScriptedBot replaces the room's engine factory with the scripted
// fake and returns the live collector of created instances, one per game.
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

// gatedBot blocks inside Search until released, holding the bot turn open.
type gatedBot struct {
	mu       sync.Mutex
	searches int
	started  sync.Once
	seen     chan struct{}
	release  chan struct{}
	closed   bool
}

func (g *gatedBot) Search(b *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats) {
	g.mu.Lock()
	g.searches++
	g.mu.Unlock()
	g.started.Do(func() { close(g.seen) })
	<-g.release
	var buf [config.BoardCells]rules.Move
	if b.LegalMoves(buf[:]) == 0 {
		panic("bot test: no legal move for the gated bot")
	}
	return buf[0], engine.SearchStats{}
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

// humanTurn reports whether the human holds the move.
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

	// Two games: the bot wins both as blue and red never leaves the human,
	// because only a red win rotates the seat.
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

	// Engine ephemerality: one fresh instance per game, both closed.
	instances := bots()
	if len(instances) != 2 {
		t.Fatalf("engine instances = %d, want one per game", len(instances))
	}
	for i, b := range instances {
		if !b.closed {
			t.Errorf("engine %d not closed", i)
		}
		if b.searches != len(botWinMoves) {
			t.Errorf("engine %d searches = %d, want %d", i, b.searches, len(botWinMoves))
		}
	}

	// Stream shape: per round one human move, one bot move, its M-line;
	// per game an end; the series end closes it.
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

	// The last M-line payload is byte-identical to the canonical renderer
	// fed the exact recorded inputs.
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

	// Bot rooms persist nothing: no pairing row, no games, no rating move.
	if id := r.SeriesID(); id != 0 {
		t.Errorf("bot room series id = %d, want 0", id)
	}
	if rows, err := s.store.MatchHistory(alice.ID); err != nil || len(rows) != 0 {
		t.Errorf("bot room history = %d rows err %v, want none", len(rows), err)
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

	// Hand the turn to the blocked bot, then prove the human cannot move
	// for it and a quit mid-search settles the room without deadlock.
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
	// Time-box: the real engine capped at a few milliseconds per move.
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

	// The series closed legally: a bot sweep, never a flag fall, one M-line
	// per bot stone.
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
	if want := MLine(rec.moveNumber, rec.side, rec.move, &rec.stats, ""); gotLast != want {
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

	// A room that closes by series end.
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

	// A room shutdown mid-flight, with the bot turn in the search window.
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
	close(gbot.release) // the worker may still be inside Search when Shutdown lands
	s.rm.Shutdown()

	// Every room worker and every engine thread must leave; retries absorb
	// scheduler lag.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("goroutines = %d after shutdown, want at most %d", runtime.NumGoroutine(), base+1)
}
