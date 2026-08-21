package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"time"
)

// PonderConfig shapes the background ponder search.
type PonderConfig struct {
	Threads   int
	MaxDepth  int
	UseVCF    bool
	TimeCapMs int64
}

// PonderOutcome is the result of a ponder search: the position searched
// (bot to move), the predicted reply that led to it, and the best move
// found there. Completed reports whether at least one depth iteration
// finished, the minimum bar for adopting the move on a ponder hit;
// ElapsedMs is the wall time the ponder actually ran, which gates whether
// a hit is deep enough to adopt instantly.
type PonderOutcome struct {
	Player         domain.Player
	PredictedReply domain.Position
	BoardHash      uint64
	BestX, BestY   int
	Stats          SearchStats
	Completed      bool
	ElapsedMs      int64
}

// PredictReply reads the TT entry the previous search stored for the
// position b (opponent to move) and returns its best move as the predicted
// opponent reply. The stored move came from the search's filtered candidate
// list (open rule included), so legality is inherent; the depth and
// emptiness checks guard against zeroed, stale, or colliding entries.
func (ai *MinimaxAI) PredictReply(b domain.Board) (domain.Position, bool) {
	entry, ok := ai.tt.Lookup(b.Hash())
	if !ok || entry.Depth == 0 {
		return domain.Position{}, false
	}
	p := domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	if !p.IsValid() || !b.IsEmptyAt(p.X, p.Y) {
		return domain.Position{}, false
	}
	return p, true
}

// StartPonder launches a background search on b (player to move), the
// position reached after the bot's own move and the predicted reply were
// applied. It shares the AI's TT, uses its own heuristics, never bumps the
// TT age, and never touches ai.stats. Returns false if a ponder is already
// running.
func (ai *MinimaxAI) StartPonder(b domain.Board, player domain.Player, predictedReply domain.Position, cfg PonderConfig) bool {
	ctx, cancel := context.WithCancel(context.Background())
	return ai.startPonderWithContext(ctx, cancel, b, player, predictedReply, cfg)
}

func (ai *MinimaxAI) startPonderWithContext(ctx context.Context, cancel context.CancelFunc, b domain.Board, player domain.Player, predictedReply domain.Position, cfg PonderConfig) bool {
	ai.ponderMu.Lock()
	if ai.ponderDone != nil {
		select {
		case <-ai.ponderDone:
			// Previous ponder finished but was never consumed; replace it.
		default:
			ai.ponderMu.Unlock()
			return false
		}
	}
	done := make(chan struct{})
	ai.ponderCancel = cancel
	ai.ponderDone = done
	ai.ponderOutcome = nil
	ai.ponderMu.Unlock()

	go func() {
		outcome := ai.runPonder(ctx, b, player, predictedReply, cfg)
		ai.ponderMu.Lock()
		ai.ponderOutcome = &outcome
		ai.ponderMu.Unlock()
		close(done)
	}()
	return true
}

// StopPonder cancels and joins any running ponder and consumes its outcome
// exactly once. Returns (outcome, true) if a ponder had been started, even
// when it already self-stopped at the time cap. Idempotent: a second call
// returns false.
func (ai *MinimaxAI) StopPonder() (PonderOutcome, bool) {
	ai.ponderMu.Lock()
	cancel := ai.ponderCancel
	done := ai.ponderDone
	ai.ponderMu.Unlock()

	if done == nil {
		return PonderOutcome{}, false
	}
	if cancel != nil {
		cancel()
	}
	<-done

	ai.ponderMu.Lock()
	outcome := ai.ponderOutcome
	ai.ponderCancel = nil
	ai.ponderDone = nil
	ai.ponderOutcome = nil
	ai.ponderMu.Unlock()

	if outcome == nil {
		return PonderOutcome{}, false
	}
	return *outcome, true
}

// PonderActive reports whether a ponder search is still running. A ponder
// that hit its time cap reports false even before the outcome is consumed.
func (ai *MinimaxAI) PonderActive() bool {
	ai.ponderMu.Lock()
	defer ai.ponderMu.Unlock()
	if ai.ponderDone == nil {
		return false
	}
	select {
	case <-ai.ponderDone:
		return false
	default:
		return true
	}
}

func (ai *MinimaxAI) runPonder(ctx context.Context, b domain.Board, player domain.Player, predictedReply domain.Position, cfg PonderConfig) PonderOutcome {
	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 || maxDepth > domain.AbsoluteMaxDepth {
		maxDepth = domain.AbsoluteMaxDepth
	}
	goroutines := min(cfg.Threads, ai.maxThreads)
	if goroutines < 1 {
		goroutines = 1
	}

	// SoftLimitMs 0 disables the soft budget: ponder has no clock pressure,
	// so the ID loop runs until the cap or MaxDepth.
	start := time.Now()
	x, y, stats := ParallelSearch(b, player, SearchConfig{
		MaxDepth:    maxDepth,
		TimeLimitMs: cfg.TimeCapMs,
		SoftLimitMs: 0,
		Goroutines:  goroutines,
		UseVCF:      cfg.UseVCF,
	}, ai.tt, NewSearchHeuristics(), ctx)

	return PonderOutcome{
		Player:         player,
		PredictedReply: predictedReply,
		BoardHash:      b.Hash(),
		BestX:          x,
		BestY:          y,
		Stats:          stats,
		Completed:      ponderCompleted(stats),
		ElapsedMs:      time.Since(start).Milliseconds(),
	}
}

func ponderCompleted(stats SearchStats) bool {
	// VCF results report DepthAchieved 0 but are solver-verified wins.
	return stats.DepthAchieved >= domain.PonderMinCompletedDepth || stats.MoveType == "vcf"
}
