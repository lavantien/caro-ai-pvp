package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchFindsWinningMove(t *testing.T) {
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
		Goroutines:  1,
	}

	x, y := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 2 || x == 7, "should find winning move at end of line, got (%d,%d)", x, y)
	assert.Equal(t, 5, y)
}

func TestSearchFindsBlockingMove(t *testing.T) {
	// Blue has 4 in a row with one end blocked by Red
	b := domain.NewBoard()
	b = b.PlaceStone(2, 5, domain.PlayerRed) // block one end
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerBlue)
	}
	b = b.PlaceStone(0, 0, domain.PlayerRed)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    4,
		TimeLimitMs: 5000,
		Goroutines:  1,
	}

	x, y := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.True(t, x == 7 && y == 5,
		"should block opponent's four at (7,5), got (%d,%d)", x, y)
}
