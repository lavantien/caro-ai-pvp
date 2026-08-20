package api

import (
	"caro-ai-pvp/internal/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyAIMoveRejectsStalePlayer(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, nil, nil, func() int { return 1 })
	_, err := s.ApplyAIMove(8, 8, domain.PlayerBlue)
	assert.ErrorIs(t, err, domain.ErrNotPlayerTurn,
		"a move computed for blue must not land while red is on the move")

	_, err = s.ApplyAIMove(8, 8, domain.PlayerRed)
	assert.NoError(t, err)
}

func TestUndoInPvAITakesBackFullTurn(t *testing.T) {
	five := 5
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvAI, nil, &five, nil, func() int { return 1 })
	_, err := s.ApplyHumanMove(8, 8)
	require.NoError(t, err)
	_, err = s.ApplyAIMove(8, 9, domain.PlayerBlue)
	require.NoError(t, err)
	assert.Equal(t, 2, s.GetResponse().MoveNumber)

	resp, err := s.UndoLastMove()
	require.NoError(t, err)
	assert.Equal(t, 0, resp.MoveNumber, "undo must remove the AI reply and the human move")
	assert.Equal(t, "red", resp.CurrentPlayer, "the human must be on the move again")
}

func TestUndoInPvpStaysSinglePly(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvP, nil, nil, nil, func() int { return 1 })
	_, err := s.ApplyHumanMove(8, 8)
	require.NoError(t, err)
	_, err = s.ApplyHumanMove(8, 9)
	require.NoError(t, err)

	resp, err := s.UndoLastMove()
	require.NoError(t, err)
	assert.Equal(t, 1, resp.MoveNumber, "pvp undo must remove exactly one move")
}

func TestBoardFullEndsInDraw(t *testing.T) {
	s := NewGameSession("15+10", 900_000, 10, domain.GameModePvP, nil, nil, nil, func() int { return 1 })

	// Build a full board minus (15,15) directly: rows come out monochrome
	// (16-runs are overlines, never a win) and columns/diagonals alternate,
	// so no exactly-five can exist for either side.
	board := domain.NewBoard()
	k := 0
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if x == domain.BoardSize-1 && y == domain.BoardSize-1 {
				continue
			}
			player := domain.PlayerBlue
			if k%2 == 0 {
				player = domain.PlayerRed
			}
			board = board.PlaceStone(x, y, player)
			k++
		}
	}
	s.mu.Lock()
	s.game.Board = board
	s.game.MoveNumber = domain.MaxMoves - 1
	s.game.CurrentPlayer = domain.PlayerRed
	s.mu.Unlock()

	resp, err := s.ApplyHumanMove(domain.BoardSize-1, domain.BoardSize-1)
	require.NoError(t, err)
	assert.True(t, resp.IsGameOver, "a full board must end the game")
	assert.Equal(t, "draw", resp.EndReason)
	assert.Equal(t, "none", resp.Winner)
}

func TestClocksCountDownBetweenMoves(t *testing.T) {
	s := NewGameSession("1+0", 60_000, 0, domain.GameModePvP, nil, nil, nil, func() int { return 1 })
	_, err := s.ApplyHumanMove(8, 8)
	require.NoError(t, err)

	// Blue is on the move: blue's displayed clock must tick down live.
	before := s.GetResponse().BlueTimeRemaining
	s.mu.Lock()
	s.lastMoveAt = time.Now().Add(-10 * time.Second)
	s.mu.Unlock()
	after := s.GetResponse().BlueTimeRemaining

	assert.InDelta(t, before-10, after, 1.5,
		"reading mid-turn must show the live clock, not the value stored at the last move")
}
