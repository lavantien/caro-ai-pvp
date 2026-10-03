package server

// The M7 bot-vs-bot surface tests: creation and validation of rooms whose
// host seat is a bot too, a scripted full bo3 driven to the majority by the
// worker alone, the render surface (shell card, room page, room detail JSON)
// over a frozen live game, and the persistence law: bot-vs-bot rooms never
// land a row in any table.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The scripted sweep's per-tier lines: the host tier closes the D-file five
// as red in game 1 and as blue in game 2, the guest tier scatters far from
// the D file, so the host takes the bo3 2-0 with both wins exact fives. The
// guest line serves both games: game 1 ends before its sixth entry.
var (
	botVsBotHostLine  = []string{"D4", "H8", "D5", "D6", "D7", "D3"}
	botVsBotGuestLine = []string{"P16", "P12", "P8", "N16", "M4", "L2"}
	// botVsBotGame2 interleaves the guest red scatter with the host blue
	// five, the loser-takes-red rotation after game 1.
	botVsBotGame2 = []string{"P16", "D4", "P12", "H8", "P8", "D5", "N16", "D6", "M4", "D7", "L2", "D3"}
)

// startGatedBot delays its first Search until the test opens the gate, so a
// spectator can subscribe before the first stone of a room whose game 1
// starts inside the constructor.
type startGatedBot struct {
	gate  chan struct{}
	once  sync.Once
	inner searcher
}

func (b *startGatedBot) Search(bd *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.once.Do(func() { <-b.gate })
	return b.inner.Search(bd, dl)
}

func (b *startGatedBot) Close() { b.inner.Close() }

// scriptBotVsBotTiers swaps the manager's engine factory for scripted bots
// dispatched by tier name, the only signal the factory receives that
// separates the two seats of a bot-vs-bot room. Must run before the create:
// game 1 starts inside the constructor. The returned func opens the start
// gate.
func scriptBotVsBotTiers(t *testing.T, s *stack, byName map[string][]string) func() {
	t.Helper()
	parsed := make(map[string][]rules.Move, len(byName))
	for name, names := range byName {
		parsed[name] = movesOf(t, names)
	}
	gate := make(chan struct{})
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		return &startGatedBot{gate: gate, inner: &scriptedBot{
			script: append([]rules.Move(nil), parsed[tier.Name]...), stats: fakeStats(),
		}}
	}
	return func() { close(gate) }
}

