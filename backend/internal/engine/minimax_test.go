package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMinimaxAIFindsWinningMove(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1)
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	opts := SearchOptions{
		TimeRemainingMs: 5000,
		IncrementMs:     0,
		MoveNumber:      6,
		ThreadCount:     1,
		TimeFraction:    1.0,
	}

	x, y, stats := ai.GetBestMove(b, domain.PlayerRed, opts, context.Background())
	assert.True(t, x == 2 || x == 7, "should find winning move, got (%d,%d)", x, y)
	assert.Equal(t, 5, y)
	assert.Greater(t, stats.NodesSearched, int64(0))

	gotStats := ai.GetStats()
	assert.Equal(t, stats.NodesSearched, gotStats.NodesSearched)
}

func TestMinimaxAIDispose(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 2)
	assert.NotPanics(t, func() { ai.Dispose() })
}

func TestNewMinimaxAIMinThreads(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 0)
	assert.NotNil(t, ai)
	ai.Dispose()

	ai = NewMinimaxAI(slog.Default(), -1)
	assert.NotNil(t, ai)
	ai.Dispose()
}

func TestMinimaxAIWithContextCancel(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1)
	defer ai.Dispose()

	b := domain.NewBoard().
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerBlue)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	x, y, _ := ai.GetBestMove(b, domain.PlayerRed, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
	}, ctx)
	assert.True(t, x >= 0 && y >= 0, "should return valid move even on timeout")
}
