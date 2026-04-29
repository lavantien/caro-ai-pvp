package domain

func IsValidSecondMove(b Board, x, y int) bool {
	if b.IsEmpty() {
		return true
	}

	redCount := 0
	blueCount := 0
	var firstRedX, firstRedY int
	for bx := range BoardSize {
		for by := range BoardSize {
			p := b.GetPlayerAt(bx, by)
			if p == PlayerRed {
				redCount++
				firstRedX, firstRedY = bx, by
			} else if p == PlayerBlue {
				blueCount++
			}
		}
	}

	if redCount != 1 || blueCount > 1 {
		return true
	}

	dx := x - firstRedX
	dy := y - firstRedY
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx >= OpenRuleMin || dy >= OpenRuleMin
}
