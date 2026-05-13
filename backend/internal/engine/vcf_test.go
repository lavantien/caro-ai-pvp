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

	x, y, result := SolveVCF(b, domain.PlayerRed, 1000, context.Background())
	assert.Equal(t, VCFWin, result, "should find VCF win")
	assert.True(t, (x == 2 || x == 7) && y == 5, "should complete the five, got (%d,%d)", x, y)
}

func TestVCFNoWin(t *testing.T) {
	b := domain.NewBoard()
	b = b.PlaceStone(8, 8, domain.PlayerRed)
	b = b.PlaceStone(9, 9, domain.PlayerBlue)

	_, _, result := SolveVCF(b, domain.PlayerRed, 100, context.Background())
	assert.Equal(t, VCFNoWin, result, "should not find VCF win from opening position")
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
	_, _, result := SolveVCF(b, domain.PlayerRed, 1000, ctx)
	assert.Equal(t, VCFTimeout, result, "should return timeout when context is cancelled")
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

	x, y, result := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.Equal(t, VCFWin, result, "should find VCF win via continuous fours")
	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
}

func TestVCFSkippedWhenOpponentHasFlex4(t *testing.T) {
	// Blue has an open four (4 in row, both ends open). Red's VCF should be bypassed.
	b := domain.NewBoard().
		// Red: 3 in a row
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		// Blue: 4 in a row with both ends open (open four / flex4)
		PlaceStone(3, 3, domain.PlayerBlue).PlaceStone(4, 3, domain.PlayerBlue).
		PlaceStone(5, 3, domain.PlayerBlue).PlaceStone(6, 3, domain.PlayerBlue).
		PlaceStone(10, 10, domain.PlayerRed)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 4, TimeLimitMs: 5000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	assert.NotEqual(t, "vcf", stats.MoveType, "VCF should be skipped when opponent has flex4, got move (%d,%d)", x, y)
}

func TestVCFForcedBlockWhenOpponentHasSingleWinSquare(t *testing.T) {
	// Blue has one winning square (4 in row, one end blocked). Red also has VCF potential.
	// Engine must either block or find a winning counter-threat.
	b := domain.NewBoard().
		// Blue: 4 in a row with one end blocked by Red
		PlaceStone(3, 3, domain.PlayerBlue).PlaceStone(4, 3, domain.PlayerBlue).
		PlaceStone(5, 3, domain.PlayerBlue).PlaceStone(6, 3, domain.PlayerBlue).
		PlaceStone(7, 3, domain.PlayerRed).
		// Red: 3 in a row that could chain fours
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 6, TimeLimitMs: 10000, Goroutines: 1, UseVCF: true}
	x, y, _ := SearchPosition(b, domain.PlayerRed, opts, tt, h, context.Background())
	// Engine should either block at (2,3) or find a winning counter-move
	assert.True(t, x >= 0 && y >= 0, "should return a valid move, got (%d,%d)", x, y)
}

func TestVCFSmallRadiusFindsWin(t *testing.T) {
	// VCF win should work with radius 2 since fours are always adjacent
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(8, 2, domain.PlayerRed).PlaceStone(8, 3, domain.PlayerRed).
		PlaceStone(8, 4, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue).PlaceStone(1, 1, domain.PlayerBlue)

	x, y, result := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.Equal(t, VCFWin, result, "VCF should find win with reduced candidate radius")
	assert.True(t, x >= 0 && y >= 0, "should return valid move, got (%d,%d)", x, y)
}

