package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWinDetectorEmpty(t *testing.T) {
	b := NewBoard()
	result := CheckWin(b)
	assert.False(t, result.HasWinner)
}

func TestWinDetectorFiveInRowHorizontal(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 8; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	result := CheckWin(b)
	assert.True(t, result.HasWinner)
	assert.Equal(t, PlayerRed, result.Winner)
	assert.Equal(t, 5, len(result.WinningLine))
}

func TestWinDetectorFiveInRowVertical(t *testing.T) {
	b := NewBoard()
	for y := 0; y < 5; y++ {
		b = b.PlaceStone(5, y, PlayerBlue)
	}
	result := CheckWin(b)
	assert.True(t, result.HasWinner)
	assert.Equal(t, PlayerBlue, result.Winner)
}

func TestWinDetectorFiveInRowDiagonal(t *testing.T) {
	b := NewBoard()
	for i := range 5 {
		b = b.PlaceStone(3+i, 3+i, PlayerRed)
	}
	result := CheckWin(b)
	assert.True(t, result.HasWinner)
	assert.Equal(t, PlayerRed, result.Winner)
}

func TestWinDetectorSixNotWin(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 9; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	result := CheckWin(b)
	assert.False(t, result.HasWinner, "6 in a row should not win in Caro (overline)")
}

func TestWinDetectorBlockedEnds(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 8; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	b = b.PlaceStone(2, 5, PlayerBlue)
	b = b.PlaceStone(8, 5, PlayerBlue)
	result := CheckWin(b)
	assert.False(t, result.HasWinner, "blocked five should not win in Caro")
}

func TestWinDetectorOpenEnd(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 8; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	result := CheckWin(b)
	assert.True(t, result.HasWinner, "open five should win")
}

func TestWinDetectorOneBlockedEnd(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 8; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	b = b.PlaceStone(2, 5, PlayerBlue)
	result := CheckWin(b)
	assert.True(t, result.HasWinner, "one blocked end still wins")
}

func TestWinDetectorFromMove(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	b = b.PlaceStone(7, 5, PlayerRed)
	result := CheckWinFromMove(b, 7, 5)
	assert.True(t, result.HasWinner)
	assert.Equal(t, PlayerRed, result.Winner)
}

func TestWinDetectorFromMoveEmpty(t *testing.T) {
	b := NewBoard()
	result := CheckWinFromMove(b, 5, 5)
	assert.False(t, result.HasWinner)
}

func TestWinDetectorFourNotWin(t *testing.T) {
	b := NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, PlayerRed)
	}
	result := CheckWin(b)
	assert.False(t, result.HasWinner, "4 in a row should not win")
}

func TestWinDetectorAntiDiagonal(t *testing.T) {
	b := NewBoard()
	for i := range 5 {
		b = b.PlaceStone(3+i, 7-i, PlayerBlue)
	}
	result := CheckWin(b)
	assert.True(t, result.HasWinner)
	assert.Equal(t, PlayerBlue, result.Winner)
}
