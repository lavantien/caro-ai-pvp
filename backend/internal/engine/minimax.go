package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"runtime/debug"
)

type SearchStats struct {
	DepthAchieved   int
	NodesSearched   int64
	NodesPerSecond  float64
	SearchScore     int
	MoveType        string
	TableHitRate    float64
	AllocatedTimeMs int64
	ThreadCount     int
}

type SearchOptions struct {
	TimeRemainingMs int64
	IncrementMs     int64
	MoveNumber      int
	ThreadCount     int
	PonderEnabled   bool
	ParallelEnabled bool
	TimeFraction    float64
	UseVCF          bool
}

type MinimaxAI struct {
	tt         *TranspositionTable
	heuristics *SearchHeuristics
	logger     *slog.Logger
	maxThreads int
	stats      SearchStats
}

func NewMinimaxAI(logger *slog.Logger, maxThreads int) *MinimaxAI {
	if maxThreads < 1 {
		maxThreads = 1
	}
	return &MinimaxAI{
		tt:         NewTranspositionTable(domain.DefaultTTSizeMB),
		heuristics: NewSearchHeuristics(),
		logger:     logger,
		maxThreads: maxThreads,
	}
}

func (ai *MinimaxAI) GetBestMove(
	b domain.Board,
	player domain.Player,
	opts SearchOptions,
	ctx context.Context,
) (int, int, SearchStats) {
	debug.SetMemoryLimit(domain.HeapHardLimitBytes)

	timeAlloc := AllocateTime(opts.TimeRemainingMs, opts.IncrementMs, opts.MoveNumber)
	hardBound := int64(float64(timeAlloc.HardBoundMs) * opts.TimeFraction)

	config := SearchConfig{
		MaxDepth:     domain.AbsoluteMaxDepth,
		TimeLimitMs:  hardBound,
		Goroutines:   min(opts.ThreadCount, ai.maxThreads),
		UseVCF:       opts.UseVCF,
		TimeFraction: opts.TimeFraction,
	}

	if config.Goroutines < 1 {
		config.Goroutines = 1
	}

	ai.heuristics.Clear()
	ai.tt.IncrementAge()

	var x, y int
	var stats SearchStats
	if opts.ParallelEnabled && config.Goroutines > 1 {
		x, y, stats = ParallelSearch(b, player, config, ai.tt, ai.heuristics, ctx)
	} else {
		x, y, stats = SearchPosition(b, player, config, ai.tt, ai.heuristics, ctx)
	}

	ai.stats = stats
	return x, y, stats
}

func (ai *MinimaxAI) GetStats() SearchStats {
	return ai.stats
}

func (ai *MinimaxAI) Dispose() {
	ai.tt.Dispose()
	ai.heuristics.Clear()
}
