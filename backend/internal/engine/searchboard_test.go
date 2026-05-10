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

func TestSearchBoardPlayerAtBounds(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(-1, 0))
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(0, -1))
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(domain.BoardSize, 0))
}

func TestSearchBoardIsEmptyBounds(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	assert.False(t, sb.IsEmpty(-1, 0))
	assert.False(t, sb.IsEmpty(0, -1))
	assert.False(t, sb.IsEmpty(domain.BoardSize, 0))
	assert.True(t, sb.IsEmpty(7, 7))
}

func TestSearchBoardNullMove(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	hashBefore := sb.Hash()
	sb.MakeNullMove()
	assert.Equal(t, domain.PlayerNone, sb.PlayerAt(-1, -1))
	sb.UnmakeNullMove()
	assert.Equal(t, hashBefore, sb.Hash())
}

func TestSearchBoardBitBoardFor(t *testing.T) {
	b := domain.NewBoard().PlaceStone(5, 5, domain.PlayerRed)
	sb := NewSearchBoard(b)
	assert.False(t, sb.BitBoardFor(domain.PlayerRed).IsZero())
	assert.True(t, sb.BitBoardFor(domain.PlayerBlue).IsZero())
}

func TestSearchBoardOccupied(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	assert.True(t, sb.Occupied().IsZero())
	sb.MakeMove(5, 5, domain.PlayerRed)
	assert.False(t, sb.Occupied().IsZero())
}
