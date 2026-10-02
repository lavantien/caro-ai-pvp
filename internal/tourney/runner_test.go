package tourney

// The conductor tests over a scripted match source: the full run flow
// (schedule persisted, every event recorded, games settled, logs written,
// run finished), real overlap under the parallelism cap, ctx cancel retiring
// live series, the resource-budget refusals, and the missed-event
// disqualifiers.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// The scripted sweep lines of the server bot tests, reused as pure event
// scripts: the D-file side closes D3..D7 (an open four, both D3 and D8
// empty at move n-1) while the opponent scatters far from the D file.
// sweepRedMoves is the full move order of a game the red seat wins at move
// 11; sweepBlueMoves the same sweep held by blue, won at move 12.
var (
	sweepRedMoves  = []string{"D4", "P16", "H8", "P12", "D5", "P8", "D6", "N16", "D7", "M4", "D3"}
	sweepBlueMoves = []string{"P16", "D4", "P12", "H8", "P8", "D5", "N16", "D6", "M4", "D7", "L2", "D3"}
)

// scriptedGame is one game of a scripted series: the interleaved cell names
// in play order plus the gameend payload.
type scriptedGame struct {
	moves   []string
	outcome string
}

// scriptedSeries renders one series' whole event stream: per game a move and
// an M-line per stone in play order then the gameend, and the series verdict
// closing the stream.
func scriptedSeries(games []scriptedGame, verdict string) []server.Event {
	var out []server.Event
	for _, g := range games {
		for i, name := range g.moves {
			out = append(out,
				server.Event{Kind: server.EventKindMove, Payload: name},
				server.Event{Kind: server.EventKindMLine, Payload: fmt.Sprintf("M%d scripted %s", i+1, name)},
			)
		}
		out = append(out, server.Event{Kind: server.EventKindGameEnd, Payload: g.outcome})
	}
	return append(out, server.Event{Kind: server.EventKindSeries, Payload: verdict})
}

// easySweeps is the scripted law of the happy-path runs: the easy tier wins
// every bo3 2-0 through the sweep, whichever seat it holds. Host sweep =
// red win then blue win (the loser-takes-red rotation); guest sweep = two
// blue wins (red keeps the seat after a blue win).
func easySweeps(host, guest string) []server.Event {
	if host == config.TierEasy.Name {
		return scriptedSeries([]scriptedGame{
			{moves: sweepRedMoves, outcome: server.OutcomeRed},
			{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
		}, server.SideHost.String())
	}
	return scriptedSeries([]scriptedGame{
		{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
		{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
	}, server.SideGuest.String())
}

// fakeStream is one scripted series' event stream. Unbuffered channel: each
// event is consumed before the next delivers, so scripts stay in order. The
// feeder parks after the script unless closeEarly ends the channel without
// the series event, the slow-consumer eviction shape. Truth derives from
// the script's own move events, the room's guarantee for a clean stream;
// the source's dropLeadMoves knob withholds delivered moves from the
// consumer only, scripting a subscription gap.
type fakeStream struct {
	src       *fakeSource
	ch        chan server.Event
	done      chan struct{}
	closeOnce sync.Once

	mu    sync.Mutex
	truth []rules.Move
}

func (f *fakeStream) Events() <-chan server.Event { return f.ch }
func (f *fakeStream) Err() error                  { return f.src.streamErr }

// TruthMoves is the scripted room's authoritative list for the game that
// just ended.
func (f *fakeStream) TruthMoves() []rules.Move {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.truth
}

func (f *fakeStream) Close() {
	f.closeOnce.Do(func() {
		close(f.done)
		f.src.noteClose()
	})
}

func (f *fakeStream) feed(events []server.Event) {
	dropped := 0
	var game []rules.Move
	for _, ev := range events {
		switch ev.Kind {
		case server.EventKindMove:
			cell, err := rules.ParseCell(ev.Payload)
			if err != nil {
				panic("tourney test: scripted move " + ev.Payload + " is not a cell")
			}
			game = append(game, rules.Move(cell))
			if dropped < f.src.dropLeadMoves {
				dropped++
				continue
			}
		case server.EventKindGameEnd:
			f.mu.Lock()
			f.truth = game
			f.mu.Unlock()
			game, dropped = nil, 0
		}
		select {
		case f.ch <- ev:
		case <-f.done:
			return
		}
	}
	if f.src.closeEarly {
		close(f.ch)
	}
}

// fakeSource scripts MatchSource without engines: every StartSeries records
// the tier order it was called with, runs the optional gate (overlap and
// cancel hooks), and feeds the script on a goroutine. dropLeadMoves
// withholds each game's first k move events from delivery while truth keeps
// them, the create-to-subscribe gap's lost prefix.
type fakeSource struct {
	script        func(host, guest string) []server.Event
	onStart       func(*fakeStream) error
	closeEarly    bool
	streamErr     error
	dropLeadMoves int

	mu      sync.Mutex
	starts  [][2]string
	closed  int
	live    int
	maxLive int
}

func (s *fakeSource) StartSeries(host, guest *config.Tier, tcIdx, boLen int) (SeriesStream, error) {
	s.mu.Lock()
	s.starts = append(s.starts, [2]string{host.Name, guest.Name})
	s.live++
	if s.live > s.maxLive {
		s.maxLive = s.live
	}
	s.mu.Unlock()
	fs := &fakeStream{src: s, ch: make(chan server.Event), done: make(chan struct{})}
	if s.onStart != nil {
		if err := s.onStart(fs); err != nil {
			fs.Close()
			return nil, err
		}
	}
	go fs.feed(s.script(host.Name, guest.Name))
	return fs, nil
}

func (s *fakeSource) noteClose() {
	s.mu.Lock()
	s.closed++
	s.live--
	s.mu.Unlock()
}

func (s *fakeSource) snapshot() (starts [][2]string, closed, maxLive int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts, s.closed, s.maxLive
}

// pointLogsAt moves the series log dir for one test and restores it after.
func pointLogsAt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := config.TournamentLogDir
	config.TournamentLogDir = dir
	t.Cleanup(func() { config.TournamentLogDir = orig })
	return dir
}

// rosterTwo builds the 2-participant roster the scripted runs ride.
func rosterTwo() []Participant {
	return []Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
}

// runStatus reads one run row's status through the store's transaction
// surface.
func runStatus(t *testing.T, ts *Store, runID int64) string {
	t.Helper()
	var status string
	err := ts.srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRowContext(context.Background(),
			`SELECT status FROM tournament_runs WHERE id = ?`, runID).Scan(&status)
	})
	if err != nil {
		t.Fatalf("read status of run %d: %v", runID, err)
	}
	return status
}

