package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlayerOpponent(t *testing.T) {
	tests := []struct {
		name     string
		player   Player
		expected Player
	}{
		{"red opponent", PlayerRed, PlayerBlue},
		{"blue opponent", PlayerBlue, PlayerRed},
		{"none opponent", PlayerNone, PlayerNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.player.Opponent())
		})
	}
}

func TestPlayerIsValid(t *testing.T) {
	assert.True(t, PlayerRed.IsValid())
	assert.True(t, PlayerBlue.IsValid())
	assert.False(t, PlayerNone.IsValid())
}

func TestPlayerString(t *testing.T) {
	assert.Equal(t, "red", PlayerRed.String())
	assert.Equal(t, "blue", PlayerBlue.String())
	assert.Equal(t, "none", PlayerNone.String())
}

func TestParsePlayer(t *testing.T) {
	p, ok := ParsePlayer("red")
	assert.True(t, ok)
	assert.Equal(t, PlayerRed, p)

	p, ok = ParsePlayer("blue")
	assert.True(t, ok)
	assert.Equal(t, PlayerBlue, p)

	p, ok = ParsePlayer("none")
	assert.True(t, ok)
	assert.Equal(t, PlayerNone, p)

	_, ok = ParsePlayer("invalid")
	assert.False(t, ok)
}
