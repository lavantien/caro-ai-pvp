package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
)

type SearchConfig struct {
	MaxDepth     int
	TimeLimitMs  int64
	Goroutines   int
	UseVCF       bool
	TimeFraction float64
}

func SearchPosition(
	b domain.Board,
	player domain.Player,
	config SearchConfig,
	tt *TranspositionTable,
	heuristics *SearchHeuristics,
	ctx context.Context,
) (int, int, SearchStats) {
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)
	candidates = FilterOpenRule(candidates, &sb, player)

	if len(candidates) == 0 {
		return -1, -1, SearchStats{}
	}
	if len(candidates) == 1 {
		return candidates[0].X, candidates[0].Y, SearchStats{}
	}

	bestX, bestY := candidates[0].X, candidates[0].Y
	monitor := NewTimeMonitor(ctx, config.TimeLimitMs)
	defer monitor.Stop()

	tt.ResetStats()
	bestScore := -domain.Infinity
	completedDepth := 0
	fullAlpha := -domain.Infinity
	fullBeta := domain.Infinity

	if config.UseVCF {
		oppSB := NewSearchBoard(b)
		if !opponentHasImmediateWin(&oppSB, player.Opponent()) {
			vcfTime := int64(float64(config.TimeLimitMs) * domain.VCFTimeFraction)
			if vx, vy, result := SolveVCF(b, player, vcfTime, ctx); result == VCFWin {
				return vx, vy, SearchStats{
					DepthAchieved:   0,
					SearchScore:     domain.WinScore,
					AllocatedTimeMs: config.TimeLimitMs,
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

	for depth := 1; depth <= config.MaxDepth; depth++ {
		if monitor.ShouldStop() {
			break
		}

		delta := domain.AspirationWindowSize
		a, b := fullAlpha, fullBeta
		if depth > 1 {
			a = max(bestScore-delta, fullAlpha)
			b = min(bestScore+delta, fullBeta)
		}

		var x, y, score int
		found := false
		for range domain.MaxAspirationAttempts {
			x, y, score = searchRoot(&sb, player, depth, a, b, tt, heuristics, candidates, monitor, vcfPreferred)
			if x < 0 || monitor.ShouldStop() {
				break
			}
			if score <= a && a > fullAlpha {
				a = max(a-delta, fullAlpha)
				delta *= 2
				continue
			}
			if score >= b && b < fullBeta {
				b = min(b+delta, fullBeta)
				delta *= 2
				continue
			}
			found = true
			break
		}

		if !found && !monitor.ShouldStop() {
			x, y, score = searchRoot(&sb, player, depth, fullAlpha, fullBeta, tt, heuristics, candidates, monitor, vcfPreferred)
			if x >= 0 {
				found = true
			}
		}

		if found {
			bestX, bestY = x, y
			bestScore = score
			completedDepth = depth
			if score >= domain.WinScore {
				break
			}
		}
	}

	elapsed := monitor.ElapsedMs()
	probes, hits := tt.Stats()
	nodes := monitor.Nodes.Load()
	var hitRate float64
	if probes > 0 {
		hitRate = float64(hits) / float64(probes)
	}
	var nps float64
	if elapsed > 0 {
		nps = float64(nodes) / float64(elapsed) * 1000
	}

	return bestX, bestY, SearchStats{
		DepthAchieved:   completedDepth,
		NodesSearched:   nodes,
		NodesPerSecond:  nps,
		SearchScore:     bestScore,
		TableHitRate:    hitRate,
		AllocatedTimeMs: config.TimeLimitMs,
		ThreadCount:     1,
	}
}

func searchRoot(
	sb *SearchBoard,
	player domain.Player,
	depth int,
	alpha, beta int,
	tt *TranspositionTable,
	heuristics *SearchHeuristics,
	candidates []domain.Position,
	monitor *TimeMonitor,
	preferredMove *domain.Position,
) (int, int, int) {
	monitor.Nodes.Add(1)
	staticEval := Evaluate(sb, player)
	var ttMove *domain.Position
	if preferredMove != nil {
		ttMove = preferredMove
	} else if entry, ok := tt.Lookup(sb.Hash()); ok {
		ttMove = &domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	}

	ordered := OrderMoves(candidates, sb, player, depth, ttMove, heuristics)

	bestScore := -domain.Infinity
	bestX, bestY := -1, -1

	for i, move := range ordered {
		if monitor.ShouldStop() {
			break
		}

		sb.MakeMove(move.X, move.Y, player)

		var score int
		if wouldWin(sb, move.X, move.Y, player) {
			score = domain.WinScore - 1
		} else if i == 0 {
			score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor, move, 1)
		} else {
			score = -alphaBeta(sb, player.Opponent(), depth-1, -alpha-1, -alpha, tt, heuristics, monitor, move, 1)
			if score > alpha && score < beta {
				score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor, move, 1)
			}
		}

		sb.UnmakeMove()

		if score > bestScore {
			bestScore = score
			bestX, bestY = move.X, move.Y
		}
		if score > alpha {
			alpha = score
		}
	}

	if bestX >= 0 && !monitor.ShouldStop() {
		tt.Store(TTEntry{
			Hash:       sb.Hash(),
			Score:      int32(adjustMateScoreForStore(bestScore, 0)),
			StaticEval: int32(staticEval),
			Depth:      uint8(depth),
			MoveX:      int8(bestX),
			MoveY:      int8(bestY),
			Flag:       TTExact,
		})
		heuristics.RecordKiller(depth, domain.Position{X: bestX, Y: bestY})
	}

	return bestX, bestY, bestScore
}

func alphaBeta(
	sb *SearchBoard,
	player domain.Player,
	depth int,
	alpha, beta int,
	tt *TranspositionTable,
	heuristics *SearchHeuristics,
	monitor *TimeMonitor,
	prevMove domain.Position,
	plyFromRoot int,
) int {
	monitor.Nodes.Add(1)
	if monitor.ShouldStop() {
		return 0
	}

	if depth <= 0 {
		return quiesce(sb, player, alpha, beta, domain.MaxQuiescenceDepth, heuristics, monitor, plyFromRoot)
	}

	origAlpha := alpha
	staticEval := Evaluate(sb, player)

	// Null-move pruning
	if depth >= domain.NullMoveMinDepth && staticEval >= beta {
		sb.MakeNullMove()
		nullPrev := domain.Position{X: -1, Y: -1}
		nullScore := -alphaBeta(sb, player.Opponent(), depth-1-domain.NullMoveReduction, -beta, -beta+1, tt, heuristics, monitor, nullPrev, plyFromRoot+1)
		sb.UnmakeNullMove()
		if nullScore >= beta && !monitor.ShouldStop() {
			return nullScore
		}
	}

	if entry, ok := tt.Lookup(sb.Hash()); ok && int(entry.Depth) >= depth {
		ttScore := adjustMateScoreForRetrieve(int(entry.Score), plyFromRoot)
		switch entry.Flag {
		case TTExact:
			return ttScore
		case TTLowerBound:
			if ttScore > alpha {
				alpha = ttScore
			}
		case TTUpperBound:
			if ttScore < beta {
				beta = ttScore
			}
		}
		if alpha >= beta {
			return ttScore
		}
	}

	candidates := GetCandidates(sb, domain.MaxSearchRadius)
	candidates = FilterOpenRule(candidates, sb, player)
	var ttMove *domain.Position
	if entry, ok := tt.Lookup(sb.Hash()); ok {
		ttMove = &domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	}

	picker := NewMovePicker(candidates, sb, player, depth, ttMove, heuristics, prevMove)

	bestScore := -domain.Infinity
	bestMoveX, bestMoveY := -1, -1
	moveIdx := 0

	for {
		move, ok := picker.Next()
		if !ok {
			break
		}
		if monitor.ShouldStop() {
			break
		}

		reduction := 0
		if depth >= domain.LMRMinDepth && moveIdx >= domain.LMRFullDepthMoves {
			reduction = 1
			if moveIdx > 8 {
				reduction = 2
			}
			histScore := heuristics.HistoryScore(player, move.X, move.Y)
			if histScore < 0 {
				reduction++
			}
			if reduction >= depth {
				reduction = depth - 1
			}
		}

		sb.MakeMove(move.X, move.Y, player)

		var score int
		if wouldWin(sb, move.X, move.Y, player) {
			score = domain.WinScore - plyFromRoot
		} else {
			newDepth := depth - 1 - reduction
			if moveIdx == 0 {
				score = -alphaBeta(sb, player.Opponent(), newDepth, -beta, -alpha, tt, heuristics, monitor, move, plyFromRoot+1)
			} else {
				score = -alphaBeta(sb, player.Opponent(), newDepth, -alpha-1, -alpha, tt, heuristics, monitor, move, plyFromRoot+1)
				if score > alpha && score < beta {
					score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor, move, plyFromRoot+1)
				}
			}
		}

		sb.UnmakeMove()

		if score > bestScore {
			bestScore = score
			bestMoveX, bestMoveY = move.X, move.Y
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			heuristics.RecordKiller(depth, move)
			heuristics.RecordHistory(player, move.X, move.Y, depth)
			heuristics.RecordContHistory(player, prevMove.X, prevMove.Y, move.X, move.Y, depth)
			if prevMove.X >= 0 {
				heuristics.RecordCounterMove(player, prevMove.X, prevMove.Y, move.X, move.Y)
			}
			break
		}
		moveIdx++
	}

	if !monitor.ShouldStop() {
		flag := TTExact
		if bestScore <= origAlpha {
			flag = TTUpperBound
		} else if bestScore >= beta {
			flag = TTLowerBound
		}
		tt.Store(TTEntry{
			Hash:       sb.Hash(),
			Score:      int32(adjustMateScoreForStore(bestScore, plyFromRoot)),
			StaticEval: int32(staticEval),
			Depth:      uint8(depth),
			MoveX:      int8(bestMoveX),
			MoveY:      int8(bestMoveY),
			Flag:       flag,
		})
	}

	return bestScore
}

