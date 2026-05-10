package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGameModeString(t *testing.T) {
	assert.Equal(t, "pvp", GameModePvP.String())
	assert.Equal(t, "pvai", GameModePvAI.String())
	assert.Equal(t, "aivai", GameModeAivAI.String())
	assert.Equal(t, "pvp", GameMode(99).String())
}

func TestParseGameMode(t *testing.T) {
	assert.Equal(t, GameModePvP, ParseGameMode("pvp"))
	assert.Equal(t, GameModePvAI, ParseGameMode("pvai"))
	assert.Equal(t, GameModeAivAI, ParseGameMode("aivai"))
	assert.Equal(t, GameModePvP, ParseGameMode("unknown"))
	assert.Equal(t, GameModePvP, ParseGameMode(""))
}

func TestCellIsEmpty(t *testing.T) {
	assert.True(t, Cell{X: 0, Y: 0, Player: PlayerNone}.IsEmpty())
	assert.False(t, Cell{X: 0, Y: 0, Player: PlayerRed}.IsEmpty())
	assert.False(t, Cell{X: 0, Y: 0, Player: PlayerBlue}.IsEmpty())
}

func TestBoardIsNotEmpty(t *testing.T) {
	b := NewBoard().PlaceStone(8, 8, PlayerRed)
	assert.False(t, b.IsEmpty())
}
