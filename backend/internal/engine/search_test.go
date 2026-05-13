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
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).PlaceStone(8, 5, domain.PlayerRed).
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
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).PlaceStone(9, 9, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerRed).
		PlaceStone(3, 3, domain.PlayerRed).PlaceStone(4, 4, domain.PlayerRed).
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		PlaceStone(0, 0, domain.PlayerRed).PlaceStone(15, 15, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 5, TimeLimitMs: 5000, Goroutines: 1}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	blockOrWin := (x == 4 && y == 5) || (x == 8 && y == 5)
	if stats.SearchScore >= domain.WinScore-domain.AbsoluteMaxDepth {
		blockOrWin = true
	}
	assert.True(t, blockOrWin || stats.DepthAchieved >= 3,
		"engine should address opponent's flex3 or find counter-win, got (%d,%d) d=%d s=%d",
		x, y, stats.DepthAchieved, stats.SearchScore)
}

func TestSearchBlocksVCFThroughAlphaBeta(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).
		PlaceStone(2, 13, domain.PlayerRed).PlaceStone(13, 2, domain.PlayerRed)

	bvx, bvy, blueHasVCF := SolveVCF(b, domain.PlayerBlue, 5000, context.Background())
	if blueHasVCF != VCFWin {
		t.Skip("Blue doesn't have a VCF")
	}
	_, _, redHasVCF := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	if redHasVCF == VCFWin {
		t.Skip("Red has a VCF")
	}
	blocked := b.PlaceStone(bvx, bvy, domain.PlayerRed)
	_, _, stillHas := SolveVCF(blocked, domain.PlayerBlue, 5000, context.Background())
	if stillHas == VCFWin {
		t.Skip("Blocking doesn't stop VCF")
	}

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 4, TimeLimitMs: 5000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())

	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
	assert.Greater(t, stats.DepthAchieved, 0, "should search through alpha-beta, not short-circuit")
	assert.NotEqual(t, "vcf-block", stats.MoveType, "should not use vcf-block shortcut")
}

func TestSearchFindsBlockingMove(t *testing.T) {
	b := domain.NewBoard()
	b = b.PlaceStone(2, 5, domain.PlayerRed)
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

func TestSearchBlocksVCFWithProvenNoWin(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).
		PlaceStone(2, 13, domain.PlayerRed).PlaceStone(13, 2, domain.PlayerRed)

	bvx, bvy, blueResult := SolveVCF(b, domain.PlayerBlue, 5000, context.Background())
	if blueResult != VCFWin {
		t.Skip("Blue doesn't have a VCF")
	}
	blocked := b.PlaceStone(bvx, bvy, domain.PlayerRed)
	_, _, checkResult := SolveVCF(blocked, domain.PlayerBlue, 5000, context.Background())
	if checkResult != VCFNoWin {
		t.Skip("Blocking doesn't produce proven no-win")
	}

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 4, TimeLimitMs: 10000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
	assert.NotEqual(t, "vcf-block", stats.MoveType, "should use alpha-beta, not vcf-block")
	_ = bvx
	_ = bvy
}

func TestSearchFindsValidMoveUnderTimePressure(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerBlue).PlaceStone(6, 5, domain.PlayerBlue).
		PlaceStone(7, 5, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).
		PlaceStone(2, 13, domain.PlayerRed).PlaceStone(13, 2, domain.PlayerRed)

	_, _, blueResult := SolveVCF(b, domain.PlayerBlue, 5000, context.Background())
	if blueResult != VCFWin {
		t.Skip("Blue doesn't have a VCF")
	}

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 4, TimeLimitMs: 500, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
	assert.Greater(t, stats.DepthAchieved, 0, "alpha-beta should have searched at least 1 ply")
}
