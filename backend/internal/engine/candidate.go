package engine

import (
	"caro-ai-pvp/internal/domain"
)

func GetCandidates(sb *SearchBoard, radius int) []domain.Position {
	occupied := sb.Occupied()
	if occupied.IsZero() {
		center := domain.BoardSize / 2
		candidates := make([]domain.Position, 0, 9)
		for dx := range 3 {
			for dy := range 3 {
				candidates = append(candidates, domain.Position{X: center + dx - 1, Y: center + dy - 1})
			}
		}
		return candidates
	}

	seen := make(map[int]bool)
	candidates := make([]domain.Position, 0, 64)

	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if !occupied.Get(x, y) {
				continue
			}
			for dx := -radius; dx <= radius; dx++ {
				for dy := -radius; dy <= radius; dy++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize {
						continue
					}
					idx := ny*domain.BoardSize + nx
					if seen[idx] || !sb.IsEmpty(nx, ny) {
						continue
					}
					seen[idx] = true
					candidates = append(candidates, domain.Position{X: nx, Y: ny})
				}
			}
		}
	}

	return candidates
}

func FilterOpenRule(candidates []domain.Position, sb *SearchBoard, player domain.Player) []domain.Position {
	if player != domain.PlayerRed {
		return candidates
	}

	redCount := 0
	blueCount := 0
	var firstRedX, firstRedY int
	for bx := range domain.BoardSize {
		for by := range domain.BoardSize {
			p := sb.PlayerAt(bx, by)
			if p == domain.PlayerRed {
				redCount++
				firstRedX, firstRedY = bx, by
			} else if p == domain.PlayerBlue {
				blueCount++
			}
		}
	}

	if redCount != 1 || blueCount > 1 {
		return candidates
	}

	filtered := make([]domain.Position, 0, len(candidates))
	for _, c := range candidates {
		dx := c.X - firstRedX
		dy := c.Y - firstRedY
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		if dx+dy >= domain.OpenRuleMin {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
