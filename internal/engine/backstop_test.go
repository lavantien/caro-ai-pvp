package engine

import (
	"sync/atomic"
	"time"
)

const ladderBackstopNodes = 16_000_000

const probeBackstopNodes = 65_536

type backstoppedFixed struct {
	inner *FixedBudget
	nodes atomic.Uint64
	limit uint64
}

func cappedBudget(d time.Duration) *backstoppedFixed {
	return &backstoppedFixed{inner: NewFixedBudget(d), limit: ladderBackstopNodes}
}

func cappedProbe(d time.Duration) *backstoppedFixed {
	return &backstoppedFixed{inner: NewFixedBudget(d), limit: probeBackstopNodes}
}

func (b *backstoppedFixed) Exceeded() bool { return b.inner.Exceeded() || b.nodes.Load() >= b.limit }

func (b *backstoppedFixed) Stop() { b.inner.Stop() }

func (b *backstoppedFixed) Budget() time.Duration { return b.inner.Budget() }

func (b *backstoppedFixed) Add(nodes uint64) { b.nodes.Add(nodes) }

func (b *backstoppedFixed) Reset(d time.Duration) {
	b.nodes.Store(0)
	b.inner.Reset(d)
}

type backstoppedNodes struct {
	inner *NodeBudget
	nodes atomic.Uint64
	limit uint64
}

func cappedNodeBudget(limit uint64) *backstoppedNodes {
	return &backstoppedNodes{inner: NewNodeBudget(limit), limit: 2 * limit}
}

func (b *backstoppedNodes) Exceeded() bool { return b.inner.Exceeded() || b.nodes.Load() >= b.limit }

func (b *backstoppedNodes) Stop() { b.inner.Stop() }

func (b *backstoppedNodes) Add(nodes uint64) {
	b.nodes.Add(nodes)
	b.inner.Add(nodes)
}

func (b *backstoppedNodes) Reset(limit uint64) {
	b.nodes.Store(0)
	b.inner.Reset(limit)
	b.limit = 2 * limit
}
