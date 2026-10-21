package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func fillTTBudget(t *testing.T, s *stack, rooms int) []*Room {
	t.Helper()
	s.rm.makeSearcher = func(config.Tier) searcher { return &contractBot{} }
	holder := seedUser(t, s.store, "holder")
	out := make([]*Room, 0, rooms)
	for range rooms {
		r, err := s.rm.Create(holder.ID, 0, config.SeriesBO3, &config.TierGrandmaster)
		if err != nil {
			t.Fatalf("fill the tt budget: %v", err)
		}
		if err := r.Ready(holder.ID); err != nil {
			t.Fatalf("ready the filler room: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func TestCreateRefusesBotRoomWhenTTBudgetFull(t *testing.T) {
	s := newStack(t)
	fill := fillTTBudget(t, s, int(config.MachineTTBudgetBytes/config.TierGrandmaster.TTBytes))
	if got := s.rm.adm.TTFree(); got != 0 {
		t.Fatalf("ttFree after the fill = %d, want 0", got)
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("idle filled rooms hold %d cores, want 0: cores attach to search windows only", got)
	}
	rows := countRows(t, s, "series")

	alice := seedUser(t, s.store, "alice")
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierGrandmaster); !errors.Is(err, ErrMachineBusy) {
		t.Errorf("create over the full tt budget = %v, want ErrMachineBusy", err)
	}
	if got := countRows(t, s, "series"); got != rows {
		t.Errorf("series rows = %d after the refused create, want the held %d", got, rows)
	}
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil); err != nil {
		t.Errorf("human room over the full tt budget: %v", err)
	}

	fill[0].Close()
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.TierGrandmaster.TTBytes })
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierGrandmaster); err != nil {
		t.Fatalf("create after the tt release: %v", err)
	}
}

func TestBotVsBotRefusesWhenTTBudgetFull(t *testing.T) {
	s := newStack(t)
	fill := fillTTBudget(t, s, int(config.MachineTTBudgetBytes/config.TierGrandmaster.TTBytes))
	if _, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierEasy, "", 0, config.SeriesBO3); !errors.Is(err, ErrMachineBusy) {
		t.Fatalf("bot vs bot over the full tt budget = %v, want ErrMachineBusy", err)
	}

	fill[0].Close()
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.TierGrandmaster.TTBytes })
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create within the budget: %v", err)
	}
	<-bot.seen
	if got, want := s.rm.LiveCoreBookings(), config.TierEasy.Cores; got != want {
		t.Fatalf("cores during the gated easy search = %d, want %d", got, want)
	}
	close(bot.release)
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the finished series = %d, want 0", got)
	}
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.TierGrandmaster.TTBytes })
}

func TestSearchWindowHoldsTierCoresOnlyDuringTheSearch(t *testing.T) {
	s := newStack(t)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMaster)
	if err != nil {
		t.Fatalf("create vs master: %v", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready vs master: %v", err)
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("cores while the human thinks = %d, want 0", got)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); err != nil {
		t.Fatalf("human opens: %v", err)
	}
	<-bot.seen
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores; got != want {
		t.Fatalf("cores during the master search = %d, want %d", got, want)
	}
	if got, want := s.rm.adm.TTFree(), config.MachineTTBudgetBytes-config.TierMaster.TTBytes; got != want {
		t.Fatalf("ttFree with master built = %d, want %d", got, want)
	}
	close(bot.release)
	waitFor(t, func() bool { return s.rm.LiveCoreBookings() == 0 })
	r.Close()
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
}

func TestBotRoomForfeitDuringSearchDrainsLedger(t *testing.T) {
	s := newStack(t)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierHard)
	if err != nil {
		t.Fatalf("create vs hard: %v", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready vs hard: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); err != nil {
		t.Fatalf("human opens: %v", err)
	}
	<-bot.seen
	if got, want := s.rm.LiveCoreBookings(), config.TierHard.Cores; got != want {
		t.Fatalf("cores during the hard search = %d, want %d", got, want)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit during the search: %v", err)
	}
	close(bot.release)
	r.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the forfeited room = %d, want 0", got)
	}
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
}

func TestBotRoomRetireReleasesLedgerExactlyOnce(t *testing.T) {
	s := newStack(t)
	open := scriptBotVsBotTiers(t, s, map[string][]string{
		config.TierEasy.Name:   botVsBotHostLine,
		config.TierMedium.Name: botVsBotGuestLine,
	})
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	open()
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	r.retire()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the stacked retires = %d, want 0", got)
	}
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
}

func TestSearchAdmissionTimeoutRetiresTheRoom(t *testing.T) {
	s := newStack(t)
	s.rm.adm.waitMax = 50 * time.Millisecond
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy)
	if err != nil {
		t.Fatalf("create vs easy: %v", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready vs easy: %v", err)
	}
	if !s.rm.adm.TryAcquireCores(config.MachineCores) {
		t.Fatal("hold the whole core budget for the turn")
	}
	if err := r.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); err != nil {
		t.Fatalf("human opens: %v", err)
	}
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	s.rm.adm.ReleaseCores(config.MachineCores)
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the timed-out room = %d, want 0", got)
	}
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
	close(bot.release)
}

