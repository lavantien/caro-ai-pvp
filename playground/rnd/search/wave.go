package main

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func wavefront(b *rules.Board, attacker rules.Color) []uint8 {
	dist := make([]uint8, config.BoardCells)
	for i := range dist {
		dist[i] = waveUnreachable
	}
	reach := dirReachBB(b, attacker)
	cands := dilateBB(stonesOf(b, attacker), candRadiusSolver)
	var queue [config.BoardCells]int
	qh, qt := 0, 0
	for w := range cands {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := w*wordBits + bits.TrailingZeros64(bit)
			if threatRank(b, rules.Cell(c), attacker, &reach) >= config.PatternClassFour {
				dist[c] = 0
				queue[qt] = c
				qt++
			}
		}
	}
	for qh < qt {
		c := queue[qh]
		qh++
		r, col := c/config.BoardStride, c%config.BoardStride
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				if dr == 0 && dc == 0 {
					continue
				}
				rr, cc := r+dr, col+dc
				if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
					continue
				}
				n := rr*config.BoardStride + cc
				if dist[n] != waveUnreachable || b.Full[n/wordBits]&(1<<(uint(n)%wordBits)) != 0 {
					continue
				}
				dist[n] = dist[c] + 1
				queue[qt] = n
				qt++
			}
		}
	}
	return dist
}
