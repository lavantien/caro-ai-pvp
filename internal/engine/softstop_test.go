package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// softLimit mirrors the driver's arithmetic so the boundary rows below stay
// exact whatever the float representation of the fraction is.
func softLimit(budget time.Duration) time.Duration {
	return time.Duration(config.SearchSoftStopFraction * float64(budget))
}

// TestSoftStopRule is the pure decision table: one banked iteration plus
// elapsed strictly past the soft fraction of the grant. The boundary rows
// kill every comparison operator and comparison-target mutant.
func TestSoftStopRule(t *testing.T) {
	big := 10 * time.Millisecond
	huge := 100 * time.Millisecond
	cases := [...]struct {
		name      string
		elapsed   time.Duration
		budget    time.Duration
		completed int
		want      bool
	}{
		{"one ns below the limit starts", softLimit(big) - 1, big, 1, false},
		{"exactly at the limit starts", softLimit(big), big, 1, false},
		{"one ns past the limit stops", softLimit(big) + 1, big, 1, true},
		{"nothing banked never stops at the budget", big, big, 0, false},
		{"nothing banked never stops past the budget", 2 * big, big, 0, false},
		{"zero budget stops once banked", 1, 0, 1, true},
		{"zero budget at zero elapsed stops too", 0, 0, 1, true},
		{"zero budget nothing banked never stops", 1, 0, 0, false},
		{"deep progress past the limit stops", 2 * big, big, 9, true},
		{"zero elapsed stops when one quantum blows the limit", 0, big, 1, true},
		{"negative elapsed is treated as one quantum", -1, big, 1, true},
		{"zero elapsed starts when the limit holds a full quantum", 0, huge, 1, false},
		{"nothing banked never stops at zero elapsed", 0, big, 0, false},
	}
	for _, tc := range cases {
		if got := softStop(tc.elapsed, tc.budget, tc.completed); got != tc.want {
			t.Errorf("%s: softStop(%d, %d, %d) = %v, want %v",
				tc.name, tc.elapsed, tc.budget, tc.completed, got, tc.want)
		}
	}
}

// hardOnlyDeadline implements Deadline without Budgeter: the soft stop must
// be skipped entirely and the hard window alone bounds the search.
type hardOnlyDeadline struct{ until time.Time }

func (h *hardOnlyDeadline) Exceeded() bool { return !time.Now().Before(h.until) }
func (h *hardOnlyDeadline) Stop()          { h.until = time.Now() }

var _ Deadline = (*hardOnlyDeadline)(nil)

// budgetDeadline decouples the hard window from the reported grant: a tiny
// Budget against a generous Exceeded window isolates the soft stop, so its
// effect is observable without wall-clock racing.
type budgetDeadline struct {
	until  time.Time
	budget time.Duration
}

func newBudgetDeadline(window, budget time.Duration) *budgetDeadline {
	return &budgetDeadline{until: time.Now().Add(window), budget: budget}
}

func (d *budgetDeadline) Exceeded() bool        { return !time.Now().Before(d.until) }
func (d *budgetDeadline) Stop()                 { d.until = time.Now() }
func (d *budgetDeadline) Budget() time.Duration { return d.budget }

var _ Budgeter = (*budgetDeadline)(nil)

// TestSoftStopSkippedWithoutBudgeter pins the skip rule single threaded: a
// deadline without a Budgeter must search to its hard window, not stop
// after depth 1 the way a zero-budget consult would.
func TestSoftStopSkippedWithoutBudgeter(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	dl := &hardOnlyDeadline{until: time.Now().Add(scaledBudget(40 * time.Millisecond))}
	mv, stats := e.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("hard-only deadline returned illegal move %d", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2: without a Budgeter only the hard window may bound the search", stats.Depth)
	}
	if stats.AllocNs != 0 {
		t.Errorf("alloc ns = %d, want 0: no Budgeter carries no grant", stats.AllocNs)
	}
}

// TestSoftStopSkippedWithoutBudgeterSMP is the same pin for the worker loop:
// the wrapped inner deadline carries no budget, so no worker may stop after
// depth 1 either.
func TestSoftStopSkippedWithoutBudgeterSMP(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	dl := &hardOnlyDeadline{until: time.Now().Add(scaledBudget(40 * time.Millisecond))}
	mv, stats := s.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("hard-only smp deadline returned illegal move %d", mv)
	}
	if stats.Threads != 2 {
		t.Errorf("threads = %d, want 2", stats.Threads)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2: without a Budgeter only the hard window may bound the workers", stats.Depth)
	}
}

