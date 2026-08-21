package engine

import (
	"caro-ai-pvp/internal/domain"
)

type Pattern4 int

// Values must stay distinct: the previous enum aliased P4Flex3 with P4Block4
// and P4Overline with P4None, which silently corrupted equality-based checks.
const (
	P4None     Pattern4 = 0
	P4Flex1    Pattern4 = 1
	P4Flex2    Pattern4 = 3
	P4Block2   Pattern4 = 4
	P4Flex3    Pattern4 = 5
	P4Block3   Pattern4 = 6
	P4Flex4    Pattern4 = 7
	P4Block4   Pattern4 = 8
	P4Exactly5 Pattern4 = 9
	P4Overline Pattern4 = 10
)

var evalDirs = [4][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}

type PlayerPattern4 struct {
	Exactly5Count int
	Flex4Count    int
	Block4Count   int
	Flex3Count    int
	Block3Count   int
	Flex2Count    int
	Block2Count   int
}

// accumulate adds (or, with sign -1, subtracts) another count set.
func (pp *PlayerPattern4) accumulate(other PlayerPattern4, sign int) {
	pp.Exactly5Count += sign * other.Exactly5Count
	pp.Flex4Count += sign * other.Flex4Count
	pp.Block4Count += sign * other.Block4Count
	pp.Flex3Count += sign * other.Flex3Count
	pp.Block3Count += sign * other.Block3Count
	pp.Flex2Count += sign * other.Flex2Count
	pp.Block2Count += sign * other.Block2Count
}

// classifyDirection classifies the pattern the stone at (x,y) participates in
// along (dx,dy), gap-aware: split fours and broken threes count like their
// straight equivalents.
func classifyDirection(sb *SearchBoard, x, y, dx, dy int, player domain.Player) Pattern4 {
	line := extractLine(sb, x, y, player, dx, dy)

	lo, hi := spanThrough(line, -1)
	if hi-lo+1 > domain.WinLength {
		return P4Overline
	}
	if spanIsFive(line, lo, hi) {
		return P4Exactly5
	}

	comps := lineCompletions(line)
	switch comps {
	case 0:
	case 1:
		return P4Block4
	default:
		return P4Flex4
	}

	switch maxCompsAfterFill(line) {
	case 0:
	case 1:
		return P4Block3
	default:
		return P4Flex3
	}

	// Twos and singles: contiguous counting is sufficient.
	positive, positiveOpen := 0, false
	for i := 1; i <= 2; i++ {
		nx, ny := x+dx*i, y+dy*i
		if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize {
			break
		}
		p := sb.PlayerAt(nx, ny)
		if p == player {
			positive++
		} else if p == domain.PlayerNone {
			positiveOpen = true
			break
		} else {
			break
		}
	}

	negative, negativeOpen := 0, false
	for i := 1; i <= 2; i++ {
		nx, ny := x-dx*i, y-dy*i
		if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize {
			break
		}
		p := sb.PlayerAt(nx, ny)
		if p == player {
			negative++
		} else if p == domain.PlayerNone {
			negativeOpen = true
			break
		} else {
			break
		}
	}

	count := 1 + positive + negative
	if count >= 3 {
		return P4None
	}
	openEnds := 0
	if positiveOpen {
		openEnds++
	}
	if negativeOpen {
		openEnds++
	}

	switch {
	case count == 2:
		switch openEnds {
		case 2:
			return P4Flex2
		case 1:
			return P4Block2
		default:
			return P4None
		}
	case count == 1:
		return P4Flex1
	}
	return P4None
}

// ClassifyStone classifies all 4-direction patterns for a single stone.
// Only processes each line once (from the starting stone) by skipping directions
// where a same-color stone precedes the current one. Shapes below four whose
// cluster is anchored by a same-color stone two cells back are also skipped to
// avoid double counting gapped clusters (XX.X anchors at its leftmost stone).
func ClassifyStone(sb *SearchBoard, x, y int, player domain.Player) PlayerPattern4 {
	var pp PlayerPattern4
	for _, dir := range evalDirs {
		dx, dy := dir[0], dir[1]

		px, py := x-dx, y-dy
		if px >= 0 && px < domain.BoardSize && py >= 0 && py < domain.BoardSize {
			if sb.PlayerAt(px, py) == player {
				continue
			}
		}

		p2x, p2y := x-2*dx, y-2*dy
		clusterAnchored := p2x >= 0 && p2x < domain.BoardSize && p2y >= 0 && p2y < domain.BoardSize &&
			sb.PlayerAt(p2x, p2y) == player

		class := classifyDirection(sb, x, y, dx, dy, player)
		if clusterAnchored && class != P4Exactly5 && class != P4Flex4 {
			continue
		}
		switch class {
		case P4Exactly5:
			pp.Exactly5Count++
		case P4Flex4:
			pp.Flex4Count++
		case P4Block4:
			pp.Block4Count++
		case P4Flex3:
			pp.Flex3Count++
		case P4Block3:
			pp.Block3Count++
		case P4Flex2:
			pp.Flex2Count++
		case P4Block2:
			pp.Block2Count++
		}
	}
	return pp
}

// ClassifyBoard classifies all patterns for a player across the entire board.
func ClassifyBoard(sb *SearchBoard, player domain.Player) PlayerPattern4 {
	var total PlayerPattern4
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if sb.PlayerAt(x, y) != player {
				continue
			}
			pp := ClassifyStone(sb, x, y, player)
			total.Exactly5Count += pp.Exactly5Count
			total.Flex3Count += pp.Flex3Count
			total.Flex4Count += pp.Flex4Count
			total.Block4Count += pp.Block4Count
			total.Block3Count += pp.Block3Count
			total.Flex2Count += pp.Flex2Count
			total.Block2Count += pp.Block2Count
		}
	}
	return total
}

// hasDoubleFlex3 returns true if a single move creates two or more open threes.
func hasDoubleFlex3(sb *SearchBoard, x, y int, player domain.Player) bool {
	sb.MakeMove(x, y, player)
	defer sb.UnmakeMove()

	flex3Count := 0
	for _, dir := range evalDirs {
		dx, dy := dir[0], dir[1]
		p := classifyDirection(sb, x, y, dx, dy, player)
		if p == P4Flex3 {
			flex3Count++
		}
	}
	return flex3Count >= 2
}

// hasFlex4PlusFlex3 returns true if a single move creates both open four and open three.
func hasFlex4PlusFlex3(sb *SearchBoard, x, y int, player domain.Player) bool {
	sb.MakeMove(x, y, player)
	defer sb.UnmakeMove()

	flex4 := false
	flex3 := false
	for _, dir := range evalDirs {
		dx, dy := dir[0], dir[1]
		p := classifyDirection(sb, x, y, dx, dy, player)
		if p == P4Flex4 {
			flex4 = true
		}
		if p == P4Flex3 {
			flex3 = true
		}
	}
	return flex4 && flex3
}
