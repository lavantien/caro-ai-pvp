package clock

import (
	"math"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// TestNewGameClockPanicMessage pins the panic text of the NewGameClock
// bounds guard. A broken guard must not merely panic: letting an
// out-of-range index through to config.TimeControls produces a runtime
// index panic instead of the package's own message, and only the exact
// message distinguishes the two.
func TestNewGameClockPanicMessage(t *testing.T) {
	const want = "clock: NewGameClock time control index out of range"
	for _, idx := range []int{-1, len(config.TimeControls), len(config.TimeControls) + 7} {
		func() {
			defer func() {
				r, ok := recover().(string)
				if !ok || r != want {
					t.Errorf("NewGameClock(%d) recover = %v, want panic %q", idx, r, want)
				}
			}()
			NewGameClock(idx)
		}()
	}
}

// TestBudgetSpreadsOverLastPlannedMove pins movesLeft at the tail of the
// drain plan. With moves one below ClockExpectedMovesPerSide the divisor
// is exactly 1, so the whole spendable remainder backs the next grant;
// a floor of 2 halves the feedforward and drops the budget below the
// clean band.
func TestBudgetSpreadsOverLastPlannedMove(t *testing.T) {
	tcIdx := -1
	for i, tc := range config.TimeControls {
		if tc.InitialSec > 0 && tc.IncrementSec > 0 {
			tcIdx = i
			break
		}
	}
	if tcIdx < 0 {
		t.Fatalf("no time control with a positive increment to test against")
	}
	tc := config.TimeControls[tcIdx]
	initMs := float64(tc.InitialSec) * float64(time.Second/time.Millisecond)
	incMs := float64(tc.IncrementSec) * float64(time.Second/time.Millisecond)

	commits := config.ClockExpectedMovesPerSide - 1
	c := NewGameClock(tcIdx)
	for i := 0; i < commits; i++ {
		c.Commit(0)
	}
	remMs := initMs + float64(commits)*incMs
	spendable := remMs - config.SearchSafetyMarginMs
	if spendable <= 0 {
		t.Fatalf("test setup: spendable %v must be positive", spendable)
	}

	incShare := incMs * config.ClockIncrementShare
	frac := config.ClockPIDClampFraction
	ffClean := spendable + incShare    // movesLeft = Max(1, 1) = 1
	ffHalved := spendable/2 + incShare // movesLeft = Max(2, 1) = 2, the mutant floor
	lo := ffClean * (1 - frac)
	hi := math.Min(ffClean*(1+frac), math.Max(spendable, config.SearchMinMoveTimeMs))
	if sep := ffHalved * (1 + frac); lo <= sep {
		t.Fatalf("test setup: clean band low edge %v does not clear the halved ceiling %v; constants drifted apart", lo, sep)
	}

	got := float64(c.Budget()) / float64(time.Millisecond)
	if got < lo || got > hi {
		t.Errorf("tc %d after %d zero commits: budget = %vms, want within [%v, %v]ms (full-spread band)", tcIdx, commits, got, lo, hi)
	}
}
