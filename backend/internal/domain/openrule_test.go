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
	assert.False(t, IsValidSecondMove(b, 9, 9), "too close to first red move")
	assert.True(t, IsValidSecondMove(b, 10, 9), "distance 3, valid")
	assert.True(t, IsValidSecondMove(b, 11, 8), "distance 3, valid")
	assert.True(t, IsValidSecondMove(b, 0, 0), "far away, valid")
}

func TestOpenRuleAfterBlueMove(t *testing.T) {
	b := NewBoard().
		PlaceStone(8, 8, PlayerRed).
		PlaceStone(0, 0, PlayerBlue)
	assert.True(t, IsValidSecondMove(b, 9, 9), "open rule only applies to red's second move")
}
