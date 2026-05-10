package engine

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDifficultyProfileLevels(t *testing.T) {
	profiles := []struct {
		level         int
		name          string
		minFraction   float64
		maxFraction   float64
		minGoroutines int
		useVCF        bool
		ponder        bool
	}{
		{1, "Novice", 0.04, 0.06, 1, false, false},
		{2, "Beginner", 0.14, 0.16, 1, false, false},
		{3, "Intermediate", 0.39, 0.41, 2, true, false},
		{4, "Advanced", 0.69, 0.71, 1, true, false},
		{5, "Grandmaster", 0.99, 1.01, 1, true, true},
	}

	for _, tc := range profiles {
		t.Run(tc.name, func(t *testing.T) {
			p := GetDifficultyProfile(tc.level)
			assert.Equal(t, tc.name, p.Name)
			assert.GreaterOrEqual(t, p.TimeFraction, tc.minFraction)
			assert.LessOrEqual(t, p.TimeFraction, tc.maxFraction)
			assert.GreaterOrEqual(t, p.Goroutines, tc.minGoroutines)
			assert.Equal(t, tc.useVCF, p.UseVCF)
			assert.Equal(t, tc.ponder, p.Ponder)
		})
	}
}

func TestDifficultyL5Goroutines(t *testing.T) {
	n := runtime.GOMAXPROCS(0)
	p := GetDifficultyProfile(5)
	expected := pow2Floor((n - 2) / 2)
	assert.Equal(t, expected, p.Goroutines)
}

func TestAllocateTime(t *testing.T) {
	alloc := AllocateTime(30000, 1000, 5)
	assert.Greater(t, alloc.OptimalMs, int64(0))
	assert.Less(t, alloc.OptimalMs, int64(30000))
	assert.Greater(t, alloc.HardBoundMs, alloc.OptimalMs)
	assert.Greater(t, alloc.OptimalMs, alloc.SoftBoundMs)
}

func TestAllocateTimeMinimum(t *testing.T) {
	alloc := AllocateTime(200, 0, 1)
	assert.Greater(t, alloc.OptimalMs, int64(0), "should have some allocation even with low time")
	assert.LessOrEqual(t, alloc.OptimalMs, int64(200), "should not exceed remaining time")
}

func TestPow2Floor(t *testing.T) {
	assert.Equal(t, 1, pow2Floor(0))
	assert.Equal(t, 1, pow2Floor(-1))
	assert.Equal(t, 1, pow2Floor(1))
	assert.Equal(t, 2, pow2Floor(2))
	assert.Equal(t, 4, pow2Floor(5))
	assert.Equal(t, 8, pow2Floor(10))
	assert.Equal(t, 16, pow2Floor(20))
}

func TestGetEngineThreadsForLoad(t *testing.T) {
	n := runtime.GOMAXPROCS(0)
	assert.Equal(t, n, GetEngineThreadsForLoad(1))
	assert.Equal(t, n, GetEngineThreadsForLoad(0))
	assert.Equal(t, n/2, GetEngineThreadsForLoad(2))
}
