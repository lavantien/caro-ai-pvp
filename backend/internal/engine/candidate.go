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

func GetTacticalCandidates(sb *SearchBoard, player domain.Player) []domain.Position {
	allCandidates := GetCandidates(sb, 2)
	if len(allCandidates) == 0 {
		return nil
	}

	opponent := player.Opponent()
	tactical := make([]domain.Position, 0, len(allCandidates))

	for _, c := range allCandidates {
		if isTacticalMove(sb, c.X, c.Y, player, opponent) {
			tactical = append(tactical, c)
		}
	}

	return tactical
}

func isTacticalMove(sb *SearchBoard, x, y int, player, opponent domain.Player) bool {
	// Win: creates exactly-5 (Caro-valid)
	sb.MakeMove(x, y, player)
	if wouldWin(sb, x, y, player) {
		sb.UnmakeMove()
		return true
	}
	sb.UnmakeMove()

	// Block: opponent would win here
	sb.MakeMove(x, y, opponent)
	if wouldWin(sb, x, y, opponent) {
		sb.UnmakeMove()
		return true
	}
	sb.UnmakeMove()

	// Creates open four or block four (forcing: opponent must respond)
	if createsFourType(sb, x, y, player) {
		return true
	}

	// Blocks opponent's open four or block four
	if createsFourType(sb, x, y, opponent) {
		return true
	}

	// Creates or blocks open three
	if createsOpenThree(sb, x, y, player) {
		return true
	}
	if createsOpenThree(sb, x, y, opponent) {
		return true
	}

	return false
}

// createsFourType checks if placing player at (x,y) creates an open four or block four.
// Open four = 4 consecutive with both ends open. Block four = 4 consecutive with one end open.
// Both are forcing: opponent must block the open end or lose.
func createsFourType(sb *SearchBoard, x, y int, player domain.Player) bool {
	sb.MakeMove(x, y, player)
	defer sb.UnmakeMove()

	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		dx, dy := dir[0], dir[1]
		positive := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			positive++
		}
		negative := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			negative++
		}

		total := 1 + positive + negative
		if total == 4 {
			afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
			beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

			afterOpen := afterX >= 0 && afterX < domain.BoardSize && afterY >= 0 && afterY < domain.BoardSize && sb.IsEmpty(afterX, afterY)
			beforeOpen := beforeX >= 0 && beforeX < domain.BoardSize && beforeY >= 0 && beforeY < domain.BoardSize && sb.IsEmpty(beforeX, beforeY)

			if afterOpen || beforeOpen {
				return true
			}
		}
	}
	return false
}

// createsOpenFour checks if placing player at (x,y) creates an open four
// (4 in a row with both ends open).
func createsOpenFour(sb *SearchBoard, x, y int, player domain.Player) bool {
	sb.MakeMove(x, y, player)
	defer sb.UnmakeMove()

	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		dx, dy := dir[0], dir[1]
		positive := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			positive++
		}
		negative := 0
		for i := 1; i <= 4; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			negative++
		}

		total := 1 + positive + negative
		if total == 4 {
			afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
			beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

			afterOpen := afterX >= 0 && afterX < domain.BoardSize && afterY >= 0 && afterY < domain.BoardSize && sb.IsEmpty(afterX, afterY)
			beforeOpen := beforeX >= 0 && beforeX < domain.BoardSize && beforeY >= 0 && beforeY < domain.BoardSize && sb.IsEmpty(beforeX, beforeY)

			if afterOpen && beforeOpen {
				return true
			}
		}
	}
	return false
}

// createsOpenThree checks if placing player at (x,y) creates an open three
// (3 in a row with both ends open).
func createsOpenThree(sb *SearchBoard, x, y int, player domain.Player) bool {
	sb.MakeMove(x, y, player)
	defer sb.UnmakeMove()

	for _, dir := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		dx, dy := dir[0], dir[1]
		positive := 0
		for i := 1; i <= 3; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			positive++
		}
		negative := 0
		for i := 1; i <= 3; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize || sb.PlayerAt(nx, ny) != player {
				break
			}
			negative++
		}

		total := 1 + positive + negative
		if total == 3 {
			afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
			beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

			afterOpen := afterX >= 0 && afterX < domain.BoardSize && afterY >= 0 && afterY < domain.BoardSize && sb.IsEmpty(afterX, afterY)
			beforeOpen := beforeX >= 0 && beforeX < domain.BoardSize && beforeY >= 0 && beforeY < domain.BoardSize && sb.IsEmpty(beforeX, beforeY)

			if afterOpen && beforeOpen {
				return true
			}
		}
	}
	return false
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
		if dx >= domain.OpenRuleMin || dy >= domain.OpenRuleMin {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
