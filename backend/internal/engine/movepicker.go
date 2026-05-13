package engine

import (
	"caro-ai-pvp/internal/domain"
	"sort"
)

const (
	ttMoveScore      = 10_000_000
	mustBlockScore   = 8_000_000
	winMoveScore     = 5_000_000
	threatScore      = 800_000
	killerScore0     = 500_000
	killerScore1     = 400_000
	counterMoveScore = 350_000
	historyScoreCap  = 300_000
	centerWeight     = 100
	proximityWeight  = 10
	goodQuietThreshold = 500
)

type ScoredMove struct {
	Pos   domain.Position
	Score int
}

type MovePicker struct {
	candidates []domain.Position
	sb         *SearchBoard
	player     domain.Player
	depth      int
	ttMove     *domain.Position
	heuristics *SearchHeuristics
	prevMove   domain.Position
	stage      int
	index      int
	staged     []domain.Position
}

const (
	stageTTMove = iota
	stageWinning
	stageMustBlock
	stageThreat
	stageKillerCounter
	stageQuiet
	stageDone
)

func NewMovePicker(
	candidates []domain.Position,
	sb *SearchBoard,
	player domain.Player,
	depth int,
	ttMove *domain.Position,
	heuristics *SearchHeuristics,
	prevMove domain.Position,
) *MovePicker {
	return &MovePicker{
		candidates: candidates,
		sb:         sb,
		player:     player,
		depth:      depth,
		ttMove:     ttMove,
		heuristics: heuristics,
		prevMove:   prevMove,
		stage:      stageTTMove,
	}
}

// Next returns the next move to search, or zero value with false if done.
func (mp *MovePicker) Next() (domain.Position, bool) {
	for {
		if mp.stage == stageTTMove {
			mp.stage = stageWinning
			if mp.ttMove != nil {
				for _, c := range mp.candidates {
					if c == *mp.ttMove {
						return c, true
					}
				}
			}
			continue
		}

		if mp.staged == nil {
			mp.staged = mp.generateStage()
			mp.index = 0
		}

		if mp.index < len(mp.staged) {
			m := mp.staged[mp.index]
			mp.index++
			if mp.stage < stageQuiet {
				if mp.ttMove != nil && m == *mp.ttMove {
					continue
				}
			}
			return m, true
		}

		mp.staged = nil
		mp.stage++
		if mp.stage >= stageDone {
			return domain.Position{}, false
		}
	}
}

func (mp *MovePicker) generateStage() []domain.Position {
	switch mp.stage {
	case stageWinning:
		return mp.genWinning()
	case stageMustBlock:
		return mp.genMustBlock()
	case stageThreat:
		return mp.genThreats()
	case stageKillerCounter:
		return mp.genKillerCounter()
	case stageQuiet:
		return mp.genQuiet()
	default:
		return nil
	}
}

func (mp *MovePicker) genMustBlock() []domain.Position {
	opponent := mp.player.Opponent()
	var result []domain.Position
	for _, c := range mp.candidates {
		if mp.ttMove != nil && c == *mp.ttMove {
			continue
		}
		mp.sb.MakeMove(c.X, c.Y, opponent)
		if wouldWin(mp.sb, c.X, c.Y, opponent) {
			result = append(result, c)
		}
		mp.sb.UnmakeMove()
	}
	return result
}

func (mp *MovePicker) genWinning() []domain.Position {
	var result []domain.Position
	for _, c := range mp.candidates {
		if mp.ttMove != nil && c == *mp.ttMove {
			continue
		}
		mp.sb.MakeMove(c.X, c.Y, mp.player)
		if wouldWin(mp.sb, c.X, c.Y, mp.player) {
			result = append(result, c)
		}
		mp.sb.UnmakeMove()
	}
	return result
}

