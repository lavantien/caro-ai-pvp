package api

import (
	"caro-ai-pvp/internal/domain"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSession() *GameSession {
	return NewGameSession(
		"rapid",
		300000,
		2,
		domain.GameModeAivAI,
		intPtr(3),
		nil,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
		func() int { return 1 },
	)
}

func intPtr(v int) *int { return &v }

func TestNewGameSessionInitialState(t *testing.T) {
	s := newTestSession()
	resp := s.GetResponse()

	assert.Equal(t, "none", resp.Winner)
	assert.False(t, resp.IsGameOver)
	assert.Equal(t, 0, resp.MoveNumber)
	assert.Equal(t, "red", resp.CurrentPlayer)
	assert.Equal(t, "rapid", resp.TimeControl)
	assert.Equal(t, 300, resp.InitialTime)
	assert.Equal(t, 2, resp.Increment)
	assert.Equal(t, "aivai", resp.GameMode)
	assert.Equal(t, 300.0, resp.RedTimeRemaining)
	assert.Equal(t, 300.0, resp.BlueTimeRemaining)
	assert.Equal(t, 3, *resp.RedDifficulty)
	assert.Nil(t, resp.BlueDifficulty)
}

func TestSessionApplyMove(t *testing.T) {
	s := newTestSession()
	resp, err := s.ApplyMove(7, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.MoveNumber)
	assert.Equal(t, "blue", resp.CurrentPlayer)
}

func TestSessionApplyMoveOutOfBounds(t *testing.T) {
	s := newTestSession()
	_, err := s.ApplyMove(99, 99)
	assert.ErrorIs(t, err, domain.ErrPositionBounds)
}

func TestSessionApplyMoveAfterGameOver(t *testing.T) {
	s := newTestSession()
	// Red(0,0), Blue(0,2), Red(3,0) [Open Rule: dist=3], Blue(1,2),
	// Red(1,0), Blue(2,2), Red(4,0), Blue(3,2), Red(2,0) -> wins
	moves := []struct{ x, y int }{
		{0, 0}, {0, 2}, // R, B
		{3, 0}, {1, 2}, // R(dist=3), B
		{1, 0}, {2, 2}, // R, B
		{4, 0}, {3, 2}, // R, B
		{2, 0},         // R wins: 0,1,2,3,4 at y=0
	}
	for _, m := range moves {
		_, err := s.ApplyMove(m.x, m.y)
		require.NoError(t, err)
	}
	assert.True(t, s.IsGameOver())
	_, err := s.ApplyMove(5, 5)
	assert.ErrorIs(t, err, domain.ErrGameOver)
}

func TestSessionExtractForAI(t *testing.T) {
	s := newTestSession()
	board, player, isOver, timeMs, inc, moveNum, diff := s.ExtractForAI()
	assert.Equal(t, domain.PlayerRed, player)
	assert.False(t, isOver)
	assert.Equal(t, int64(300000), timeMs)
	assert.Equal(t, 2, inc)
	assert.Equal(t, 0, moveNum)
	assert.NotNil(t, diff)
	assert.Equal(t, 3, *diff)
	_ = board
}

func TestSessionExtractForAIBlue(t *testing.T) {
	s := newTestSession()
	_, err := s.ApplyMove(7, 7)
	require.NoError(t, err)
	_, player, _, timeMs, _, _, diff := s.ExtractForAI()
	assert.Equal(t, domain.PlayerBlue, player)
	assert.Equal(t, int64(300000), timeMs)
	assert.Nil(t, diff)
}

func TestSessionGetOrCreateAI(t *testing.T) {
	s := newTestSession()
	ai := s.GetOrCreateAI(domain.PlayerRed)
	assert.NotNil(t, ai)
	ai2 := s.GetOrCreateAI(domain.PlayerRed)
	assert.Same(t, ai, ai2)
	ai3 := s.GetOrCreateAI(domain.PlayerBlue)
	assert.NotNil(t, ai3)
}

func TestSessionDisposeAI(t *testing.T) {
	s := newTestSession()
	_ = s.GetOrCreateAI(domain.PlayerRed)
	_ = s.GetOrCreateAI(domain.PlayerBlue)
	s.DisposeAI()
	ai := s.GetOrCreateAI(domain.PlayerRed)
	assert.NotNil(t, ai)
}

func TestSessionDynamicThreadCount(t *testing.T) {
	var count atomic.Int32
	count.Store(5)
	s := NewGameSession(
		"rapid", 300000, 2,
		domain.GameModeAivAI, intPtr(3), nil,
		slog.New(slog.NewTextHandler(os.Stderr, nil)),
		func() int { return int(count.Load()) },
	)
	_ = s.GetOrCreateAI(domain.PlayerRed)
}

func TestSessionUndoMove(t *testing.T) {
	s := newTestSession()
	_, err := s.ApplyMove(7, 7)
	require.NoError(t, err)
	assert.Equal(t, 1, s.GetResponse().MoveNumber)

	resp, err := s.UndoLastMove()
	require.NoError(t, err)
	assert.Equal(t, 0, resp.MoveNumber)
	assert.Equal(t, "red", resp.CurrentPlayer)
}
