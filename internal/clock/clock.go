// Package clock implements the per-side time law of the engine. The budget
// for the next move spreads the spendable remainder over the expected moves
// left, adds a share of the increment, and lets a PID controller steer the
// grant against the planned drain trajectory. The floor wins over the
// reserve: a fully drained 1+0 clock still funds SearchMinMoveTimeMs moves
// forever, Remaining never goes negative, and a timeout can never decide a
// game. Games end by win, loss, or draw only.
//
// The controller error is the surplus R - target. Holding more time than the
// plan (underspending) yields a positive correction and a larger budget,
// overspending a smaller one, so the feedback converges onto the trajectory
// instead of running away from it.
package clock

import (
	"math"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// GameClock is one side's bank under one time control. Exported methods
// speak time.Duration; the law itself runs in float milliseconds.
type GameClock struct {
	remainingMs float64
	initialMs   float64
	incrementMs float64
	moves       int
	pid         pid
	budgetMs    float64
	budgeted    bool
}

// NewGameClock starts a side's clock at the initial time of the given time
// control index into config.TimeControls. Panics on an out-of-range index.
func NewGameClock(tcIdx int) *GameClock {
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		panic("clock: NewGameClock time control index out of range")
	}
	tc := config.TimeControls[tcIdx]
	return &GameClock{
		remainingMs: float64(tc.InitialSec) * float64(time.Second/time.Millisecond),
		initialMs:   float64(tc.InitialSec) * float64(time.Second/time.Millisecond),
		incrementMs: float64(tc.IncrementSec) * float64(time.Second/time.Millisecond),
		pid:         newPID(config.ClockPID[tcIdx]),
	}
}

func (c *GameClock) Remaining() time.Duration {
	return time.Duration(c.remainingMs * float64(time.Millisecond))
}

func (c *GameClock) Moves() int {
	return c.moves
}

// Budget grants the next move's allocation. It is idempotent between
// commits: the controller advances only when a grant is actually made, so
// polling Budget repeatedly per move changes nothing.
func (c *GameClock) Budget() time.Duration {
	if !c.budgeted {
		c.budgetMs = c.law()
		c.budgeted = true
	}
	return time.Duration(c.budgetMs * float64(time.Millisecond))
}

// Commit settles a move: elapsed time is charged (an overshoot floors the
// bank at zero), the increment lands, and the move count advances. The pid
// state persists across commits, which is the whole point of integral and
// derivative action across moves. Negative elapsed is a programmer error:
// monotonic clock reads cannot produce it, and banking it would overflow
// the ns conversion and break the never-negative invariant.
func (c *GameClock) Commit(elapsed time.Duration) {
	if elapsed < 0 {
		panic("clock: Commit with negative elapsed")
	}
	elapsedMs := float64(elapsed) / float64(time.Millisecond)
	c.remainingMs = math.Max(0, c.remainingMs-elapsedMs) + c.incrementMs
	c.moves++
	c.budgeted = false
}

func (c *GameClock) law() float64 {
	spendable := math.Max(0, c.remainingMs-config.SearchSafetyMarginMs)
	movesLeft := math.Max(1, float64(config.ClockExpectedMovesPerSide-c.moves))
	feedforward := spendable/movesLeft + c.incrementMs*config.ClockIncrementShare
	target := math.Max(config.SearchSafetyMarginMs, c.initialMs+float64(c.moves)*c.incrementMs-(float64(c.moves)/config.ClockExpectedMovesPerSide)*(c.initialMs+config.ClockExpectedMovesPerSide*c.incrementMs))
	err := c.remainingMs - target
	bound := config.ClockPIDClampFraction * feedforward
	corr := clamp(c.pid.step(err), -bound, bound)
	return clamp(feedforward+corr, config.SearchMinMoveTimeMs, math.Max(spendable, config.SearchMinMoveTimeMs))
}
