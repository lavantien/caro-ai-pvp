package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredictReplyAfterSearch(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	b := domain.NewBoard().
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerBlue)

	x, y, _ := ai.GetBestMove(b, domain.PlayerRed, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
		MaxDepth:        4,
	}, context.Background())
	require.GreaterOrEqual(t, x, 0)
	require.GreaterOrEqual(t, y, 0)

	child, err := b.PlaceStoneChecked(x, y, domain.PlayerRed)
	require.NoError(t, err)

	reply, ok := ai.PredictReply(child)
	assert.True(t, ok, "child of the searched root should have a TT entry")
	assert.True(t, reply.IsValid())
	assert.True(t, child.IsEmptyAt(reply.X, reply.Y))
}

func TestPredictReplyEmptyTT(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	b := domain.NewBoard().PlaceStone(7, 7, domain.PlayerRed)
	_, ok := ai.PredictReply(b)
	assert.False(t, ok, "no search ran, no prediction should be made")

	assert.False(t, func() bool {
		_, ok := ai.PredictReply(domain.NewBoard())
		return ok
	}(), "zero hash must not false-positive on a fresh table")
}

func TestPredictReplyRejectsOccupiedCell(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	b := domain.NewBoard().PlaceStone(7, 7, domain.PlayerRed)
	ai.tt.Store(TTEntry{Hash: b.Hash(), Depth: 3, MoveX: 7, MoveY: 7})

	_, ok := ai.PredictReply(b)
	assert.False(t, ok, "an entry pointing at an occupied cell must be rejected")
}
