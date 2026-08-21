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

	// Stack-allocated dedup: a heap map per node dominated the profile.
	var seen [domain.BoardSize * domain.BoardSize]bool
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

	// Four for either side (creating or blocking), plus double threats: a
	// move creating a four-or-open-three shape in two directions at once is
	// forcing, because the opponent cannot answer both lines with one stone.
	// A lone open three stays non-forcing (the opponent may convert or
	// ignore it), so it stays visible to eval and move ordering only.
	ownThreatDirs, oppThreatDirs := 0, 0
	for _, dir := range evalDirs {
		line := extractLine(sb, x, y, player, dir[0], dir[1])
		oppLine := negateLine(line)
		if lineCompletions(line) >= 1 || lineCompletions(oppLine) >= 1 {
			return true
		}
		if maxCompsAfterFill(line) >= 2 {
			ownThreatDirs++
		}
		if maxCompsAfterFill(oppLine) >= 2 {
			oppThreatDirs++
		}
	}
	return ownThreatDirs >= 2 || oppThreatDirs >= 2
}

// createsFourType, createsOpenFour and createsOpenThree live in pattern_window.go.

func FilterOpenRule(candidates []domain.Position, sb *SearchBoard, player domain.Player) []domain.Position {
	if player != domain.PlayerRed {
		return candidates
	}
	// The rule only constrains red's second move: at most two stones exist
	// then. Skip the full-board scan everywhere else.
	if sb.StoneCount() > 2 {
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
