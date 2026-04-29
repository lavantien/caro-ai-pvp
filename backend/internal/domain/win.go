package domain

type WinResult struct {
	HasWinner   bool
	Winner      Player
	WinningLine []Position
}

var noWin = WinResult{}

var winDirections = [4][2]int{
	{1, 0},
	{0, 1},
	{1, 1},
	{1, -1},
}

func CheckWin(b Board) WinResult {
	for x := range BoardSize {
		for y := range BoardSize {
			p := b.GetPlayerAt(x, y)
			if p == PlayerNone {
				continue
			}
			if result := checkWinFrom(b, x, y, p); result.HasWinner {
				return result
			}
		}
	}
	return noWin
}

func CheckWinFromMove(b Board, x, y int) WinResult {
	p := b.GetPlayerAt(x, y)
	if p == PlayerNone {
		return noWin
	}
	return checkWinFrom(b, x, y, p)
}

func checkWinFrom(b Board, x, y int, player Player) WinResult {
	for _, dir := range winDirections {
		dx, dy := dir[0], dir[1]

		positive := 0
		for i := 1; i <= WinLength; i++ {
			nx, ny := x+dx*i, y+dy*i
			if nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize {
				break
			}
			if b.GetPlayerAt(nx, ny) != player {
				break
			}
			positive++
		}

		negative := 0
		for i := 1; i <= WinLength; i++ {
			nx, ny := x-dx*i, y-dy*i
			if nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize {
				break
			}
			if b.GetPlayerAt(nx, ny) != player {
				break
			}
			negative++
		}

		total := 1 + positive + negative
		if total != WinLength {
			continue
		}

		// Caro: both ends blocked = no win
		afterX, afterY := x+dx*(positive+1), y+dy*(positive+1)
		beforeX, beforeY := x-dx*(negative+1), y-dy*(negative+1)

		afterBlocked := afterX < 0 || afterX >= BoardSize || afterY < 0 || afterY >= BoardSize ||
			b.GetPlayerAt(afterX, afterY) != PlayerNone
		beforeBlocked := beforeX < 0 || beforeX >= BoardSize || beforeY < 0 || beforeY >= BoardSize ||
			b.GetPlayerAt(beforeX, beforeY) != PlayerNone

		if afterBlocked && beforeBlocked {
			continue
		}

		startX := x - dx*negative
		startY := y - dy*negative
		line := make([]Position, WinLength)
		for i := range WinLength {
			line[i] = Position{X: startX + dx*i, Y: startY + dy*i}
		}
		return WinResult{HasWinner: true, Winner: player, WinningLine: line}
	}
	return noWin
}