// persistedGame is one tournament_games row of the happy-path asserts.
type persistedGame struct {
	Idx               int
	RedSlot, BlueSlot int
	Outcome           string
	WonBy             *string
	FullTurns         int
}

func persistedGames(t *testing.T, ts *Store, seriesID int64) []persistedGame {
	t.Helper()
	var out []persistedGame
	err := ts.srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(context.Background(), `
		SELECT idx_in_series, red_slot, blue_slot, outcome, won_by, full_turns
		FROM tournament_games WHERE series_id = ? ORDER BY idx_in_series`, seriesID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var g persistedGame
			if err := rows.Scan(&g.Idx, &g.RedSlot, &g.BlueSlot, &g.Outcome, &g.WonBy, &g.FullTurns); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read games of series %d: %v", seriesID, err)
	}
	return out
}

// seriesLogFile finds the one log file of a series id under the log dir.
func seriesLogFile(t *testing.T, dir string, seriesID int64) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	var path string
	for _, e := range entries {
		if strings.Contains(e.Name(), fmt.Sprintf("_s%d_", seriesID)) {
			if path != "" {
				t.Fatalf("series %d has several log files: %v", seriesID, entries)
			}
			path = filepath.Join(dir, e.Name())
		}
	}
	if path == "" {
		t.Fatalf("no log file for series %d in %v", seriesID, entries)
	}
	return path
}

