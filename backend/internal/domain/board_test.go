package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoardIsEmpty(t *testing.T) {
	b := NewBoard()
	assert.True(t, b.IsEmpty())
	for x := range BoardSize {
		for y := range BoardSize {
			assert.Equal(t, PlayerNone, b.GetCell(x, y).Player)
		}
	}
	assert.Equal(t, uint64(0), b.Hash())
}

func TestBoardPlaceStoneImmutable(t *testing.T) {
	original := NewBoard()
	placed := original.PlaceStone(8, 8, PlayerRed)

	assert.Equal(t, PlayerNone, original.GetCell(8, 8).Player)
	assert.Equal(t, PlayerRed, placed.GetCell(8, 8).Player)
	assert.NotEqual(t, original.Hash(), placed.Hash())
}

func TestBoardPlaceStoneMultiple(t *testing.T) {
	b := NewBoard().
		PlaceStone(8, 8, PlayerRed).
		PlaceStone(7, 7, PlayerBlue).
		PlaceStone(9, 9, PlayerRed)

	assert.Equal(t, PlayerRed, b.GetCell(8, 8).Player)
	assert.Equal(t, PlayerBlue, b.GetCell(7, 7).Player)
	assert.Equal(t, PlayerRed, b.GetCell(9, 9).Player)
}

func TestBoardPlaceStoneOccupied(t *testing.T) {
	b := NewBoard().PlaceStone(8, 8, PlayerRed)
	_, err := b.PlaceStoneChecked(8, 8, PlayerBlue)
	assert.ErrorIs(t, err, ErrCellOccupied)
}

func TestBoardPlaceStoneOutOfBounds(t *testing.T) {
	b := NewBoard()
	_, err := b.PlaceStoneChecked(-1, 0, PlayerRed)
	assert.ErrorIs(t, err, ErrPositionBounds)

	_, err = b.PlaceStoneChecked(16, 0, PlayerRed)
	assert.ErrorIs(t, err, ErrPositionBounds)
}

func TestBoardBitBoardBits(t *testing.T) {
	b := NewBoard().PlaceStone(0, 0, PlayerRed)
	redBits := b.BitBoardBits(PlayerRed)
	assert.NotZero(t, redBits[0])

	blueBits := b.BitBoardBits(PlayerBlue)
	assert.Zero(t, blueBits[0])
}

func TestBoardHashIncremental(t *testing.T) {
	b1 := NewBoard().PlaceStone(5, 5, PlayerRed)
	expectedHash := uint64(0) ^ ZobristKey(5, 5, PlayerRed)
	assert.Equal(t, expectedHash, b1.Hash())

	b2 := b1.PlaceStone(6, 6, PlayerBlue)
	expectedHash2 := expectedHash ^ ZobristKey(6, 6, PlayerBlue)
	assert.Equal(t, expectedHash2, b2.Hash())
}

func TestBoardIsEmptyAt(t *testing.T) {
	b := NewBoard()
	assert.True(t, b.IsEmptyAt(8, 8))
	assert.False(t, b.IsEmptyAt(-1, 0))

	placed := b.PlaceStone(8, 8, PlayerRed)
	assert.False(t, placed.IsEmptyAt(8, 8))
}

func TestBoardGetPlayerAt(t *testing.T) {
	b := NewBoard().PlaceStone(3, 4, PlayerBlue)
	assert.Equal(t, PlayerBlue, b.GetPlayerAt(3, 4))
	assert.Equal(t, PlayerNone, b.GetPlayerAt(5, 5))
	assert.Equal(t, PlayerNone, b.GetPlayerAt(-1, 0))
}

func TestBoardBitBoardOps(t *testing.T) {
	b := NewBoard()
	for x := range 4 {
		b = b.PlaceStone(x, 0, PlayerRed)
	}
	redBits := b.BitBoardBits(PlayerRed)
	assert.Equal(t, uint64(0x0F), redBits[0])
}

func TestBoardPlaceStoneCheckedRequiresValid(t *testing.T) {
	require := require.New(t)
	b := NewBoard().PlaceStone(5, 5, PlayerRed)

	_, err := b.PlaceStoneChecked(5, 5, PlayerBlue)
	require.ErrorIs(err, ErrCellOccupied)

	_, err = b.PlaceStoneChecked(-1, 5, PlayerRed)
	require.ErrorIs(err, ErrPositionBounds)
}

func TestBoardGetCellOutOfBounds(t *testing.T) {
	b := NewBoard()
	cell := b.GetCell(-1, 0)
	assert.Equal(t, PlayerNone, cell.Player)
	cell = b.GetCell(0, -1)
	assert.Equal(t, PlayerNone, cell.Player)
	cell = b.GetCell(BoardSize, 0)
	assert.Equal(t, PlayerNone, cell.Player)
}

func TestBoardPlaceStonePanicsOnOccupied(t *testing.T) {
	b := NewBoard().PlaceStone(5, 5, PlayerRed)
	assert.Panics(t, func() {
		b.PlaceStone(5, 5, PlayerBlue)
	})
}

func TestBoardPlaceStonePanicsOutOfBounds(t *testing.T) {
	b := NewBoard()
	assert.Panics(t, func() {
		b.PlaceStone(-1, 0, PlayerRed)
	})
}

func TestBoardIsEmptyWithStones(t *testing.T) {
	b := NewBoard().PlaceStone(0, 0, PlayerRed)
	assert.False(t, b.IsEmpty())
}
