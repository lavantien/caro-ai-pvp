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

func TestCoreLedgerBookRefuseRelease(t *testing.T) {
	rm := NewRoomManager(nil, nil, nil)
	if got := rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("fresh ledger = %d, want 0", got)
	}

	rm.mu.Lock()
	over := rm.bookCoresLocked(config.MachineCores + 1)
	rm.mu.Unlock()
	if over {
		t.Errorf("book %d over an empty budget = true, want refuse", config.MachineCores+1)
	}
	if got := rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the refused book = %d, want 0", got)
	}

	rm.mu.Lock()
	fits := rm.bookCoresLocked(config.MachineCores - 1)
	rm.mu.Unlock()
	if !fits {
		t.Fatal("book to budget-1 = false, want the booking")
	}
	rm.mu.Lock()
	last := rm.bookCoresLocked(1)
	rm.mu.Unlock()
	if !last {
		t.Fatal("exact-fit book to the budget = false, want the booking")
	}
	if got := rm.LiveCoreBookings(); got != config.MachineCores {
		t.Fatalf("ledger at the exact fit = %d, want %d", got, config.MachineCores)
	}
	rm.mu.Lock()
	one := rm.bookCoresLocked(1)
	rm.mu.Unlock()
	if one {
		t.Error("book one core over the full budget = true, want refuse")
	}

	rm.mu.Lock()
	rm.releaseCoresLocked(config.MachineCores)
	rm.mu.Unlock()
	if got := rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the release = %d, want 0", got)
	}
}

func TestCreateRefusesBotRoomWhenLedgerFull(t *testing.T) {
	s := newStack(t)
	s.rm.makeSearcher = func(config.Tier) searcher { return &contractBot{} }
	alice := seedUser(t, s.store, "alice")

	master1, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMaster)
	if err != nil {
		t.Fatalf("create the first master room: %v", err)
	}
	master2, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierMaster)
	if err != nil {
		t.Fatalf("create the second master room: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), 2*config.TierMaster.Cores; got != want {
		t.Fatalf("ledger = %d, want the two master bookings %d", got, want)
	}
	rows := countRows(t, s, "series")

	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy); !errors.Is(err, ErrMachineBusy) {
		t.Errorf("create over the full budget = %v, want ErrMachineBusy", err)
	}
	if got := countRows(t, s, "series"); got != rows {
		t.Errorf("series rows = %d after the refused create, want the held %d: the pairing row never persisted", got, rows)
	}
	if got, want := s.rm.LiveCoreBookings(), config.MachineCores; got != want {
		t.Errorf("ledger after the refusal = %d, want the held %d", got, want)
	}
	if rooms := s.rm.List(); len(rooms) != 2 ||
		rooms[0].ID != master1.ID() && rooms[1].ID != master1.ID() ||
		rooms[0].ID != master2.ID() && rooms[1].ID != master2.ID() {
		t.Errorf("grid = %+v, want the two master rooms", rooms)
	}

	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil); err != nil {
		t.Errorf("create a human room over the full budget: %v", err)
	}

	master1.Close()
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores; got != want {
		t.Fatalf("ledger after the first close = %d, want the held %d", got, want)
	}
	easy, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy)
	if err != nil {
		t.Fatalf("create after the release: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+config.TierEasy.Cores; got != want {
		t.Errorf("ledger = %d, want the master and easy bookings %d", got, want)
	}
	master2.Close()
	easy.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("ledger after every close = %d, want 0", got)
	}
}

func TestCreateBotVsBotRefusesWhenLedgerFull(t *testing.T) {
	s := newStack(t)
	bot := &gatedBot{seen: make(chan struct{}), release: make(chan struct{})}
	s.rm.makeSearcher = func(config.Tier) searcher { return bot }
	alice := seedUser(t, s.store, "alice")

	r, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierEasy, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create master vs easy: %v", err)
	}
	<-bot.seen
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+config.TierEasy.Cores; got != want {
		t.Fatalf("ledger = %d, want the ponding sum %d", got, want)
	}

	var fillers []*Room
	for _, tier := range []*config.Tier{&config.TierHard, &config.TierMedium, &config.TierEasy} {
		f, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, tier)
		if err != nil {
			t.Fatalf("fill with %s: %v", tier.Name, err)
		}
		fillers = append(fillers, f)
	}
	if got, want := s.rm.LiveCoreBookings(), config.MachineCores; got != want {
		t.Fatalf("ledger = %d, want the full budget %d", got, want)
	}

	if _, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierEasy, "", 0, config.SeriesBO3); !errors.Is(err, ErrMachineBusy) {
		t.Errorf("create over the full budget = %v, want ErrMachineBusy", err)
	}
	if boards := s.rm.BotBoards(); len(boards) != 1 || boards[0].RoomID != r.ID() {
		t.Errorf("live boards = %+v, want only the gated room", boards)
	}

	close(bot.release)
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	for _, f := range fillers {
		f.Close()
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the swept series = %d, want 0", got)
	}
}

func TestCreateBotVsBotPonderFunding(t *testing.T) {
	s := newStack(t)
	var masters []*ponderBot
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if !tier.Ponder {
			return &contractBot{}
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
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+config.TierHard.Cores; got != want {
		t.Fatalf("master/hard ledger = %d, want the ponding sum %d", got, want)
	}
	seat := masters[0]
	waitFor(t, func() bool {
		_, starts, _ := seat.counts()
		return starts == 1
	})
	r1.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("ledger after the ponding room close = %d, want 0", got)
	}

	r2, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierMaster, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create master vs master: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), 2*config.TierMaster.Cores; got != want {
		t.Fatalf("master/master ledger = %d, want the ponding sum %d", got, want)
	}
	r2.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Fatalf("ledger after both closes = %d, want 0", got)
	}
}

