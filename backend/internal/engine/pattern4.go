package engine

import (
	"caro-ai-pvp/internal/domain"
)

type Pattern4 int

const (
	P4None       Pattern4 = 0
	P4Flex1      Pattern4 = 1
	P4Block1     Pattern4 = 1
	P4Flex2      Pattern4 = 2
	P4Block2     Pattern4 = 2
	P4Flex3      Pattern4 = 4
	P4Block3     Pattern4 = 3
	P4Flex4      Pattern4 = 8
	P4Block4     Pattern4 = 4
	P4Exactly5   Pattern4 = 64
	P4Overline   Pattern4 = 0
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

// classifyDirection classifies the pattern formed by player's stones in direction (dx,dy)
// starting from the stone at (x,y).
func classifyDirection(sb *SearchBoard, x, y, dx, dy int, player domain.Player) Pattern4 {
	positive := 0
	positiveOpen := false
	for i := 1; i <= 5; i++ {
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

	negative := 0
	negativeOpen := false
	for i := 1; i <= 5; i++ {
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
	openEnds := 0
	if positiveOpen {
		openEnds++
	}
	if negativeOpen {
		openEnds++
	}

	if count >= 6 {
		return P4Overline
	}

	if count == 5 {
		afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
		beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

		afterBlocked := afterX < 0 || afterX >= domain.BoardSize || afterY < 0 || afterY >= domain.BoardSize ||
			(sb.PlayerAt(afterX, afterY) != domain.PlayerNone && sb.PlayerAt(afterX, afterY) != player)
		beforeBlocked := beforeX < 0 || beforeX >= domain.BoardSize || beforeY < 0 || beforeY >= domain.BoardSize ||
			(sb.PlayerAt(beforeX, beforeY) != domain.PlayerNone && sb.PlayerAt(beforeX, beforeY) != player)

		if afterBlocked && beforeBlocked {
			return P4None
		}
		return P4Exactly5
	}

	switch {
	case count == 4:
		switch openEnds {
		case 2:
			return P4Flex4
		case 1:
			return P4Block4
		default:
			return P4None
		}
	case count == 3:
		switch openEnds {
		case 2:
			return P4Flex3
		case 1:
			return P4Block3
		default:
			return P4None
		}
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
// where a same-color stone precedes the current one.
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

		classifyAndAccumulate(sb, x, y, dx, dy, player, &pp)
	}
	return pp
}

func classifyAndAccumulate(sb *SearchBoard, x, y, dx, dy int, player domain.Player, pp *PlayerPattern4) {
	positive := 0
	positiveOpen := false
	for i := 1; i <= 5; i++ {
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

	negative := 0
	negativeOpen := false
	for i := 1; i <= 5; i++ {
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
	openEnds := 0
	if positiveOpen {
		openEnds++
	}
	if negativeOpen {
		openEnds++
	}

	if count >= 6 {
		return
	}
	if count == 5 {
		afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
		beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)
		afterBlocked := afterX < 0 || afterX >= domain.BoardSize || afterY < 0 || afterY >= domain.BoardSize ||
			(sb.PlayerAt(afterX, afterY) != domain.PlayerNone && sb.PlayerAt(afterX, afterY) != player)
		beforeBlocked := beforeX < 0 || beforeX >= domain.BoardSize || beforeY < 0 || beforeY >= domain.BoardSize ||
			(sb.PlayerAt(beforeX, beforeY) != domain.PlayerNone && sb.PlayerAt(beforeX, beforeY) != player)
		if afterBlocked && beforeBlocked {
			return
		}
		pp.Exactly5Count++
		return
	}

	switch {
	case count == 4:
		if openEnds == 2 {
			pp.Flex4Count++
		} else if openEnds == 1 {
			pp.Block4Count++
		}
	case count == 3:
		if openEnds == 2 {
			pp.Flex3Count++
		} else if openEnds == 1 {
			pp.Block3Count++
		}
	case count == 2:
		if openEnds == 2 {
			pp.Flex2Count++
		} else if openEnds == 1 {
			pp.Block2Count++
		}
	}
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
