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

	x, y, stats := ParallelSearch(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 2 || x == 7, "should find winning move, got (%d,%d)", x, y)
	assert.Equal(t, 5, y)
	assert.Greater(t, stats.NodesSearched, int64(0))
	assert.Equal(t, 2, stats.ThreadCount)
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

	x, y, _ := ParallelSearch(b, domain.PlayerBlue, opts, tt, heuristics, context.Background())
	assert.True(t, x >= 0 && x < domain.BoardSize, "x should be valid, got %d", x)
	assert.True(t, y >= 0 && y < domain.BoardSize, "y should be valid, got %d", y)
}

func TestParallelSearchSharesTT(t *testing.T) {
	b := domain.NewBoard()
	b = b.PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue)

	tt := NewTranspositionTable(4)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    3,
		TimeLimitMs: 3000,
		Goroutines:  3,
	}

	_, _, stats := ParallelSearch(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	probes, _ := tt.Stats()
	assert.Greater(t, probes, int64(0), "shared TT should have probes from all workers")
	assert.Equal(t, 3, stats.ThreadCount)
}

func TestParallelSearchVCFFlag(t *testing.T) {
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
		UseVCF:      true,
	}

	_, _, stats := ParallelSearch(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.Equal(t, "vcf", stats.MoveType, "should detect VCF win")
}
