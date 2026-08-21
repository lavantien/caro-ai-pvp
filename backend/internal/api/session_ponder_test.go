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

func TestPonderCapDerivedFromOpponentClock(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	s.ponderTimeCapMs = 0 // auto: derive from the opponent's live clock
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)

	// Blue has the full 60s bullet clock minus the time elapsed since
	// red's move landed.
	assert.Greater(t, s.activePonder.timeCapMs, int64(55_000))
	assert.LessOrEqual(t, s.activePonder.timeCapMs, int64(60_000))

	// The cap scales with the opponent's clock, not a fixed constant.
	s2 := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	s2.ponderTimeCapMs = 0
	s2.mu.Lock()
	s2.blueTimeMs = 5_000
	s2.mu.Unlock()
	playSearchedAIMove(t, s2, domain.PlayerRed)
	require.NotNil(t, s2.activePonder)
	assert.Greater(t, s2.activePonder.timeCapMs, int64(4_000))
	assert.LessOrEqual(t, s2.activePonder.timeCapMs, int64(5_000))
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
	require.NotNil(t, s.pendingPonder)
	assert.False(t, s.pendingPonder.hit, "a different reply is a miss")
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

func TestPonderHitRecordedOnPredictedReply(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply

	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 10*time.Millisecond, "the short cap lets the ponder finish")

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	require.NotNil(t, s.pendingPonder, "the ponder outcome is recorded for stats")
	assert.Equal(t, domain.PlayerRed, s.pendingPonder.player)
	assert.True(t, s.pendingPonder.hit, "the predicted reply matched")
}

func TestPonderMissRecordedAsNotHit(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply
	alt := legalAlternativeReply(t, s.game.Board, pred)

	_, err := s.ApplyAIMove(alt.X, alt.Y, domain.PlayerBlue)
	require.NoError(t, err)
	require.NotNil(t, s.pendingPonder, "a miss is still recorded for stats")
	assert.False(t, s.pendingPonder.hit)
	assert.Nil(t, s.activePonder)
}

func TestPonderIncompleteIsNotHit(t *testing.T) {
	s := newPonderSession(domain.GameModeAivAI, intPtr(5), nil)
	s.ponderTimeCapMs = -1 // forced zero budget: no depth can ever complete
	playSearchedAIMove(t, s, domain.PlayerRed)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply

	require.Eventually(t, func() bool { return !s.redAI.PonderActive() },
		2*time.Second, 5*time.Millisecond)

	_, err := s.ApplyAIMove(pred.X, pred.Y, domain.PlayerBlue)
	require.NoError(t, err)
	require.NotNil(t, s.pendingPonder)
	assert.False(t, s.pendingPonder.hit, "an incomplete ponder is not a hit")
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
