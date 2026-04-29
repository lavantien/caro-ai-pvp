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
) (int, int) {
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)
	candidates = FilterOpenRule(candidates, &sb, player)

	if len(candidates) == 0 {
		return -1, -1
	}
	if len(candidates) == 1 {
		return candidates[0].X, candidates[0].Y
	}

	bestX, bestY := candidates[0].X, candidates[0].Y
	monitor := NewTimeMonitor(ctx, config.TimeLimitMs)
	defer monitor.Stop()

	for depth := 1; depth <= config.MaxDepth; depth++ {
		if monitor.ShouldStop() {
			break
		}

		x, y, score := searchRoot(&sb, player, depth, tt, heuristics, candidates, monitor)
		if x >= 0 {
			bestX, bestY = x, y
			if score >= domain.WinScore {
				break
			}
		}
	}

	return bestX, bestY
}

func searchRoot(
	sb *SearchBoard,
	player domain.Player,
	depth int,
	tt *TranspositionTable,
	heuristics *SearchHeuristics,
	candidates []domain.Position,
	monitor *TimeMonitor,
) (int, int, int) {
	var ttMove *domain.Position
	if entry, ok := tt.Lookup(sb.Hash()); ok {
		ttMove = &domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	}

	ordered := OrderMoves(candidates, sb, player, depth, ttMove, heuristics)

	bestScore := -domain.WinScore * 2
	bestX, bestY := -1, -1
	alpha, beta := -domain.WinScore*2, domain.WinScore*2

	for i, move := range ordered {
		if monitor.ShouldStop() {
			break
		}

		sb.MakeMove(move.X, move.Y, player)

		var score int
		if i == 0 {
			score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor)
		} else {
			score = -alphaBeta(sb, player.Opponent(), depth-1, -alpha-1, -alpha, tt, heuristics, monitor)
			if score > alpha && score < beta {
				score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor)
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

	if bestX >= 0 {
		tt.Store(TTEntry{
			Hash:  sb.Hash(),
			Score: int32(bestScore),
			Depth: uint8(depth),
			MoveX: int8(bestX),
			MoveY: int8(bestY),
			Flag:  TTExact,
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
) int {
	if monitor.ShouldStop() {
		return 0
	}

	if depth <= 0 {
		return quiesce(sb, player, alpha, beta, domain.MaxQuiescenceDepth, heuristics, monitor)
	}

	origAlpha := alpha
	if entry, ok := tt.Lookup(sb.Hash()); ok && int(entry.Depth) >= depth {
		switch entry.Flag {
		case TTExact:
			return int(entry.Score)
		case TTLowerBound:
			if int(entry.Score) > alpha {
				alpha = int(entry.Score)
			}
		case TTUpperBound:
			if int(entry.Score) < beta {
				beta = int(entry.Score)
			}
		}
		if alpha >= beta {
			return int(entry.Score)
		}
	}

	candidates := GetCandidates(sb, 2)
	var ttMove *domain.Position
	if entry, ok := tt.Lookup(sb.Hash()); ok {
		ttMove = &domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	}
	ordered := OrderMoves(candidates, sb, player, depth, ttMove, heuristics)

	bestScore := -domain.WinScore * 2
	bestMoveX, bestMoveY := -1, -1

	for i, move := range ordered {
		if monitor.ShouldStop() {
			break
		}

		reduction := 0
		if depth >= domain.LMRMinDepth && i >= domain.LMRFullDepthMoves {
			reduction = 1
			if i > 8 {
				reduction = 2
			}
		}

		sb.MakeMove(move.X, move.Y, player)

		var score int
		newDepth := depth - 1 - reduction

		if i == 0 {
			score = -alphaBeta(sb, player.Opponent(), newDepth, -beta, -alpha, tt, heuristics, monitor)
		} else {
			score = -alphaBeta(sb, player.Opponent(), newDepth, -alpha-1, -alpha, tt, heuristics, monitor)
			if score > alpha && score < beta {
				score = -alphaBeta(sb, player.Opponent(), depth-1, -beta, -alpha, tt, heuristics, monitor)
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
			break
		}
	}

	flag := TTExact
	if bestScore <= origAlpha {
		flag = TTUpperBound
	} else if bestScore >= beta {
		flag = TTLowerBound
	}
	tt.Store(TTEntry{
		Hash:  sb.Hash(),
		Score: int32(bestScore),
		Depth: uint8(depth),
		MoveX: int8(bestMoveX),
		MoveY: int8(bestMoveY),
		Flag:  flag,
	})

	return bestScore
}

func quiesce(
	sb *SearchBoard,
	player domain.Player,
	alpha, beta int,
	maxPly int,
	heuristics *SearchHeuristics,
	monitor *TimeMonitor,
) int {
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

	candidates := GetCandidates(sb, 1)
	for _, move := range candidates {
		if monitor.ShouldStop() {
			break
		}

		sb.MakeMove(move.X, move.Y, player)
		score := -quiesce(sb, player.Opponent(), -beta, -alpha, maxPly-1, heuristics, monitor)
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
