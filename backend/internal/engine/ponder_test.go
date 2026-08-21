package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"testing"
	"time"

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

func ponderTestBoard() domain.Board {
	return domain.NewBoard().
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerBlue)
}

// ponderReachedDepthOne polls the shared TT: searchRoot stores an entry at
// the pondered root after each completed depth, so a hit with Depth >= 1
// means at least one full iteration finished.
func ponderReachedDepthOne(ai *MinimaxAI, b domain.Board) func() bool {
	return func() bool {
		entry, ok := ai.tt.Lookup(b.Hash())
		return ok && entry.Depth >= 1
	}
}

func TestStartPonderStopLifecycle(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()
	b := ponderTestBoard()

	ok := ai.StartPonder(b, domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads:   1,
		MaxDepth:  50,
		TimeCapMs: 10_000,
	})
	require.True(t, ok, "no ponder running, start should succeed")
	assert.True(t, ai.PonderActive())
	require.Eventually(t, ponderReachedDepthOne(ai, b), 2*time.Second, 5*time.Millisecond,
		"depth 1 must complete before the stop")

	outcome, stopped := ai.StopPonder()
	require.True(t, stopped)
	assert.True(t, outcome.Completed, "depth 1 finished before the stop")
	assert.True(t, outcome.BestX >= 0 && outcome.BestY >= 0)
	assert.Equal(t, domain.PlayerRed, outcome.Player)
	assert.Equal(t, domain.Position{X: 9, Y: 9}, outcome.PredictedReply)
	assert.Equal(t, ponderTestBoard().Hash(), outcome.BoardHash)

	_, stopped = ai.StopPonder()
	assert.False(t, stopped, "outcome is consumed exactly once")
	assert.False(t, ai.PonderActive())
}

func TestStartPonderRefusesWhileRunning(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	require.True(t, ai.StartPonder(ponderTestBoard(), domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 10_000,
	}))
	assert.False(t, ai.StartPonder(ponderTestBoard(), domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 10_000,
	}), "a second start while running must be refused")
	ai.StopPonder()
}

func TestPonderCancelledBeforeCompletion(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.True(t, ai.startPonderWithContext(ctx, cancel, ponderTestBoard(), domain.PlayerRed,
		domain.Position{X: 9, Y: 9}, PonderConfig{Threads: 1, MaxDepth: 8, TimeCapMs: 5_000}))

	outcome, stopped := ai.StopPonder()
	require.True(t, stopped)
	assert.False(t, outcome.Completed, "a cancelled ponder never completed a depth")
}

func TestPonderTimeCapEndsSearch(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	require.True(t, ai.StartPonder(ponderTestBoard(), domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 50,
	}))
	require.Eventually(t, func() bool { return !ai.PonderActive() },
		2*time.Second, 10*time.Millisecond, "the cap must stop an idle ponder")
}

func TestPonderSharesTTNotHeuristics(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()
	b := ponderTestBoard()

	require.True(t, ai.StartPonder(b, domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 6, TimeCapMs: 3_000,
	}))
	require.Eventually(t, ponderReachedDepthOne(ai, b), 2*time.Second, 5*time.Millisecond)
	outcome, stopped := ai.StopPonder()
	require.True(t, stopped)
	require.True(t, outcome.Completed)

	entry, has := ai.tt.Lookup(b.Hash())
	assert.True(t, has, "ponder must warm the shared TT at the pondered root")
	assert.GreaterOrEqual(t, int(entry.Depth), 1)

	x, y, _ := ai.GetBestMove(b, domain.PlayerRed, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
	}, context.Background())
	assert.True(t, x >= 0 && y >= 0, "normal search must still work after pondering")
}

func TestGetBestMoveStopsPonder(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()
	b := ponderTestBoard()

	require.True(t, ai.StartPonder(b, domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 10_000,
	}))

	x, y, _ := ai.GetBestMove(b, domain.PlayerRed, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
	}, context.Background())
	assert.True(t, x >= 0 && y >= 0)
	assert.False(t, ai.PonderActive(), "GetBestMove must drain any running ponder first")
}

func TestPonderDisposeDuringPonderNoRace(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	require.True(t, ai.StartPonder(ponderTestBoard(), domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 10_000,
	}))
	assert.NotPanics(t, func() { ai.Dispose() })
	assert.False(t, ai.PonderActive())
}

func TestPonderCompletedVCF(t *testing.T) {
	assert.True(t, ponderCompleted(SearchStats{MoveType: "vcf"}))
	assert.True(t, ponderCompleted(SearchStats{DepthAchieved: 3}))
	assert.False(t, ponderCompleted(SearchStats{}))
	assert.False(t, ponderCompleted(SearchStats{MoveType: "timeout-fallback"}))
}

func TestTTIsolationBetweenAIInstances(t *testing.T) {
	red := NewMinimaxAI(slog.Default(), 1, 64)
	defer red.Dispose()
	blue := NewMinimaxAI(slog.Default(), 1, 64)
	defer blue.Dispose()

	b := ponderTestBoard()
	x, y, _ := red.GetBestMove(b, domain.PlayerRed, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
		MaxDepth:        4,
	}, context.Background())
	require.GreaterOrEqual(t, x, 0)
	searchRoot, err := b.PlaceStoneChecked(x, y, domain.PlayerRed)
	require.NoError(t, err)

	require.True(t, red.StartPonder(searchRoot, domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 4, TimeCapMs: 500,
	}))
	outcome, stopped := red.StopPonder()
	require.True(t, stopped)

	// Red searched and pondered; every position it touched must be absent
	// from blue's table.
	for _, hash := range []uint64{b.Hash(), searchRoot.Hash(), outcome.BoardHash} {
		_, ok := blue.tt.Lookup(hash)
		assert.False(t, ok, "red's work leaked into blue's table (hash %d)", hash)
	}

	// Symmetric: blue searches a position red never saw (the root hash is
	// side-independent, so a shared root would be legitimate overlap, not
	// contamination — use a distinct stone configuration).
	b2 := b.PlaceStone(0, 0, domain.PlayerRed).PlaceStone(0, 1, domain.PlayerBlue)
	bx, by, _ := blue.GetBestMove(b2, domain.PlayerBlue, SearchOptions{
		TimeRemainingMs: 5000,
		ThreadCount:     1,
		TimeFraction:    1.0,
		MaxDepth:        4,
	}, context.Background())
	require.GreaterOrEqual(t, bx, 0)
	require.GreaterOrEqual(t, by, 0)
	_, ok := blue.tt.Lookup(b2.Hash())
	assert.True(t, ok, "blue's own search populated its table")
	_, ok = red.tt.Lookup(b2.Hash())
	assert.False(t, ok, "blue's search leaked into red's table")
}

func TestPonderOutcomeRecordsElapsed(t *testing.T) {
	ai := NewMinimaxAI(slog.Default(), 1, 64)
	defer ai.Dispose()

	require.True(t, ai.StartPonder(ponderTestBoard(), domain.PlayerRed, domain.Position{X: 9, Y: 9}, PonderConfig{
		Threads: 1, MaxDepth: 50, TimeCapMs: 80,
	}))
	require.Eventually(t, func() bool { return !ai.PonderActive() },
		2*time.Second, 5*time.Millisecond, "the short cap ends the search")

	outcome, ok := ai.StopPonder()
	require.True(t, ok)
	assert.Greater(t, outcome.ElapsedMs, int64(0))
	assert.LessOrEqual(t, outcome.ElapsedMs, int64(2000), "elapsed should track the capped run")
}
