package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"sync"
)

type parallelResult struct {
	x, y  int
	score int
	depth int
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
			return candidates[0].X, candidates[0].Y, SearchStats{ThreadCount: numWorkers}
		}
		return -1, -1, SearchStats{ThreadCount: numWorkers}
	}

	monitor := NewTimeMonitor(ctx, config.TimeLimitMs)
	defer monitor.Stop()

	if config.UseVCF {
		oppSB := NewSearchBoard(b)
		if !opponentHasImmediateWin(&oppSB, player.Opponent()) {
			vcfTime := int64(float64(config.TimeLimitMs) * domain.VCFTimeFraction)
			if vx, vy, result := SolveVCF(b, player, vcfTime, ctx); result == VCFWin {
				return vx, vy, SearchStats{
					DepthAchieved:   0,
					SearchScore:     domain.WinScore,
					AllocatedTimeMs: config.TimeLimitMs,
					ThreadCount:     numWorkers,
					MoveType:        "vcf",
				}
			}
		}
	}

	var vcfPreferred *domain.Position
	if config.UseVCF {
		oppVcfTime := int64(float64(config.TimeLimitMs) * domain.VCFTimeFraction / 2)
		if vx, vy, result := SolveVCF(b, player.Opponent(), oppVcfTime, ctx); result == VCFWin {
			blocked := b.PlaceStone(vx, vy, player)
			blockCheckTime := int64(float64(config.TimeLimitMs) * domain.VCFTimeFraction / 4)
			if _, _, checkResult := SolveVCF(blocked, player.Opponent(), blockCheckTime, ctx); checkResult != VCFWin {
				vcfPreferred = &domain.Position{X: vx, Y: vy}
			}
		}
	}

	tt.ResetStats()
	results := make(chan parallelResult, numWorkers*config.MaxDepth)

	var wg sync.WaitGroup
	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerSB := NewSearchBoard(b)
			workerH := NewSearchHeuristics()

			prevScore := -domain.Infinity
			completedDepth := 0
			startDepth := 1 + workerID%2

			for depth := startDepth; depth <= config.MaxDepth; depth++ {
				if monitor.ShouldStop() {
					break
				}

				delta := domain.AspirationWindowSize
				a, bnd := -domain.Infinity, domain.Infinity
				if completedDepth > 0 {
					a = max(prevScore-delta, -domain.Infinity)
					bnd = min(prevScore+delta, domain.Infinity)
				}

				var x, y, score int
				found := false
				for range domain.MaxAspirationAttempts {
					x, y, score = searchRoot(&workerSB, player, depth, a, bnd, tt, workerH, candidates, monitor, vcfPreferred)
					if x < 0 || monitor.ShouldStop() {
						break
					}
					if score <= a && a > -domain.Infinity {
						a = max(a-delta, -domain.Infinity)
						delta *= 2
						continue
					}
					if score >= bnd && bnd < domain.Infinity {
						bnd = min(bnd+delta, domain.Infinity)
						delta *= 2
						continue
					}
					found = true
					break
				}

				if !found && !monitor.ShouldStop() {
					x, y, score = searchRoot(&workerSB, player, depth, -domain.Infinity, domain.Infinity, tt, workerH, candidates, monitor, vcfPreferred)
					if x >= 0 {
						found = true
					}
				}

				if !found {
					break
				}

				prevScore = score
				completedDepth = depth
				results <- parallelResult{x: x, y: y, score: score, depth: depth}

				if score >= domain.WinScore-domain.AbsoluteMaxDepth {
					break
				}
			}
		}(w)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	bestX, bestY := candidates[0].X, candidates[0].Y
	bestScore := -domain.Infinity
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
	probes, hits := tt.Stats()
	var nps float64
	if elapsed > 0 {
		nps = float64(nodes) / float64(elapsed) * 1000
	}
	var ttHitRate float64
	if probes > 0 {
		ttHitRate = float64(hits) / float64(probes)
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
