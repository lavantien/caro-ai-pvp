package engine

import (
	"caro-ai-pvp/internal/domain"
)

var scoreTable = [6][3]int{
	{0, 0, 0},
	{0, 1, 10},
	{0, 10, 100},
	{0, 100, 1000},
	{0, 1000, 10000},
	{100000, 100000, 100000},
}

var evalDirections = [4][2]int{
	{1, 0}, {0, 1}, {1, 1}, {1, -1},
}

func Evaluate(sb *SearchBoard, player domain.Player) int {
	var total int
	opponent := player.Opponent()

	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if sb.PlayerAt(x, y) != player {
				continue
			}
			for _, dir := range evalDirections {
				dx, dy := dir[0], dir[1]
				consecutive, openEnds := countLine(sb, x, y, dx, dy, player)
				if consecutive > 0 && consecutive <= 5 {
					total += scoreTable[consecutive][openEnds]
				}
			}
		}
	}

	var opponentTotal int
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if sb.PlayerAt(x, y) != opponent {
				continue
			}
			for _, dir := range evalDirections {
				dx, dy := dir[0], dir[1]
				consecutive, openEnds := countLine(sb, x, y, dx, dy, opponent)
				if consecutive > 0 && consecutive <= 5 {
					opponentTotal += scoreTable[consecutive][openEnds]
				}
			}
		}
	}

	return total - int(float64(opponentTotal)*1.5) + centerBonus(sb, player)
}

func countLine(sb *SearchBoard, x, y, dx, dy int, player domain.Player) (consecutive, openEnds int) {
	px, py := x-dx, y-dy
	if px >= 0 && px < domain.BoardSize && py >= 0 && py < domain.BoardSize {
		if sb.PlayerAt(px, py) == player {
			return 0, 0
		}
	}

	for i := range 6 {
		nx, ny := x+dx*i, y+dy*i
		if nx < 0 || nx >= domain.BoardSize || ny < 0 || ny >= domain.BoardSize {
			break
		}
		if sb.PlayerAt(nx, ny) != player {
			break
		}
		consecutive++
	}

	endX, endY := x+dx*consecutive, y+dy*consecutive
	if endX >= 0 && endX < domain.BoardSize && endY >= 0 && endY < domain.BoardSize {
		if sb.IsEmpty(endX, endY) {
			openEnds++
		}
	}
	if px >= 0 && px < domain.BoardSize && py >= 0 && py < domain.BoardSize {
		if sb.IsEmpty(px, py) {
			openEnds++
		}
	}

	return
}

func centerBonus(sb *SearchBoard, player domain.Player) int {
	center := domain.BoardSize / 2
	bonus := 0
	playerBits := sb.BitBoardFor(player)
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if playerBits.Get(x, y) {
				dist := abs(x-center) + abs(y-center)
				bonus += (domain.BoardSize - dist) * 2
			}
		}
	}
	return bonus
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
