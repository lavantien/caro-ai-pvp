package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"
	"time"

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

	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())

	completes := (x == 2 || x == 7) && y == 5
	assert.True(t, completes,
		"with no time for any depth the fallback must be an ordered move (the winning completion), got (%d,%d)", x, y)
	assert.Equal(t, "timeout-fallback", stats.MoveType,
		"a move picked without any completed depth must be flagged in the stats")
}

func TestSoftLimitStopsBeforeHardBound(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(9, 6, domain.PlayerBlue)
	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 10, TimeLimitMs: 120_000, SoftLimitMs: 2_000, Goroutines: 1}

	start := time.Now()
	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, stats.DepthAchieved, 1)
	// The soft limit only gates starting new depths, so one straddling
	// iteration may run past it; with the incremental eval that iteration
	// can be seconds. The ceiling below still leaves an order of magnitude
	// of headroom to the 120s hard bound, which a regression that ignores
	// the soft limit would burn toward.
	assert.Less(t, elapsed.Milliseconds(), int64(40_000),
		"search must stop near the soft limit instead of burning to the hard bound (elapsed %dms)", elapsed.Milliseconds())
}

func TestTTStoreStampsCurrentAge(t *testing.T) {
	tt := NewTranspositionTable(1)
	// Two distinct hashes sharing one slot (offset by the table stride).
	h1 := uint64(0xABCD)
	h2 := h1 + uint64(len(tt.shards[0].slots))
	tt.Store(TTEntry{Hash: h1, Score: 100, Depth: 10})

	for range 3 {
		tt.IncrementAge()
	}
	// A fresh shallow entry must outcompete the now-stale deep one for the
	// slot (same-hash replacement stays depth-only by design).
	tt.Store(TTEntry{Hash: h2, Score: 5, Depth: 2})

	_, ok := tt.Lookup(h1)
	assert.False(t, ok, "aged entry must lose the slot to the fresh write")
	entry, ok := tt.Lookup(h2)
	assert.True(t, ok)
	assert.Equal(t, int32(5), entry.Score)
	assert.Equal(t, uint8(3), entry.Age, "stored entries carry the current age")
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
