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
	yielded    [4]uint64
	forced     bool
}

func (mp *MovePicker) markYielded(p domain.Position) {
	c := posToCell(p.X, p.Y)
	mp.yielded[c/64] |= 1 << (c % 64)
}

func (mp *MovePicker) alreadyYielded(p domain.Position) bool {
	c := posToCell(p.X, p.Y)
	return mp.yielded[c/64]&(1<<(c%64)) != 0
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
						mp.markYielded(c)
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
			if mp.alreadyYielded(m) {
				continue
			}
			mp.markYielded(m)
			return m, true
		}

		mp.staged = nil
		if mp.forced && mp.stage == stageMustBlock {
			// The opponent threatens a five: every non-forcing reply loses
			// on the spot, so the TT move, own wins (stageWinning), the
			// blocks, and the flank defenses exhaust the sensible moves.
			return domain.Position{}, false
		}
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
		out := mp.genMustBlock()
		mp.forced = len(out) > 0
		return out
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
		wins := wouldWin(mp.sb, c.X, c.Y, opponent)
		mp.sb.UnmakeMove()
		if !wins {
			continue
		}
		result = append(result, c)

		// Occupying a flank of the five this completion would create can
		// leave it both-ends-blocked, which does not win in caro: those
		// cells are real defenses and stay searchable.
		before, after := winFlanks(mp.sb, c.X, c.Y, opponent)
		for _, f := range [2]domain.Position{before, after} {
			if f.IsValid() && mp.sb.IsEmpty(f.X, f.Y) {
				result = append(result, f)
			}
		}
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
	own := analyzePlacement(mp.sb, x, y, mp.player)
	score := 0
	if own.openFour() {
		score += 700_000
	} else if own.four() {
		score += 400_000
	}
	if own.flex3 {
		score += 300_000
	}

	opponent := mp.player.Opponent()
	theirs := analyzePlacement(mp.sb, x, y, opponent)
	if theirs.openFour() {
		score += 500_000
	} else if theirs.four() {
		score += 350_000
	}
	if theirs.flex3 {
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
		if !mp.sb.IsEmpty(k.X, k.Y) {
			continue
		}
		result = append(result, k)
	}

	if mp.prevMove.X >= 0 && mp.prevMove.Y >= 0 {
		cm := mp.heuristics.CounterMoveFor(mp.player, mp.prevMove.X, mp.prevMove.Y)
		if cm.X >= 0 && cm.X < domain.BoardSize && cm.Y >= 0 && cm.Y < domain.BoardSize {
			if mp.sb.IsEmpty(cm.X, cm.Y) {
				result = append(result, cm)
			}
		}
	}

	return result
}

func (mp *MovePicker) genQuiet() []domain.Position {
	scored := make([]ScoredMove, 0, len(mp.candidates))
	for _, c := range mp.candidates {
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
	_, _, _, _, ok := winningFive(sb, x, y, player)
	return ok
}

// winningFive finds the exact five placing player at (x,y) completes and
// returns the cells flanking it. ok is false when the placement completes
// no winning five (caro: overlines and both-ends-blocked fives do not win).
func winningFive(sb *SearchBoard, x, y int, player domain.Player) (beforeX, beforeY, afterX, afterY int, ok bool) {
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

		ax, ay := x+dx*(positive+1), y+dy*(positive+1)
		bx, by := x-dx*(negative+1), y-dy*(negative+1)

		afterBlocked := ax < 0 || ax >= domain.BoardSize || ay < 0 || ay >= domain.BoardSize ||
			(sb.PlayerAt(ax, ay) != domain.PlayerNone && sb.PlayerAt(ax, ay) != player)
		beforeBlocked := bx < 0 || bx >= domain.BoardSize || by < 0 || by >= domain.BoardSize ||
			(sb.PlayerAt(bx, by) != domain.PlayerNone && sb.PlayerAt(bx, by) != player)

		if afterBlocked && beforeBlocked {
			continue
		}
		return bx, by, ax, ay, true
	}
	return 0, 0, 0, 0, false
}

// winFlanks returns the cells flanking the five that placing player at
// (x,y) would complete; zero Positions when the placement does not win.
func winFlanks(sb *SearchBoard, x, y int, player domain.Player) (domain.Position, domain.Position) {
	bx, by, ax, ay, ok := winningFive(sb, x, y, player)
	if !ok {
		return domain.Position{}, domain.Position{}
	}
	return domain.Position{X: bx, Y: by}, domain.Position{X: ax, Y: ay}
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
