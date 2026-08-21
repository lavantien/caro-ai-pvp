package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"os"
	"strings"
	"time"
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
// the predicted opponent reply the background search is built on, and the
// time cap that search was launched with.
type activePonderState struct {
	player         domain.Player
	predictedReply domain.Position
	timeCapMs      int64
}

// ponderInfo records the ponder that ran while the opponent was thinking:
// whether the opponent's move matched the prediction, and what the
// background search reached. It is observability only. The real move is
// always decided by a fresh budgeted search over the TT the ponder warmed;
// pondering buys depth through the warm table, never a shortcut move.
type ponderInfo struct {
	player domain.Player
	hit    bool
	stats  engine.SearchStats
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

// stopPonderLocked joins the active ponder, if any, and records what it
// produced for the stats of the next move. Requires s.mu.
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
	if !ok {
		return
	}
	hit := outcome.Completed &&
		outcome.PredictedReply == (domain.Position{X: actualX, Y: actualY})
	s.pendingPonder = &ponderInfo{
		player: active.player,
		hit:    hit,
		stats:  outcome.Stats,
	}
}

// TakePonderInfo returns and clears the recorded ponder info for expected
// player, if the opponent's last move ended that player's ponder.
func (s *GameSession) TakePonderInfo(expectedPlayer domain.Player) (engine.SearchStats, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := s.pendingPonder
	s.pendingPonder = nil
	if info == nil || info.player != expectedPlayer {
		return engine.SearchStats{}, false, false
	}
	return info.stats, info.hit, true
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
	// ponderTimeCapMs: 0 derives the cap from the opponent's live clock
	// (they must move or flag within it, so it scales with the time
	// control); negative forces a zero budget, the deterministic
	// incompleteness seam for tests.
	capMs := s.ponderTimeCapMs
	if capMs == 0 {
		capMs = s.liveClockMsLocked(mover.Opponent())
	}
	if capMs < 0 {
		capMs = 0
	}
	if !ai.StartPonder(pondered, mover, predicted, engine.PonderConfig{
		Threads:   profile.Goroutines,
		MaxDepth:  profile.MaxDepth,
		UseVCF:    profile.UseVCF,
		TimeCapMs: capMs,
	}) {
		return
	}
	s.activePonder = &activePonderState{player: mover, predictedReply: predicted, timeCapMs: capMs}
}

// liveClockMsLocked returns p's remaining time accounting for the clock
// burning since the last move. Requires s.mu.
func (s *GameSession) liveClockMsLocked(p domain.Player) int64 {
	remaining := s.redTimeMs
	if p == domain.PlayerBlue {
		remaining = s.blueTimeMs
	}
	if s.game.IsGameOver {
		return 0
	}
	elapsed := time.Since(s.lastMoveAt).Milliseconds()
	return max(0, remaining-elapsed)
}

// clearPonderStateLocked joins any running ponder without recording an
// outcome and drops all ponder state. The undo and teardown path.
// Requires s.mu.
func (s *GameSession) clearPonderStateLocked() {
	if s.activePonder != nil {
		if ai := s.aiForPlayerLocked(s.activePonder.player); ai != nil {
			ai.StopPonder()
		}
		s.activePonder = nil
	}
	s.pendingPonder = nil
}
