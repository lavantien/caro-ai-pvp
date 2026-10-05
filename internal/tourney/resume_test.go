package tourney

// The resume tests: a drive that dies mid-run (cancelled, crashed, or the
// machine cut) leaves the run row ongoing with the settled series settled and
// the in-flight series holding whatever partial games landed. Resume skips
// the settled series, scrubs and replays the interrupted ones, finishes the
// run, and writes the one summary in the run's own folder.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func TestConductorResumeAfterInterruptedRun(t *testing.T) {
	ts, srv := newTestStore(t)
	logDir := pointLogsAt(t)
	ctx := context.Background()

	// The dying source: pairing 0 plays its sweep, pairing 1's start fails
	// the run, the shape every abrupt termination leaves behind.
	var calls int32
	dying := &fakeSource{script: easySweeps, onStart: func(*fakeStream) error {
		if atomic.AddInt32(&calls, 1) >= 2 {
			return errors.New("the room surface died")
		}
		return nil
	}}
	c := NewConductor(dying)
	run, tiers, err := c.startRun(ctx, ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if _, err := c.drive(ctx, ts, run, rosterTwo(), tiers, 1); err == nil ||
		!strings.Contains(err.Error(), "the room surface died") {
		t.Fatalf("dying drive = %v, want the room-surface failure", err)
	}
	if status := runStatus(t, ts, run.ID); status != RunStateOngoing {
		t.Fatalf("status after the abort = %q, want %q", status, RunStateOngoing)
	}
	sched := mustSchedule(t, srv, run.ID)
	if sched[0].FinishedAt == nil {
		t.Fatalf("pairing 0 unfinished after the abort: %+v", sched[0])
	}
	if sched[1].FinishedAt != nil {
		t.Fatalf("pairing 1 settled though its start failed: %+v", sched[1])
	}

	// The in-flight shape: one partial game of the interrupted series, the
	// state a kill lands on when it cuts inside a series.
	mustAppend(t, ts, gameOf(run.ID, sched[1].ID, 0,
		sched[1].RedFirstSlot, sched[1].BlueFirstSlot, server.RedWins))
	if s := mustSchedule(t, srv, run.ID)[1]; s.RedFirstWins != 1 {
		t.Fatalf("partial game did not land: %+v", s)
	}

	fresh := &fakeSource{script: easySweeps}
	res, err := NewConductor(fresh).Resume(ctx, ts, run.ID, 1)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	// Only the interrupted series played: pairing 1, medium-1 hosting.
	starts, closed, _ := fresh.snapshot()
	if len(starts) != 1 || closed != 1 {
		t.Errorf("resume saw %d starts and %d closes, want exactly the interrupted series", len(starts), closed)
	} else if starts[0] != [2]string{"medium-1", "easy-1"} {
		t.Errorf("resume start = %v, want pairing 1 with medium-1 hosting", starts[0])
	}

	// The settled series reconstructs its persisted line, the replayed one
	// settles again, and no game row survived the scrub.
	if res.Series[0].WinnerSlot == nil || *res.Series[0].WinnerSlot != 0 ||
		res.Series[0].RedFirstWins != 2 || res.Series[0].BlueFirstWins != 0 {
		t.Errorf("reconstructed line = %+v, want the persisted 2-0 sweep of slot 0", res.Series[0])
	}
	if res.Series[1].WinnerSlot == nil || *res.Series[1].WinnerSlot != 0 ||
		res.Series[1].RedFirstWins+res.Series[1].BlueFirstWins != 2 {
		t.Errorf("replayed line = %+v, want the sweep settling again", res.Series[1])
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_games WHERE run_id = ?`, run.ID); n != 4 {
		t.Errorf("games after resume = %d, want 4 (2 settled + scrubbed replay of 2)", n)
	}
	if status := runStatus(t, ts, run.ID); status != RunStateFinished {
		t.Errorf("status after resume = %q, want %q", status, RunStateFinished)
	}
	sum := 0
	for _, st := range res.Board {
		sum += st.Rating
	}
	if sum != 2*config.TournamentStartRating {
		t.Errorf("board ratings sum = %d, want the zero-sum %d", sum, 2*config.TournamentStartRating)
	}

	// One summary, and the replayed series' file holds exactly one header:
	// the aborted drive's stale partial record did not survive.
	summaries, err := filepath.Glob(filepath.Join(logDir, "*", config.TournamentSummaryName))
	if err != nil {
		t.Fatalf("glob summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries = %v, want exactly the resumed run's one", summaries)
	}
	body, err := os.ReadFile(seriesLogFile(t, logDir, sched[1].ID))
	if err != nil {
		t.Fatalf("read replayed series log: %v", err)
	}
	if n := strings.Count(string(body), "run "+strconv.FormatInt(run.ID, 10)+" series "+strconv.FormatInt(sched[1].ID, 10)); n != 1 {
		t.Errorf("replayed series log holds %d headers, want 1:\n%s", n, body)
	}
}

func TestConductorResumeRefusals(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	c := NewConductor(&fakeSource{script: easySweeps})

	if _, err := c.Resume(ctx, ts, 999, 1); !errors.Is(err, server.ErrNotFound) {
		t.Errorf("resume unknown run = %v, want server.ErrNotFound", err)
	}

	done, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test")
	if err != nil {
		t.Fatalf("create finished run: %v", err)
	}
	if err := ts.CloseStalledRun(ctx, done.ID, 1); err != nil {
		t.Fatalf("finish run: %v", err)
	}
	if _, err := c.Resume(ctx, ts, done.ID, 1); err == nil ||
		!strings.Contains(err.Error(), "already") {
		t.Errorf("resume finished run = %v, want the already-%s refusal", err, RunStateFinished)
	}

	// Two ongoing rows planted straight through the store (the conductor's
	// gate lives in startRun, not the store): resuming the younger one must
	// refuse on the older one holding the machine-wide gate.
	first, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test")
	if err != nil {
		t.Fatalf("create first ongoing run: %v", err)
	}
	second, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test")
	if err != nil {
		t.Fatalf("create second ongoing run: %v", err)
	}
	_, err = c.Resume(ctx, ts, second.ID, 1)
	var gate *RunInProgressError
	if !errors.As(err, &gate) || gate.RunID != first.ID {
		t.Errorf("resume over another ongoing run = %v, want the gate naming run %d", err, first.ID)
	}
	if _, err := c.Resume(ctx, ts, first.ID, 1); err != nil {
		t.Errorf("resume of the gate-holding run itself = %v, want it legal", err)
	}
}