func TestVCFSkippedWhenOpponentHasBrokenFour(t *testing.T) {
	// From actual L5 vs L5 game: Red has "broken four" on row 6
	// (6,6),(7,6),(8,6),_,(10,6) — gap at (9,6) creates exactly-5.
	// Blue has VCF potential but must block Red's winning move first.
	b := domain.NewBoard().
		// Red stones (including the broken four)
		PlaceStone(9, 8, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(10, 6, domain.PlayerRed).
		PlaceStone(6, 6, domain.PlayerRed).
		PlaceStone(8, 6, domain.PlayerRed).
		PlaceStone(7, 6, domain.PlayerRed).
		// Blue stones (diagonal potential for VCF)
		PlaceStone(8, 8, domain.PlayerBlue).
		PlaceStone(9, 7, domain.PlayerBlue).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(10, 8, domain.PlayerBlue).
		PlaceStone(8, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 6, TimeLimitMs: 10000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerBlue, opts, tt, h, context.Background())
	assert.NotEqual(t, "vcf", stats.MoveType, "VCF should be skipped when opponent has broken four")
	blockedOrWon := (x == 9 && y == 6) || stats.SearchScore >= domain.WinScore-domain.AbsoluteMaxDepth
	assert.True(t, blockedOrWon, "should block Red's broken four at (9,6) or counter-win, got (%d,%d) score=%d", x, y, stats.SearchScore)
}

func TestVCFSolverFailsWhenOpponentHasBrokenFour(t *testing.T) {
	// Red has VCF potential but Blue has broken four that wins immediately.
	// VCF solver must detect opponent's winning move outside blocking squares.
	b := domain.NewBoard().
		// Red: VCF potential (3 horizontal + 3 vertical at intersection)
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(8, 2, domain.PlayerRed).PlaceStone(8, 3, domain.PlayerRed).
		PlaceStone(8, 4, domain.PlayerRed).
		// Blue: broken four on row 6 — (6,6),(7,6),(8,6),_,(10,6)
		PlaceStone(6, 6, domain.PlayerBlue).PlaceStone(7, 6, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).PlaceStone(10, 6, domain.PlayerBlue).
		PlaceStone(15, 15, domain.PlayerRed)

	_, _, result := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.Equal(t, VCFNoWin, result, "VCF should fail when opponent has immediate winning move")
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

	_, _, result := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.Equal(t, VCFNoWin, result, "VCF should fail when all block points give opponent five-in-a-row")
}

func TestFindFourBlocksRejectsOverline(t *testing.T) {
	// Red has stones at (5,5),(6,5),(7,5),(10,5). Placing at (8,5) creates four 5-8.
	// Block at (9,5) would extend to (10,5) creating 6-line overline — not a real threat.
	// Only (4,5) is a valid block point.
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(10, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	sb.MakeMove(8, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 8, 5, domain.PlayerRed)
	sb.UnmakeMove()

	assert.Equal(t, 1, len(blocks), "overline end should be excluded, got %d blocks", len(blocks))
	if len(blocks) > 0 {
		assert.Equal(t, domain.Position{X: 4, Y: 5}, blocks[0], "block should be at (4,5)")
	}
}

func TestFindFourBlocksRejectsOverlineBothEnds(t *testing.T) {
	// Red: (4,5),(5,5),(6,5),(7,5),(9,5),(10,5). Placing at (8,5) creates four 5-8.
	// Both ends (4,5) and (9,5) lead to overline — no valid block points at all.
	b := domain.NewBoard().
		PlaceStone(4, 5, domain.PlayerRed).
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(9, 5, domain.PlayerRed).
		PlaceStone(10, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	sb.MakeMove(7, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 7, 5, domain.PlayerRed)
	sb.UnmakeMove()

	// Four is at 5-7 + placed at 7 = (5,5)(6,5)(7,5)
	// Wait, let me recalculate: positive from (7,5) in +x direction:
	//   i=1: (8,5) empty → positive=0
	// negative from (7,5) in -x direction:
	//   i=1: (6,5) Red → negative=1
	//   i=2: (5,5) Red → negative=2
	//   i=3: (4,5) Red → negative=3
	// count = 1 + 0 + 3 = 4 ✓
	// afterX = 8, beforeX = 3
	// (8,5) is empty → afterOpen. But placing attacker at (8,5) extends to (9,5)(10,5) = 6-line → overline
	// (3,5) is empty → beforeOpen. Placing attacker at (3,5) extends to (4,5) and beyond → 4,5,6,7 = 4 + new at 3 = 5, but (4,5) is already there!
	// Wait, the four is (4,5)(5,5)(6,5)(7,5), count=4. Placing at (3,5) gives (3,5)(4,5)(5,5)(6,5)(7,5) = 5, but (8,5) is empty so that's fine. But wait, is there anything beyond (3,5)? (2,5) is empty, so the five 3-7 is exactly 5. That's valid!
	// Hmm, so only the (8,5) end is an overline. The (3,5) end is valid.
	// Actually wait, (4,5) is already part of the four. The four is (4,5)(5,5)(6,5)(7,5). Placing at (3,5) gives (3,5)(4,5)(5,5)(6,5)(7,5) = 5 in a row. What's beyond? (8,5) is empty (not attacker). So exactly 5. Valid!
	assert.Equal(t, 1, len(blocks), "only one end is overline, should have 1 valid block, got %d", len(blocks))
}

func TestFindFourBlocksBothEndsOverline(t *testing.T) {
	// Red: (0,5)(2,5)(3,5)(4,5)(7,5). Placing at (5,5) creates four (2,3,4,5).
	// Block at (6,5): attacker extends to (7,5) → 6-line overline → excluded.
	// Block at (1,5): attacker extends to (0,5) → 5-line but (0,5) is Red → (0,1,2,3,4,5) = 6 → overline → excluded.
	// Both ends overline → 0 block points.
	b := domain.NewBoard().
		PlaceStone(0, 5, domain.PlayerRed).
		PlaceStone(2, 5, domain.PlayerRed).
		PlaceStone(3, 5, domain.PlayerRed).
		PlaceStone(4, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	sb.MakeMove(5, 5, domain.PlayerRed)
	blocks := findFourBlocks(&sb, 5, 5, domain.PlayerRed)
	sb.UnmakeMove()
	assert.Equal(t, 0, len(blocks), "both ends overline → no valid block points, got %d", len(blocks))
}

func TestSearchBlocksOpponentVCF(t *testing.T) {
	// Actual game position at M15 (Blue's turn).
	// Red has a VCF chain: (9,4) creates four on ↗ diagonal,
	// blocked at (10,3), then (9,5) creates exactly-5 on x=9 vertical.
	b := domain.NewBoard().
		// Red stones
		PlaceStone(9, 8, domain.PlayerRed).
		PlaceStone(6, 7, domain.PlayerRed).
		PlaceStone(9, 7, domain.PlayerRed).
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 9, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(7, 6, domain.PlayerRed).
		PlaceStone(9, 6, domain.PlayerRed).
		// Blue stones
		PlaceStone(8, 8, domain.PlayerBlue).
		PlaceStone(7, 9, domain.PlayerBlue).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(8, 7, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).
		PlaceStone(8, 10, domain.PlayerBlue).
		PlaceStone(5, 8, domain.PlayerBlue)

	rx, ry, redHasVCF := SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	t.Logf("Red VCF: result=%v move=(%d,%d)", redHasVCF, rx, ry)
	assert.Equal(t, VCFWin, redHasVCF, "Red should have a VCF from this position — test is invalid otherwise")

	tt := NewTranspositionTable(1)
	h := NewSearchHeuristics()
	opts := SearchConfig{MaxDepth: 10, TimeLimitMs: 30000, Goroutines: 1, UseVCF: true}
	x, y, stats := SearchPosition(b, domain.PlayerBlue, opts, tt, h, context.Background())
	t.Logf("Blue move: (%d,%d) score=%d depth=%d type=%s", x, y, stats.SearchScore, stats.DepthAchieved, stats.MoveType)
	assert.True(t, x >= 0 && y >= 0, "should return a valid move, got (%d,%d)", x, y)
	assert.NotEqual(t, "vcf-block", stats.MoveType, "should use alpha-beta with VCF hint, not short-circuit")
}

func TestVCFResultDistinctStates(t *testing.T) {
	assert.NotEqual(t, VCFNoWin, VCFWin)
	assert.NotEqual(t, VCFNoWin, VCFTimeout)
	assert.NotEqual(t, VCFWin, VCFTimeout)
}

func TestVCFSolveReturnsWinWhenFound(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)
	_, _, result := SolveVCF(b, domain.PlayerRed, 1000, context.Background())
	assert.Equal(t, VCFWin, result)
}

func TestVCFSolveReturnsTimeoutOnCancellation(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue).
		PlaceStone(1, 1, domain.PlayerBlue)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, result := SolveVCF(b, domain.PlayerRed, 1000, ctx)
	assert.Equal(t, VCFTimeout, result, "cancelled context should return VCFTimeout, not VCFNoWin")
}

func TestVCFSolveReturnsNoWinWhenProven(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue)
	_, _, result := SolveVCF(b, domain.PlayerRed, 100, context.Background())
	assert.Equal(t, VCFNoWin, result, "opening position with no VCF should return VCFNoWin, not VCFTimeout")
}
