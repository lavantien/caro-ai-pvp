package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTimesOutCurrentPlayer(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, nil, nil, func() int { return 1 })
	_, err := s.ApplyMove(8, 8)
	require.NoError(t, err) // red moved; blue (AI) to move

	// Blue never moves; far more time passes than blue has left is simulated
	// by rewinding the last activity timestamp.
	s.mu.Lock()
	s.lastMoveAt = time.Now().Add(-2 * time.Hour)
	s.mu.Unlock()

	_, err = s.ApplyMove(8, 9) // out-of-turn move attempt; timeout must adjudicate first
	assert.Error(t, err)
	resp := s.GetResponse()
	assert.True(t, resp.IsGameOver, "flagged player must lose on time")
	assert.Equal(t, "red", resp.Winner)
	assert.Equal(t, "timeout", resp.EndReason)
	assert.Equal(t, float64(0), resp.BlueTimeRemaining)
}

func TestSessionTimeoutOnRead(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, nil, nil, func() int { return 1 })
	s.mu.Lock()
	s.lastMoveAt = time.Now().Add(-3 * time.Hour)
	s.mu.Unlock()

	resp := s.GetResponse()
	assert.True(t, resp.IsGameOver)
	assert.Equal(t, "blue", resp.Winner, "red moved first, so red is the one who flagged")
	assert.Equal(t, "timeout", resp.EndReason)
}

func TestSessionNoTimeoutWhileClockRuns(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, nil, nil, func() int { return 1 })
	resp := s.GetResponse()
	assert.False(t, resp.IsGameOver)
	assert.Empty(t, resp.EndReason)
}

// Levels must be strictly stronger than their predecessor, but not on the
// depth axis alone: measured at bullet, ID depth past ~6 stops buying
// strength, so L3 shares L2's cap and separates by VCF sight and time
// fraction instead (see TestDifficultyLadderOrdersStrengthAxes in engine).
func TestDifficultyDepthCapsMonotone(t *testing.T) {
	prev := 0
	for level := 1; level <= 5; level++ {
		p := engine.GetDifficultyProfile(level)
		assert.GreaterOrEqual(t, p.MaxDepth, prev, "L%d cap must not fall below L%d", level, level-1)
		assert.LessOrEqual(t, p.MaxDepth, domain.AbsoluteMaxDepth)
		if level == 3 {
			assert.True(t, p.UseVCF && p.VCFDepth > 0, "L3 must out-class L2 via VCF sight")
		}
		prev = p.MaxDepth
	}
}

func TestAllocateTimeNeverNegative(t *testing.T) {
	for _, remaining := range []int64{0, 1, 10, 50, 100} {
		alloc := engine.AllocateTime(remaining, 0, 10)
		assert.GreaterOrEqual(t, alloc.HardBoundMs, int64(0),
			"hard bound must not go negative at remaining=%dms", remaining)
	}
	alloc := engine.AllocateTime(1000, 0, 10)
	assert.Greater(t, alloc.HardBoundMs, int64(0), "with time left the engine must get a positive budget")
}

func TestApplyHumanMoveGuards(t *testing.T) {
	blue := 5
	t.Run("spectator cannot move in aivai", func(t *testing.T) {
		s := NewGameSession("1+0", 60_000, 0, domain.GameModeAivAI, nil, &blue, nil, func() int { return 1 })
		_, err := s.ApplyHumanMove(8, 8)
		assert.ErrorIs(t, err, domain.ErrNotPlayerTurn)
	})

	t.Run("human cannot move on engine turn in pvai", func(t *testing.T) {
		s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, &blue, nil, func() int { return 1 })
		_, err := s.ApplyHumanMove(8, 8) // red is human: allowed
		assert.NoError(t, err)
		_, err = s.ApplyHumanMove(8, 9) // blue is the engine now
		assert.ErrorIs(t, err, domain.ErrNotPlayerTurn)
	})

	t.Run("both humans may move in pvp", func(t *testing.T) {
		s := NewGameSession("1+0", 60_000, 0, domain.GameModePvP, nil, nil, nil, func() int { return 1 })
		_, err := s.ApplyHumanMove(8, 8)
		assert.NoError(t, err)
		_, err = s.ApplyHumanMove(8, 9)
		assert.NoError(t, err)
	})
}