// countRows reads one table's row count, the persistence law's whole assert.
func countRows(t *testing.T, s *stack, table string) int {
	t.Helper()
	var n int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestCreateBotVsBotValidation(t *testing.T) {
	s := newStack(t)
	easy, medium := &config.TierEasy, &config.TierMedium

	if _, err := s.rm.CreateBotVsBot(nil, medium, 0, config.SeriesBO3); !errors.Is(err, ErrBadTier) {
		t.Errorf("nil host tier = %v, want ErrBadTier", err)
	}
	if _, err := s.rm.CreateBotVsBot(easy, nil, 0, config.SeriesBO3); !errors.Is(err, ErrBadTier) {
		t.Errorf("nil guest tier = %v, want ErrBadTier", err)
	}
	if _, err := s.rm.CreateBotVsBot(nil, nil, 0, config.SeriesBO3); !errors.Is(err, ErrBadTier) {
		t.Errorf("both tiers nil = %v, want ErrBadTier", err)
	}
	if _, err := s.rm.CreateBotVsBot(easy, medium, -1, config.SeriesBO3); !errors.Is(err, ErrBadTimeControl) {
		t.Errorf("bad tc = %v, want ErrBadTimeControl", err)
	}
	if _, err := s.rm.CreateBotVsBot(easy, medium, 0, 4); !errors.Is(err, ErrBadSeriesLength) {
		t.Errorf("bad bo = %v, want ErrBadSeriesLength", err)
	}
	if rooms := s.rm.List(); len(rooms) != 0 {
		t.Errorf("grid after rejected creates = %d rooms, want 0", len(rooms))
	}
}

func TestCreateBotVsBotSeatingAndLiveGame(t *testing.T) {
	s := newStack(t)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }

	r, err := s.rm.CreateBotVsBot(&config.TierEasy, &config.TierMedium, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	<-bot.seen // game 1 is live with the host bot mid-search

	rooms := s.rm.List()
	if len(rooms) != 1 || rooms[0].ID != r.ID() {
		t.Fatalf("grid = %+v, want the one bot-vs-bot room", rooms)
	}
	info, ok := r.Info()
	if !ok {
		t.Fatal("info: want the room live")
	}
	if info.HostUserID != botHostUserID || info.GuestUserID != botGuestUserID {
		t.Errorf("seats = %d vs %d, want the two bot sentinels", info.HostUserID, info.GuestUserID)
	}
	if info.HostBotTier != config.TierEasy.Name || info.VsBotTier != config.TierMedium.Name {
		t.Errorf("tiers = host %q guest %q, want easy vs medium", info.HostBotTier, info.VsBotTier)
	}
	if info.State != SeriesReady || info.HostWins != 0 || info.GuestWins != 0 {
		t.Errorf("info = %+v, want game 1 live at 0-0", info)
	}

	// Both handshakes landed inside the constructor; game 1 runs the host
	// bot on red per the spec's host-first rotation.
	if exists, hostReady, guestReady := r.readyFlags(); !exists || !hostReady || !guestReady {
		t.Errorf("readiness = %t %t %t, want the series formed with both ready", exists, hostReady, guestReady)
	}
	snap := r.gameSnapshot()
	if snap == nil || snap.Turn != "red" || snap.TurnUserID != botHostUserID || snap.RedUserID != botHostUserID {
		t.Errorf("snapshot = %+v, want game 1 live with the host bot on red", snap)
	}

	// The seats are closed to humans.
	bob := seedUser(t, s.store, "bob")
	if err := s.rm.Join(r.ID(), bob.ID); !errors.Is(err, ErrRoomFull) {
		t.Errorf("join bot-vs-bot room = %v, want ErrRoomFull", err)
	}
	if id := r.SeriesID(); id != 0 {
		t.Errorf("series id = %d, want 0: bot rooms persist never", id)
	}

	close(bot.release)
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	for _, table := range []string{"series", "games", "game_stats", "rating_events"} {
		if n := countRows(t, s, table); n != 0 {
			t.Errorf("%s rows after the finished room = %d, want 0", table, n)
		}
	}
}

func TestBotVsBotSeriesScriptedToMajority(t *testing.T) {
	s := newStack(t)
	open := scriptBotVsBotTiers(t, s, map[string][]string{
		config.TierEasy.Name:   botVsBotHostLine,
		config.TierMedium.Name: botVsBotGuestLine,
	})
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, &config.TierMedium, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	open()

	waitFor(t, func() bool { _, ok := r.Info(); return !ok })

	// The host closed the bo3 at the 2-of-3 majority in two games.
	r.mu.Lock()
	hostWins, guestWins := r.series.Score()
	played, winner, state := r.series.GamesPlayed(), r.series.Winner(), r.series.State()
	r.mu.Unlock()
	if hostWins != 2 || guestWins != 0 || played != 2 || winner != SideHost || state != SeriesFinished {
		t.Errorf("final line = %d-%d over %d games winner %v state %v, want 2-0 sweep by host",
			hostWins, guestWins, played, winner, state)
	}
	if rooms := s.rm.List(); len(rooms) != 0 {
		t.Errorf("grid after the sweep = %d rooms, want 0", len(rooms))
	}
	for _, table := range []string{"series", "games", "game_stats", "rating_events"} {
		if n := countRows(t, s, table); n != 0 {
			t.Errorf("%s rows after the sweep = %d, want 0", table, n)
		}
	}

	// Stream shape: every stone as move plus M-line, a gameend per game
	// (red then blue, the rotation between them), the series close.
	seq := drainEvents(sub)
	g1 := hostWinsRed
	if want := 2*(len(g1)+len(botVsBotGame2)+1) + 1; len(seq) != want {
		t.Fatalf("stream = %d events, want %d: %v", len(seq), want, seq)
	}
	for i, name := range g1 {
		if seq[2*i] != EventKindMove+" "+name {
			t.Errorf("game 1 event %d = %q, want move %s", 2*i, seq[2*i], name)
		}
		if !strings.HasPrefix(seq[2*i+1], EventKindMLine+" M") {
			t.Errorf("game 1 event %d = %q, want an M-line", 2*i+1, seq[2*i+1])
		}
	}
	if got := seq[2*len(g1)]; got != EventKindGameEnd+" "+OutcomeRed {
		t.Errorf("game 1 end = %q, want red", got)
	}
	base := 2*len(g1) + 1
	for i, name := range botVsBotGame2 {
		if seq[base+2*i] != EventKindMove+" "+name {
			t.Errorf("game 2 event %d = %q, want move %s", base+2*i, seq[base+2*i], name)
		}
		if !strings.HasPrefix(seq[base+2*i+1], EventKindMLine+" M") {
			t.Errorf("game 2 event %d = %q, want an M-line", base+2*i+1, seq[base+2*i+1])
		}
	}
	if got := seq[base+2*len(botVsBotGame2)]; got != EventKindGameEnd+" "+OutcomeBlue {
		t.Errorf("game 2 end = %q, want blue", got)
	}
	if got := seq[len(seq)-1]; got != EventKindSeries+" "+SideHost.String() {
		t.Errorf("series end = %q, want host", got)
	}
}

// TestRoomLastGameMoves pins the room's authoritative record of the game
// that just ended: nil before any completion, the finished game's played
// cells readable into the next game, and the next completion overwriting
// it. The tournament conductor reconciles its delivered stream against this
// list, so the accessor must mirror exactly what the room applied.
func TestRoomLastGameMoves(t *testing.T) {
	s := newStack(t)

	// Before any completion the accessor is nil: game 1 is live, nothing
	// has finished.
	parked := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return parked }
	fresh, err := s.rm.CreateBotVsBot(&config.TierEasy, &config.TierMedium, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	<-parked.seen
	if got := fresh.LastGameMoves(); got != nil {
		t.Errorf("last game moves before any completion = %v, want nil", got)
	}
	close(parked.release)
	waitFor(t, func() bool { _, ok := fresh.Info(); return !ok })

	// Game 1's list equals the played cells and survives into game 2, which
	// parks mid-search; game 2's completion overwrites it.
	var hostEngines atomic.Int32
	game2 := make(chan struct{})
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		script := movesOf(t, botVsBotHostLine)
		if tier.Name != config.TierEasy.Name {
			script = movesOf(t, botVsBotGuestLine)
		} else if hostEngines.Add(1) == 2 {
			return &startGatedBot{gate: game2,
				inner: &scriptedBot{script: script, stats: fakeStats()}}
		}
		return &scriptedBot{script: script, stats: fakeStats()}
	}
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, &config.TierMedium, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	waitFor(t, func() bool {
		info, ok := r.Info()
		return ok && info.HostWins == 1
	})
	if got, want := r.LastGameMoves(), movesOf(t, hostWinsRed); !slices.Equal(got, want) {
		t.Errorf("last game moves while game 2 parks = %v, want game 1's %v", got, want)
	}
	close(game2)
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	if got, want := r.LastGameMoves(), movesOf(t, botVsBotGame2); !slices.Equal(got, want) {
		t.Errorf("last game moves after game 2 = %v, want game 2's %v", got, want)
	}
}