func TestSecondMasterRoomWaitsForCoresThenProceeds(t *testing.T) {
	s := newStack(t)
	s.rm.adm.waitMax = 5 * time.Second
	gated := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	nobook := config.TierMaster
	nobook.Book = false
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if tier.Book {
			return gated
		}
		return &contractBot{}
	}
	alice := seedUser(t, s.store, "alice")
	casual, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMaster)
	if err != nil {
		t.Fatalf("create the casual master room: %v", err)
	}
	if err := casual.Ready(alice.ID); err != nil {
		t.Fatalf("ready the casual room: %v", err)
	}
	if err := casual.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); err != nil {
		t.Fatalf("human opens the casual room: %v", err)
	}
	<-gated.seen
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores; got != want {
		t.Fatalf("cores held by the casual search = %d, want %d", got, want)
	}

	const squeezed = 5
	if !s.rm.adm.TryAcquireCores(squeezed) {
		t.Fatal("squeeze the remaining free cores")
	}
	second, err := s.rm.CreateBotVsBot(&nobook, "", &nobook, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create the second master room: %v", err)
	}
	if _, ok := second.Info(); !ok {
		t.Fatal("the second master room retired instead of waiting for cores")
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+squeezed; got != want {
		t.Fatalf("cores during the wait = %d, want the casual %d plus the squeeze %d with no partial squat", got, want-squeezed, squeezed)
	}
	s.rm.adm.ReleaseCores(squeezed)
	waitFor(t, func() bool {
		snap := second.gameSnapshot()
		return snap != nil && len(snap.Moves) >= 1
	})
	if _, ok := second.Info(); !ok {
		t.Fatal("the second master room died after its wait cleared")
	}
	second.Close()
	close(gated.release)
	casual.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the closed rooms = %d, want 0", got)
	}
}

func TestPonderArmsThroughTheLedgerAtFullWidth(t *testing.T) {
	s := newStack(t)
	hard := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	var masters []*ponderBot
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if tier.Name != config.TierMaster.Name {
			return hard
		}
		b := &ponderBot{}
		mv := movesOf(t, []string{"H8"})[0]
		reply := movesOf(t, []string{"P16"})[0]
		b.answers = []ponderAnswer{{move: mv, pv: []rules.Move{mv, reply},
			stats: engine.SearchStats{Depth: 12, Threads: 8}}}
		masters = append(masters, b)
		return b
	}
	r1, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierHard, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create master vs hard: %v", err)
	}
	<-hard.seen
	seat := masters[0]
	waitFor(t, func() bool {
		_, starts, _ := seat.counts()
		return starts == 1
	})
	peak := config.TierHard.Cores + config.TierMaster.PonderThreads
	if got := s.rm.LiveCoreBookings(); got != peak {
		t.Fatalf("cores during hard searching while master ponders = %d, want the peak %d", got, peak)
	}
	close(hard.release)
	r1.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the closed rooms = %d, want 0", got)
	}
}

func TestPonderLaneRefusesArmingWhenCoresUnfunded(t *testing.T) {
	adm := newAdmission()
	starved := config.MachineCores - config.TierMaster.PonderThreads + 1
	if !adm.TryAcquireCores(starved) {
		t.Fatalf("starve the budget to %d free cores", config.TierMaster.PonderThreads-1)
	}
	lane := newPonderLane()
	done := make(chan struct{})
	go func() {
		lane.run()
		close(done)
	}()
	bot := &ponderBot{}
	lane.start(bot, rules.Red, rules.NewBoard(), 0, 0, adm, config.TierMaster.PonderThreads)
	if res := lane.stop(rules.Red); res.active {
		t.Fatal("the unfunded arm went active, want the full-width-or-off refusal")
	}
	if _, starts, _ := bot.counts(); starts != 0 {
		t.Fatalf("StartPonder calls on an unfunded arm = %d, want 0", starts)
	}
	adm.ReleaseCores(starved)
	lane.start(bot, rules.Red, rules.NewBoard(), 0, 0, adm, config.TierMaster.PonderThreads)
	res := lane.stop(rules.Red)
	if !res.active {
		t.Fatal("the funded arm stayed inactive, want pondering")
	}
	if _, starts, _ := bot.counts(); starts != 1 {
		t.Fatalf("StartPonder calls after funding = %d, want 1", starts)
	}
	lane.close()
	<-done
	if got := adm.LiveCores(); got != 0 {
		t.Errorf("cores after the lane closed = %d, want 0", got)
	}
}

