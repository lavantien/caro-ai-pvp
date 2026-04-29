package engine

import (
	"caro-ai-pvp/internal/domain"
	"sort"
)

const (
	ttMoveScore     = 10_000_000
	blockScore      = 2_000_000
	winMoveScore    = 5_000_000
	historyScoreCap = 300_000
	centerWeight    = 100
	proximityWeight = 10
)

type ScoredMove struct {
	Pos   domain.Position
	Score int
}

func OrderMoves(
	candidates []domain.Position,
	board *SearchBoard,
	player domain.Player,
	depth int,
	ttMove *domain.Position,
	heuristics *SearchHeuristics,
) []domain.Position {
	if len(candidates) <= 1 {
		return candidates
	}

	scored := make([]ScoredMove, len(candidates))

	for i, c := range candidates {
		score := 0

		if ttMove != nil && *ttMove == c {
			scored[i] = ScoredMove{c, ttMoveScore}
			continue
		}

		score += evaluateTactical(board, c.X, c.Y, player)
		score += heuristics.KillerScore(depth, c)

		h := heuristics.HistoryScore(player, c.X, c.Y) * 2
		if h > historyScoreCap {
			h = historyScoreCap
		}
		score += h

		center := domain.BoardSize / 2
		dist := abs(c.X-center) + abs(c.Y-center)
		score += (domain.BoardSize*2 - 4 - dist) * centerWeight

		score += proximityScore(board, c.X, c.Y) * proximityWeight

		scored[i] = ScoredMove{c, score}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	result := make([]domain.Position, len(scored))
	for i, s := range scored {
		result[i] = s.Pos
	}
	return result
}

func evaluateTactical(sb *SearchBoard, x, y int, player domain.Player) int {
	score := 0
	opponent := player.Opponent()

	sb.MakeMove(x, y, opponent)
	if wouldWin(sb, x, y, opponent) {
		score += blockScore
	}
	sb.UnmakeMove()

	sb.MakeMove(x, y, player)
	if wouldWin(sb, x, y, player) {
		score += winMoveScore
	}
	sb.UnmakeMove()

	return score
}

func wouldWin(sb *SearchBoard, x, y int, player domain.Player) bool {
	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		count := 1
		for i := 1; i < 6; i++ {
			if sb.PlayerAt(x+dir[0]*i, y+dir[1]*i) != player {
				break
			}
			count++
		}
		for i := 1; i < 6; i++ {
			if sb.PlayerAt(x-dir[0]*i, y-dir[1]*i) != player {
				break
			}
			count++
		}
		if count == 5 {
			return true
		}
	}
	return false
}

func proximityScore(sb *SearchBoard, x, y int) int {
	score := 0
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			nx, ny := x+dx, y+dy
			if nx >= 0 && nx < domain.BoardSize && ny >= 0 && ny < domain.BoardSize {
				p := sb.PlayerAt(nx, ny)
				if p == domain.PlayerRed || p == domain.PlayerBlue {
					score += 3
				}
			}
		}
	}
	return score
}
