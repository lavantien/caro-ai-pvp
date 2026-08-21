package engine

import (
	"caro-ai-pvp/internal/domain"
)

// Placing or removing a stone can only change the classification of stones
// within five cells along the four evaluation directions through the
// changed cell (classification windows are eleven cells wide). Reclassifying
// that fixed region on every mutation keeps the per-player aggregates exact
// while turning Evaluate into O(1) reads.

func (sb *SearchBoard) patterns(p domain.Player) PlayerPattern4 {
	return sb.patternAgg[p]
}

func (sb *SearchBoard) subtractPatternsAround(x, y int) {
	sb.adjustPatternsAround(x, y, -1)
}

func (sb *SearchBoard) addPatternsAround(x, y int) {
	sb.adjustPatternsAround(x, y, 1)
}

func (sb *SearchBoard) adjustPatternsAround(x, y int, sign int) {
	sb.adjustStonePattern(x, y, sign)
	for _, dir := range evalDirs {
		for k := 1; k <= 5; k++ {
			nx, ny := x+dir[0]*k, y+dir[1]*k
			if sb.patternInBounds(nx, ny) {
				sb.adjustStonePattern(nx, ny, sign)
			}
			mx, my := x-dir[0]*k, y-dir[1]*k
			if sb.patternInBounds(mx, my) {
				sb.adjustStonePattern(mx, my, sign)
			}
		}
	}
}

func (sb *SearchBoard) adjustStonePattern(x, y int, sign int) {
	p := sb.cells[x*domain.BoardSize+y]
	if p == domain.PlayerNone {
		return
	}
	sb.patternAgg[p].accumulate(ClassifyStone(sb, x, y, p), sign)
	sb.centerSum[p] += sign * cellCenterBonus(x, y)
	sb.stoneCount[p] += sign
}

func (sb *SearchBoard) patternInBounds(x, y int) bool {
	return x >= 0 && x < domain.BoardSize && y >= 0 && y < domain.BoardSize
}

func cellCenterBonus(x, y int) int {
	center := domain.BoardSize / 2
	return (domain.BoardSize - abs(x-center) - abs(y-center)) * 2
}
