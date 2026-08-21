package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
)

type VCFResult int

const (
	VCFNoWin VCFResult = iota
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
	return SolveVCFWithDepth(b, player, domain.VCFSearchDepth, allocatedMs, ctx)
}

// SolveVCFWithDepth bounds the forcing chain length: depth counts attacker
// moves, so depth 1 sees only immediate fours. It is the per-level tactical
// sight knob behind DifficultyProfile.VCFDepth.
func SolveVCFWithDepth(
	b domain.Board,
	player domain.Player,
	depth int,
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

	if depth <= 0 {
		depth = domain.VCFSearchDepth
	}
	if v.search(depth) {
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
// created by placing attacker at (x,y): every empty cell whose fill would
// complete an exact five for the attacker, gapped or straight. Returns empty
// if no four was created.
func findFourBlocks(sb *SearchBoard, x, y int, attacker domain.Player) []domain.Position {
	var blocks []domain.Position
	seen := make(map[domain.Position]bool)
	for _, dir := range evalDirs {
		for _, c := range fiveCompletionsInDir(sb, x, y, attacker, dir[0], dir[1]) {
			if !seen[c] {
				seen[c] = true
				blocks = append(blocks, c)
			}
		}
	}
	return blocks
}