func TestConductorRunScriptedHappyPath(t *testing.T) {
	ts, _ := newTestStore(t)
	logDir := pointLogsAt(t)
	src := &fakeSource{script: easySweeps}

	res, err := NewConductor(src).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// The match source was called per pairing with the red-first seat as
	// host, both directions of the twice-pair.
	starts, closed, maxLive := src.snapshot()
	wantStarts := [][2]string{
		{config.TierEasy.Name, config.TierMedium.Name},
		{config.TierMedium.Name, config.TierEasy.Name},
	}
	if len(starts) != 2 || starts[0] != wantStarts[0] || starts[1] != wantStarts[1] {
		t.Errorf("starts = %v, want %v (red-first hosts)", starts, wantStarts)
	}
	if closed != 2 || maxLive != 1 {
		t.Errorf("closed = %d, maxLive = %d, want 2 streams closed, 1 live at a time", closed, maxLive)
	}

	// Schedule persisted, both series settled for the easy sweep, run closed.
	sched := mustSchedule(t, ts.srv, res.Run.ID)
	if len(sched) != 2 {
		t.Fatalf("schedule = %d series, want 2", len(sched))
	}
	if sched[0].RedFirstSlot != 0 || sched[0].BlueFirstSlot != 1 ||
		sched[1].RedFirstSlot != 1 || sched[1].BlueFirstSlot != 0 {
		t.Errorf("schedule seats = %+v, want the twice-pair", sched)
	}
	for i, s := range sched {
		if s.FinishedAt == nil {
			t.Errorf("series %d unfinished", i)
		}
		if s.WinnerSlot == nil || *s.WinnerSlot != 0 {
			t.Errorf("series %d winner = %v, want the easy slot 0", i, s.WinnerSlot)
		}
	}
	if status := runStatus(t, ts, res.Run.ID); status != RunStateFinished {
		t.Errorf("run status = %q, want %q", status, RunStateFinished)
	}

	// Result lines mirror the settled series: the easy red-first sweep 2-0,
	// then the medium red-first sweep 0-2.
	if line := res.Series[0]; line.PairingSlot != 0 || line.RedFirstWins != 2 || line.BlueFirstWins != 0 ||
		line.WinnerSlot == nil || *line.WinnerSlot != 0 {
		t.Errorf("series line 0 = %+v, want the easy red-first sweep 2-0", line)
	}
	if line := res.Series[1]; line.PairingSlot != 1 || line.RedFirstWins != 0 || line.BlueFirstWins != 2 ||
		line.WinnerSlot == nil || *line.WinnerSlot != 0 {
		t.Errorf("series line 1 = %+v, want the easy guest sweep 0-2", line)
	}

	// Game rows: pairing 0 rides the rotation (easy red win, then easy blue
	// win), pairing 1 is two easy blue wins with red kept by the loser;
	// every decisive game carries the sweep's open-four tag.
	g0 := persistedGames(t, ts, sched[0].ID)
	if len(g0) != 2 || len(persistedGames(t, ts, sched[1].ID)) != 2 {
		t.Fatalf("games per series = %d and %d, want 2 and 2", len(g0), len(persistedGames(t, ts, sched[1].ID)))
	}
	if g0[0].RedSlot != 0 || g0[0].BlueSlot != 1 || g0[0].Outcome != server.OutcomeRed ||
		g0[0].FullTurns != len(sweepRedMoves)/2 {
		t.Errorf("series 0 game 1 = %+v, want the easy red win", g0[0])
	}
	if g0[1].RedSlot != 1 || g0[1].BlueSlot != 0 || g0[1].Outcome != server.OutcomeBlue ||
		g0[1].FullTurns != len(sweepBlueMoves)/2 {
		t.Errorf("series 0 game 2 = %+v, want the easy blue win after the rotation", g0[1])
	}
	for i, g := range persistedGames(t, ts, sched[1].ID) {
		if g.RedSlot != 1 || g.BlueSlot != 0 || g.Outcome != server.OutcomeBlue {
			t.Errorf("series 1 game %d = %+v, want the easy blue win, red kept by the loser", i, g)
		}
	}
	for _, group := range [][]persistedGame{g0, persistedGames(t, ts, sched[1].ID)} {
		for _, g := range group {
			if g.WonBy == nil || *g.WonBy != server.WonByOpenFour {
				t.Errorf("game idx %d won_by = %v, want %q", g.Idx, g.WonBy, server.WonByOpenFour)
			}
		}
	}

	// Leaderboard: zero-sum at the seed, easy first, 4 games each.
	if len(res.Board) != 2 {
		t.Fatalf("leaderboard = %d rows, want 2", len(res.Board))
	}
	sum := 0
	for _, st := range res.Board {
		sum += st.Rating
		if st.GamesPlayed != 4 {
			t.Errorf("slot %d games = %d, want 4", st.Slot, st.GamesPlayed)
		}
	}
	if sum != 2*config.TournamentStartRating {
		t.Errorf("rating sum = %d, want the zero-sum %d", sum, 2*config.TournamentStartRating)
	}
	if res.Board[0].Slot != 0 || res.Board[1].Slot != 1 {
		t.Errorf("leaderboard order = %d then %d, want the easy slot first", res.Board[0].Slot, res.Board[1].Slot)
	}

	// Series logs: one file per series, the header block, one M-line per
	// move, and the verdict line. Pairing 0 rides both sweep directions
	// (23 moves), pairing 1 two guest sweeps (24 moves).
	wantMovesByPairing := []int{len(sweepRedMoves) + len(sweepBlueMoves), 2 * len(sweepBlueMoves)}
	for slot, s := range sched {
		body, err := os.ReadFile(seriesLogFile(t, logDir, s.ID))
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		wantMoves := wantMovesByPairing[slot]
		if len(lines) != 3+wantMoves+1 {
			t.Errorf("series %d log = %d lines, want %d (header, M-lines, verdict)", s.ID, len(lines), 3+wantMoves+1)
		}
		if !strings.Contains(string(body), fmt.Sprintf("run %d series %d", res.Run.ID, s.ID)) {
			t.Errorf("series %d log misses its header:\n%s", s.ID, body)
		}
		if got := strings.Count(string(body), " scripted "); got != wantMoves {
			t.Errorf("series %d log M-lines = %d, want %d", s.ID, got, wantMoves)
		}
		if last := lines[len(lines)-1]; !strings.HasPrefix(last, "series ") {
			t.Errorf("series %d log final line = %q, want the verdict", s.ID, last)
		}
	}
}

