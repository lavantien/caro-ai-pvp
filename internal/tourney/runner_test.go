package tourney

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
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

var (
	sweepRedMoves  = []string{"D4", "P16", "H8", "P12", "D5", "P8", "D6", "N16", "D7", "M4", "D3"}
	sweepBlueMoves = []string{"P16", "D4", "P12", "H8", "P8", "D5", "N16", "D6", "M4", "D7", "L2", "D3"}
)

type scriptedGame struct {
	moves   []string
	outcome string
}

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

func easySweeps(host, guest string) []server.Event {
	if host == config.TierEasy.Name {
		return scriptedSeries([]scriptedGame{
			{moves: sweepRedMoves, outcome: server.OutcomeRed},
			{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
		}, server.SideHost.String())
	}
	return scriptedSeries([]scriptedGame{
		{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
		{moves: sweepRedMoves, outcome: server.OutcomeRed},
	}, server.SideGuest.String())
}

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

func (s *fakeSource) StartSeries(host, guest *config.Tier, hostName, guestName string, tcIdx, boLen int) (SeriesStream, error) {
	s.mu.Lock()
	if hostName == "" {
		hostName = host.Name
	}
	if guestName == "" {
		guestName = guest.Name
	}
	s.starts = append(s.starts, [2]string{hostName, guestName})
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

func pointLogsAt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := config.TournamentLogRoot
	config.TournamentLogRoot = dir
	t.Cleanup(func() { config.TournamentLogRoot = orig })
	return dir
}

func rosterTwo() []Participant {
	return []Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
}

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

type persistedGame struct {
	Idx               int
	RedSlot, BlueSlot int
	Outcome           string
	WonBy             *string
	FullTurns         int
	Stones            int
}

func persistedGames(t *testing.T, ts *Store, seriesID int64) []persistedGame {
	t.Helper()
	var out []persistedGame
	err := ts.srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(context.Background(), `
		SELECT idx_in_series, red_slot, blue_slot, outcome, won_by, full_turns, length(moves)/2
		FROM tournament_games WHERE series_id = ? ORDER BY idx_in_series`, seriesID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var g persistedGame
			if err := rows.Scan(&g.Idx, &g.RedSlot, &g.BlueSlot, &g.Outcome, &g.WonBy, &g.FullTurns, &g.Stones); err != nil {
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

func seriesLogFile(t *testing.T, dir string, seriesID int64) string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.Contains(d.Name(), fmt.Sprintf("_s%d_", seriesID)) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk log dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("series %d has %d log files under %s: %v", seriesID, len(files), dir, files)
	}
	return files[0]
}

func TestConductorRunScriptedHappyPath(t *testing.T) {
	ts, _ := newTestStore(t)
	logDir := pointLogsAt(t)
	src := &fakeSource{script: easySweeps}

	res, err := NewConductor(src).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	starts, closed, maxLive := src.snapshot()
	wantStarts := [][2]string{
		{"easy-1", "medium-1"},
		{"medium-1", "easy-1"},
	}
	if len(starts) != 2 || starts[0] != wantStarts[0] || starts[1] != wantStarts[1] {
		t.Errorf("starts = %v, want %v (red-first hosts)", starts, wantStarts)
	}
	if closed != 2 || maxLive != 1 {
		t.Errorf("closed = %d, maxLive = %d, want 2 streams closed, 1 live at a time", closed, maxLive)
	}

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

	if line := res.Series[0]; line.PairingSlot != 0 || line.RedFirstWins != 2 || line.BlueFirstWins != 0 ||
		line.WinnerSlot == nil || *line.WinnerSlot != 0 {
		t.Errorf("series line 0 = %+v, want the easy red-first sweep 2-0", line)
	}
	if line := res.Series[1]; line.PairingSlot != 1 || line.RedFirstWins != 0 || line.BlueFirstWins != 2 ||
		line.WinnerSlot == nil || *line.WinnerSlot != 0 {
		t.Errorf("series line 1 = %+v, want the easy guest sweep 0-2", line)
	}

	g0 := persistedGames(t, ts, sched[0].ID)
	g1 := persistedGames(t, ts, sched[1].ID)
	if len(g0) != 2 || len(g1) != 2 {
		t.Fatalf("games per series = %d and %d, want 2 and 2", len(g0), len(g1))
	}
	if g0[0].RedSlot != 0 || g0[0].BlueSlot != 1 || g0[0].Outcome != server.OutcomeRed ||
		g0[0].FullTurns != len(sweepRedMoves)/2 {
		t.Errorf("series 0 game 1 = %+v, want the easy red win", g0[0])
	}
	if g0[1].RedSlot != 1 || g0[1].BlueSlot != 0 || g0[1].Outcome != server.OutcomeBlue ||
		g0[1].FullTurns != len(sweepBlueMoves)/2 {
		t.Errorf("series 0 game 2 = %+v, want the easy blue win after the alternation", g0[1])
	}
	if g1[0].RedSlot != 1 || g1[0].BlueSlot != 0 || g1[0].Outcome != server.OutcomeBlue ||
		g1[0].FullTurns != len(sweepBlueMoves)/2 {
		t.Errorf("series 1 game 1 = %+v, want the easy blue win off the medium red", g1[0])
	}
	if g1[1].RedSlot != 0 || g1[1].BlueSlot != 1 || g1[1].Outcome != server.OutcomeRed ||
		g1[1].FullTurns != len(sweepRedMoves)/2 {
		t.Errorf("series 1 game 2 = %+v, want the easy red win the alternation handed it", g1[1])
	}
	for _, group := range [][]persistedGame{g0, g1} {
		for _, g := range group {
			if g.WonBy == nil || *g.WonBy != server.WonByOpenFour {
				t.Errorf("game idx %d won_by = %v, want %q", g.Idx, g.WonBy, server.WonByOpenFour)
			}
		}
	}

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

	wantMovesByPairing := []int{len(sweepRedMoves) + len(sweepBlueMoves), len(sweepBlueMoves) + len(sweepRedMoves)}
	wantGamesByPairing := []int{2, 2}
	for slot, s := range sched {
		body, err := os.ReadFile(seriesLogFile(t, logDir, s.ID))
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		wantMoves := wantMovesByPairing[slot]
		wantGames := wantGamesByPairing[slot]
		if len(lines) != 3+wantMoves+wantGames+1 {
			t.Errorf("series %d log = %d lines, want %d (header, M-lines, game summaries, verdict)", s.ID, len(lines), 3+wantMoves+wantGames+1)
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

func TestConductorReportsProgress(t *testing.T) {
	ts, _ := newTestStore(t)
	logDir := pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	var out bytes.Buffer
	saved := progressOut
	progressOut = &out
	t.Cleanup(func() { progressOut = saved })

	res, err := NewConductor(src).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	console := out.String()
	for _, want := range []string{
		fmt.Sprintf("run %d drive claimed: 0 of 2 series settled", res.Run.ID),
		fmt.Sprintf("run %d series 1/2 started: easy-1 vs medium-1", res.Run.ID),
		fmt.Sprintf("run %d series 1/2 settled: easy-1 2-0 medium-1", res.Run.ID),
		fmt.Sprintf("run %d series 2/2 started: medium-1 vs easy-1", res.Run.ID),
		fmt.Sprintf("run %d series 2/2 settled: medium-1 0-2 easy-1", res.Run.ID),
		fmt.Sprintf("run %d finished: summary in", res.Run.ID),
	} {
		if !strings.Contains(console, want) {
			t.Errorf("console progress misses %q:\n%s", want, console)
		}
	}

	matches, gerr := filepath.Glob(filepath.Join(logDir, "*", config.TournamentProgressName))
	if gerr != nil || len(matches) != 1 {
		t.Fatalf("progress files = %v (err %v), want one under %s", matches, gerr, logDir)
	}
	data, rerr := os.ReadFile(matches[0])
	if rerr != nil {
		t.Fatalf("read progress: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 {
		t.Fatalf("progress lines = %d, want 6 (claim, 2 starts, 2 settles, finish):\n%s", len(lines), data)
	}
	wants := []struct {
		text   string
		prefix bool
	}{
		{fmt.Sprintf("run %d drive claimed: 0 of 2 series settled", res.Run.ID), false},
		{fmt.Sprintf("run %d series 1/2 started: easy-1 vs medium-1", res.Run.ID), false},
		{fmt.Sprintf("run %d series 1/2 settled: easy-1 2-0 medium-1 in ", res.Run.ID), true},
		{fmt.Sprintf("run %d series 2/2 started: medium-1 vs easy-1", res.Run.ID), false},
		{fmt.Sprintf("run %d series 2/2 settled: medium-1 0-2 easy-1 in ", res.Run.ID), true},
		{fmt.Sprintf("run %d finished: summary in ", res.Run.ID), true},
	}
	for i, want := range wants {
		if len(lines[i]) < 17 || lines[i][15] != ' ' {
			t.Errorf("progress line %d = %q, want a leading 20060102-150405 stamp", i, lines[i])
			continue
		}
		got := lines[i][16:]
		if want.prefix && !strings.HasPrefix(got, want.text) || !want.prefix && got != want.text {
			t.Errorf("progress line %d = %q, want %q", i, got, want.text)
		}
	}
}

var gapRedMoves = []string{"A1", "A15", "P16", "H8", "E5", "N16", "E6", "M4", "E4", "L2", "E7", "G10", "E3"}

func TestConductorFailsOnTruncatedDelivery(t *testing.T) {
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
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err == nil || !strings.Contains(err.Error(), "11 moves against the room's 13") {
		t.Fatalf("run error = %v, want the truth reconciliation naming both counts", err)
	}
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
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 2, "test"); err != nil {
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
			mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
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
			mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, parallel, "test")
		return err
	}

	if err := run(rosterDefault(), 3); err == nil || !strings.Contains(err.Error(), "cores") {
		t.Errorf("3 parallel rooms over the full roster = %v, want the core budget refusal", err)
	}
	if err := run(rosterTwo(), 0); err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Errorf("parallel 0 = %v, want the parallelism refusal", err)
	}
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

func TestConductorRefusesSecondOngoingRun(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	ctx := context.Background()
	held, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test")
	if err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}

	_, err = NewConductor(src).Run(ctx, ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
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
			"lost first move",
			&fakeSource{script: easySweeps, dropLeadMoves: 1},
			"10 moves against the room's 11",
		},
		{
			"unreplayable record",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				return append(ev[:0], ev[2:]...)
			}},
			"completes no",
		},
		{
			"evicted subscriber",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				return ev[:len(ev)-1]
			}, closeEarly: true, streamErr: server.ErrSlowConsumer},
			"too slow",
		},
		{
			"verdict mismatch",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				ev[len(ev)-1].Payload = server.SideGuest.String()
				return ev
			}},
			"disagrees",
		},
		{
			"early series close",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				cut := 2*len(sweepRedMoves) + 1
				out := append([]server.Event(nil), ev[:cut]...)
				return append(out, ev[len(ev)-1])
			}},
			"settles",
		},
		{
			"source refusal",
			&fakeSource{script: easySweeps, onStart: func(*fakeStream) error {
				return errors.New("the source refused the pairing")
			}},
			"the source refused the pairing",
		},
		{
			"stream end without explainer",
			&fakeSource{script: func(host, guest string) []server.Event {
				ev := easySweeps(host, guest)
				return ev[:len(ev)-1]
			}, closeEarly: true},
			"event stream ended before the series event",
		},
		{
			"room overplays the bo cap",
			&fakeSource{script: func(host, guest string) []server.Event {
				return scriptedSeries([]scriptedGame{
					{moves: sweepRedMoves, outcome: server.OutcomeRed},
					{moves: sweepRedMoves, outcome: server.OutcomeRed},
					{moves: sweepBlueMoves, outcome: server.OutcomeBlue},
					{moves: sweepRedMoves, outcome: server.OutcomeRed},
				}, server.SideHost.String())
			}},
			"already played its 3 games",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := newTestStore(t)
			pointLogsAt(t)
			_, err := NewConductor(tc.source).Run(context.Background(), ts, rosterTwo(),
				mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
			if err == nil {
				t.Fatal("run succeeded, want the disqualifier")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("run error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
