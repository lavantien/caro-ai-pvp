package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"testing"

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
