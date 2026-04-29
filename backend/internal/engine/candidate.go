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
