package main

// The composition tests of the serve tree's tournament arm: the adapter maps
// the tourney manager's domain shapes onto the page service (a scripted
// MatchSource drives a real manager over a real store, no engine runs), and
// the root mux mounts the tournament pages beside the shell, room, and API
// surfaces.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

// The scripted sweep lines, mirroring the tourney package's fixture: the
// D-file side closes D3..D7 (an open four at the final stone) while the
// opponent scatters far from the D file. sweepRed wins at move 11, sweepBlue
// at move 12.
var (
	sweepRedMoves  = []string{"D4", "P16", "H8", "P12", "D5", "P8", "D6", "N16", "D7", "M4", "D3"}
	sweepBlueMoves = []string{"P16", "D4", "P12", "H8", "P8", "D5", "N16", "D6", "M4", "D7", "L2", "D3"}
)

// hostSweep scripts the host seat winning a bo3 2-0: a red win, then the
// loser-takes-red rotation hands the host blue for the second win.
func hostSweep() []server.Event {
	var out []server.Event
	for _, game := range []struct {
		moves   []string
		outcome string
	}{{sweepRedMoves, server.OutcomeRed}, {sweepBlueMoves, server.OutcomeBlue}} {
		for _, name := range game.moves {
			out = append(out, server.Event{Kind: server.EventKindMove, Payload: name})
		}
		out = append(out, server.Event{Kind: server.EventKindGameEnd, Payload: game.outcome})
	}
	return append(out, server.Event{Kind: server.EventKindSeries, Payload: server.SideHost.String()})
}

// cmdStream is one scripted series over the exported tourney seam: the
// feeder parks once the script drains, Close ends it. Truth derives from
// the script's own move events, the room's guarantee for a clean stream.
type cmdStream struct {
	ch   chan server.Event
	done chan struct{}
	once sync.Once

	mu    sync.Mutex
	truth []rules.Move
}

func (s *cmdStream) Events() <-chan server.Event { return s.ch }
func (s *cmdStream) Err() error                  { return nil }

// TruthMoves is the scripted room's authoritative list for the game that
// just ended.
func (s *cmdStream) TruthMoves() []rules.Move {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.truth
}

func (s *cmdStream) Close() {
	s.once.Do(func() { close(s.done) })
}

func (s *cmdStream) feed(events []server.Event) {
	var game []rules.Move
	for _, ev := range events {
		switch ev.Kind {
		case server.EventKindMove:
			cell, err := rules.ParseCell(ev.Payload)
			if err != nil {
				panic("cmd test: scripted move " + ev.Payload + " is not a cell")
			}
			game = append(game, rules.Move(cell))
		case server.EventKindGameEnd:
			s.mu.Lock()
			s.truth = game
			s.mu.Unlock()
			game = nil
		}
		select {
		case s.ch <- ev:
		case <-s.done:
			return
		}
	}
}

type cmdSource struct {
	script func(host, guest string) []server.Event
}

func (s *cmdSource) StartSeries(host, guest *config.Tier, _, _ int) (tourney.SeriesStream, error) {
	st := &cmdStream{ch: make(chan server.Event), done: make(chan struct{})}
	go st.feed(s.script(host.Name, guest.Name))
	return st, nil
}

// tourneyStack is one composition boot for the tests: the store, rooms, and
// the adapter over a manager whose source is scripted.
type tourneyStack struct {
	svc   tourneyService
	store *server.Store
	rooms *server.RoomManager
}

