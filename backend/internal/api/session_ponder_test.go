package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPonderSession(mode domain.GameMode, red, blue *int) *GameSession {
	s := NewGameSession("1+0", 60_000, 0, mode, red, blue, nil, func() int { return 1 })
	s.ponderTimeCapMs = 300
	return s
}

// playSearchedAIMove computes a real searched move for player (so the TT
// carries a prediction) and applies it, mirroring the handler flow.
func playSearchedAIMove(t *testing.T, s *GameSession, player domain.Player) {
	t.Helper()
	ai := s.GetOrCreateAI(player)
	board, _, over, timeMs, inc, moveNum, diff := s.ExtractForAI()
	require.False(t, over)
	require.NotNil(t, diff)

	x, y, _ := ai.GetBestMove(board, player, engine.SearchOptions{
		TimeRemainingMs: timeMs,
		IncrementMs:     int64(inc) * 1000,
		MoveNumber:      moveNum,
		ThreadCount:     1,
		TimeFraction:    0.1,
		MaxDepth:        4,
	}, context.Background())
	require.GreaterOrEqual(t, x, 0)

	_, err := s.ApplyAIMove(x, y, player)
	require.NoError(t, err)
}

// legalAlternativeReply returns an empty cell that satisfies the open rule
// and differs from the predicted reply.
func legalAlternativeReply(t *testing.T, b domain.Board, predicted domain.Position) domain.Position {
	t.Helper()
	for y := range domain.BoardSize {
		for x := range domain.BoardSize {
			p := domain.Position{X: x, Y: y}
			if p == predicted || !b.IsEmptyAt(x, y) {
				continue
			}
			if domain.IsValidSecondMove(b, x, y) {
				return p
			}
		}
	}
	t.Fatal("no alternative legal reply found")
	return domain.Position{}
}

func TestPonderStartsAfterAIMoveAivAI(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	assert.Equal(t, domain.PlayerRed, s.activePonder.player)
}

func TestPonderStartsAfterAIMovePvAI(t *testing.T) {
	s := newPonderSession(domain.GameModePvAI, nil, intPtr(5))
	_, err := s.ApplyHumanMove(7, 7) // human red
	require.NoError(t, err)

	playSearchedAIMove(t, s, domain.PlayerBlue)
	require.NotNil(t, s.activePonder)
	assert.Equal(t, domain.PlayerBlue, s.activePonder.player)

	pred := s.activePonder.predictedReply
	alt := legalAlternativeReply(t, s.game.Board, pred)
	_, err = s.ApplyHumanMove(alt.X, alt.Y)
	require.NoError(t, err)
	assert.Nil(t, s.activePonder, "the human's reply stops the ponder")
	assert.Nil(t, s.pendingPonder, "a different reply is a miss")
}

func TestPonderDisabledForLowerLevels(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(4), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	assert.Nil(t, s.activePonder)
}

func TestPonderKillSwitch(t *testing.T) {
	prev := ponderEnvDisabled
	ponderEnvDisabled = true
	t.Cleanup(func() { ponderEnvDisabled = prev })

	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	assert.Nil(t, s.activePonder)
}

func TestPonderHitDetectedOnPredictedReply(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply

	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 10*time.Millisecond, "the short cap lets the ponder finish")

	// Shrink red's clock so the 300ms ponder clears the adoption gate.
	s.mu.Lock()
	s.redTimeMs = 400
	s.mu.Unlock()

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	require.NotNil(t, s.pendingPonder)
	assert.Equal(t, domain.PlayerRed, s.pendingPonder.player)
}

func TestPonderHitGatedByTimeBudget(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	s.ponderTimeCapMs = 50 // far below the gate on a full 60s clock
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply

	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 5*time.Millisecond)

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	assert.Nil(t, s.pendingPonder,
		"a hit from a sub-second ponder window must not replace a full search")
}

func TestPonderGatePassed(t *testing.T) {
	assert.True(t, ponderGatePassed(5000, 6720, "exact"))
	assert.False(t, ponderGatePassed(400, 6720, "exact"))
	assert.True(t, ponderGatePassed(1, 6720, "vcf"), "solver-verified wins are exempt")
	assert.True(t, ponderGatePassed(10, 0, "exact"), "no budget left: adopt whatever was pondered")
}

func TestPonderMissDiscards(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply
	alt := legalAlternativeReply(t, s.game.Board, pred)

	_, err := s.ApplyAIMove(alt.X, alt.Y, domain.PlayerBlue)
	require.NoError(t, err)
	assert.Nil(t, s.pendingPonder, "a different reply is a miss")
	assert.Nil(t, s.activePonder)
}

func TestPonderIncompleteIsMiss(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	s.ponderTimeCapMs = 0 // zero budget: no depth can ever complete
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply

	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 5*time.Millisecond)

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	assert.Nil(t, s.pendingPonder, "an incomplete ponder must not stage a hit")
}

func TestPonderNotStartedWhenPredictionAbsent(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	_ = s.GetOrCreateAI(domain.PlayerRed)
	_, err := s.ApplyAIMove(7, 7, domain.PlayerRed) // no search ran
	require.NoError(t, err)
	assert.Nil(t, s.activePonder, "no TT prediction means no ponder")
}

func TestUndoInvalidatesPonder(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply
	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 10*time.Millisecond)

	s.mu.Lock()
	s.redTimeMs = 400
	s.mu.Unlock()

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	require.NotNil(t, s.pendingPonder)

	_, err = s.UndoLastMove()
	require.NoError(t, err)
	assert.Nil(t, s.pendingPonder)
	assert.Nil(t, s.activePonder)
	assert.False(t, s.redAI.PonderActive())
}

func TestDisposeAIDuringPonderDrains(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	ai := s.redAI
	require.True(t, ai.PonderActive())

	require.NotPanics(t, func() { s.DisposeAI() })
	assert.False(t, ai.PonderActive())
	assert.Nil(t, s.activePonder)
	assert.Nil(t, s.pendingPonder)
}

func TestFlagFallStopsPonder(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	ai := s.redAI
	require.True(t, ai.PonderActive())

	s.mu.Lock()
	s.lastMoveAt = time.Now().Add(-2 * time.Hour)
	s.mu.Unlock()

	resp := s.GetResponse()
	assert.True(t, resp.IsGameOver)
	assert.Equal(t, "timeout", resp.EndReason)
	assert.False(t, ai.PonderActive(), "flag fall must drain the ponder")
}

func TestStoreDeleteDrainsPonder(t *testing.T) {
	store := NewInMemoryStore()
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	ai := s.redAI
	require.True(t, ai.PonderActive())

	store.Set("g1", s)
	store.Delete("g1")
	assert.False(t, ai.PonderActive(), "store teardown must drain the ponder")
}
