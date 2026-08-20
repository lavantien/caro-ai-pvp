package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"log/slog"
	"sync"
	"time"
)

type GameSession struct {
	mu              sync.Mutex
	game            domain.GameState
	redTimeMs       int64
	blueTimeMs      int64
	lastMoveAt      time.Time
	redDifficulty   *int
	blueDifficulty  *int
	logger          *slog.Logger
	activeGameCount func() int
	redAI           *engine.MinimaxAI
	blueAI          *engine.MinimaxAI
}

func NewGameSession(
	timeControl string,
	initialTimeMs int64,
	incrementSeconds int,
	mode domain.GameMode,
	redDiff, blueDiff *int,
	logger *slog.Logger,
	activeGameCount func() int,
) *GameSession {
	return &GameSession{
		game:            domain.NewGameState(mode, timeControl, initialTimeMs, incrementSeconds),
		redTimeMs:       initialTimeMs,
		blueTimeMs:      initialTimeMs,
		lastMoveAt:      time.Now(),
		redDifficulty:   redDiff,
		blueDifficulty:  blueDiff,
		logger:          logger,
		activeGameCount: activeGameCount,
	}
}

func (s *GameSession) GetResponse() GameResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()
	return s.buildResponse()
}

func (s *GameSession) IsGameOver() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()
	return s.game.IsGameOver
}

// checkTimeoutLocked adjudicates a flag fall: if the player on the clock has
// let it run out since the last move, they lose on time. Requires s.mu.
func (s *GameSession) checkTimeoutLocked() {
	if s.game.IsGameOver {
		return
	}
	elapsed := time.Since(s.lastMoveAt).Milliseconds()
	clock := &s.redTimeMs
	if s.game.CurrentPlayer == domain.PlayerBlue {
		clock = &s.blueTimeMs
	}
	if elapsed >= *clock {
		*clock = 0
		winner := domain.PlayerBlue
		if s.game.CurrentPlayer == domain.PlayerBlue {
			winner = domain.PlayerRed
		}
		s.game = s.game.WithTimeout(winner)
		s.DisposeAI()
	}
}

func (s *GameSession) LastActivityAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMoveAt
}

func (s *GameSession) ExtractForAI() (domain.Board, domain.Player, bool, int64, int, int, *int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()

	timeRemaining := s.redTimeMs
	diff := s.redDifficulty
	if s.game.CurrentPlayer == domain.PlayerBlue {
		timeRemaining = s.blueTimeMs
		diff = s.blueDifficulty
	}

	return s.game.Board, s.game.CurrentPlayer, s.game.IsGameOver,
		timeRemaining, s.game.IncrementSeconds, s.game.MoveNumber, diff
}

func (s *GameSession) GetOrCreateAI(player domain.Player) *engine.MinimaxAI {
	threads := engine.GetEngineThreadsForLoad(s.activeGameCount())
	diff := s.redDifficulty
	if player == domain.PlayerBlue {
		diff = s.blueDifficulty
	}
	ttSizeMB := engine.DefaultSessionTTSizeMB
	if diff != nil && *diff >= 1 && *diff <= 5 {
		ttSizeMB = engine.GetDifficultyProfile(*diff).TTSizeMB
	}
	if player == domain.PlayerRed {
		if s.redAI == nil {
			s.redAI = engine.NewMinimaxAI(s.logger, threads, ttSizeMB)
		}
		return s.redAI
	}
	if s.blueAI == nil {
		s.blueAI = engine.NewMinimaxAI(s.logger, threads, ttSizeMB)
	}
	return s.blueAI
}

// ApplyHumanMove validates that a human may move right now: spectators
// cannot inject moves into AI-vs-AI games, and in player-vs-AI the human
// cannot move on the engine's turn.
func (s *GameSession) ApplyHumanMove(x, y int) (GameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()

	if s.game.IsGameOver {
		return GameResponse{}, domain.ErrGameOver
	}
	switch s.game.GameMode {
	case domain.GameModeAivAI:
		return GameResponse{}, domain.ErrNotPlayerTurn
	case domain.GameModePvAI:
		aiIsRed := s.redDifficulty != nil
		if (aiIsRed && s.game.CurrentPlayer == domain.PlayerRed) ||
			(!aiIsRed && s.game.CurrentPlayer == domain.PlayerBlue) {
			return GameResponse{}, domain.ErrNotPlayerTurn
		}
	}
	return s.applyMoveLocked(x, y)
}

func (s *GameSession) ApplyMove(x, y int) (GameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()

	if s.game.IsGameOver {
		return GameResponse{}, domain.ErrGameOver
	}
	return s.applyMoveLocked(x, y)
}

// applyMoveLocked applies a move for the current player. Requires s.mu.
func (s *GameSession) applyMoveLocked(x, y int) (GameResponse, error) {
	newGame, err := s.game.WithMove(x, y)
	if err != nil {
		return GameResponse{}, err
	}

	result := domain.CheckWinFromMove(newGame.Board, x, y)
	if result.HasWinner {
		newGame = newGame.WithGameOver(result.Winner, result.WinningLine)
	}

	now := time.Now()
	elapsed := now.Sub(s.lastMoveAt).Milliseconds()
	inc := int64(newGame.IncrementSeconds) * 1000
	if s.game.CurrentPlayer == domain.PlayerRed {
		s.redTimeMs = max(0, s.redTimeMs-elapsed+inc)
	} else {
		s.blueTimeMs = max(0, s.blueTimeMs-elapsed+inc)
	}
	s.lastMoveAt = now

	s.game = newGame

	if newGame.IsGameOver {
		s.DisposeAI()
	}

	return s.buildResponse(), nil
}

func (s *GameSession) UndoLastMove() (GameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newGame, err := s.game.UndoMove()
	if err != nil {
		return GameResponse{}, err
	}
	s.game = newGame
	return s.buildResponse(), nil
}

func (s *GameSession) DisposeAI() {
	if s.redAI != nil {
		s.redAI.Dispose()
		s.redAI = nil
	}
	if s.blueAI != nil {
		s.blueAI.Dispose()
		s.blueAI = nil
	}
}

func (s *GameSession) buildResponse() GameResponse {
	cells := make([]CellResponse, 0, domain.BoardSize*domain.BoardSize)
	for y := range domain.BoardSize {
		for x := range domain.BoardSize {
			player := s.game.Board.GetPlayerAt(x, y)
			cells = append(cells, CellResponse{X: x, Y: y, Player: player.String()})
		}
	}

	winningLine := make([]PositionResponse, len(s.game.WinningLine))
	for i, p := range s.game.WinningLine {
		winningLine[i] = PositionResponse{X: p.X, Y: p.Y}
	}

	return GameResponse{
		Board:             cells,
		CurrentPlayer:     s.game.CurrentPlayer.String(),
		MoveNumber:        s.game.MoveNumber,
		IsGameOver:        s.game.IsGameOver,
		Winner:            s.game.Winner.String(),
		EndReason:         s.game.EndReason,
		WinningLine:       winningLine,
		RedTimeRemaining:  float64(s.redTimeMs) / 1000.0,
		BlueTimeRemaining: float64(s.blueTimeMs) / 1000.0,
		TimeControl:       s.game.TimeControl,
		InitialTime:       int(s.game.InitialTimeMs / 1000),
		Increment:         s.game.IncrementSeconds,
		GameMode:          s.game.GameMode.String(),
		RedDifficulty:     s.redDifficulty,
		BlueDifficulty:    s.blueDifficulty,
	}
}
