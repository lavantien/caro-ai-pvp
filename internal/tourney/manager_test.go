package tourney

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

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
		"core budget": {rosterDefault(), 3, "cores"},
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
	if d.Run.Status != RunStateOngoing {
		t.Errorf("run status = %q, want %q after the abort", d.Run.Status, RunStateOngoing)
	}
	if len(d.Series) != 2 || d.Series[0].FinishedAt == nil || d.Series[1].FinishedAt != nil {
		t.Errorf("series = %+v, want pairing 0 settled on its games, pairing 1 unstarted", d.Series)
	}
}

func TestManagerDetailSamplesDriveStateBeforeTables(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test", "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	first := mustSchedule(t, srv, run.ID)[0]
	m := NewManager(ts, nil)
	terminal := errors.New("drive died mid-poll")
	st := &driveState{done: make(chan struct{})}
	m.mu.Lock()
	m.drives[run.ID] = st
	type polled struct {
		d   Detail
		err error
	}
	res := make(chan polled, 1)
	entered := make(chan struct{})
	go func() {
		close(entered)
		d, derr := m.Detail(ctx, run.ID)
		res <- polled{d, derr}
	}()
	<-entered
	mustAppend(t, ts, gameOf(run.ID, first.ID, 0, 0, 1, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, first.ID, 1, 0, 1, server.RedWins))
	st.err = terminal
	close(st.done)
	m.mu.Unlock()
	out := <-res
	if out.err != nil {
		t.Fatalf("detail: %v", out.err)
	}
	if !errors.Is(out.d.Failure, terminal) {
		t.Fatalf("failure = %v, want the drive's terminal error", out.d.Failure)
	}
	if out.d.Running {
		t.Error("detail reports a finished drive as running")
	}
	if len(out.d.Series) != 2 || out.d.Series[0].FinishedAt == nil ||
		out.d.Series[0].RedFirstWins != 2 || out.d.Series[0].WinnerSlot == nil || *out.d.Series[0].WinnerSlot != 0 {
		t.Errorf("series = %+v, want pairing 0 holding the games the drive committed before it finished", out.d.Series)
	}
	if out.d.Series[1].FinishedAt != nil {
		t.Errorf("series = %+v, want pairing 1 untouched", out.d.Series)
	}
}

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

	runs, err := m.Runs(context.Background())
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs after the race = %v (%v), want the one ongoing row", runs, err)
	}
	close(release)
	waitDetail(t, m, runs[0].ID, func(d Detail) bool { return d.Run.Status == RunStateFinished })
}

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

	stalled, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test", "")
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

func TestManagerCloseStalledRefusesFreshLease(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	m := NewManager(ts, nil)

	fresh, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
	if err != nil {
		t.Fatalf("create fresh-leased run: %v", err)
	}
	ts.fakePid = 4242
	if err := ts.ClaimDrive(ctx, fresh.ID, time.Now().Unix()); err != nil {
		t.Fatalf("claim fresh lease: %v", err)
	}
	if err := m.CloseStalled(ctx, fresh.ID); err == nil || !strings.Contains(err.Error(), "driven by pid") {
		t.Errorf("close over a fresh lease = %v, want the live-drive refusal", err)
	}
	if status := runStatus(t, ts, fresh.ID); status != RunStateOngoing {
		t.Errorf("status after the refused close = %q, want %q", status, RunStateOngoing)
	}

	stale, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
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
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, rosterTwo(), "test", "")
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

func TestManagerDetailDropsRunningOnceTheRowFinishes(t *testing.T) {
	ts, srv := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	m := NewManager(ts, gatedSource(release))

	run, err := m.StartRun(ctx, RunSpec{Roster: rosterTwo(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating}, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	d, err := m.Detail(ctx, run.ID)
	if err != nil {
		t.Fatalf("detail while gated: %v", err)
	}
	if !d.Running || d.Run.Status != RunStateOngoing {
		t.Fatalf("gated detail = running %t status %q, want live and ongoing", d.Running, d.Run.Status)
	}

	now := time.Now().Unix()
	runSQL(t, srv,
		`UPDATE tournament_series SET winner_slot = 0, finished_at = ? WHERE run_id = ?`, now, run.ID)
	if err := ts.FinishRun(ctx, run.ID, now); err != nil {
		t.Fatalf("finish the run row under the open drive: %v", err)
	}
	d, err = m.Detail(ctx, run.ID)
	if err != nil {
		t.Fatalf("detail after the row finished: %v", err)
	}
	if d.Run.Status != RunStateFinished {
		t.Fatalf("status = %q, want finished", d.Run.Status)
	}
	if d.Running {
		t.Error("detail reports a finished row as running: the row outranks the drive map")
	}
}
