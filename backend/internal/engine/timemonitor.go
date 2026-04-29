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
	mu          sync.Mutex
}

func NewTimeMonitor(ctx context.Context, hardBoundMs int64) *TimeMonitor {
	ctx, cancel := context.WithCancel(ctx)
	tm := &TimeMonitor{
		hardBoundMs: hardBoundMs,
		startTime:   time.Now(),
		cancel:      cancel,
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
	return tm.stopped.Load() || tm.ElapsedMs() >= tm.hardBoundMs
}

func (tm *TimeMonitor) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.stopped.CompareAndSwap(false, true) {
		tm.cancel()
	}
}
