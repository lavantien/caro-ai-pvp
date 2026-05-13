package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type TimeMonitor struct {
	hardBoundMs int64
	startTime   time.Time
	cancel      context.CancelFunc
	stopped     atomic.Bool
	ctxDone     <-chan struct{}
	mu          sync.Mutex
	Nodes       atomic.Int64
}

func NewTimeMonitor(ctx context.Context, hardBoundMs int64) *TimeMonitor {
	ctx, cancel := context.WithCancel(ctx)
	tm := &TimeMonitor{
		hardBoundMs: hardBoundMs,
		startTime:   time.Now(),
		cancel:      cancel,
		ctxDone:     ctx.Done(),
	}
	go tm.watch(ctx)
	return tm
}

func (tm *TimeMonitor) watch(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			tm.Stop()
			return
		case <-ticker.C:
			if tm.ElapsedMs() >= tm.hardBoundMs {
				tm.Stop()
				return
			}
		}
	}
}

func (tm *TimeMonitor) ElapsedMs() int64 {
	return time.Since(tm.startTime).Milliseconds()
}

func (tm *TimeMonitor) ShouldStop() bool {
	if tm.stopped.Load() {
		return true
	}
	if tm.ElapsedMs() >= tm.hardBoundMs {
		return true
	}
	select {
	case <-tm.ctxDone:
		tm.Stop()
		return true
	default:
		return false
	}
}

func (tm *TimeMonitor) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.stopped.CompareAndSwap(false, true) {
		tm.cancel()
	}
}
