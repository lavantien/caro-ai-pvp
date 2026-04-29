package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchBoardMakeUnmake(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)

	hashBefore := sb.Hash()
	sb.MakeMove(8, 8, domain.PlayerRed)
	assert.Equal(t, domain.PlayerRed, sb.PlayerAt(8, 8))
	assert.NotEqual(t, hashBefore, sb.Hash())

	sb.UnmakeMove()
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(8, 8))
	assert.Equal(t, hashBefore, sb.Hash())
}

func TestSearchBoardMultipleMoves(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)

	sb.MakeMove(8, 8, domain.PlayerRed)
	sb.MakeMove(7, 7, domain.PlayerBlue)
	sb.MakeMove(9, 9, domain.PlayerRed)

	assert.Equal(t, domain.PlayerRed, sb.PlayerAt(8, 8))
	assert.Equal(t, domain.PlayerBlue, sb.PlayerAt(7, 7))
	assert.Equal(t, domain.PlayerRed, sb.PlayerAt(9, 9))

	sb.UnmakeMove()
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(9, 9))
	assert.Equal(t, domain.PlayerBlue, sb.PlayerAt(7, 7))
}

func TestSearchBoardFromDomain(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 6, domain.PlayerBlue)

	sb := NewSearchBoard(b)
	assert.Equal(t, domain.PlayerRed, sb.PlayerAt(5, 5))
	assert.Equal(t, domain.PlayerBlue, sb.PlayerAt(6, 6))
	assert.Equal(t, b.Hash(), sb.Hash())
}
