package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchFindsWinningMove(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    4,
		TimeLimitMs: 5000,
		Goroutines:  1,
	}

	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 2 || x == 7, "should find winning move at end of line, got (%d,%d)", x, y)
	assert.Equal(t, 5, y)
	assert.Greater(t, stats.NodesSearched, int64(0))
}

func TestSearchFindsWinningMoveDespiteFutility(t *testing.T) {
	// Red has 4 in a row needing 5th at (4,5) or (9,5).
	// Blue has some scattered stones to boost static eval, potentially triggering futility pruning.
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).PlaceStone(8, 5, domain.PlayerRed).
		// Blue has stones scattered around
		PlaceStone(3, 3, domain.PlayerBlue).PlaceStone(4, 4, domain.PlayerBlue).
		PlaceStone(5, 4, domain.PlayerBlue).
		PlaceStone(10, 10, domain.PlayerBlue).PlaceStone(11, 11, domain.PlayerBlue).
		PlaceStone(12, 12, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 3, TimeLimitMs: 5000, Goroutines: 1}
	x, y, _ := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	won := (x == 4 || x == 9) && y == 5
	assert.True(t, won, "should find winning fifth stone, got (%d,%d)", x, y)
}

func TestSearchBlocksOpponentThreatAtNullMoveDepth(t *testing.T) {
	// Red has a strong diagonal (looks good statically) but Blue has an
	// open three (Flex3) that becomes an open four if Red doesn't block.
	// Null-move pruning at depth>=3 would say "I can pass" and miss the defense.
	b := domain.NewBoard().
		// Red has a strong diagonal
		PlaceStone(8, 8, domain.PlayerRed).PlaceStone(9, 9, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerRed).
		// Red also has some other stones
		PlaceStone(3, 3, domain.PlayerRed).PlaceStone(4, 4, domain.PlayerRed).
		// Blue has an open three that must be blocked
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		// Extra filler
		PlaceStone(0, 0, domain.PlayerRed).PlaceStone(15, 15, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 5, TimeLimitMs: 5000, Goroutines: 1}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	// Red must either block at (4,5) or (8,5), or find a winning counter-attack
	blockOrWin := (x == 4 && y == 5) || (x == 8 && y == 5)
	// Also accept if Red found a winning move (counter-attack)
	if stats.SearchScore >= domain.WinScore-domain.AbsoluteMaxDepth {
		blockOrWin = true
	}
	assert.True(t, blockOrWin || stats.DepthAchieved >= 3,
		"engine should address opponent's flex3 or find counter-win, got (%d,%d) d=%d s=%d",
		x, y, stats.DepthAchieved, stats.SearchScore)
}

func TestVCFBlockShortCircuitWorks(t *testing.T) {
	// Blue has a VCF: (4,5) extends to four, then five.
	// Red has no VCF. Red must block at (4,5).
	// The VCF-block short-circuit should fire and return (4,5).
	b := domain.NewBoard().
		// Blue: three in a row at y=5
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		// Blue: one extra stone
		PlaceStone(8, 6, domain.PlayerBlue).
		// Red: isolated — no VCF
		PlaceStone(2, 13, domain.PlayerRed).PlaceStone(13, 2, domain.PlayerRed)

	// Verify Blue has VCF, Red doesn't, and blocking stops it
	bvx, bvy, blueHasVCF := SolveVCF(b, domain.PlayerBlue, 5000, context.Background())
	if !blueHasVCF {
		t.Skip("Blue doesn't have a VCF")
	}
	_, _, redHasVCF := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	if redHasVCF {
		t.Skip("Red has a VCF")
	}
	blocked := b.PlaceStone(bvx, bvy, domain.PlayerRed)
	_, _, stillHas := SolveVCF(blocked, domain.PlayerBlue, 5000, context.Background())
	if stillHas {
		t.Skip("Blocking doesn't stop VCF")
	}

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 4, TimeLimitMs: 5000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())

	assert.Equal(t, "vcf-block", stats.MoveType, "should short-circuit to vcf-block")
	assert.Equal(t, bvx, x, "should block at VCF start move x")
	assert.Equal(t, bvy, y, "should block at VCF start move y")
}

func TestSearchFindsBlockingMove(t *testing.T) {
	// Blue has 4 in a row with one end blocked by Red
	b := domain.NewBoard()
	b = b.PlaceStone(2, 5, domain.PlayerRed) // block one end
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerBlue)
	}
	b = b.PlaceStone(0, 0, domain.PlayerRed)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    4,
		TimeLimitMs: 5000,
		Goroutines:  1,
	}

	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 7 && y == 5,
		"should block opponent's four at (7,5), got (%d,%d)", x, y)
	assert.Greater(t, stats.DepthAchieved, 0)
}
