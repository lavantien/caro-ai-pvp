package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParallelSearchFindsWinningMove(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    4,
		TimeLimitMs: 5000,
		Goroutines:  2,
	}

	x, y := ParallelSearch(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 2 || x == 7, "should find winning move, got (%d,%d)", x, y)
	assert.Equal(t, 5, y)
}

func TestParallelSearchFallsBackToSingleThread(t *testing.T) {
	b := domain.NewBoard()
	b = b.PlaceStone(8, 8, domain.PlayerRed)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    2,
		TimeLimitMs: 1000,
		Goroutines:  1,
	}

	x, y := ParallelSearch(b, domain.PlayerBlue, opts, tt, heuristics, context.Background())
	assert.True(t, x >= 0 && x < domain.BoardSize, "x should be valid, got %d", x)
	assert.True(t, y >= 0 && y < domain.BoardSize, "y should be valid, got %d", y)
}
