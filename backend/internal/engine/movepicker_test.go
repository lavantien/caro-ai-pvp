package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderMovesTTFirst(t *testing.T) {
	b := domain.NewBoard().PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	h := NewSearchHeuristics()

	candidates := []domain.Position{{X: 7, Y: 7}, {X: 9, Y: 9}, {X: 6, Y: 6}}
	ttMove := domain.Position{X: 9, Y: 9}

	ordered := OrderMoves(candidates, &sb, domain.PlayerBlue, 0, &ttMove, h)
	assert.Equal(t, domain.Position{X: 9, Y: 9}, ordered[0], "TT move should be first")
}

func TestOrderMovesWinningMove(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	sb := NewSearchBoard(b)
	h := NewSearchHeuristics()

	candidates := GetCandidates(&sb, 2)
	ordered := OrderMoves(candidates, &sb, domain.PlayerRed, 0, nil, h)

	top := ordered[0]
	assert.True(t, (top.X == 2 || top.X == 7) && top.Y == 5,
		"winning move should be (2,5) or (7,5), got (%d,%d)", top.X, top.Y)
}

func TestMovePickerWinningBeforeBlocking(t *testing.T) {
	// Red can win immediately (4 in a row, needs 5th).
	// Blue also has an open four threat.
	// Winning move must be yielded before blocking moves.
	b := domain.NewBoard().
		// Red: 4 in a row, need 5th at (7,5)
		PlaceStone(3, 5, domain.PlayerRed).PlaceStone(4, 5, domain.PlayerRed).
		PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
		// Blue: 4 in a row with both ends open
		PlaceStone(3, 3, domain.PlayerBlue).PlaceStone(4, 3, domain.PlayerBlue).
		PlaceStone(5, 3, domain.PlayerBlue).PlaceStone(6, 3, domain.PlayerBlue).
		PlaceStone(10, 10, domain.PlayerRed)

	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)
	picker := NewMovePicker(candidates, &sb, domain.PlayerRed, 4, nil, NewSearchHeuristics(), domain.Position{X: -1, Y: -1})

	first, ok := picker.Next()
	assert.True(t, ok, "should yield at least one move")
	// The first non-TT-move should be the winning move at (7,5) or (2,5)
	assert.True(t, (first.X == 7 || first.X == 2) && first.Y == 5,
		"winning move should be yielded before blocking moves, got (%d,%d)", first.X, first.Y)
}

func TestOrderMovesBlocksThreat(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerBlue)
	}
	b = b.PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	h := NewSearchHeuristics()

	candidates := GetCandidates(&sb, 2)
	ordered := OrderMoves(candidates, &sb, domain.PlayerRed, 0, nil, h)

	assert.True(t, len(ordered) > 0)
	top := ordered[0]
	assert.True(t, top.X == 2 || top.X == 7,
		"should block opponent four-in-a-row at (2,5) or (7,5), got (%d,%d)", top.X, top.Y)
}
