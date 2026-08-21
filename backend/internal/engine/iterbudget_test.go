package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIterationGrowthDefaultsWithoutHistory(t *testing.T) {
	assert.Equal(t, iterGrowthDefault, iterationGrowth(120, 0),
		"no previous iteration means the default growth factor applies")
	assert.Equal(t, iterGrowthDefault, iterationGrowth(0, 100),
		"a missing last iteration must not produce a zero or negative growth")
}

func TestIterationGrowthClampsMeasuredRatio(t *testing.T) {
	assert.Equal(t, iterGrowthMin, iterationGrowth(10, 100),
		"iteration times can shrink on a warm TT, but predictions must not assume it")
	assert.Equal(t, iterGrowthMax, iterationGrowth(1000, 10),
		"one noisy re-search must not predict runaway growth")
	assert.InDelta(t, 3.0, iterationGrowth(300, 100), 1e-9,
		"a clean measured ratio is used as-is")
}

func TestNextIterationFits(t *testing.T) {
	assert.True(t, nextIterationFits(800, 100, 100, 1000),
		"a cheap predicted iteration still fits inside the soft budget")
	assert.False(t, nextIterationFits(800, 500, 100, 1000),
		"starting an iteration predicted to blow past soft must be refused")
	assert.True(t, nextIterationFits(9900, 500, 100, 0),
		"soft limit 0 disables the gate entirely (hard bound still applies)")
	assert.True(t, nextIterationFits(800, 0, 0, 1000),
		"no measured iteration yet: the first depths are cheap, let them run")
	assert.True(t, nextIterationFits(850, 100, 100, 1000),
		"boundary: elapsed plus predicted exactly equal to soft is allowed")
}

func TestIterationGatingKeepsSpendNearSoft(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(9, 6, domain.PlayerBlue)
	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: domain.AbsoluteMaxDepth, TimeLimitMs: 5000, SoftLimitMs: 500, Goroutines: 1}

	start := time.Now()
	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, stats.DepthAchieved, 1)
	// Predictive gating must not merely stop STARTING iterations at soft: it
	// must refuse iterations that cannot FINISH inside soft. The unfixed
	// behavior is one straddling iteration burning toward the 5s hard bound;
	// 4x soft leaves ample room for -race and concurrent-package slowdowns.
	assert.Less(t, elapsed.Milliseconds(), int64(2000),
		"iteration start must be gated on predicted completion, not just elapsed (took %dms)", elapsed.Milliseconds())
}
