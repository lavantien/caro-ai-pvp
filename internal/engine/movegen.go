package engine

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// generate fills the ply move list with candidate moves and their ordering
// scores, returning the count. Candidates are the free in-region cells within
// SearchRingRadius Chebyshev rings of any stone whose static window score is
// nonzero for either color: a cell with no live pattern interaction in any of
// its 8 centered windows cannot be a win, a block, or a development this
// horizon. Every forcing move survives by construction: a win-in-1 cell
// centers a Four window of its own color, a forced block centers the
// opponent's Four window. On any reachable non-full board with at least one
// stone the ring is never empty: grid cells of the empty and stone sets are
// adjacent across their boundary, and every ring contains its stones. When
// every candidate is quiet (opening positions with at most isolated stones
// classify every window None), the quiet cells are kept instead.
func (e *Engine) generate(b *rules.Board, ply int, ttm rules.Move) int {
	if b.MoveCount == 0 {
		e.moves[ply][0] = rules.Move(config.SearchEmptyBoardCell)
		e.scores[ply][0] = 0
		return 1
	}
	near := dilate(bitmask(b.Full), e.radius)
	anchor, constrained := openingAnchor(b)
	side := int(b.Side)
	n := e.fill(b, ply, ttm, side, anchor, constrained, near, true)
	if n == 0 {
		n = e.fill(b, ply, ttm, side, anchor, constrained, near, false)
	}
	return n
}

func (e *Engine) fill(b *rules.Board, ply int, ttm rules.Move, side int, anchor uint16, constrained bool, near bitmask, requireLive bool) int {
	n := 0
	for w := range near {
		free := near[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			cell := uint16(w*wordBits + bits.TrailingZeros64(bit))
			if constrained && chebyshevCell(anchor, cell) < config.OpeningChebyshevMin {
				continue
			}
			static := e.eval.cell[side][cell] + e.eval.cell[side^1][cell]
			if requireLive && static == 0 {
				continue
			}
			e.moves[ply][n] = rules.Move(cell)
			e.scores[ply][n] = e.orderScore(static, cell, ply, ttm, side)
			n++
		}
	}
	return n
}

// orderScore layers the ordering: transposition move, two killers, then the
// static threat plus denial value of the cell plus history. The static part
// reads the incremental per-cell window sums, so it costs two array reads,
// and its maximum stays below the killer layer by the config bounds.
func (e *Engine) orderScore(static int, cell uint16, ply int, ttm rules.Move, side int) int {
	if e.noOrder {
		return 0
	}
	m := rules.Move(cell)
	if m == ttm {
		return config.SearchOrderTT
	}
	if e.killers[ply][0] == m {
		return config.SearchOrderKiller1
	}
	if e.killers[ply][1] == m {
		return config.SearchOrderKiller2
	}
	return static + e.history[side][cell]
}

// pickMax swaps the i-th best remaining move into position i. Selection on
// demand: beta cutoffs usually happen after a handful of moves.
func pickMax(moves []rules.Move, scores []int, i, n int) {
	best := i
	for j := i + 1; j < n; j++ {
		if scores[j] > scores[best] {
			best = j
		}
	}
	if best != i {
		moves[i], moves[best] = moves[best], moves[i]
		scores[i], scores[best] = scores[best], scores[i]
	}
}

func (e *Engine) recordCutoff(m rules.Move, side rules.Color, ply, depth int) {
	if e.noOrder {
		return
	}
	if e.killers[ply][0] != m {
		e.killers[ply][1] = e.killers[ply][0]
		e.killers[ply][0] = m
	}
	e.history[side][m] += depth * depth
	if e.history[side][m] > config.SearchHistoryMax {
		for c := range e.history {
			for j := range e.history[c] {
				e.history[c][j] >>= 1
			}
		}
	}
}