func (mp *MovePicker) genThreats() []domain.Position {
	var result []ScoredMove
	for _, c := range mp.candidates {
		if mp.ttMove != nil && c == *mp.ttMove {
			continue
		}
		score := mp.threatScore(c.X, c.Y)
		if score > 0 {
			result = append(result, ScoredMove{c, score})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	out := make([]domain.Position, len(result))
	for i, s := range result {
		out[i] = s.Pos
	}
	return out
}

func (mp *MovePicker) threatScore(x, y int) int {
	score := 0
	if createsOpenFour(mp.sb, x, y, mp.player) {
		score += 700_000
	} else if createsFourType(mp.sb, x, y, mp.player) {
		score += 400_000
	}
	opponent := mp.player.Opponent()
	if createsOpenFour(mp.sb, x, y, opponent) {
		score += 500_000
	} else if createsFourType(mp.sb, x, y, opponent) {
		score += 350_000
	}
	if createsOpenThree(mp.sb, x, y, mp.player) {
		score += 300_000
	}
	if createsOpenThree(mp.sb, x, y, opponent) {
		score += 200_000
	}
	return score
}

func (mp *MovePicker) genKillerCounter() []domain.Position {
	var result []domain.Position
	for slot := range 2 {
		if mp.depth < 0 || mp.depth >= maxKillerDepth {
			continue
		}
		k := mp.heuristics.killerMoves[mp.depth][slot]
		if k.X < 0 || k.X >= domain.BoardSize || k.Y < 0 || k.Y >= domain.BoardSize {
			continue
		}
		if mp.ttMove != nil && k == *mp.ttMove {
			continue
		}
		if !mp.sb.IsEmpty(k.X, k.Y) {
			continue
		}
		found := false
		for _, r := range result {
			if r == k {
				found = true
				break
			}
		}
		if !found {
			result = append(result, k)
		}
	}

	if mp.prevMove.X >= 0 && mp.prevMove.Y >= 0 {
		cm := mp.heuristics.CounterMoveFor(mp.player, mp.prevMove.X, mp.prevMove.Y)
		if cm.X >= 0 && cm.X < domain.BoardSize && cm.Y >= 0 && cm.Y < domain.BoardSize {
			if mp.ttMove == nil || cm != *mp.ttMove {
				if mp.sb.IsEmpty(cm.X, cm.Y) {
					found := false
					for _, r := range result {
						if r == cm {
							found = true
							break
						}
					}
					if !found {
						result = append(result, cm)
					}
				}
			}
		}
	}

	return result
}

func (mp *MovePicker) genQuiet() []domain.Position {
	seen := make(map[domain.Position]bool)
	if mp.ttMove != nil {
		seen[*mp.ttMove] = true
	}

	scored := make([]ScoredMove, 0, len(mp.candidates))
	for _, c := range mp.candidates {
		if seen[c] {
			continue
		}

		score := mp.heuristics.HistoryScore(mp.player, c.X, c.Y) * 2
		if score > historyScoreCap {
			score = historyScoreCap
		}
		score += mp.heuristics.KillerScore(mp.depth, c)
		score += mp.heuristics.ContHistoryScore(mp.player, mp.prevMove.X, mp.prevMove.Y, c.X, c.Y)

		center := domain.BoardSize / 2
		dist := abs(c.X-center) + abs(c.Y-center)
		score += (domain.BoardSize*2 - 4 - dist) * centerWeight

		score += proximityScore(mp.sb, c.X, c.Y) * proximityWeight

		scored = append(scored, ScoredMove{c, score})
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	out := make([]domain.Position, len(scored))
	for i, s := range scored {
		out[i] = s.Pos
	}
	return out
}

// OrderMoves remains as the all-at-once fallback for root search.
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

	picker := NewMovePicker(candidates, board, player, depth, ttMove, heuristics, domain.Position{X: -1, Y: -1})
	var result []domain.Position
	for {
		m, ok := picker.Next()
		if !ok {
			break
		}
		result = append(result, m)
	}
	return result
}

func wouldWin(sb *SearchBoard, x, y int, player domain.Player) bool {
	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		dx, dy := dir[0], dir[1]
		positive := 0
		for i := 1; i <= 5; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			positive++
		}
		negative := 0
		for i := 1; i <= 5; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			negative++
		}

		if 1+positive+negative != domain.WinLength {
			continue
		}

		afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
		beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

		afterBlocked := afterX < 0 || afterX >= domain.BoardSize || afterY < 0 || afterY >= domain.BoardSize ||
			(sb.PlayerAt(afterX, afterY) != domain.PlayerNone && sb.PlayerAt(afterX, afterY) != player)
		beforeBlocked := beforeX < 0 || beforeX >= domain.BoardSize || beforeY < 0 || beforeY >= domain.BoardSize ||
			(sb.PlayerAt(beforeX, beforeY) != domain.PlayerNone && sb.PlayerAt(beforeX, beforeY) != player)

		if afterBlocked && beforeBlocked {
			continue
		}
		return true
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
