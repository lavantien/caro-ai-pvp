package engine

import (
	"sync/atomic"
	"time"
)

type Deadline interface {
	Exceeded() bool
	Stop()
}

type Budgeter interface {
	Budget() time.Duration
}

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

type Stoppable struct {
	stopped atomic.Bool
}

func NewStoppable() *Stoppable { return &Stoppable{} }

func (s *Stoppable) Exceeded() bool { return s.stopped.Load() }

func (s *Stoppable) Stop() { s.stopped.Store(true) }

func (s *Stoppable) Reset() { s.stopped.Store(false) }
