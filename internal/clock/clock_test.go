package clock

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// simPID and oracleBudget recompute the budget law straight from the config
// constants, independently of clock.go, so exact-equality assertions pin the
// law instead of the implementation against itself.

type simPID struct {
	gains   config.PIDGains
	integ   float64
	prevErr float64
	primed  bool
}

func (p *simPID) step(err float64) float64 {
	p.integ = clampF(p.integ+err, -config.ClockPIDIntegClampMs, config.ClockPIDIntegClampMs)
	deriv := 0.0
	if p.primed {
		deriv = p.gains.Kd * (err - p.prevErr)
	}
	p.prevErr = err
	p.primed = true
	return p.gains.Kp*err + p.gains.Ki*p.integ + deriv
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func oracleBudget(k int, remMs, initMs, incMs float64, sim *simPID) time.Duration {
	spendable := math.Max(0, remMs-config.SearchSafetyMarginMs)
	movesLeft := math.Max(1, float64(config.ClockExpectedMovesPerSide-k))
	feedforward := spendable/movesLeft + incMs*config.ClockIncrementShare
	target := math.Max(config.SearchSafetyMarginMs, initMs+float64(k)*incMs-(float64(k)/config.ClockExpectedMovesPerSide)*(initMs+config.ClockExpectedMovesPerSide*incMs))
	err := remMs - target
	corr := clampF(sim.step(err), -config.ClockPIDClampFraction*feedforward, config.ClockPIDClampFraction*feedforward)
	budget := clampF(feedforward+corr, config.SearchMinMoveTimeMs, math.Max(spendable, config.SearchMinMoveTimeMs))
	return time.Duration(budget * float64(time.Millisecond))
}

func TestNewGameClockInitialState(t *testing.T) {
	for tcIdx, tc := range config.TimeControls {
		c := NewGameClock(tcIdx)
		if got, want := c.Remaining(), time.Duration(tc.InitialMin)*time.Minute; got != want {
			t.Errorf("tc %d initial remaining = %v, want %v", tcIdx, got, want)
		}
		if got := c.Moves(); got != 0 {
			t.Errorf("tc %d initial moves = %d, want 0", tcIdx, got)
		}
	}
}

func TestNewGameClockPanicsOutOfRange(t *testing.T) {
	for _, idx := range []int{-1, len(config.TimeControls), len(config.TimeControls) + 7} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewGameClock(%d) did not panic", idx)
				}
			}()
			NewGameClock(idx)
		}()
	}
}

func TestCommitPanicsOnNegativeElapsed(t *testing.T) {
	for _, elapsed := range []time.Duration{-1, -time.Second, math.MinInt64} {
		func() {
			defer func() {
				r, ok := recover().(string)
				if !ok || r != "clock: Commit with negative elapsed" {
					t.Errorf("Commit(%v) recover = %v, want the negative elapsed panic", elapsed, recover())
				}
			}()
			c := NewGameClock(0)
			c.Commit(elapsed)
		}()
	}
}

func TestBudgetExactTables(t *testing.T) {
	for _, tc := range []struct {
		tcIdx   int
		elapsed []int // milliseconds per move: underspend, on-plan, overshoot, drain
	}{
		{0, []int{5, 25, 400, 2000}},
		{1, []int{100, 562, 1500, 2500}},
		{2, []int{50, 1100, 3000, 700}},
		{3, []int{5000, 22000, 40000, 120000}},
	} {
		ctl := config.TimeControls[tc.tcIdx]
		c := NewGameClock(tc.tcIdx)
		initMs := float64(ctl.InitialMin*60) * float64(time.Second/time.Millisecond)
		incMs := float64(ctl.IncrementSec) * float64(time.Second/time.Millisecond)
		remMs := initMs
		sim := &simPID{gains: config.ClockPID[tc.tcIdx]}
		check := func(k int) {
			t.Helper()
			want := oracleBudget(k, remMs, initMs, incMs, sim)
			if got := c.Budget(); got != want {
				t.Errorf("tc %d move %d: budget = %v, want %v", tc.tcIdx, k, got, want)
			}
		}
		check(0)
		for i, ms := range tc.elapsed {
			elapsed := time.Duration(ms) * time.Millisecond
			c.Commit(elapsed)
			remMs = math.Max(0, remMs-float64(elapsed)/float64(time.Millisecond)) + incMs
			check(i + 1)
		}
	}
}

