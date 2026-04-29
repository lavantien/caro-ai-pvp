package engine

import (
	"testing"

	"caro-ai-pvp/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestKillerMoves(t *testing.T) {
	h := NewSearchHeuristics()
	pos := domain.Position{X: 5, Y: 5}
	h.RecordKiller(3, pos)
	assert.True(t, h.IsKiller(3, pos))
	assert.False(t, h.IsKiller(2, pos))
}

func TestKillerMovesDisplaces(t *testing.T) {
	h := NewSearchHeuristics()
	pos1 := domain.Position{X: 3, Y: 3}
	pos2 := domain.Position{X: 7, Y: 7}
	h.RecordKiller(0, pos1)
	h.RecordKiller(0, pos2)
	assert.True(t, h.IsKiller(0, pos1), "old killer should be in slot 1")
	assert.True(t, h.IsKiller(0, pos2), "new killer should be in slot 0")
}

func TestKillerScore(t *testing.T) {
	h := NewSearchHeuristics()
	pos := domain.Position{X: 4, Y: 4}
	assert.Equal(t, 0, h.KillerScore(0, pos))
	h.RecordKiller(0, pos)
	assert.Equal(t, 500000, h.KillerScore(0, pos))

	other := domain.Position{X: 3, Y: 3}
	h.RecordKiller(0, other)
	assert.Equal(t, 400000, h.KillerScore(0, pos))
}

func TestHistoryScore(t *testing.T) {
	h := NewSearchHeuristics()
	h.RecordHistory(domain.PlayerRed, 5, 5, 4)
	assert.Greater(t, h.HistoryScore(domain.PlayerRed, 5, 5), 0)
	assert.Equal(t, 0, h.HistoryScore(domain.PlayerBlue, 5, 5))
}

func TestHistoryClamp(t *testing.T) {
	h := NewSearchHeuristics()
	for range 2000 {
		h.RecordHistory(domain.PlayerRed, 0, 0, 64)
	}
	assert.LessOrEqual(t, h.HistoryScore(domain.PlayerRed, 0, 0), 1_000_000)
}

func TestHeuristicsClear(t *testing.T) {
	h := NewSearchHeuristics()
	h.RecordKiller(0, domain.Position{X: 1, Y: 1})
	h.RecordHistory(domain.PlayerRed, 5, 5, 10)
	h.Clear()
	assert.False(t, h.IsKiller(0, domain.Position{X: 1, Y: 1}))
	assert.Equal(t, 0, h.HistoryScore(domain.PlayerRed, 5, 5))
}
