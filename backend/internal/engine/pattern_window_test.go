package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// splitFourBoard: red has 5,6 gap 8 on row 5. Playing (7,5) fills the gap into
// a five; playing (9,5) or (4,5) extends into a split four with the gap as its
// only completion.
func splitThreeBoard() domain.Board {
	return domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue).
		PlaceStone(0, 1, domain.PlayerBlue)
}

func TestGapFillIsWinningCompletion(t *testing.T) {
	// Red holds a split four: 5,6 gap 8,9 on row 5. Filling (7,5) makes
	// an exact five (ends at 4 and 10 open).
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(9, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	sb.MakeMove(7, 5, domain.PlayerRed)
	assert.True(t, wouldWin(&sb, 7, 5, domain.PlayerRed),
		"filling the gap of XX.XX must make an exact five")
	sb.UnmakeMove()
}

func TestSplitFourPlacementIsFour(t *testing.T) {
	b := splitThreeBoard()
	sb := NewSearchBoard(b)

	assert.True(t, createsFourType(&sb, 9, 5, domain.PlayerRed),
		"playing (9,5) leaves a split four whose gap fill wins: a four")
	assert.True(t, createsFourType(&sb, 4, 5, domain.PlayerRed),
		"playing (4,5) leaves a split four on the other side: a four")
}

func TestBrokenThreePlacementIsFlex3(t *testing.T) {
	// Only 5,6 on the row: playing (8,5) makes a broken three that can
	// become a flex four by filling (7,5).
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	assert.True(t, createsOpenThree(&sb, 8, 5, domain.PlayerRed),
		"a broken three (can reach a two-completion four) must classify as flex three")
}

func TestEvalValuesSplitFour(t *testing.T) {
	// Red already holds a split four on the board (5,6 gap 8,9 on row 5).
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(9, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	pp := ClassifyBoard(&sb, domain.PlayerRed)
	assert.GreaterOrEqual(t, pp.Block4Count+pp.Flex4Count, 1,
		"a split four on the board must count as a four-class pattern")

	score := Evaluate(&sb, domain.PlayerRed)
	assert.GreaterOrEqual(t, score, 4000,
		"static eval must value a split four like a simple four")
}

func TestVCFWinsViaSplitFour(t *testing.T) {
	// Row 5: red 4,5 gap 7 (fill 7 after extending to 8 makes five 4-8).
	// Row 6: red 4,5,6 (extending to 7 makes a two-completion four).
	// Forced win: red plays (8,5) (split four, blue must block (6,5)),
	// then red plays (7,6) making .XXXX. on row 6, unstoppable.
	b := domain.NewBoard().
		PlaceStone(4, 5, domain.PlayerRed).
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(4, 6, domain.PlayerRed).
		PlaceStone(5, 6, domain.PlayerRed).
		PlaceStone(6, 6, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue).
		PlaceStone(0, 1, domain.PlayerBlue)

	x, y, result := SolveVCF(b, domain.PlayerRed, 2000, context.Background())
	assert.Equal(t, VCFWin, result,
		"VCF must find forced wins built on split-four threats")
	assert.True(t, x >= 0 && y >= 0)
}

func TestPlacementPatternHelpersAgreeWithClasses(t *testing.T) {
	b := domain.NewBoard()
	// Straight open four placement: 5,6,7 on row 5, playing 8 with both
	// ends open gives two completions.
	for x := 5; x <= 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	assert.True(t, createsOpenFour(&sb, 8, 5, domain.PlayerRed),
		"straight open four must still classify as flex four")
	assert.False(t, createsOpenFour(&sb, 8, 5, domain.PlayerBlue))
}
