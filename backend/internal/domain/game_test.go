package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGameState(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	assert.Equal(t, PlayerRed, g.CurrentPlayer)
	assert.Equal(t, 0, g.MoveNumber)
	assert.False(t, g.IsGameOver)
	assert.Equal(t, PlayerNone, g.Winner)
	assert.Equal(t, GameModePvP, g.GameMode)
	assert.True(t, g.Board.IsEmpty())
}

func TestGameStateWithMove(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	g2, err := g.WithMove(8, 8)
	require.NoError(t, err)

	assert.Equal(t, PlayerBlue, g2.CurrentPlayer)
	assert.Equal(t, 1, g2.MoveNumber)
	assert.Equal(t, PlayerRed, g2.Board.GetPlayerAt(8, 8))

	assert.Equal(t, 0, g.MoveNumber)
	assert.True(t, g.Board.IsEmpty())
}

func TestGameStateWithMoveGameOver(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	g2 := g.WithGameOver(PlayerRed, nil)
	_, err := g2.WithMove(5, 5)
	assert.ErrorIs(t, err, ErrGameOver)
}

func TestGameStateUndoMove(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	g2, _ := g.WithMove(8, 8)
	g3, err := g2.UndoMove()
	require.NoError(t, err)

	assert.Equal(t, 0, g3.MoveNumber)
	assert.Equal(t, PlayerRed, g3.CurrentPlayer)
	assert.True(t, g3.Board.IsEmpty())
}

func TestGameStateUndoNoMoves(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	_, err := g.UndoMove()
	assert.ErrorIs(t, err, ErrNoMoves)
}

func TestGameStateWithGameOver(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	line := []Position{{X: 3, Y: 5}, {X: 4, Y: 5}, {X: 5, Y: 5}, {X: 6, Y: 5}, {X: 7, Y: 5}}
	g2 := g.WithGameOver(PlayerRed, line)

	assert.True(t, g2.IsGameOver)
	assert.Equal(t, PlayerRed, g2.Winner)
	assert.Equal(t, 5, len(g2.WinningLine))
	assert.Equal(t, PlayerNone, g2.CurrentPlayer)
}

func TestGameStateCanUndo(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	assert.False(t, g.CanUndo())

	g2, _ := g.WithMove(8, 8)
	assert.True(t, g2.CanUndo())

	g3 := g2.WithGameOver(PlayerRed, nil)
	assert.False(t, g3.CanUndo())
}

func TestGameStateOpenRuleViolation(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	// Red's first move
	g2, err := g.WithMove(8, 8)
	require.NoError(t, err)
	// Blue's first move
	g3, err := g2.WithMove(0, 0)
	require.NoError(t, err)
	// Red's second move inside 5x5 zone (Chebyshev distance 2)
	_, err = g3.WithMove(10, 9)
	assert.ErrorIs(t, err, ErrOpenRule)
}

func TestGameStateOpenRuleValid(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	// Red's first move
	g2, err := g.WithMove(8, 8)
	require.NoError(t, err)
	// Blue's first move
	g3, err := g2.WithMove(0, 0)
	require.NoError(t, err)
	// Red's second move outside 5x5 zone (Chebyshev distance 3)
	g4, err := g3.WithMove(11, 8)
	require.NoError(t, err)
	assert.Equal(t, PlayerBlue, g4.CurrentPlayer)
}

func TestGameStateOpenRuleNotAppliedAfterMoreMoves(t *testing.T) {
	g := NewGameState(GameModePvP, "7+5", 420000, 5)
	// Red move 1
	g2, _ := g.WithMove(8, 8)
	// Blue move 1
	g3, _ := g2.WithMove(0, 0)
	// Red move 2 (outside 5x5 zone)
	g4, _ := g3.WithMove(11, 8)
	// Blue move 2 - close to red move 2 is fine
	_, err := g4.WithMove(11, 9)
	assert.NoError(t, err)
}
