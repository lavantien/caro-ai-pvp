package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeMonitorExpiry(t *testing.T) {
	ctx := context.Background()
	tm := NewTimeMonitor(ctx, 50)
	defer tm.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, tm.ShouldStop())
}

func TestTimeMonitorNotExpired(t *testing.T) {
	ctx := context.Background()
	tm := NewTimeMonitor(ctx, 5000)
	defer tm.Stop()

	assert.False(t, tm.ShouldStop())
}

func TestTimeMonitorStop(t *testing.T) {
	ctx := context.Background()
	tm := NewTimeMonitor(ctx, 5000)
	tm.Stop()

	assert.True(t, tm.ShouldStop())
}

func TestTimeMonitorElapsedMs(t *testing.T) {
	ctx := context.Background()
	tm := NewTimeMonitor(ctx, 5000)
	defer tm.Stop()

	time.Sleep(50 * time.Millisecond)
	elapsed := tm.ElapsedMs()
	assert.GreaterOrEqual(t, elapsed, int64(40))
}
