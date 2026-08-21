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
	activePonder    *activePonderState
	pendingPonder   *ponderHit
	ponderTimeCapMs int64
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

// applyRandomOpening plays a seeded two-stone opening (red from the center
// region, blue replying locally) so engine-vs-engine samples are not all the
// same game. Deterministic per seed.
func (s *GameSession) applyRandomOpening(seed int64) {
	rng := newOpeningRNG(seed)
	low := domain.BoardSize/2 - 3
	high := domain.BoardSize/2 + 2
	rx := low + rng.next(high-low+1)
	ry := low + rng.next(high-low+1)
	s.game, _ = s.game.WithMove(rx, ry)

	bx := rx - 3 + rng.next(7)
	by := ry - 3 + rng.next(7)
	bx = min(max(bx, 0), domain.BoardSize-1)
	by = min(max(by, 0), domain.BoardSize-1)
	if bx == rx && by == ry {
		bx = (bx + 1) % domain.BoardSize
	}
	s.game, _ = s.game.WithMove(bx, by)
}

// openingRNG is a splitmix64 generator: small, deterministic, seedable.
type openingRNG struct{ state uint64 }

func newOpeningRNG(seed int64) *openingRNG {
	return &openingRNG{state: uint64(seed)}
}

func (r *openingRNG) next(n int) int {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z = z ^ (z >> 31)
	return int(z % uint64(n))
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
	// Compute the thread budget before taking s.mu: the callback locks the
	// store, and the store's ActiveGameCount locks sessions, so locking in
	// the other order would deadlock.
	threads := engine.GetEngineThreadsForLoad(s.activeGameCount())
	diff := s.redDifficulty
	if player == domain.PlayerBlue {
		diff = s.blueDifficulty
	}
	ttSizeMB := engine.DefaultSessionTTSizeMB
	if diff != nil && *diff >= 1 && *diff <= 5 {
		ttSizeMB = engine.GetDifficultyProfile(*diff).TTSizeMB
	}

	s.mu.Lock()
	defer s.mu.Unlock()
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

// ApplyAIMove applies a move the engine computed for expectedPlayer. The
// search runs unlocked for seconds, so the turn is re-validated here: if
// another move landed first, the stale result is rejected instead of being
// played for the wrong color.
func (s *GameSession) ApplyAIMove(x, y int, expectedPlayer domain.Player) (GameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()

	if s.game.IsGameOver {
		return GameResponse{}, domain.ErrGameOver
	}
	if s.game.CurrentPlayer != expectedPlayer {
		return GameResponse{}, domain.ErrNotPlayerTurn
	}
	return s.applyMoveLocked(x, y)
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
	mover := s.game.CurrentPlayer
	newGame, err := s.game.WithMove(x, y)
	if err != nil {
		return GameResponse{}, err
	}
	s.stopPonderLocked(x, y)

	result := domain.CheckWinFromMove(newGame.Board, x, y)
	if result.HasWinner {
		newGame = newGame.WithGameOver(result.Winner, result.WinningLine)
	} else if newGame.MoveNumber >= domain.MaxMoves {
		newGame = newGame.WithDraw()
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
	} else {
		s.startPonderLocked(mover)
	}

	return s.buildResponse(), nil
}

func (s *GameSession) UndoLastMove() (GameResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Any ponder or staged hit refers to a position that is about to
	// disappear; drop it all before taking moves back.
	s.clearPonderStateLocked()

	newGame, err := s.game.UndoMove()
	if err != nil {
		return GameResponse{}, err
	}
	s.game = newGame

	// In player-vs-AI a single ply of undo would hand the turn straight to
	// the engine (its reply comes free). Take back a full turn so the human
	// is on the move again.
	if s.game.GameMode == domain.GameModePvAI && !s.game.IsGameOver && s.aiOwnsTurnLocked() && len(s.game.BoardHistory) > 0 {
		if newGame, err := s.game.UndoMove(); err == nil {
			s.game = newGame
		}
	}
	return s.buildResponse(), nil
}

// aiOwnsTurnLocked reports whether the engine side is to move. Requires s.mu.
func (s *GameSession) aiOwnsTurnLocked() bool {
	aiIsRed := s.redDifficulty != nil
	if aiIsRed {
		return s.game.CurrentPlayer == domain.PlayerRed
	}
	return s.game.CurrentPlayer == domain.PlayerBlue
}

func (s *GameSession) DisposeAI() {
	s.clearPonderStateLocked()
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

	// Clocks display live: the player on the move has been burning time
	// since the last move landed.
	redTime, blueTime := s.redTimeMs, s.blueTimeMs
	if !s.game.IsGameOver && s.game.MoveNumber >= 0 {
		elapsed := time.Since(s.lastMoveAt).Milliseconds()
		if s.game.CurrentPlayer == domain.PlayerRed {
			redTime = max(0, redTime-elapsed)
		} else if s.game.CurrentPlayer == domain.PlayerBlue {
			blueTime = max(0, blueTime-elapsed)
		}
	}

	return GameResponse{
		Board:             cells,
		CurrentPlayer:     s.game.CurrentPlayer.String(),
		MoveNumber:        s.game.MoveNumber,
		IsGameOver:        s.game.IsGameOver,
		Winner:            s.game.Winner.String(),
		EndReason:         s.game.EndReason,
		WinningLine:       winningLine,
		RedTimeRemaining:  float64(redTime) / 1000.0,
		BlueTimeRemaining: float64(blueTime) / 1000.0,
		TimeControl:       s.game.TimeControl,
		InitialTime:       int(s.game.InitialTimeMs / 1000),
		Increment:         s.game.IncrementSeconds,
		GameMode:          s.game.GameMode.String(),
		RedDifficulty:     s.redDifficulty,
		BlueDifficulty:    s.blueDifficulty,
	}
}
