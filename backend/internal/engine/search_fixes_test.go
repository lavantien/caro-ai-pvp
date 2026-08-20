package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// redHasOpenFour gives red an open four on row 5 (columns 3-6): any completion
// at (2,5) or (7,5) is an immediate win.
func redHasOpenFour() domain.Board {
	b := domain.NewBoard()
	for x := 3; x <= 6; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	return b.PlaceStone(10, 10, domain.PlayerBlue)
}

func TestForcedWinStopsDeepening(t *testing.T) {
	b := redHasOpenFour()
	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: domain.AbsoluteMaxDepth, TimeLimitMs: 10_000, Goroutines: 1}

	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())

	assert.Equal(t, 1, stats.DepthAchieved,
		"iterative deepening must stop after the depth that proves a forced win")
	assert.GreaterOrEqual(t, stats.SearchScore, domain.WinScore-domain.AbsoluteMaxDepth)
}

func TestZeroTimeFallbackIsOrdered(t *testing.T) {
	b := redHasOpenFour()
	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: domain.AbsoluteMaxDepth, TimeLimitMs: 0, Goroutines: 1}

	x, y, _ := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())

	completes := (x == 2 || x == 7) && y == 5
	assert.True(t, completes,
		"with no time for any depth the fallback must be an ordered move (the winning completion), got (%d,%d)", x, y)
}

func TestRootSearchStoresBoundFlagOnFailLow(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)
	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	monitor := NewTimeMonitor(context.Background(), 5000)
	defer monitor.Stop()

	// Quiet position: true score is far below alpha, so the root search must fail low.
	_, _, score := searchRoot(&sb, domain.PlayerRed, 2, 24_000, 25_000, tt, heuristics, candidates, monitor, nil)
	assert.LessOrEqual(t, score, 24_000, "precondition: search must fail low against a high alpha")

	entry, ok := tt.Lookup(sb.Hash())
	assert.True(t, ok, "root search must store its result")
	assert.Equal(t, TTUpperBound, entry.Flag,
		"a fail-low root result is an upper bound, not an exact score")
}

func TestQuiescenceIsFailSoft(t *testing.T) {
	b := redHasOpenFour()
	sb := NewSearchBoard(b)
	heuristics := NewSearchHeuristics()
	monitor := NewTimeMonitor(context.Background(), 5000)
	defer monitor.Stop()

	standPat := Evaluate(&sb, domain.PlayerRed)
	assert.Greater(t, standPat, 200, "precondition: stand-pat must exceed beta")

	score := quiesce(&sb, domain.PlayerRed, 100, 200, domain.MaxQuiescenceDepth, heuristics, monitor, 0)
	assert.Equal(t, standPat, score,
		"quiescence cutoffs must return the fail-soft stand-pat score, not beta")
}

func TestTTSizeScalesWithLevel(t *testing.T) {
	low := GetDifficultyProfile(1)
	high := GetDifficultyProfile(5)
	assert.Greater(t, high.TTSizeMB, low.TTSizeMB,
		"grandmaster must get a larger transposition table than novice")
	assert.LessOrEqual(t, low.TTSizeMB, 64,
		"novice must not allocate a large table for a ~300ms budget")
}
