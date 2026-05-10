package engine

import (
	"caro-ai-pvp/internal/domain"
)

const (
	maxKillerDepth     = 64
	historyMax         = 1_000_000
	contHistMax        = 30_000
	boardCells         = domain.BoardSize * domain.BoardSize
	contHistBonusScale = 300
)

type SearchHeuristics struct {
	killerMoves    [maxKillerDepth][2]domain.Position
	historyRed     [domain.BoardSize][domain.BoardSize]int
	historyBlue    [domain.BoardSize][domain.BoardSize]int
	contHistory    [2][boardCells][boardCells]int
	counterMove    [2][boardCells]domain.Position
	lastMoveCell   int
}

func NewSearchHeuristics() *SearchHeuristics {
	return &SearchHeuristics{}
}

func (h *SearchHeuristics) RecordKiller(depth int, pos domain.Position) {
	if depth < 0 || depth >= maxKillerDepth {
		return
	}
	h.killerMoves[depth][1] = h.killerMoves[depth][0]
	h.killerMoves[depth][0] = pos
}

func (h *SearchHeuristics) IsKiller(depth int, pos domain.Position) bool {
	if depth < 0 || depth >= maxKillerDepth {
		return false
	}
	return h.killerMoves[depth][0] == pos || h.killerMoves[depth][1] == pos
}

func (h *SearchHeuristics) KillerScore(depth int, pos domain.Position) int {
	if depth < 0 || depth >= maxKillerDepth {
		return 0
	}
	if h.killerMoves[depth][0] == pos {
		return 500_000
	}
	if h.killerMoves[depth][1] == pos {
		return 400_000
	}
	return 0
}

func (h *SearchHeuristics) RecordHistory(player domain.Player, x, y, depth int) {
	if x < 0 || x >= domain.BoardSize || y < 0 || y >= domain.BoardSize {
		return
	}
	table := &h.historyRed
	if player == domain.PlayerBlue {
		table = &h.historyBlue
	}
	table[x][y] += depth * depth
	if table[x][y] > historyMax {
		table[x][y] = historyMax
	}
}

func (h *SearchHeuristics) HistoryScore(player domain.Player, x, y int) int {
	if x < 0 || x >= domain.BoardSize || y < 0 || y >= domain.BoardSize {
		return 0
	}
	if player == domain.PlayerRed {
		return h.historyRed[x][y]
	}
	return h.historyBlue[x][y]
}

func (h *SearchHeuristics) Clear() {
	*h = *NewSearchHeuristics()
}

func posToCell(x, y int) int {
	return y*domain.BoardSize + x
}

func playerIdx(p domain.Player) int {
	if p == domain.PlayerBlue {
		return 1
	}
	return 0
}

func (h *SearchHeuristics) RecordContHistory(player domain.Player, prevX, prevY, x, y, depth int) {
	if prevX < 0 || prevY < 0 || x < 0 || y < 0 {
		return
	}
	pi := playerIdx(player)
	prevCell := posToCell(prevX, prevY)
	cell := posToCell(x, y)
	bonus := depth * depth * contHistBonusScale / 100
	h.contHistory[pi][prevCell][cell] += bonus
	if h.contHistory[pi][prevCell][cell] > contHistMax {
		h.contHistory[pi][prevCell][cell] = contHistMax
	}
}

func (h *SearchHeuristics) ContHistoryScore(player domain.Player, prevX, prevY, x, y int) int {
	if prevX < 0 || prevY < 0 || x < 0 || y < 0 {
		return 0
	}
	pi := playerIdx(player)
	prevCell := posToCell(prevX, prevY)
	cell := posToCell(x, y)
	return h.contHistory[pi][prevCell][cell]
}

func (h *SearchHeuristics) RecordCounterMove(player domain.Player, oppX, oppY, x, y int) {
	if oppX < 0 || oppY < 0 || x < 0 || y < 0 {
		return
	}
	pi := playerIdx(player)
	oppCell := posToCell(oppX, oppY)
	h.counterMove[pi][oppCell] = domain.Position{X: x, Y: y}
}

func (h *SearchHeuristics) CounterMoveFor(player domain.Player, oppX, oppY int) domain.Position {
	if oppX < 0 || oppY < 0 {
		return domain.Position{X: -1, Y: -1}
	}
	pi := playerIdx(player)
	oppCell := posToCell(oppX, oppY)
	return h.counterMove[pi][oppCell]
}
