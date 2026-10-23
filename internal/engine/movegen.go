package engine

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func (e *Engine) generate(b *rules.Board, ply int, ttm rules.Move) int {
	if b.MoveCount == 0 {
		e.moves[ply][0] = rules.Move(b.Geom.Center)
		e.scores[ply][0] = 0
		return 1
	}
	near := dilate(bitmask(b.Full), e.radius, b.Geom)
	anchor, constrained := openingAnchor(b)
	side := int(b.Side)
	minStatic := 0
	if e.eval.fours[side^1] > 0 {
		minStatic = config.PatternWeightFour
	}
	n := e.fill(b, ply, ttm, side, anchor, constrained, near, true, minStatic)
	if n == 0 {
		n = e.fill(b, ply, ttm, side, anchor, constrained, near, true, 0)
	}
	if n == 0 {
		n = e.fill(b, ply, ttm, side, anchor, constrained, near, false, 0)
	}
	return n
}

func (e *Engine) fill(b *rules.Board, ply int, ttm rules.Move, side int, anchor uint16, constrained bool, near bitmask, requireLive bool, minStatic int) int {
	n := 0
	for w := range b.Geom.Words {
		free := near[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			cell := uint16(w*wordBits + bits.TrailingZeros64(bit))
			if constrained && b.Geom.Distance(int(anchor), int(cell)) < b.Geom.OpeningMin {
				continue
			}
			static := e.eval.cell[side][cell] + e.eval.cell[side^1][cell]
			if requireLive && static == 0 {
				continue
			}
			if static < minStatic {
				continue
			}
			e.moves[ply][n] = rules.Move(cell)
			e.scores[ply][n] = e.orderScore(static, cell, ply, ttm, side)
			n++
		}
	}
	return n
}

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
