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

// ponderGatePassed reports whether a completed ponder is deep enough to
// adopt instantly: the ponder must have run at least
// PonderAdoptionFraction of the soft budget a normal search would get, or
// be a solver-verified VCF win (a forced win is valid at any depth). A
// fast opponent shrinks the ponder window below the gate, and the hit
// becomes a TT head start for the normal search instead.
func ponderGatePassed(elapsedMs, softBudgetMs int64, moveType string) bool {
	if moveType == "vcf" {
		return true
	}
	return elapsedMs >= int64(float64(softBudgetMs)*domain.PonderAdoptionFraction)
}

// stopPonderLocked joins the active ponder, if any, and stages a pending
// hit when the move just played matches the predicted reply, the ponder
// completed at least one depth, and the ponder window was long enough to
// be worth adopting. Requires s.mu.
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
	if !ponderGatePassed(outcome.ElapsedMs, s.ponderSoftBudgetLocked(active.player), outcome.Stats.MoveType) {
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

// ponderSoftBudgetLocked returns the soft time budget a normal search for
// p would receive right now, mirroring GetBestMove's allocation math.
// Requires s.mu.
func (s *GameSession) ponderSoftBudgetLocked(p domain.Player) int64 {
	remaining := s.redTimeMs
	if p == domain.PlayerBlue {
		remaining = s.blueTimeMs
	}
	incMs := int64(s.game.IncrementSeconds) * 1000
	alloc := engine.AllocateTime(remaining, incMs, s.game.MoveNumber)
	fraction := 1.0
	if diff := s.difficultyForLocked(p); diff != nil {
		fraction = engine.GetDifficultyProfile(*diff).TimeFraction
	}
	return int64(float64(alloc.SoftBoundMs) * fraction)
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

// TryPonderMove consumes a pending ponder hit for expectedPlayer: under the
// session mutex it re-validates flags, turn, and that the board still
// matches the pondered position, then applies the pondered move through the
// normal legality path. ok is true only when the move was actually played.
func (s *GameSession) TryPonderMove(expectedPlayer domain.Player) (GameResponse, int, int, engine.SearchStats, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkTimeoutLocked()

	if s.pendingPonder == nil || s.game.IsGameOver || s.game.CurrentPlayer != expectedPlayer {
		return GameResponse{}, -1, -1, engine.SearchStats{}, false
	}
	hit := s.pendingPonder
	if hit.player != expectedPlayer || hit.boardHash != s.game.Board.Hash() {
		// The position changed since the ponder (undo or duplicate
		// request): downgrade to a miss.
		s.pendingPonder = nil
		return GameResponse{}, -1, -1, engine.SearchStats{}, false
	}
	s.pendingPonder = nil

	resp, err := s.applyMoveLocked(hit.x, hit.y)
	if err != nil {
		return GameResponse{}, -1, -1, engine.SearchStats{}, false
	}
	return resp, hit.x, hit.y, hit.stats, true
}
