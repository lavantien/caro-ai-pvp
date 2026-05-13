package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
)

type VCFResult int

const (
	VCFNoWin   VCFResult = iota
	VCFWin
	VCFTimeout
)

type VCFSolver struct {
	sb       *SearchBoard
	attacker domain.Player
	monitor  *TimeMonitor
	winX     int
	winY     int
	timedOut bool
}

func SolveVCF(
	b domain.Board,
	player domain.Player,
	allocatedMs int64,
	ctx context.Context,
) (int, int, VCFResult) {
	sb := NewSearchBoard(b)
	monitor := NewTimeMonitor(ctx, allocatedMs)
	defer monitor.Stop()

	v := &VCFSolver{
		sb:       &sb,
		attacker: player,
		monitor:  monitor,
	}

	if v.search(domain.VCFSearchDepth) {
		return v.winX, v.winY, VCFWin
	}
	if v.timedOut {
		return -1, -1, VCFTimeout
	}
	return -1, -1, VCFNoWin
}

func (v *VCFSolver) search(depth int) bool {
	if v.monitor.ShouldStop() {
		v.timedOut = true
		return false
	}
	if depth <= 0 {
		return false
	}

	candidates := GetCandidates(v.sb, 2)

	for _, c := range candidates {
		if v.monitor.ShouldStop() {
			v.timedOut = true
			return false
		}

		v.sb.MakeMove(c.X, c.Y, v.attacker)

		if wouldWin(v.sb, c.X, c.Y, v.attacker) {
			v.sb.UnmakeMove()
			v.winX, v.winY = c.X, c.Y
			return true
		}

		blocks := findFourBlocks(v.sb, c.X, c.Y, v.attacker)
		if len(blocks) == 0 {
			v.sb.UnmakeMove()
			continue
		}

		// Opponent may have a winning response outside the blocking squares.
		if opponentHasImmediateWin(v.sb, v.attacker.Opponent()) {
			v.sb.UnmakeMove()
			continue
		}

		allWin := true
		for _, block := range blocks {
			v.sb.MakeMove(block.X, block.Y, v.attacker.Opponent())

			if wouldWin(v.sb, block.X, block.Y, v.attacker.Opponent()) {
				allWin = false
				v.sb.UnmakeMove()
				break
			}
			if !v.search(depth - 1) {
				allWin = false
				v.sb.UnmakeMove()
				break
			}
			v.sb.UnmakeMove()
		}

		v.sb.UnmakeMove()

		if v.timedOut {
			return false
		}

		if allWin {
			v.winX, v.winY = c.X, c.Y
			return true
		}
	}

	return false
}

func opponentHasImmediateWin(sb *SearchBoard, opponent domain.Player) bool {
	candidates := GetCandidates(sb, 2)
	for _, c := range candidates {
		sb.MakeMove(c.X, c.Y, opponent)
		wins := wouldWin(sb, c.X, c.Y, opponent)
		sb.UnmakeMove()
		if wins {
			return true
		}
	}
	return false
}

// findFourBlocks returns the cells the opponent must play to block a four
// created by placing attacker at (x,y). Returns empty if no four was created.
func findFourBlocks(sb *SearchBoard, x, y int, attacker domain.Player) []domain.Position {
	var blocks []domain.Position
	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		dx, dy := dir[0], dir[1]
		positive := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != attacker {
				break
			}
			positive++
		}
		negative := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != attacker {
				break
			}
			negative++
		}

		count := 1 + positive + negative
		if count != 4 {
			continue
		}

		afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
		beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

		afterOpen := afterX >= 0 && afterX < domain.BoardSize && afterY >= 0 && afterY < domain.BoardSize && sb.IsEmpty(afterX, afterY)
		beforeOpen := beforeX >= 0 && beforeX < domain.BoardSize && beforeY >= 0 && beforeY < domain.BoardSize && sb.IsEmpty(beforeX, beforeY)

		if afterOpen {
			// Check placing attacker here creates exactly 5, not 6+ (overline)
			beyondX, beyondY := afterX+dx, afterY+dy
			if beyondX < 0 || beyondX >= domain.BoardSize || beyondY < 0 || beyondY >= domain.BoardSize || sb.PlayerAt(beyondX, beyondY) != attacker {
				blocks = append(blocks, domain.Position{X: afterX, Y: afterY})
			}
		}
		if beforeOpen {
			beyondX, beyondY := beforeX-dx, beforeY-dy
			if beyondX < 0 || beyondX >= domain.BoardSize || beyondY < 0 || beyondY >= domain.BoardSize || sb.PlayerAt(beyondX, beyondY) != attacker {
				blocks = append(blocks, domain.Position{X: beforeX, Y: beforeY})
			}
		}
	}
	return blocks
}
