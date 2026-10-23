package main

import (
	"context"
	"database/sql"
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

var (
	sweepRedMoves  = []string{"D4", "P16", "H8", "P12", "D5", "P8", "D6", "N16", "D7", "M4", "D3"}
	sweepBlueMoves = []string{"P16", "D4", "P12", "H8", "P8", "D5", "N16", "D6", "M4", "D7", "L2", "D3"}
)

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

type cmdStream struct {
	ch         chan server.Event
	done       chan struct{}
	once       sync.Once
	closeEarly bool
	streamErr  error

	mu    sync.Mutex
	truth []rules.Move
}

func (s *cmdStream) Events() <-chan server.Event { return s.ch }
func (s *cmdStream) Err() error                  { return s.streamErr }

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
	if s.closeEarly {
		close(s.ch)
	}
}

type cmdSource struct {
	script     func(host, guest string) []server.Event
	closeEarly bool
	streamErr  error
}

func (s *cmdSource) StartSeries(host, guest *config.Tier, hostName, guestName string, _, _ int, _ string) (tourney.SeriesStream, error) {
	if hostName == "" {
		hostName = host.Name
	}
	if guestName == "" {
		guestName = guest.Name
	}
	st := &cmdStream{ch: make(chan server.Event), done: make(chan struct{}),
		closeEarly: s.closeEarly, streamErr: s.streamErr}
	go st.feed(s.script(hostName, guestName))
	return st, nil
}

type tourneyStack struct {
	svc   tourneyService
	store *server.Store
	rooms *server.RoomManager
}

func newTourneyStack(t *testing.T, source tourney.MatchSource) *tourneyStack {
	t.Helper()
	orig := config.TournamentLogRoot
	config.TournamentLogRoot = t.TempDir()
	t.Cleanup(func() { config.TournamentLogRoot = orig })
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
		svc:   newTourneyService(tourney.NewManager(tourney.NewStore(store), source), rooms),
		store: store, rooms: rooms,
	}
}

func sweepSource() tourney.MatchSource {
	return &cmdSource{script: func(string, string) []server.Event { return hostSweep() }}
}

func TestLiveBoardOfFollowsRed(t *testing.T) {
	base := server.LiveBotBoard{
		RoomID: "roomxyz", HostName: "easy-2", GuestName: "medium-1",
		Moves: []string{"H8", "I9"}, Turn: "red", RedIsHost: true,
		HostWins: 2, GuestWins: 1,
	}
	host := liveBoardOf(base)
	if host.RoomID != "roomxyz" || host.RedName != "easy-2" || host.BlueName != "medium-1" {
		t.Errorf("host-red board = %+v, want easy-2 red over medium-1", host)
	}
	if host.RedWins != 2 || host.BlueWins != 1 || host.Turn != "red" {
		t.Errorf("host-red line = %d-%d turn %q, want 2-1 red", host.RedWins, host.BlueWins, host.Turn)
	}
	if len(host.Moves) != 2 || host.Moves[0] != "H8" || host.Moves[1] != "I9" {
		t.Errorf("moves = %v, want the play order verbatim", host.Moves)
	}

	guest := base
	guest.RedIsHost = false
	g := liveBoardOf(guest)
	if g.RedName != "medium-1" || g.BlueName != "easy-2" {
		t.Errorf("guest-red board = %+v, want medium-1 red over easy-2", g)
	}
	if g.RedWins != 1 || g.BlueWins != 2 {
		t.Errorf("guest-red line = %d-%d, want the score swapped to red's side", g.RedWins, g.BlueWins)
	}

	s := newTourneyStack(t, sweepSource())
	if boards := s.svc.LiveBoards(); len(boards) != 0 {
		t.Errorf("live boards with no rooms = %+v, want none", boards)
	}
}

