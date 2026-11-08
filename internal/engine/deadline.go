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

type ponderDeadline interface {
	Deadline
	Reset()
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

type nodeFeeder interface {
	Add(nodes uint64)
}

type NodeBudget struct {
	count   atomic.Uint64
	limit   uint64
	stopped atomic.Bool
}

func NewNodeBudget(limit uint64) *NodeBudget { return &NodeBudget{limit: limit} }

func (n *NodeBudget) Add(nodes uint64) { n.count.Add(nodes) }

func (n *NodeBudget) Exceeded() bool { return n.stopped.Load() || n.count.Load() >= n.limit }

func (n *NodeBudget) Stop() { n.stopped.Store(true) }

func (n *NodeBudget) Count() uint64 { return n.count.Load() }

func (n *NodeBudget) Reset(limit uint64) {
	n.count.Store(0)
	n.limit = limit
	n.stopped.Store(false)
}
