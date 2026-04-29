package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenRuleFirstMove(t *testing.T) {
	b := NewBoard()
	assert.True(t, IsValidSecondMove(b, 5, 5), "first move is always valid")
}

func TestOpenRuleSecondRedMove(t *testing.T) {
	b := NewBoard().PlaceStone(8, 8, PlayerRed)
	assert.False(t, IsValidSecondMove(b, 9, 9), "inside 5x5 zone")
	assert.False(t, IsValidSecondMove(b, 10, 9), "inside 5x5 zone")
	assert.True(t, IsValidSecondMove(b, 11, 8), "outside 5x5 zone")
	assert.True(t, IsValidSecondMove(b, 8, 11), "outside 5x5 zone")
	assert.True(t, IsValidSecondMove(b, 0, 0), "far away, valid")
}

func TestOpenRuleAfterBlueMove(t *testing.T) {
	b := NewBoard().
		PlaceStone(8, 8, PlayerRed).
		PlaceStone(0, 0, PlayerBlue)
	assert.False(t, IsValidSecondMove(b, 9, 9), "inside 5x5 zone even after blue has played")
	assert.False(t, IsValidSecondMove(b, 10, 9), "inside 5x5 zone")
	assert.True(t, IsValidSecondMove(b, 11, 8), "outside 5x5 zone after blue move is valid")
}