func quiesce(
	sb *SearchBoard,
	player domain.Player,
	alpha, beta int,
	maxPly int,
	heuristics *SearchHeuristics,
	monitor *TimeMonitor,
	plyFromRoot int,
) int {
	monitor.Nodes.Add(1)
	if monitor.ShouldStop() {
		return 0
	}

	standPat := Evaluate(sb, player)
	if standPat >= beta {
		return beta
	}
	if standPat > alpha {
		alpha = standPat
	}
	if maxPly <= 0 {
		return standPat
	}

	candidates := GetTacticalCandidates(sb, player)
	for _, move := range candidates {
		if monitor.ShouldStop() {
			break
		}

		sb.MakeMove(move.X, move.Y, player)
		var score int
		if wouldWin(sb, move.X, move.Y, player) {
			score = domain.WinScore - plyFromRoot
		} else {
			score = -quiesce(sb, player.Opponent(), -beta, -alpha, maxPly-1, heuristics, monitor, plyFromRoot+1)
		}
		sb.UnmakeMove()

		if score >= beta {
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}

	return alpha
}

func isMateScore(score int) bool {
	return score > domain.WinScore-domain.AbsoluteMaxDepth ||
		score < -domain.WinScore+domain.AbsoluteMaxDepth
}

func adjustMateScoreForStore(score int, plyFromRoot int) int {
	if score > domain.WinScore-domain.AbsoluteMaxDepth {
		return score + plyFromRoot
	}
	if score < -domain.WinScore+domain.AbsoluteMaxDepth {
		return score - plyFromRoot
	}
	return score
}

func adjustMateScoreForRetrieve(storedScore int, plyFromRoot int) int {
	if storedScore >= domain.WinScore-domain.AbsoluteMaxDepth+1 {
		return storedScore - plyFromRoot
	}
	if storedScore <= -(domain.WinScore - domain.AbsoluteMaxDepth) - 1 {
		return storedScore + plyFromRoot
	}
	return storedScore
}
