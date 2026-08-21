package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
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
	ParallelEnabled bool
	TimeFraction    float64
	UseVCF          bool
	MaxDepth        int
}

type MinimaxAI struct {
	tt         *TranspositionTable
	heuristics *SearchHeuristics
	logger     *slog.Logger
	maxThreads int
	stats      SearchStats

	ponderMu      sync.Mutex
	ponderCancel  context.CancelFunc
	ponderDone    chan struct{}
	ponderOutcome *PonderOutcome
}

func NewMinimaxAI(logger *slog.Logger, maxThreads int, ttSizeMB int) *MinimaxAI {
	if maxThreads < 1 {
		maxThreads = 1
	}
	if ttSizeMB < 1 {
		ttSizeMB = domain.DefaultTTSizeMB
	}
	return &MinimaxAI{
		tt:         NewTranspositionTable(ttSizeMB),
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
	// A ponder must never overlap the official search on the same AI.
	ai.StopPonder()

	debug.SetMemoryLimit(domain.HeapHardLimitBytes)

	timeAlloc := AllocateTime(opts.TimeRemainingMs, opts.IncrementMs, opts.MoveNumber)
	hardBound := int64(float64(timeAlloc.HardBoundMs) * opts.TimeFraction)
	if hardBound < 0 {
		hardBound = 0
	}
	softBound := int64(float64(timeAlloc.SoftBoundMs) * opts.TimeFraction)
	if softBound < 0 {
		softBound = 0
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 || maxDepth > domain.AbsoluteMaxDepth {
		maxDepth = domain.AbsoluteMaxDepth
	}

	config := SearchConfig{
		MaxDepth:     maxDepth,
		TimeLimitMs:  hardBound,
		SoftLimitMs:  softBound,
		Goroutines:   min(opts.ThreadCount, ai.maxThreads),
		UseVCF:       opts.UseVCF,
		TimeFraction: opts.TimeFraction,
	}

	if config.Goroutines < 1 {
		config.Goroutines = 1
	}

	ai.heuristics.AgeForNewMove()
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
	// Join the ponder before freeing the table: a straggler search would
	// index the nilled shard slices.
	ai.StopPonder()
	ai.tt.Dispose()
	ai.heuristics.Clear()
}