func TestCreateBotVsBotPonderFallsBackWhenSumCannotFund(t *testing.T) {
	s := newStack(t)
	holder := seedUser(t, s.store, "holder")
	if _, err := s.rm.Create(holder.ID, 0, config.SeriesBO3, &config.TierMaster); err != nil {
		t.Fatalf("fill the ledger: %v", err)
	}
	var masters []*ponderBot
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if !tier.Ponder {
			return &contractBot{}
		}
		b := &ponderBot{}
		mv := movesOf(t, []string{"H8"})[0]
		reply := movesOf(t, []string{"P16"})[0]
		b.answers = []ponderAnswer{{move: mv, pv: []rules.Move{mv, reply},
			stats: engine.SearchStats{Depth: 12, Threads: 8}}}
		masters = append(masters, b)
		return b
	}

	r, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierHard, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create over the sum: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.MachineCores; got != want {
		t.Fatalf("fallback ledger = %d, want the full-width max booking %d", got, want)
	}
	seat := masters[0]
	waitFor(t, func() bool {
		searches, _, _ := seat.counts()
		return searches >= 1
	})
	time.Sleep(150 * time.Millisecond)
	if _, starts, _ := seat.counts(); starts != 0 {
		t.Fatalf("fallback room armed a ponder %d times, want the whole game ponder-off", starts)
	}
	r.Close()
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores; got != want {
		t.Fatalf("ledger after the fallback close = %d, want the filler's %d", got, want)
	}
}

func TestCreateBotVsBotBusyWhenEvenMaxCannotFund(t *testing.T) {
	s := newStack(t)
	holder := seedUser(t, s.store, "holder")
	if _, err := s.rm.Create(holder.ID, 0, config.SeriesBO3, &config.TierMaster); err != nil {
		t.Fatalf("book the master filler: %v", err)
	}
	if _, err := s.rm.Create(holder.ID, 0, config.SeriesBO3, &config.TierEasy); err != nil {
		t.Fatalf("book the easy filler: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+config.TierEasy.Cores; got != want {
		t.Fatalf("filler ledger = %d, want %d", got, want)
	}
	s.rm.makeSearcher = func(config.Tier) searcher { return &contractBot{} }
	if _, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierHard, "", 0, config.SeriesBO3); !errors.Is(err, ErrMachineBusy) {
		t.Errorf("master/hard over the full budget = %v, want ErrMachineBusy", err)
	}
	if _, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierEasy, "", 0, config.SeriesBO3); err != nil {
		t.Fatalf("easy/easy within the budget: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMaster.Cores+2*config.TierEasy.Cores; got != want {
		t.Fatalf("ledger = %d, want the fillers plus the easy room %d", got, want)
	}
}

func TestBotRoomRetireReleasesBookingExactlyOnce(t *testing.T) {
	s := newStack(t)
	open := scriptBotVsBotTiers(t, s, map[string][]string{
		config.TierEasy.Name:   botVsBotHostLine,
		config.TierMedium.Name: botVsBotGuestLine,
	})
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierMedium.Cores; got != want {
		t.Fatalf("ledger = %d, want the medium booking %d", got, want)
	}

	open()
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	r.retire()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the stacked retires = %d, want exactly one release", got)
	}
}

func TestBotRoomForfeitReleasesBooking(t *testing.T) {
	s := newStack(t)
	s.rm.makeSearcher = func(config.Tier) searcher { return &contractBot{} }
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierHard)
	if err != nil {
		t.Fatalf("create vs hard: %v", err)
	}
	if got, want := s.rm.LiveCoreBookings(), config.TierHard.Cores; got != want {
		t.Fatalf("ledger = %d, want the hard booking %d", got, want)
	}

	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the forfeit = %d, want 0", got)
	}
	if rooms := s.rm.List(); len(rooms) != 0 {
		t.Errorf("grid after the forfeit = %d rooms, want 0", len(rooms))
	}
}

func TestCoreLedgerHammer(t *testing.T) {
	rm := NewRoomManager(nil, nil, nil)
	const workers, iters = 16, 250
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func() {
			defer wg.Done()
			for i := range iters {
				n := 1 + (w+i)%3
				rm.mu.Lock()
				booked := rm.bookCoresLocked(n)
				rm.mu.Unlock()
				if booked {
					rm.mu.Lock()
					rm.releaseCoresLocked(n)
					rm.mu.Unlock()
				}
			}
		}()
	}

	stop := make(chan struct{})
	breach := make(chan int, 1)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if got := rm.LiveCoreBookings(); got < 0 || got > config.MachineCores {
				breach <- got
				return
			}
		}
	}()

	wg.Wait()
	close(stop)
	select {
	case got := <-breach:
		t.Fatalf("ledger left the budget bounds at %d", got)
	default:
	}
	if got := rm.LiveCoreBookings(); got != 0 {
		t.Errorf("ledger after the hammer = %d, want 0", got)
	}
}

func TestShellCreateRoomRendersMachineBusy(t *testing.T) {
	s := newStack(t)
	s.rm.makeSearcher = func(config.Tier) searcher { return &contractBot{} }
	holder := seedUser(t, s.store, "holder")
	for range 2 {
		if _, err := s.rm.Create(holder.ID, 0, config.SeriesBO3, &config.TierMaster); err != nil {
			t.Fatalf("fill the ledger: %v", err)
		}
	}
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	token := shellRegister(t, c, srv.URL, "alice", "hunter2")

	status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)}, "bot": {config.TierMaster.Name},
	})
	if status != http.StatusConflict {
		t.Fatalf("create over the full budget: status = %d, want 409 (body %s)", status, body)
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