func TestTourneyServiceOngoingRunBanner(t *testing.T) {
	held := make(chan struct{})
	s := newTourneyStack(t, &cmdSource{script: func(string, string) []server.Event {
		<-held
		return hostSweep()
	}})
	ctx := context.Background()

	if _, ok, err := s.svc.OngoingRun(ctx); ok || err != nil {
		t.Fatalf("ongoing before start = %t %v, want none", ok, err)
	}
	id, err := s.svc.StartRun(ctx, server.TourneySetup{
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

	deadline := time.Now().Add(10 * time.Second)
	var b server.TourneyBanner
	for {
		var ok bool
		b, ok, err = s.svc.OngoingRun(ctx)
		if err != nil || !ok {
			t.Fatalf("ongoing while held = %t %v, want the driven run", ok, err)
		}
		if b.RunID == id && b.Total == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("banner never reached 2 pairings: %+v", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if b.Done != 0 {
		t.Errorf("settled while held = %d, want 0", b.Done)
	}

	close(held)
	for {
		snap, err := s.svc.RunSnapshot(ctx, id)
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
	if _, ok, err := s.svc.OngoingRun(ctx); ok || err != nil {
		t.Fatalf("ongoing after finish = %t %v, want none", ok, err)
	}
}

func TestTourneyServiceAdapterDrivesAndMaps(t *testing.T) {
	s := newTourneyStack(t, sweepSource())

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

	if _, err := s.svc.RunSnapshot(context.Background(), 999); !errors.Is(err, server.ErrNotFound) {
		t.Errorf("snapshot of run 999 = %v, want server.ErrNotFound", err)
	}
}

func TestTourneyServiceStartMapsRunGate(t *testing.T) {
	s := newTourneyStack(t, sweepSource())

	_, err := tourney.NewStore(s.store).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, []tourney.Participant{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		}, "test", "")
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

func TestTourneyServiceCloseStalled(t *testing.T) {
	s := newTourneyStack(t, sweepSource())
	ctx := context.Background()
	stalled, err := tourney.NewStore(s.store).CreateRun(ctx, 1,
		config.SeriesBO3, config.TournamentStartRating, []tourney.Participant{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		}, "test", "")
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
	s := newTourneyStack(t, sweepSource())
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

	if status, body := get("/"); status != http.StatusOK || !strings.Contains(body, `id="rooms"`) {
		t.Errorf("GET / = %d, want the shell home with the rooms grid", status)
	}
	if status, body := get("/tourney"); status != http.StatusOK ||
		strings.Contains(body, `action="/tourney"`) {
		t.Errorf("GET /tourney as guest = %d, want the public read-only page (body %s)", status, body)
	}
	if status, _ := get("/tourney/run/1"); status != http.StatusNotFound {
		t.Errorf("GET /tourney/run/1 = %d, want 404", status)
	}
	if status, body := get("/api/rooms"); status != http.StatusOK || !strings.HasPrefix(body, "[") {
		t.Errorf("GET /api/rooms = %d %s, want the untouched API subtree's array", status, body)
	}
	if status, _ := get("/static/htmx.min.js"); status != http.StatusOK {
		t.Errorf("GET /static/htmx.min.js = %d, want the vendored asset", status)
	}
}

func stackSQL(t *testing.T, store *server.Store, query string) {
	t.Helper()
	if err := store.WithinTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), query)
		return err
	}); err != nil {
		t.Fatalf("run %q: %v", query, err)
	}
}

func plantRun(t *testing.T, store *server.Store) int64 {
	t.Helper()
	run, err := tourney.NewStore(store).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, []tourney.Participant{
			{Slot: 0, Name: "alpha", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		}, "test", "")
	if err != nil {
		t.Fatalf("plant run: %v", err)
	}
	return run.ID
}

func TestTourneyServiceStartMapsPlainRefusal(t *testing.T) {
	s := newTourneyStack(t, sweepSource())
	_, err := s.svc.StartRun(context.Background(), server.TourneySetup{
		Seats: []server.TourneySeat{
			{Slot: 0, Name: "alpha", Tier: "mythic"},
			{Slot: 1, Name: "beta", Tier: config.TierEasy.Name},
		},
		TCIdx: 1, BOLen: config.SeriesBO3,
		StartRating: config.TournamentStartRating, Parallel: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "mythic") {
		t.Fatalf("start = %v, want the unknown-tier refusal", err)
	}
	var blocked *server.TourneyBlockedError
	if errors.As(err, &blocked) {
		t.Errorf("plain refusal = %v, want it not to pose as the run gate", err)
	}
}

func evictedSource() tourney.MatchSource {
	return &cmdSource{
		script: func(string, string) []server.Event {
			ev := hostSweep()
			return ev[:len(ev)-1]
		},
		closeEarly: true, streamErr: server.ErrSlowConsumer,
	}
}

func TestTourneyServiceSnapshotMapsDriveFailure(t *testing.T) {
	s := newTourneyStack(t, evictedSource())

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
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, err := s.svc.RunSnapshot(context.Background(), id)
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		if snap.Run.Failure != "" {
			if !strings.Contains(snap.Run.Failure, "too slow") {
				t.Errorf("failure = %q, want the eviction explainer", snap.Run.Failure)
			}
			if snap.Run.Running {
				t.Error("snapshot reports a failed drive still running")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("drive failure never surfaced, snapshot = %+v", snap.Run)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTourneyServiceRunsFaults(t *testing.T) {
	s := newTourneyStack(t, sweepSource())
	stackSQL(t, s.store, `DROP TABLE tournament_runs`)
	if _, err := s.svc.Runs(context.Background()); err == nil || !strings.Contains(err.Error(), "list runs") {
		t.Errorf("runs over a dropped table = %v, want the list failure", err)
	}

	s2 := newTourneyStack(t, sweepSource())
	plantRun(t, s2.store)
	stackSQL(t, s2.store, `DELETE FROM tournament_series`)
	stackSQL(t, s2.store, `DROP TABLE tournament_participants`)
	if _, err := s2.svc.Runs(context.Background()); err == nil || !strings.Contains(err.Error(), "roster") {
		t.Errorf("runs over a broken detail = %v, want the detail failure", err)
	}
}

func TestTourneyServiceRunsLeaderOfUnplayedRun(t *testing.T) {
	s := newTourneyStack(t, sweepSource())
	plantRun(t, s.store)

	runs, err := s.svc.Runs(context.Background())
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Finished {
		t.Fatalf("runs = %+v, want the one open planted run", runs)
	}
	if runs[0].Leader != "alpha" || len(runs[0].Seats) != 2 {
		t.Errorf("unplayed run summary = %+v, want the seeded roster and its first seat as leader", runs[0])
	}

	s2 := newTourneyStack(t, sweepSource())
	plantRun(t, s2.store)
	stackSQL(t, s2.store, `DELETE FROM tournament_series`)
	stackSQL(t, s2.store, `DELETE FROM tournament_participants`)
	runs, err = s2.svc.Runs(context.Background())
	if err != nil {
		t.Fatalf("runs over the gutted run: %v", err)
	}
	if len(runs) != 1 || runs[0].Leader != "" || len(runs[0].Seats) != 0 {
		t.Errorf("gutted run summary = %+v, want no seats and an empty leader", runs[0])
	}
}
