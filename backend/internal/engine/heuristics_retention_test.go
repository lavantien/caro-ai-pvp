package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeuristicsAgeForNewMoveHalvesTables(t *testing.T) {
	h := NewSearchHeuristics()
	h.RecordHistory(domain.PlayerRed, 7, 7, 10) // 100
	h.RecordHistory(domain.PlayerBlue, 8, 8, 4) // 16
	h.RecordContHistory(domain.PlayerRed, 5, 5, 6, 6, 10)
	h.RecordKiller(4, domain.Position{X: 3, Y: 3})
	h.RecordCounterMove(domain.PlayerRed, 5, 5, 6, 6)

	h.AgeForNewMove()

	assert.Equal(t, 50, h.HistoryScore(domain.PlayerRed, 7, 7), "history halves between moves")
	assert.Equal(t, 8, h.HistoryScore(domain.PlayerBlue, 8, 8), "history halves between moves")
	assert.Equal(t, 150, h.ContHistoryScore(domain.PlayerRed, 5, 5, 6, 6), "continuation history (depth 10 -> bonus 300) halves between moves")
	assert.True(t, h.IsKiller(4, domain.Position{X: 3, Y: 3}), "killers survive aging; slots overwrite naturally")
	cm := h.CounterMoveFor(domain.PlayerRed, 5, 5)
	assert.Equal(t, 6, cm.X, "counter moves survive aging")
}

func TestGetBestMoveAgesHeuristicsInsteadOfClearing(t *testing.T) {
	ai := NewMinimaxAI(nil, 1, 1)
	defer ai.Dispose()
	ai.heuristics.RecordHistory(domain.PlayerRed, 7, 7, 10)
	seeded := ai.heuristics.HistoryScore(domain.PlayerRed, 7, 7)

	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue)
	opts := SearchOptions{TimeRemainingMs: 0, MoveNumber: 1, ThreadCount: 1, TimeFraction: 1.0}
	ai.GetBestMove(b, domain.PlayerRed, opts, context.Background())

	after := ai.heuristics.HistoryScore(domain.PlayerRed, 7, 7)
	assert.GreaterOrEqual(t, after, seeded/2,
		"game-level ordering knowledge must carry to the next move (aged), not be wiped")
	assert.LessOrEqual(t, after, seeded,
		"aging halves the table; only search cutoffs can grow it")
}

func TestParallelSearchPreservesHeuristics(t *testing.T) {
	shared := NewSearchHeuristics()
	shared.RecordHistory(domain.PlayerRed, 7, 7, 10)
	seeded := shared.HistoryScore(domain.PlayerRed, 7, 7)

	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue)
	tt := NewTranspositionTable(1)
	opts := SearchConfig{MaxDepth: 6, TimeLimitMs: 300, SoftLimitMs: 250, Goroutines: 4}

	ParallelSearch(b, domain.PlayerRed, opts, tt, shared, context.Background())

	assert.GreaterOrEqual(t, shared.HistoryScore(domain.PlayerRed, 7, 7), seeded,
		"the shared heuristics must survive a parallel search (worker 0 evolves it, never wipes it)")
}
