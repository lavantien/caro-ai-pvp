package rules

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func chebyshev(a, b Cell) int {
	dr := int(a/config.BoardStride) - int(b/config.BoardStride)
	dc := int(a%config.BoardStride) - int(b%config.BoardStride)
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}
	if dr < dc {
		return dc
	}
	return dr
}

func (b *Board) openingAnchor() (Cell, bool) {
	if b.Side != Red {
		return 0, false
	}
	var anchor Cell
	redStones := 0
	for w := range b.Red {
		if b.Red[w] != 0 {
			anchor = Cell(w*wordBits + bits.TrailingZeros64(b.Red[w]))
			redStones += bits.OnesCount64(b.Red[w])
		}
	}
	return anchor, redStones == 1
}

func (b *Board) IsLegal(cell Cell) bool {
	if int(cell) >= config.BoardCells {
		return false
	}
	w, m := bitOf(cell)
	if b.Full[w]&m != 0 || b.Region[w]&m == 0 {
		return false
	}
	if anchor, constrained := b.openingAnchor(); constrained && chebyshev(anchor, cell) < config.OpeningChebyshevMin {
		return false
	}
	return true
}

// LegalMoves fills buf with legal moves for the side to move and returns the
// count. buf must hold at least config.BoardCells entries.
func (b *Board) LegalMoves(buf []Move) int {
	anchor, constrained := b.openingAnchor()
	n := 0
	for w := range b.Region {
		free := b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			cell := Cell(w*wordBits + bits.TrailingZeros64(bit))
			if constrained && chebyshev(anchor, cell) < config.OpeningChebyshevMin {
				continue
			}
			buf[n] = Move(cell)
			n++
		}
	}
	return n
}
