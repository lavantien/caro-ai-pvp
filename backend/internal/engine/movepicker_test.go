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