// softStopCalibFactor places the soft limit at half the measured banked
// prefix, far enough below the prefix cost that clock quantization and run
// to run jitter cannot push the limit past it, close enough that the blow
// up iteration after the prefix can never start.
const softStopCalibFactor = 0.5

// calibrateGrant times one cold pass of the deterministic depth 1..5 ladder
// on a throwaway instance. On the fixed midgame position that prefix is the
// banked region and every iteration past it is a blow up costing orders of
// magnitude more, so a grant at half the prefix lands the soft limit inside
// the banked region on any host: the measurement scales with machine speed
// while the placement stays proportional. The measured search must run on
// its own fresh instance: a warm table collapses the prefix below the host
// clock quantum.
func calibrateGrant(run func(dl Deadline)) time.Duration {
	window := scaledBudget(500 * time.Millisecond)
	start := time.Now()
	run(newBudgetDeadline(window, window))
	return time.Duration(float64(time.Since(start)) * softStopCalibFactor / config.SearchSoftStopFraction)
}

// TestSearchSoftStopFiresBeforeHardWindow is the behavioral pin: with the
// soft limit inside the banked prefix and a hard window far beyond it, the
// search cannot finish the ladder, cannot mate, and cannot reach the window,
// so returning with no partial iteration discarded (stopped false) and well
// before the window means the soft stop ended it. Without the consult the
// blow up iteration starts and the hard window aborts it.
func TestSearchSoftStopFiresBeforeHardWindow(t *testing.T) {
	b := midgameBoard(t)
	window := scaledBudget(500 * time.Millisecond)
	grant := calibrateGrant(func(dl Deadline) {
		_, _ = New(testTTBytes).SearchDepth(b, dl, 5)
	})
	e := New(testTTBytes)
	mv, stats := e.Search(b, newBudgetDeadline(window, grant))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("soft-stopped search returned illegal move %d", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2 before the soft limit", stats.Depth)
	}
	if stats.AllocNs != int64(grant) {
		t.Errorf("alloc ns = %d, want the reported grant %d", stats.AllocNs, grant)
	}
	if e.stopped {
		t.Error("hard deadline aborted a partial iteration inside a window it never reached")
	}
	if stats.ElapsedNs >= int64(window/2) {
		t.Errorf("elapsed %dns approached the %dns hard window: the soft stop must end the search", stats.ElapsedNs, window)
	}
}

// TestSMPWorkerSoftStopStopsAtHead drives one worker directly, the same way
// the pool lifecycle tests do: a nanosecond grant puts the soft limit under
// any positive clock reading, so the worker banks depth 1, refuses the next
// head, and never touches the far-away hard window. Pool-level wall-clock
// pinning is not portable here: four workers thrash the shared table and
// their cold drains vary by orders of magnitude.
func TestSMPWorkerSoftStopStopsAtHead(t *testing.T) {
	s := newSMP(2, testTTBytes)
	defer s.Close()
	w, b, res := s.workers[0], &s.boards[0], &s.results[0]
	*b = *midgameBoard(t)
	w.resetForSearch(b)
	s.jobMaxDepth = config.SearchMaxPly
	s.jobSoft = true
	s.halt.Store(false)
	s.haltDL.inner = newBudgetDeadline(scaledBudget(500*time.Millisecond), time.Nanosecond)
	s.runWorker(w, b, res)
	if res.completed != 1 {
		t.Fatalf("completed = %d, want exactly 1: a nanosecond grant refuses the head after the first banked iteration on every clock, quantized or not", res.completed)
	}
	if w.stopped {
		t.Error("hard deadline aborted a partial iteration inside a window it never reached")
	}
}

// TestSoftStopQuantumGuardHoldsSingleThread is the clock-quantum killer: on
// the 64 Hz Windows timer a zero time.Since reading can hide a whole tick,
// which once let the consult open iterations past a nanosecond grant until
// the hard window burned. With the quantum rule the search returns after the
// first banked iteration no matter what the clock reads.
func TestSoftStopQuantumGuardHoldsSingleThread(t *testing.T) {
	b := midgameBoard(t)
	window := scaledBudget(250 * time.Millisecond)
	e := New(testTTBytes)
	start := time.Now()
	mv, stats := e.Search(b, newBudgetDeadline(window, time.Nanosecond))
	elapsed := time.Since(start)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("quantum-guarded search returned illegal move %d", mv)
	}
	if stats.Depth != 1 {
		t.Errorf("depth = %d, want exactly 1: the head after the first banked iteration must be refused on every clock", stats.Depth)
	}
	if elapsed >= window/2 {
		t.Errorf("elapsed %v burned toward the %v hard window: the quantum guard must hold it", elapsed, window)
	}
}

