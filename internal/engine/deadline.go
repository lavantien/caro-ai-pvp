package engine

import "time"

// Deadline is the cancellation surface shared by the search, the VCF and VCT
// solvers, and the clock. Exceeded must be cheap enough for every
// SearchNodeCheckInterval nodes, Stop forces an immediate hard stop.
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
// once the fixed duration elapsed since construction or after Stop.
type FixedBudget struct {
	until    time.Time
	duration time.Duration
	stopped  bool
}

func NewFixedBudget(d time.Duration) *FixedBudget {
	f := &FixedBudget{}
	f.Reset(d)
	return f
}

// Reset re-arms the budget in place so repeated searches reuse one object
// without allocating.
func (f *FixedBudget) Reset(d time.Duration) {
	f.until = time.Now().Add(d)
	f.duration = d
	f.stopped = false
}

func (f *FixedBudget) Exceeded() bool { return f.stopped || !time.Now().Before(f.until) }

func (f *FixedBudget) Stop() { f.stopped = true }

func (f *FixedBudget) Budget() time.Duration { return f.duration }