func TestBotVsBotPonderFollowsTierWidth(t *testing.T) {
	s := newStack(t)
	var mu sync.Mutex
	var spies []*ponderSpyBot
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		script := movesOf(t, botVsBotGuestLine)
		if tier.Name == config.TierEasy.Name {
			script = movesOf(t, botVsBotHostLine)
		}
		b := &ponderSpyBot{inner: &scriptedBot{script: script, stats: fakeStats()}}
		mu.Lock()
		spies = append(spies, b)
		mu.Unlock()
		return b
	}
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	mu.Lock()
	defer mu.Unlock()
	for i, b := range spies {
		want := 0
		if i == 1 {
			want = 1
		}
		if got := b.startCount(); got < want {
			t.Errorf("engine %d StartPonder calls = %d, want at least %d", i, got, want)
		}
	}
}

func TestCasualAndBotVsBotRoomsCoexist(t *testing.T) {
	s := newStack(t)
	gated := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	scriptGate := make(chan struct{})
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if tier.Name == config.TierMaster.Name {
			return gated
		}
		script := movesOf(t, botVsBotGuestLine)
		if tier.Name == config.TierEasy.Name {
			script = movesOf(t, botVsBotHostLine)
		}
		return &startGatedBot{gate: scriptGate, inner: &scriptedBot{script: script, stats: fakeStats()}}
	}
	alice := seedUser(t, s.store, "alice")
	casual, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMaster)
	if err != nil {
		t.Fatalf("create the casual master room: %v", err)
	}
	if err := casual.Ready(alice.ID); err != nil {
		t.Fatalf("ready the casual room: %v", err)
	}
	if err := casual.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); err != nil {
		t.Fatalf("human opens: %v", err)
	}
	<-gated.seen
	close(scriptGate)
	botRoom, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create the bot room under the held search: %v", err)
	}
	waitFor(t, func() bool { _, ok := botRoom.Info(); return !ok })
	if _, ok := casual.Info(); !ok {
		t.Fatal("the casual room died while the bot series ran")
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores; got != want {
		t.Fatalf("cores after the bot series under the gated search = %d, want the casual %d", got, want)
	}
	close(gated.release)
	casual.Close()
}

func TestNextGameTTRefusalParksTheRoom(t *testing.T) {
	s := newStack(t)
	hostTier := config.Tier{Name: "parkhost", Cores: 1, TTBytes: 1}
	guestTier := config.Tier{Name: "parkguest", Cores: 1, TTBytes: 1}
	constructions := 0
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		constructions++
		if constructions == 3 {
			if err := s.rm.adm.AcquireTT(config.MachineTTBudgetBytes - 1); err != nil {
				t.Errorf("hold the remaining tt budget: %v", err)
			}
		}
		script := movesOf(t, botVsBotGuestLine)
		if tier.Name == hostTier.Name {
			script = movesOf(t, botVsBotHostLine)
		}
		return &scriptedBot{script: script, stats: fakeStats()}
	}
	r, err := s.rm.CreateBotVsBot(&hostTier, "", &guestTier, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	s.rm.adm.ReleaseTT(config.MachineTTBudgetBytes - 1)
	r.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the parked room closed = %d, want 0", got)
	}
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
}

func TestReadyTTRefusalParksTheRoomAndRefusesMoves(t *testing.T) {
	s := newStack(t)
	s.rm.makeSearcher = func(config.Tier) searcher {
		return &scriptedBot{script: movesOf(t, botVsBotGuestLine), stats: fakeStats()}
	}
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMedium)
	if err != nil {
		t.Fatalf("create vs medium: %v", err)
	}
	drain := config.MachineTTBudgetBytes - config.TierMedium.TTBytes + 1
	if err := s.rm.adm.AcquireTT(drain); err != nil {
		t.Fatalf("drain the tt budget: %v", err)
	}
	if err := r.Ready(alice.ID); !errors.Is(err, ErrMachineBusy) {
		t.Fatalf("ready over the drained budget = %v, want ErrMachineBusy", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(movesOf(t, []string{"H8"})[0])); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("move into the parked room = %v, want ErrRoomClosed", err)
	}
	s.rm.adm.ReleaseTT(drain)
	r.Close()
	waitFor(t, func() bool { return s.rm.adm.TTFree() == config.MachineTTBudgetBytes })
	if _, ok := r.Info(); ok {
		t.Error("the parked room still reports live info")
	}
}

func TestShellCreateRoomRendersMachineBusy(t *testing.T) {
	s := newStack(t)
	fillTTBudget(t, s, int(config.MachineTTBudgetBytes/config.TierGrandmaster.TTBytes))
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	token := shellRegister(t, c, srv.URL, "alice", "hunter2")

	status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)}, "bot": {config.TierMaster.Name},
	})
	if status != http.StatusConflict {
		t.Fatalf("create over the full tt budget: status = %d, want 409 (body %s)", status, body)
	}
	if !strings.Contains(body, "fully booked") {
		t.Errorf("body misses the machine-busy refusal (body %s)", body)
	}
	if strings.Contains(body, "invalid room settings") {
		t.Error("the machine-busy refusal rendered as bad settings")
	}
	if code, status := errorResponse(ErrMachineBusy); code != codeMachineBusy || status != http.StatusConflict {
		t.Errorf("errorResponse(ErrMachineBusy) = %q %d, want %q %d", code, status, codeMachineBusy, http.StatusConflict)
	}
}