// TestSoftStopQuantumGuardHoldsSMP repeats the killer across the pool: no
// worker may open a second iteration under a nanosecond grant, so the job
// returns in the depth 1 cost range, never near the hard window.
func TestSoftStopQuantumGuardHoldsSMP(t *testing.T) {
	b := midgameBoard(t)
	window := scaledBudget(250 * time.Millisecond)
	s := newSMP(4, testTTBytes)
	defer s.Close()
	for job := range 8 {
		start := time.Now()
		mv, stats := s.Search(b, newBudgetDeadline(window, time.Nanosecond))
		elapsed := time.Since(start)
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("job %d returned illegal move %d", job, mv)
		}
		if stats.Depth > 2 {
			t.Errorf("job %d depth = %d, want at most 2 banked iterations under a nanosecond grant", job, stats.Depth)
		}
		if elapsed >= window/2 {
			t.Errorf("job %d elapsed %v burned toward the %v hard window: the quantum guard must hold it", job, elapsed, window)
		}
	}
}

// TestSearchDepthIgnoresSoftGrant pins the exemption: the fixed-depth
// driver runs the same ladder under the same decoupled grant and is bounded
// only by the hard window, so it must burn into the window instead of
// returning at the soft limit.
func TestSearchDepthIgnoresSoftGrant(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	window, grant := scaledBudget(300*time.Millisecond), scaledBudget(10*time.Millisecond)
	mv, stats := e.SearchDepth(b, newBudgetDeadline(window, grant), config.SearchMaxPly)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("fixed-depth search returned illegal move %d", mv)
	}
	if stats.Depth < 1 {
		t.Errorf("depth = %d, want at least 1", stats.Depth)
	}
	if stats.ElapsedNs < int64(window/2) {
		t.Errorf("elapsed %dns returned at the %dns soft grant: a fixed target depth is bounded by the hard deadline only",
			stats.ElapsedNs, window)
	}
}

// TestSMPSearchDepthIgnoresSoftGrant is the exemption pin for the worker
// loop: the soft flag is armed per job, so a fixed-depth SMP job burns into
// the hard window too.
func TestSMPSearchDepthIgnoresSoftGrant(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	window, grant := scaledBudget(300*time.Millisecond), scaledBudget(10*time.Millisecond)
	mv, stats := s.SearchDepth(b, newBudgetDeadline(window, grant), config.SearchMaxPly)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("fixed-depth smp search returned illegal move %d", mv)
	}
	if stats.ElapsedNs < int64(window/2) {
		t.Errorf("elapsed %dns returned at the %dns soft grant: a fixed target depth is bounded by the hard deadline only",
			stats.ElapsedNs, window)
	}
}

// TestSearchSoftStopEndsInsideGrant observes the realistic FixedBudget
// shape, where the grant and the hard window coincide: iteration times on
// this position grow faster than the soft tail, so a partial iteration can
// still be discarded, but the return stays inside the grant plus the
// node-check latency of the hard stop. Depth is wall-clock dependent and
// not pinned here.
func TestSearchSoftStopEndsInsideGrant(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	budget := scaledBudget(250 * time.Millisecond)
	mv, stats := e.Search(b, NewFixedBudget(budget))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("soft-stopped search returned illegal move %d", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2 inside the grant", stats.Depth)
	}
	if slack := time.Duration(stats.ElapsedNs - stats.AllocNs); slack > scaledBudget(50*time.Millisecond) {
		t.Errorf("elapsed %dns overshoots the %dns grant by %v, want at most the hard stop latency",
			stats.ElapsedNs, stats.AllocNs, slack)
	}
}

// TestSMPSoftStopSearchCoversWorkers runs the budgeted SMP path under the
// soft stop: four workers, a fixed grant, a legal move, and at least one
// banked iteration somewhere in the pool.
func TestSMPSoftStopSearchCoversWorkers(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(4, testTTBytes)
	defer s.Close()
	mv, stats := s.Search(b, NewFixedBudget(scaledBudget(80*time.Millisecond)))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("smp soft-stopped search returned illegal move %d", mv)
	}
	if stats.Threads != 4 {
		t.Errorf("threads = %d, want 4", stats.Threads)
	}
	if stats.Depth < 1 {
		t.Fatalf("depth = %d, want at least one banked iteration in the pool", stats.Depth)
	}
}
