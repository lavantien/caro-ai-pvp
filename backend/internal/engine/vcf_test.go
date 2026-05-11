package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVCFFindsImmediateWin(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	x, y, found := SolveVCF(b, domain.PlayerRed, 1000, context.Background())
	assert.True(t, found, "should find VCF win")
	assert.True(t, (x == 2 || x == 7) && y == 5, "should complete the five, got (%d,%d)", x, y)
}

func TestVCFNoWin(t *testing.T) {
	b := domain.NewBoard()
	b = b.PlaceStone(8, 8, domain.PlayerRed)
	b = b.PlaceStone(9, 9, domain.PlayerBlue)

	_, _, found := SolveVCF(b, domain.PlayerRed, 100, context.Background())
	assert.False(t, found, "should not find VCF win from opening position")
}

func TestVCFCancelled(t *testing.T) {
	// Use a sparse board with no immediate win so the search must iterate
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue).
		PlaceStone(1, 1, domain.PlayerBlue)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, found := SolveVCF(b, domain.PlayerRed, 1000, ctx)
	assert.False(t, found, "should not find VCF when context is cancelled")
}

func TestVCFFourBlocks(t *testing.T) {
	// Red has 3 in a row, placing a 4th creates a four with one open end → block needed
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(9, 5, domain.PlayerBlue).
		PlaceStone(10, 10, domain.PlayerBlue)
	// Placing at (8,5) makes 4 horizontal with right blocked, left open
	sb := NewSearchBoard(b)
	sb.MakeMove(8, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 8, 5, domain.PlayerRed)
	sb.UnmakeMove()
	assert.Equal(t, 1, len(blocks), "should have one block point (left end)")
	if len(blocks) > 0 {
		assert.Equal(t, 4, blocks[0].X)
	}
}

func TestVCFFourBlocksBothOpen(t *testing.T) {
	// 3 reds in a row, placing a 4th creates open four → 2 block points
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	sb.MakeMove(8, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 8, 5, domain.PlayerRed)
	sb.UnmakeMove()
	assert.Equal(t, 2, len(blocks), "open four should have two block points")
}

func TestVCFFourBlocksNoFour(t *testing.T) {
	// Only 2 in a row → no four
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	sb.MakeMove(7, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 7, 5, domain.PlayerRed)
	sb.UnmakeMove()
	assert.Equal(t, 0, len(blocks), "three in a row is not a four")
}

func TestVCFSearchFindsWinViaContinuousFours(t *testing.T) {
	// Set up a position where red can force a win by playing continuous fours:
	// Red has XXX_ (needs one more for four) and another direction XXX_
	// This tests the recursive search path where opponent blocks one four
	// but red plays another four
	b := domain.NewBoard().
		// Horizontal: 3 reds at (5,5),(6,5),(7,5) → placing (8,5) creates four
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		// Vertical: 3 reds at (8,2),(8,3),(8,4) → if red gets (8,5) creates another four
		PlaceStone(8, 2, domain.PlayerRed).
		PlaceStone(8, 3, domain.PlayerRed).
		PlaceStone(8, 4, domain.PlayerRed).
		// Blue blockers
		PlaceStone(0, 0, domain.PlayerBlue).
		PlaceStone(1, 1, domain.PlayerBlue)

	x, y, found := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.True(t, found, "should find VCF win via continuous fours")
	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
}

func TestVCFOpponentCounterWin(t *testing.T) {
	// Red creates a four. Both block points complete Blue's five.
	// VCF must return false since any block results in Blue winning.
	b := domain.NewBoard().
		// Red: 3 in a row at row 0
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		// Blue: 4 in a row left and right of Red's line
		PlaceStone(0, 5, domain.PlayerBlue).
		PlaceStone(1, 5, domain.PlayerBlue).
		PlaceStone(2, 5, domain.PlayerBlue).
		PlaceStone(3, 5, domain.PlayerBlue).
		PlaceStone(10, 5, domain.PlayerBlue).
		PlaceStone(11, 5, domain.PlayerBlue).
		PlaceStone(12, 5, domain.PlayerBlue).
		PlaceStone(13, 5, domain.PlayerBlue).
		PlaceStone(15, 15, domain.PlayerBlue)

	_, _, found := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.False(t, found, "VCF should fail when all block points give opponent five-in-a-row")
}