// TestNewGameClockWithGainsSeam pins the tuning seam: explicit committed
// gains must reproduce NewGameClock exactly over a commit trace, and the
// seam constructor carries the same bounds panic.
func TestNewGameClockWithGainsSeam(t *testing.T) {
	for _, tcIdx := range []int{0, len(config.TimeControls) - 1} {
		prod := NewGameClock(tcIdx)
		seam := NewGameClockWithGains(tcIdx, config.ClockPID[tcIdx])
		for move := range 10 {
			if prod.Budget() != seam.Budget() {
				t.Fatalf("tc %d move %d: seam budget %v differs from production %v", tcIdx, move, seam.Budget(), prod.Budget())
			}
			elapsed := time.Duration(100+move*50) * time.Millisecond
			prod.Commit(elapsed)
			seam.Commit(elapsed)
		}
	}
	const want = "clock: NewGameClock time control index out of range"
	for _, idx := range []int{-1, len(config.TimeControls)} {
		func() {
			defer func() {
				r, ok := recover().(string)
				if !ok || r != want {
					t.Errorf("NewGameClockWithGains(%d) recover = %v, want panic %q", idx, r, want)
				}
			}()
			NewGameClockWithGains(idx, config.PIDGains{})
		}()
	}
}

func TestDrainedClockFundsMinimumMoves(t *testing.T) {
	c := NewGameClock(0) // 1+0
	floor := time.Duration(config.SearchMinMoveTimeMs) * time.Millisecond
	for move := 1; move <= 50; move++ {
		c.Commit(time.Hour)
		if got := c.Budget(); got != floor {
			t.Fatalf("move %d: drained budget = %v, want exactly %v", move, got, floor)
		}
		if got := c.Remaining(); got != 0 {
			t.Fatalf("move %d: drained remaining = %v, want 0", move, got)
		}
		if got := c.Moves(); got != move {
			t.Fatalf("move %d: moves = %d", move, got)
		}
	}
}

func TestCeilingPropertySoak(t *testing.T) {
	for tcIdx := range config.TimeControls {
		ctl := config.TimeControls[tcIdx]
		c := NewGameClock(tcIdx)
		incMs := float64(ctl.IncrementSec) * float64(time.Second/time.Millisecond)
		remMs := float64(ctl.InitialMin*60) * float64(time.Second/time.Millisecond)
		rng := rand.New(rand.NewPCG(uint64(tcIdx)+1, 0xC0FFEE))
		for move := 1; move <= 4000; move++ {
			budget := c.Budget()
			grantedRem := remMs
			var elapsed time.Duration
			switch rng.IntN(4) {
			case 0:
				elapsed = time.Duration(rng.Int64N(int64(3 * time.Second)))
			case 1:
				elapsed = time.Duration(float64(budget) * (0.1 + 2.9*rng.Float64()))
			case 2:
				elapsed = 0
			case 3:
				elapsed = time.Duration(float64(budget) * 3.5) // guaranteed overshoot
			}
			c.Commit(elapsed)
			remMs = math.Max(0, remMs-float64(elapsed)/float64(time.Millisecond)) + incMs

			spendable := math.Max(0, grantedRem-config.SearchSafetyMarginMs)
			// The bound is asserted in nanoseconds: recovering milliseconds
			// from the truncated Duration can round one ULP above the law's
			// internal float, while the ns domain stays exact.
			hiNs := int64(math.Floor(math.Max(spendable, config.SearchMinMoveTimeMs) * float64(time.Millisecond)))
			if got, lo := int64(budget), int64(config.SearchMinMoveTimeMs)*int64(time.Millisecond); got < lo || got > hiNs {
				t.Fatalf("tc %d move %d: budget %dns escapes [%d, %d]ns", tcIdx, move, got, lo, hiNs)
			}
			if got, want := c.Remaining(), time.Duration(remMs*float64(time.Millisecond)); got != want || got < 0 {
				t.Fatalf("tc %d move %d: remaining = %v, want %v (never negative)", tcIdx, move, got, want)
			}
			if got := c.Moves(); got != move {
				t.Fatalf("tc %d move %d: moves = %d", tcIdx, move, got)
			}
		}
	}
}