func TestBotVsBotRenderSurface(t *testing.T) {
	s := newStack(t)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, &config.TierMedium, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	<-bot.seen
	defer close(bot.release)

	// The shell card names both bots by tier, never the synthetic id.
	shellSrv := httptest.NewServer(NewShellPages(s.store, s.rm))
	defer shellSrv.Close()
	status, _, body := doShell(t, shellSrv.Client(), http.MethodGet, shellSrv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d, want 200", status)
	}
	easyName, mediumName := botDisplayName("easy", r.ID()), botDisplayName("medium", r.ID())
	if !strings.Contains(body, easyName+" vs "+mediumName) {
		t.Errorf("home card misses %q", easyName+" vs "+mediumName)
	}
	if strings.Contains(body, "#-3") {
		t.Error("home card leaks the synthetic host id")
	}

	// The room page renders tier names on the clock labels and the score
	// line; a store lookup of the synthetic id would read as a missing
	// account and fail the page.
	pageSrv := newPageServer(t, s)
	status, body = getRoomPage(t, pageSrv, "/rooms/"+r.ID(), "")
	if status != http.StatusOK {
		t.Fatalf("room page: status = %d, want 200", status)
	}
	for _, want := range []string{
		"red / " + easyName, "blue / " + mediumName, easyName + " 0 - 0 " + mediumName, "red to move (" + easyName + ")",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("room page misses %q", want)
		}
	}
	if strings.Contains(body, "#-3") {
		t.Error("room page leaks the synthetic host id")
	}

	// The room detail JSON carries the host bot tier beside the guest's.
	apiSrv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer apiSrv.Close()
	status, _, body = doGet(t, apiSrv.Client(), apiSrv.URL+"/api/rooms/"+r.ID())
	if status != http.StatusOK {
		t.Fatalf("room detail: status = %d, want 200", status)
	}
	for _, want := range []string{`"hostBotTier":"easy"`, `"vsBotTier":"medium"`} {
		if !strings.Contains(body, want) {
			t.Errorf("room detail misses %q: %s", want, body)
		}
	}
}
