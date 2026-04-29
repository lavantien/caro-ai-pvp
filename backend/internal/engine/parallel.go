package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"sort"
	"sync"
)

type parallelResult struct {
	x, y      int
	score     int
	depth     int
	ttProbes  int64
	ttHits    int64
}

func ParallelSearch(
	b domain.Board,
	player domain.Player,
	config SearchConfig,
	tt *TranspositionTable,
	heuristics *SearchHeuristics,
	ctx context.Context,
) (int, int, SearchStats) {
	numWorkers := config.Goroutines
	if numWorkers <= 1 {
		return SearchPosition(b, player, config, tt, heuristics, ctx)
	}

	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)
	candidates = FilterOpenRule(candidates, &sb, player)
	if len(candidates) <= 1 {
		if len(candidates) == 1 {
			return candidates[0].X, candidates[0].Y, SearchStats{}
		}
		return -1, -1, SearchStats{}
	}

	monitor := NewTimeMonitor(ctx, config.TimeLimitMs)
	defer monitor.Stop()

	type job struct {
		depth int
	}

	jobs := make(chan job, config.MaxDepth)
	results := make(chan parallelResult, numWorkers)

	var wg sync.WaitGroup

	workerTTStats := make([]struct{ probes, hits int64 }, numWorkers)
	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerSB := NewSearchBoard(b)
			workerH := NewSearchHeuristics()
			workerTT := NewTranspositionTable(1)
			for job := range jobs {
				if monitor.ShouldStop() {
					return
				}

				x, y, score := searchRoot(&workerSB, player, job.depth, workerTT, workerH, candidates, monitor)

				if x >= 0 && !monitor.ShouldStop() {
					p, h := workerTT.Stats()
					workerTTStats[workerID] = struct{ probes, hits int64 }{p, h}
					results <- parallelResult{x: x, y: y, score: score, depth: job.depth}
				}
			}
		}(w)
	}

	go func() {
		for depth := 1; depth <= config.MaxDepth; depth++ {
			if monitor.ShouldStop() {
				break
			}
			jobs <- job{depth: depth}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	bestX, bestY := candidates[0].X, candidates[0].Y
	bestScore := -domain.WinScore * 2
	bestDepth := 0

	for r := range results {
		if r.depth > bestDepth || (r.depth == bestDepth && r.score > bestScore) {
			bestScore = r.score
			bestX, bestY = r.x, r.y
			bestDepth = r.depth
		}
	}

	elapsed := monitor.ElapsedMs()
	nodes := monitor.Nodes.Load()
	var nps float64
	if elapsed > 0 {
		nps = float64(nodes) / float64(elapsed) * 1000
	}

	var ttHitRate float64
	var rates []float64
	for _, s := range workerTTStats {
		if s.probes > 0 {
			rates = append(rates, float64(s.hits)/float64(s.probes))
		}
	}
	if len(rates) > 0 {
		sort.Float64s(rates)
		var sum float64
		for _, r := range rates {
			sum += r
		}
		mean := sum / float64(len(rates))
		median := rates[len(rates)/2]
		if len(rates)%2 == 0 {
			median = (rates[len(rates)/2-1] + rates[len(rates)/2]) / 2
		}
		ttHitRate = (mean + median) / 2
	}

	return bestX, bestY, SearchStats{
		DepthAchieved:   bestDepth,
		NodesSearched:   nodes,
		NodesPerSecond:  nps,
		SearchScore:     bestScore,
		TableHitRate:    ttHitRate,
		AllocatedTimeMs: config.TimeLimitMs,
		ThreadCount:     numWorkers,
	}
}