// gapRedMoves is the even-gap sweep: the leading pair (one red, one blue
// stone) sits far from the E-file five, so dropping it leaves a delivery
// that replays clean onto the same terminal position, the exact corruption
// the replay net cannot catch and the truth reconciliation exists for.
var gapRedMoves = []string{"A1", "A15", "P16", "H8", "E5", "N16", "E6", "M4", "E4", "L2", "E7", "G10", "E3"}

// TestConductorFailsOnTruncatedDelivery pins the truth reconciliation: a
// stream that lost an even-length move prefix while the room holds the full
// list fails the run before anything persists, though the truncated game
// replays clean.
func TestConductorFailsOnTruncatedDelivery(t *testing.T) {
	// Pin the premise first: without the truth net the corrupted delivery
	// passes the replay validator on its own.
	delivered := make([]rules.Move, 0, len(gapRedMoves)-2)
	for _, name := range gapRedMoves[2:] {
		cell, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		delivered = append(delivered, rules.Move(cell))
	}
	if wonBy, rerr := replayGame(delivered, server.RedWins); rerr != nil || wonBy == nil {
		t.Fatalf("truncated replay = (%v, %v), want it clean without the truth net", wonBy, rerr)
	}

	ts, srv := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{
		script: func(host, guest string) []server.Event {
			return scriptedSeries([]scriptedGame{{moves: gapRedMoves, outcome: server.OutcomeRed}},
				server.SideHost.String())
		},
		dropLeadMoves: 2,
	}

	_, err := NewConductor(src).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err == nil || !strings.Contains(err.Error(), "11 moves against the room's 13") {
		t.Fatalf("run error = %v, want the truth reconciliation naming both counts", err)
	}
	// The corrupted game never lands and the run row stays ongoing for the
	// post-mortem, every series error's failure contract.
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_games`); n != 0 {
		t.Errorf("games after the truncated delivery = %d, want 0", n)
	}
	if status := runStatus(t, ts, 1); status != RunStateOngoing {
		t.Errorf("run status = %q, want %q after the failure", status, RunStateOngoing)
	}
}

func TestConductorOverlapsUpToParallel(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)

	// The gate holds every StartSeries until two sit inside it at once: a
	// serialized conductor times out and fails the run.
	var mu sync.Mutex
	inside := 0
	release := make(chan struct{})
	gate := func(*fakeStream) error {
		mu.Lock()
		inside++
		if inside == 2 {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
			return nil
		case <-time.After(10 * time.Second):
			return errors.New("series never overlapped")
		}
	}
	src := &fakeSource{script: easySweeps, onStart: gate}

	if _, err := NewConductor(src).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 2); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, _, maxLive := src.snapshot(); maxLive != 2 {
		t.Errorf("max live series = %d, want 2 under the parallel cap", maxLive)
	}
}

func TestConductorCancelRetiresEverything(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)

	started := make(chan struct{})
	var once sync.Once
	// The script parks after two moves: no gameend, no series event, so the
	// conductor's event loop waits on ctx alone.
	src := &fakeSource{
		script: func(host, guest string) []server.Event {
			return []server.Event{
				{Kind: server.EventKindMove, Payload: sweepRedMoves[0]},
				{Kind: server.EventKindMLine, Payload: "M1 scripted"},
				{Kind: server.EventKindMove, Payload: sweepRedMoves[1]},
			}
		},
		onStart: func(*fakeStream) error {
			once.Do(func() { close(started) })
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := NewConductor(src).Run(ctx, ts, rosterTwo(),
			mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
		done <- err
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run error = %v, want ctx cancel", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run never returned after cancel")
	}
	starts, closed, _ := src.snapshot()
	if len(starts) != 1 {
		t.Errorf("started = %d series, want only the first under parallel 1", len(starts))
	}
	if closed != len(starts) {
		t.Errorf("closed = %d of %d started, want every live stream retired", closed, len(starts))
	}
	if status := runStatus(t, ts, 1); status != RunStateOngoing {
		t.Errorf("run status = %q, want %q after the abort", status, RunStateOngoing)
	}
}

func TestConductorRunRefusals(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	run := func(roster []Participant, parallel int) error {
		_, err := NewConductor(src).Run(context.Background(), ts, roster,
			mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, parallel)
		return err
	}

	// Worst-case live-search demand over the machine budget.
	if err := run(rosterSix(), 3); err == nil || !strings.Contains(err.Error(), "cores") {
		t.Errorf("3 parallel rooms over the full roster = %v, want the core budget refusal", err)
	}
	if err := run(rosterTwo(), 0); err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Errorf("parallel 0 = %v, want the parallelism refusal", err)
	}
	// Unknown tier names refuse before anything persists.
	bad := []Participant{
		{Slot: 0, Name: "a", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "b", Tier: "mythic"},
	}
	if err := run(bad, 1); err == nil || !strings.Contains(err.Error(), "mythic") {
		t.Errorf("unknown tier = %v, want the roster refusal", err)
	}
	if n := countRows(t, ts.srv, `SELECT COUNT(*) FROM tournament_runs`); n != 0 {
		t.Errorf("runs after refusals = %d, want 0", n)
	}
	if starts, _, _ := src.snapshot(); len(starts) != 0 {
		t.Errorf("started = %d series after refusals, want 0", len(starts))
	}
}

// TestConductorRefusesSecondOngoingRun pins the machine-wide run gate: with
// any ongoing row in the store, a new run refuses with ErrRunInProgress
// naming the holder before anything persists or any series starts.
func TestConductorRefusesSecondOngoingRun(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	ctx := context.Background()
	held, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, rosterTwo())
	if err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}

	_, err = NewConductor(src).Run(ctx, ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	var gate *RunInProgressError
	if !errors.As(err, &gate) || gate.RunID != held.ID {
		t.Fatalf("run error = %v, want RunInProgressError naming run %d", err, held.ID)
	}
	if !errors.Is(err, ErrRunInProgress) {
		t.Errorf("run error = %v, want it to match ErrRunInProgress", err)
	}
	if n := countRows(t, ts.srv, `SELECT COUNT(*) FROM tournament_runs`); n != 1 {
		t.Errorf("runs after the refusal = %d, want only the planted one", n)
	}
	if starts, _, _ := src.snapshot(); len(starts) != 0 {
		t.Errorf("started = %d series after the refusal, want 0", len(starts))
	}
}

func TestConductorDisqualifiesMissedEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source *fakeSource
		want   string
	}{
		{
			// The subscription missed the first stone, an odd lost prefix:
			// the truth reconciliation fires before the replay.
			"lost first move",
			&fakeSource{script: easySweeps, dropLeadMoves: 1},
			"10 moves against the room's 11",
		},
		{
			// The room's own record is the corrupt one: truth matches the
			// delivery but the game does not replay, the second net's catch.
			"unreplayable record",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				return append(ev[:0], ev[2:]...) // drop move 1 and its M-line
			}},
			"completes no",
		},
		{
			// The stream closes before the series event with the eviction
			// explainer: a hard gap, never a silent partial record.
			"evicted subscriber",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				return ev[:len(ev)-1] // everything but the series event
			}, closeEarly: true, streamErr: server.ErrSlowConsumer},
			"too slow",
		},
		{
			// The room's verdict disagrees with the billed line: corrupt
			// seat bookkeeping fails the run instead of persisting.
			"verdict mismatch",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				ev[len(ev)-1].Payload = server.SideGuest.String()
				return ev
			}},
			"disagrees",
		},
		{
			// A series event before the bo law settles (1-0 in a bo3): the
			// room closed early or events went missing.
			"early series close",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				cut := 2*len(sweepRedMoves) + 1 // game 1, its M-lines, its end
				out := append([]server.Event(nil), ev[:cut]...)
				return append(out, ev[len(ev)-1])
			}},
			"settles",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := newTestStore(t)
			pointLogsAt(t)
			_, err := NewConductor(tc.source).Run(context.Background(), ts, rosterTwo(),
				mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
			if err == nil {
				t.Fatal("run succeeded, want the disqualifier")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("run error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
