package rules

import (
	"math/bits"
)

func (b *Board) openingAnchor() (Cell, bool) {
	if b.Side != Red {
		return 0, false
	}
	var anchor Cell
	redStones := 0
	for w := range b.Geom.Words {
		if b.Red[w] != 0 {
			anchor = Cell(w*wordBits + bits.TrailingZeros64(b.Red[w]))
			redStones += bits.OnesCount64(b.Red[w])
		}
	}
	return anchor, redStones == 1
}

func (b *Board) IsLegal(cell Cell) bool {
	if int(cell) >= b.Geom.Cells {
		return false
	}
	w, m := bitOf(cell)
	if b.Full[w]&m != 0 || b.Region[w]&m == 0 {
		return false
	}
	if anchor, constrained := b.openingAnchor(); constrained && b.Geom.Distance(int(anchor), int(cell)) < b.Geom.OpeningMin {
		return false
	}
	return true
}

func (b *Board) LegalMoves(buf []Move) int {
	anchor, constrained := b.openingAnchor()
	n := 0
	for w := range b.Geom.Words {
		free := b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			cell := Cell(w*wordBits + bits.TrailingZeros64(bit))
			if constrained && b.Geom.Distance(int(anchor), int(cell)) < b.Geom.OpeningMin {
				continue
			}
			buf[n] = Move(cell)
			n++
		}
	}
	return n
}
