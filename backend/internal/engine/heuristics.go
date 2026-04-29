package engine

import (
	"caro-ai-pvp/internal/domain"
)

const (
	maxKillerDepth = 64
	historyMax     = 1_000_000
)

type SearchHeuristics struct {
	killerMoves [maxKillerDepth][2]domain.Position
	historyRed  [domain.BoardSize][domain.BoardSize]int
	historyBlue [domain.BoardSize][domain.BoardSize]int
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
