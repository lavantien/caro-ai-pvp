package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"os"
	"strings"
)

// ponderEnvDisabled is the process-wide kill switch, read once at startup
// like MATCH_DB_PATH. A package var so tests can flip it.
var ponderEnvDisabled = isPonderDisabledByEnv()

func isPonderDisabledByEnv() bool {
	switch strings.ToLower(os.Getenv("CARO_DISABLE_PONDER")) {
	case "1", "true":
		return true
	}
	return false
}

// activePonderState records the ponder a player started after its own move:
// the predicted opponent reply the background search is built on.
type activePonderState struct {
	player         domain.Player
	predictedReply domain.Position
}

// ponderHit is a consumed ponder outcome whose predicted reply matched the
// opponent's actual move. boardHash pins it to the exact position the
// pondered move is legal in, so any state change in between (undo,
// duplicate requests) downgrades it to a miss.
type ponderHit struct {
	player    domain.Player
	x, y      int
	boardHash uint64
	stats     engine.SearchStats
}

// ponderEnabledForLocked reports whether player p's side should ponder.
// Requires s.mu.
func (s *GameSession) ponderEnabledForLocked(p domain.Player) bool {
	if ponderEnvDisabled {
		return false
	}
	diff := s.difficultyForLocked(p)
	if diff == nil {
		return false
	}
	return engine.GetDifficultyProfile(*diff).Ponder
}

// difficultyForLocked returns the difficulty pointer for p's side.
// Requires s.mu.
func (s *GameSession) difficultyForLocked(p domain.Player) *int {
	if p == domain.PlayerBlue {
		return s.blueDifficulty
	}
	return s.redDifficulty
}

// aiForPlayerLocked returns p's AI instance if it exists. Requires s.mu.
func (s *GameSession) aiForPlayerLocked(p domain.Player) *engine.MinimaxAI {
	if p == domain.PlayerRed {
		return s.redAI
	}
	return s.blueAI
}

// stopPonderLocked joins the active ponder, if any, and stages a pending
// hit when the move just played matches the predicted reply and the ponder
// completed at least one depth. Requires s.mu.
func (s *GameSession) stopPonderLocked(actualX, actualY int) {
	active := s.activePonder
	if active == nil {
		return
	}
	s.activePonder = nil

	ai := s.aiForPlayerLocked(active.player)
	if ai == nil {
		return
	}
	outcome, ok := ai.StopPonder()
	if !ok || !outcome.Completed {
		return
	}
	if outcome.PredictedReply != (domain.Position{X: actualX, Y: actualY}) {
		return
	}
	s.pendingPonder = &ponderHit{
		player:    active.player,
		x:         outcome.BestX,
		y:         outcome.BestY,
		boardHash: outcome.BoardHash,
		stats:     outcome.Stats,
	}
}

// startPonderLocked launches mover's ponder on the position after its own
// move plus the predicted reply. Skipped when pondering is disabled, the
// AI has no search history, or no prediction is available. Requires s.mu.
func (s *GameSession) startPonderLocked(mover domain.Player) {
	if !s.ponderEnabledForLocked(mover) {
		return
	}
	ai := s.aiForPlayerLocked(mover)
	if ai == nil {
		return
	}
	predicted, ok := ai.PredictReply(s.game.Board)
	if !ok {
		return
	}
	pondered, err := s.game.Board.PlaceStoneChecked(predicted.X, predicted.Y, mover.Opponent())
	if err != nil {
		return
	}

	profile := engine.GetDifficultyProfile(*s.difficultyForLocked(mover))
	if !ai.StartPonder(pondered, mover, predicted, engine.PonderConfig{
		Threads:   profile.Goroutines,
		MaxDepth:  profile.MaxDepth,
		UseVCF:    profile.UseVCF,
		TimeCapMs: s.ponderTimeCapMs,
	}) {
		return
	}
	s.activePonder = &activePonderState{player: mover, predictedReply: predicted}
}

// clearPonderStateLocked joins any running ponder without hit detection and
// drops all ponder state. The undo and teardown path. Requires s.mu.
func (s *GameSession) clearPonderStateLocked() {
	if s.activePonder != nil {
		if ai := s.aiForPlayerLocked(s.activePonder.player); ai != nil {
			ai.StopPonder()
		}
		s.activePonder = nil
	}
	s.pendingPonder = nil
}
