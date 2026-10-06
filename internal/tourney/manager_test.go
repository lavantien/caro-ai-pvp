package tourney

// The manager tests: the M7 UI seam over the conductor. StartRun persists
// synchronously (the run id exists before any series starts), the drive runs
// in the background to a finished run, invalid specs refuse without
// persisting, a failed drive surfaces on Detail while the run row stays
// ongoing for the post-mortem, and an unknown run maps onto
// server.ErrNotFound.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// waitDetail polls Detail until cond holds or the deadline passes.
func waitDetail(t *testing.T, m *Manager, runID int64, cond func(Detail) bool) Detail {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		d, err := m.Detail(context.Background(), runID)
		if err != nil {
			t.Fatalf("detail of run %d: %v", runID, err)
		}
		if cond(d) {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %d never reached the wanted state, detail = %+v", runID, d)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerStartRunDrivesInBackground(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	m := NewManager(ts, src)

	run, err := m.StartRun(context.Background(), RunSpec{
		Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	// The row exists the moment StartRun returns: the page can redirect onto
	// the run's live board without racing the first series.
	if run.ID == 0 || run.Status != RunStateOngoing {
		t.Fatalf("started run = %+v, want a persisted ongoing row", run)
	}

	d := waitDetail(t, m, run.ID, func(d Detail) bool { return d.Run.Status == RunStateFinished })
	if d.Failure != nil {
		t.Fatalf("failure = %v, want a clean run", d.Failure)
	}
	if d.Running {
		t.Error("detail reports the run still driven after it finished")
	}
	// The scripted sweep: both series of the twice-pair settle for slot 0,
	// the roster reads back in slot order, and the leaderboard seats the
	// easy tier first at the zero-sum.
	if len(d.Roster) != 2 || d.Roster[0].Name != "easy-1" || d.Roster[1].Name != "medium-1" {
		t.Errorf("roster = %+v, want the created participants in slot order", d.Roster)
	}
	for i, s := range d.Series {
		if s.FinishedAt == nil {
			t.Errorf("series %d unfinished after the run closed", i)
		}
		if s.WinnerSlot == nil || *s.WinnerSlot != 0 {
			t.Errorf("series %d winner = %v, want the easy slot 0", i, s.WinnerSlot)
		}
	}
	sum := 0
	for _, st := range d.Board {
		sum += st.Rating
	}
	if len(d.Board) != 2 || d.Board[0].Slot != 0 || sum != 2*config.TournamentStartRating {
		t.Errorf("board = %+v, want the easy slot first at the zero-sum %d",
			d.Board, 2*config.TournamentStartRating)
	}
	if starts, closed, _ := src.snapshot(); closed != len(starts) || len(starts) != 2 {
		t.Errorf("source saw %d starts and %d closes, want 2 and 2", len(starts), closed)
	}
	runs, err := m.Runs(context.Background())
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID || runs[0].Status != RunStateFinished {
		t.Errorf("runs = %+v, want the finished run %d newest first", runs, run.ID)
	}
}

func TestManagerStartRunRefusesWithoutPersisting(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	src := &fakeSource{script: easySweeps}
	m := NewManager(ts, src)
	spec := func(roster []Participant) RunSpec {
		return RunSpec{Roster: roster, TCIdx: mustTC(1, 0),
			BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating}
	}

	for name, tc := range map[string]struct {
		roster   []Participant
		parallel int
		want     string
	}{
		"unknown tier": {[]Participant{
			{Slot: 0, Name: "a", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "b", Tier: "mythic"},
		}, 1, "mythic"},
		"core budget": {rosterSix(), 3, "cores"},
		"one participant": {[]Participant{
			{Slot: 0, Name: "a", Tier: config.TierEasy.Name},
		}, 1, "at least 2"},
	} {
		_, err := m.StartRun(context.Background(), spec(tc.roster), tc.parallel)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: start = %v, want a refusal mentioning %q", name, err, tc.want)
		}
	}
	if n := countRows(t, ts.srv, `SELECT COUNT(*) FROM tournament_runs`); n != 0 {
		t.Errorf("runs after refusals = %d, want 0", n)
	}
	if starts, _, _ := src.snapshot(); len(starts) != 0 {
		t.Errorf("started = %d series after refusals, want 0", len(starts))
	}
}

func TestManagerFailedDriveMarksDetail(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	// The eviction shape: the stream ends before the series event.
	src := &fakeSource{script: func(host, guest string) []server.Event {
		ev := easySweeps(host, guest)
		return ev[:len(ev)-1]
	}, closeEarly: true, streamErr: server.ErrSlowConsumer}
	m := NewManager(ts, src)

	run, err := m.StartRun(context.Background(), RunSpec{
		Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	d := waitDetail(t, m, run.ID, func(d Detail) bool { return d.Failure != nil })
	if d.Running {
		t.Error("detail reports a failed drive still running")
	}
	if !errors.Is(d.Failure, server.ErrSlowConsumer) {
		t.Errorf("failure = %v, want the stream's eviction error", d.Failure)
	}
	// The aborted run row stays ongoing for the post-mortem. The first
	// pairing's games landed before the eviction, so the store's own majority
	// law settled that series row; the second never started.
	if d.Run.Status != RunStateOngoing {
		t.Errorf("run status = %q, want %q after the abort", d.Run.Status, RunStateOngoing)
	}
	if len(d.Series) != 2 || d.Series[0].FinishedAt == nil || d.Series[1].FinishedAt != nil {
		t.Errorf("series = %+v, want pairing 0 settled on its games, pairing 1 unstarted", d.Series)
	}
}

// gatedSource parks every StartSeries until released, holding a run's drive
// live without spinning.
func gatedSource(release chan struct{}) *fakeSource {
	return &fakeSource{script: easySweeps, onStart: func(*fakeStream) error {
		<-release
		return nil
	}}
}

func TestManagerRefusesSecondConcurrentRun(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	release := make(chan struct{})
	m := NewManager(ts, gatedSource(release))
	spec := RunSpec{Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating}

	// Posts racing in-process serialize on the manager's lock: exactly one
	// wins the run gate, every other reads its ongoing row and refuses.
	const posts = 8
	errs := make(chan error, posts)
	for range posts {
		go func() {
			_, err := m.StartRun(context.Background(), spec, 1)
			errs <- err
		}()
	}
	refused := 0
	for range posts {
		err := <-errs
		if err == nil {
			continue
		}
		var gate *RunInProgressError
		if !errors.As(err, &gate) {
			t.Fatalf("racing post = %v, want the RunInProgressError refusal", err)
		}
		refused++
	}
	if refused != posts-1 {
		t.Fatalf("refused = %d of %d posts, want exactly one winner", refused, posts)
	}
	if n := countRows(t, ts.srv, `SELECT COUNT(*) FROM tournament_runs`); n != 1 {
		t.Errorf("runs after the race = %d, want the one winner", n)
	}

	// The winner still finishes cleanly once its gate opens.
	runs, err := m.Runs(context.Background())
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs after the race = %v (%v), want the one ongoing row", runs, err)
	}
	close(release)
	waitDetail(t, m, runs[0].ID, func(d Detail) bool { return d.Run.Status == RunStateFinished })
}

// TestManagerOngoingRun pins the home banner's read: nothing before any
// start, the driven run with its twice-pair while the gate holds, nothing
// once the drive returns.
func TestManagerOngoingRun(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	release := make(chan struct{})
	m := NewManager(ts, gatedSource(release))
	ctx := context.Background()

	if _, ok, err := m.OngoingRun(ctx); ok || err != nil {
		t.Fatalf("ongoing before any start = %t %v, want none", ok, err)
	}
	run, err := m.StartRun(ctx, RunSpec{Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating}, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	d, ok, err := m.OngoingRun(ctx)
	if err != nil || !ok {
		t.Fatalf("ongoing while gated = %t %v, want the driven run", ok, err)
	}
	if d.Run.ID != run.ID || !d.Running {
		t.Errorf("ongoing = run %d running %t, want run %d live", d.Run.ID, d.Running, run.ID)
	}
	if len(d.Series) != 2 {
		t.Errorf("ongoing series = %d pairings, want the twice-pair", len(d.Series))
	}

	close(release)
	waitDetail(t, m, run.ID, func(d Detail) bool { return d.Run.Status == RunStateFinished })
	if _, ok, err := m.OngoingRun(ctx); ok || err != nil {
		t.Fatalf("ongoing after the finish = %t %v, want none", ok, err)
	}
}

func TestManagerCloseStalled(t *testing.T) {
	ts, srv := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	spec := RunSpec{Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating}

	if err := NewManager(ts, &fakeSource{script: easySweeps}).CloseStalled(ctx, 999); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("close of an unknown run = %v, want server.ErrNotFound", err)
	}

	// A live drive owns the row and refuses the close.
	release := make(chan struct{})
	m := NewManager(ts, gatedSource(release))
	run, err := m.StartRun(ctx, spec, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := m.CloseStalled(ctx, run.ID); !errors.Is(err, ErrDriveLive) {
		t.Fatalf("close of a live drive = %v, want ErrDriveLive", err)
	}
	if status := runStatus(t, ts, run.ID); status != RunStateOngoing {
		t.Errorf("status after the refused close = %q, want %q", status, RunStateOngoing)
	}
	close(release)
	waitDetail(t, m, run.ID, func(d Detail) bool { return d.Run.Status == RunStateFinished })
	if err := m.CloseStalled(ctx, run.ID); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("close of a finished run = %v, want the already-finished refusal", err)
	}

	// A stalled row no process drives closes without a standings snapshot;
	// the leaderboard keeps deriving from the games.
	stalled, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test")
	if err != nil {
		t.Fatalf("create stalled run: %v", err)
	}
	if err := m.CloseStalled(ctx, stalled.ID); err != nil {
		t.Fatalf("close stalled run: %v", err)
	}
	d, err := m.Detail(ctx, stalled.ID)
	if err != nil {
		t.Fatalf("detail of the closed run: %v", err)
	}
	if d.Run.Status != RunStateFinished || d.Run.FinishedAt == nil || d.Running || d.Failure != nil {
		t.Errorf("closed run detail = %+v, want finished, stamped, not running, unfailed", d.Run)
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_standings WHERE run_id = ?`, stalled.ID); n != 0 {
		t.Errorf("standings after a stalled close = %d, want 0", n)
	}

	// A failed drive leaves the row ongoing for the post-mortem; with the
	// drive dead the close resolves it.
	failM := NewManager(ts, &fakeSource{script: func(host, guest string) []server.Event {
		ev := easySweeps(host, guest)
		return ev[:len(ev)-1]
	}, closeEarly: true, streamErr: server.ErrSlowConsumer})
	failed, err := failM.StartRun(ctx, spec, 1)
	if err != nil {
		t.Fatalf("start failed-drive run: %v", err)
	}
	waitDetail(t, failM, failed.ID, func(d Detail) bool { return d.Failure != nil })
	if err := failM.CloseStalled(ctx, failed.ID); err != nil {
		t.Fatalf("close of a failed drive's run: %v", err)
	}
	if status := runStatus(t, ts, failed.ID); status != RunStateFinished {
		t.Errorf("status after closing the failed run = %q, want %q", status, RunStateFinished)
	}
}

// TestManagerCloseStalledRefusesFreshLease pins the close-side fence across
// processes: a fresh drive lease is a live drive somewhere else and the
// close refuses, while a claim past the stale window is a dead drive's and
// the close proceeds.
func TestManagerCloseStalledRefusesFreshLease(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	m := NewManager(ts, nil)

	fresh, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test")
	if err != nil {
		t.Fatalf("create fresh-leased run: %v", err)
	}
	ts.fakePid = 4242 // stand in for the other process's drive
	if err := ts.ClaimDrive(ctx, fresh.ID, time.Now().Unix()); err != nil {
		t.Fatalf("claim fresh lease: %v", err)
	}
	if err := m.CloseStalled(ctx, fresh.ID); err == nil || !strings.Contains(err.Error(), "driven by pid") {
		t.Errorf("close over a fresh lease = %v, want the live-drive refusal", err)
	}
	if status := runStatus(t, ts, fresh.ID); status != RunStateOngoing {
		t.Errorf("status after the refused close = %q, want %q", status, RunStateOngoing)
	}

	// A claim stamped past the stale window is a dead drive's: the close
	// proceeds and clears the dead claim with the row.
	stale, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test")
	if err != nil {
		t.Fatalf("create stale-leased run: %v", err)
	}
	old := time.Now().Add(-time.Duration(config.TournamentDriveStaleSec+5) * time.Second).Unix()
	if err := ts.ClaimDrive(ctx, stale.ID, old); err != nil {
		t.Fatalf("claim stale lease: %v", err)
	}
	ts.fakePid = 0
	if err := m.CloseStalled(ctx, stale.ID); err != nil {
		t.Errorf("close over a stale lease = %v, want nil", err)
	}
	if status := runStatus(t, ts, stale.ID); status != RunStateFinished {
		t.Errorf("status after the stale-lease close = %q, want %q", status, RunStateFinished)
	}
}

func TestManagerDetailUnknownAndUndrivenRuns(t *testing.T) {
	ts, _ := newTestStore(t)
	m := NewManager(ts, &fakeSource{script: easySweeps})

	if _, err := m.Detail(context.Background(), 999); !errors.Is(err, server.ErrNotFound) {
		t.Errorf("detail of run 999 = %v, want server.ErrNotFound", err)
	}
	// A run this process never drives (a previous lifetime's, or one created
	// straight through the store) reads back unfailed and not running.
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	d, err := m.Detail(ctx, run.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.Running || d.Failure != nil || d.Run.Status != RunStateOngoing {
		t.Errorf("undriven run detail = %+v, want ongoing, not running, unfailed", d)
	}
	if len(d.Series) != 2 {
		t.Errorf("series = %d rows, want the 2 the create persisted", len(d.Series))
	}
}