func TestCommitAccounting(t *testing.T) {
	c := NewGameClock(1) // 2+1
	if got, want := c.Remaining(), 2*time.Minute; got != want {
		t.Errorf("initial remaining = %v, want %v", got, want)
	}
	c.Commit(90 * time.Second)
	if got, want := c.Remaining(), 31*time.Second; got != want {
		t.Errorf("after 90s: remaining = %v, want %v", got, want)
	}
	c.Commit(32 * time.Second) // overshoot floors at zero, then the increment lands
	if got, want := c.Remaining(), time.Second; got != want {
		t.Errorf("after overshoot: remaining = %v, want %v", got, want)
	}
	c.Commit(500 * time.Microsecond)
	if got, want := c.Remaining(), 1999500*time.Microsecond; got != want {
		t.Errorf("after sub-ms commit: remaining = %v, want %v", got, want)
	}
	c.Commit(0)
	if got, want := c.Remaining(), 2999500*time.Microsecond; got != want {
		t.Errorf("after zero commit: remaining = %v, want %v", got, want)
	}
	if got := c.Moves(); got != 4 {
		t.Errorf("moves = %d, want 4", got)
	}

	d := NewGameClock(0) // 1+0: no increment, so a drained clock stays drained
	d.Commit(time.Minute)
	if got := d.Remaining(); got != 0 {
		t.Errorf("drained remaining = %v, want 0", got)
	}
	d.Commit(time.Hour)
	if got := d.Remaining(); got != 0 {
		t.Errorf("post-drain remaining = %v, want 0", got)
	}
	if got := d.Moves(); got != 2 {
		t.Errorf("moves = %d, want 2", got)
	}
}

func TestBudgetIdempotentBetweenCommits(t *testing.T) {
	c := NewGameClock(2)
	c.Commit(3 * time.Millisecond)
	b1 := c.Budget()
	if b2 := c.Budget(); b1 != b2 {
		t.Fatalf("budget moved between calls: %v then %v", b1, b2)
	}
	// A clock polled three times per move must behave exactly like one
	// polled once: repeated grants neither advance nor disturb the controller.
	polled := NewGameClock(2)
	polled.Commit(3 * time.Millisecond)
	polled.Budget()
	polled.Budget()
	polled.Budget()
	polled.Commit(97 * time.Millisecond)
	once := NewGameClock(2)
	once.Commit(3 * time.Millisecond)
	once.Budget()
	once.Commit(97 * time.Millisecond)
	if got, want := polled.Budget(), once.Budget(); got != want {
		t.Errorf("over-polled next budget = %v, want %v", got, want)
	}
}

func feedforwardMs(c *GameClock, tcIdx int) float64 {
	remMs := float64(c.Remaining()) / float64(time.Millisecond)
	spendable := math.Max(0, remMs-config.SearchSafetyMarginMs)
	movesLeft := math.Max(1, float64(config.ClockExpectedMovesPerSide-c.Moves()))
	incMs := float64(config.TimeControls[tcIdx].IncrementSec) * float64(time.Second/time.Millisecond)
	return spendable/movesLeft + incMs*config.ClockIncrementShare
}

func TestCorrectionDirection(t *testing.T) {
	// eps absorbs Budget's nanosecond truncation when the correction sits
	// exactly on the clamp edge.
	const eps = 0.001

	under := NewGameClock(0)
	under.Commit(300 * time.Millisecond) // granted ~2s, spent 0.3s
	uFf := feedforwardMs(under, 0)
	uB := float64(under.Budget()) / float64(time.Millisecond)
	if uB <= uFf {
		t.Errorf("underspender budget %v must exceed feedforward %v", uB, uFf)
	}
	if band := config.ClockPIDClampFraction * uFf; math.Abs(uB-uFf) > band+eps {
		t.Errorf("underspender correction %v escapes clamp band %v", math.Abs(uB-uFf), band)
	}

	over := NewGameClock(0)
	over.Commit(40 * time.Second) // R falls below the drain trajectory
	oFf := feedforwardMs(over, 0)
	oB := float64(over.Budget()) / float64(time.Millisecond)
	if oB >= oFf {
		t.Errorf("overspender budget %v must fall below feedforward %v", oB, oFf)
	}
	if band := config.ClockPIDClampFraction * oFf; math.Abs(oFf-oB) > band+eps {
		t.Errorf("overspender correction %v escapes clamp band %v", math.Abs(oFf-oB), band)
	}
	if oB < config.SearchMinMoveTimeMs {
		t.Errorf("overspender budget %v below the floor", oB)
	}
}