// newTourneyStack boots the stack with the series logs pointed at a temp
// dir.
func newTourneyStack(t *testing.T, script func(host, guest string) []server.Event) *tourneyStack {
	t.Helper()
	orig := config.TournamentLogDir
	config.TournamentLogDir = t.TempDir()
	t.Cleanup(func() { config.TournamentLogDir = orig })
	store, err := server.Open(filepath.Join(t.TempDir(), "caro.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, store, wq)
	t.Cleanup(func() {
		rooms.Shutdown()
		wq.Close()
		hub.Close()
	})
	return &tourneyStack{
		svc:   newTourneyService(tourney.NewManager(tourney.NewStore(store), &cmdSource{script: script})),
		store: store, rooms: rooms,
	}
}

func TestTourneyServiceAdapterDrivesAndMaps(t *testing.T) {
	s := newTourneyStack(t, func(string, string) []server.Event { return hostSweep() })

	id, err := s.svc.StartRun(context.Background(), server.TourneySetup{
		Seats: []server.TourneySeat{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		},
		TCIdx: 1, BOLen: config.SeriesBO3,
		StartRating: config.TournamentStartRating, Parallel: 1,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if id == 0 {
		t.Fatal("start run handed back id 0")
	}

	// The drive lands and the snapshot maps the domain shapes onto the page
	// shapes: the twice-pair in red-first order, the settled winners, and
	// the zero-sum board.
	var snap server.TourneySnapshot
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, err = s.svc.RunSnapshot(context.Background(), id)
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		if snap.Run.Finished {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run never finished, snapshot = %+v", snap)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if snap.Run.Failure != "" || snap.Run.Running {
		t.Errorf("run = failed %q running %t after the finish, want clean and done",
			snap.Run.Failure, snap.Run.Running)
	}
	if len(snap.Seats) != 2 || snap.Seats[0].Name != "alpha" || snap.Seats[1].Tier != config.TierEasy.Name {
		t.Errorf("seats = %+v, want the roster in slot order", snap.Seats)
	}
	if len(snap.Series) != 2 {
		t.Fatalf("series = %d lines, want the twice-pair", len(snap.Series))
	}
	line0, line1 := snap.Series[0], snap.Series[1]
	if line0.RedFirstSlot != 0 || line0.BlueFirstSlot != 1 ||
		line0.WinnerSlot == nil || *line0.WinnerSlot != 0 || !line0.Finished ||
		line0.RedFirstWins != 2 || line0.BlueFirstWins != 0 {
		t.Errorf("pairing 0 = %+v, want alpha red-first sweeping 2-0", line0)
	}
	if line1.RedFirstSlot != 1 || line1.BlueFirstSlot != 0 ||
		line1.WinnerSlot == nil || *line1.WinnerSlot != 1 || !line1.Finished {
		t.Errorf("pairing 1 = %+v, want the color-swap with beta winning at home", line1)
	}
	sum := 0
	for _, st := range snap.Board {
		sum += st.Rating
	}
	if len(snap.Board) != 2 || sum != 2*config.TournamentStartRating {
		t.Errorf("board = %+v, want the zero-sum %d", snap.Board, 2*config.TournamentStartRating)
	}

	// The runs list carries the roster and the standings head.
	runs, err := s.svc.Runs(context.Background())
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != id || !runs[0].Finished {
		t.Fatalf("runs = %+v, want the finished run %d", runs, id)
	}
	if runs[0].Leader != "alpha" && runs[0].Leader != "beta" {
		t.Errorf("leader = %q, want a roster name", runs[0].Leader)
	}
	if len(runs[0].Seats) != 2 || runs[0].Seats[0].Name != "alpha" {
		t.Errorf("run seats = %+v, want the roster", runs[0].Seats)
	}

	// An unknown run maps onto the store sentinel the pages 404 on.
	if _, err := s.svc.RunSnapshot(context.Background(), 999); !errors.Is(err, server.ErrNotFound) {
		t.Errorf("snapshot of run 999 = %v, want server.ErrNotFound", err)
	}
}

// TestTourneyServiceStartMapsRunGate pins the adapter's gate mapping: the
// manager's refusal crosses the seam as the page sentinel naming the
// blocking run, not a bare outage.
func TestTourneyServiceStartMapsRunGate(t *testing.T) {
	s := newTourneyStack(t, func(string, string) []server.Event { return hostSweep() })

	// A planted ongoing row holds the machine-wide gate with no drive at
	// all, the stalled shape a previous process leaves behind.
	_, err := tourney.NewStore(s.store).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, []tourney.Participant{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		})
	if err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}

	_, err = s.svc.StartRun(context.Background(), server.TourneySetup{
		Seats: []server.TourneySeat{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		},
		TCIdx: 1, BOLen: config.SeriesBO3,
		StartRating: config.TournamentStartRating, Parallel: 1,
	})
	var blocked *server.TourneyBlockedError
	if !errors.As(err, &blocked) || blocked.RunID != 1 {
		t.Fatalf("start = %v, want TourneyBlockedError naming run 1", err)
	}
}

// TestTourneyServiceCloseStalled drives the stalled close across the
// adapter: a planted ongoing row this process never drove closes, and the
// row reads back finished.
func TestTourneyServiceCloseStalled(t *testing.T) {
	s := newTourneyStack(t, func(string, string) []server.Event { return hostSweep() })
	ctx := context.Background()
	stalled, err := tourney.NewStore(s.store).CreateRun(ctx, 1,
		config.SeriesBO3, config.TournamentStartRating, []tourney.Participant{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		})
	if err != nil {
		t.Fatalf("plant stalled run: %v", err)
	}
	if err := s.svc.CloseStalledRun(ctx, stalled.ID); err != nil {
		t.Fatalf("close stalled run: %v", err)
	}
	snap, err := s.svc.RunSnapshot(ctx, stalled.ID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snap.Run.Finished {
		t.Errorf("run after the close = %+v, want finished", snap.Run)
	}
}

func TestServeRootMuxMountsTournamentPages(t *testing.T) {
	s := newTourneyStack(t, func(string, string) []server.Event { return hostSweep() })
	muxSrv := httptest.NewServer(newRootMux(s.store, s.rooms, s.svc))
	defer muxSrv.Close()
	c := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	get := func(path string) (int, string) {
		t.Helper()
		resp, err := c.Get(muxSrv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return resp.StatusCode, string(body)
	}

	// The shell home still owns the root, the tournament setup bounces
	// guests exactly like the history tab, an unknown run is the page 404,
	// and the API subtree keeps its exact patterns.
	if status, body := get("/"); status != http.StatusOK || !strings.Contains(body, `id="rooms"`) {
		t.Errorf("GET / = %d, want the shell home with the rooms grid", status)
	}
	resp, err := c.Get(muxSrv.URL + "/tourney")
	if err != nil {
		t.Fatalf("get /tourney: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/login" {
		t.Errorf("GET /tourney as guest = %d %q, want 303 /login", resp.StatusCode, resp.Header.Get("Location"))
	}
	if status, _ := get("/tourney/run/1"); status != http.StatusNotFound {
		t.Errorf("GET /tourney/run/1 = %d, want 404", status)
	}
	// The rooms list is a bare JSON array, empty over a fresh store.
	if status, body := get("/api/rooms"); status != http.StatusOK || !strings.HasPrefix(body, "[") {
		t.Errorf("GET /api/rooms = %d %s, want the untouched API subtree's array", status, body)
	}
	if status, _ := get("/static/htmx.min.js"); status != http.StatusOK {
		t.Errorf("GET /static/htmx.min.js = %d, want the vendored asset", status)
	}
}
