package engine

import (
	"sync/atomic"
	"time"
)

// Deadline is the cancellation surface shared by the search, the VCF and VCT
// solvers, and the clock. Exceeded must be cheap enough for every
// SearchNodeCheckInterval nodes, Stop forces an immediate hard stop. Both
// methods may be called from any goroutine: one deadline object is shared by
// every worker of an instance.
type Deadline interface {
	Exceeded() bool
	Stop()
}

// Budgeter is the optional interface a Deadline implements when it carries an
// explicit allocation, so SearchStats can report the granted budget.
type Budgeter interface {
	Budget() time.Duration
}

// FixedBudget is a context-free deadline over the monotonic clock: exceeded
// once the fixed duration elapsed since construction or after Stop. Safe for
// concurrent Exceeded and Stop calls. Reset re-arms the window in place but
// must not race an in-flight Exceeded, so callers reset before handing the
// deadline to workers.
type FixedBudget struct {
	until    time.Time
	duration time.Duration
	stopped  atomic.Bool
}

func NewFixedBudget(d time.Duration) *FixedBudget {
	f := &FixedBudget{}
	f.Reset(d)
	return f
}

func (f *FixedBudget) Exceeded() bool { return f.stopped.Load() || !time.Now().Before(f.until) }

func (f *FixedBudget) Stop() { f.stopped.Store(true) }

func (f *FixedBudget) Budget() time.Duration { return f.duration }

func (f *FixedBudget) Reset(d time.Duration) {
	f.until = time.Now().Add(d)
	f.duration = d
	f.stopped.Store(false)
}
