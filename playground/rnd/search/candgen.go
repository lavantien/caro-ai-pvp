package main

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type candTracker struct {
	count [config.BoardCells]uint8
	set   bb
}

func (t *candTracker) apply(cell int, delta int8) {
	r, c := cell/config.BoardStride, cell%config.BoardStride
	for dr := -candRadiusSearch; dr <= candRadiusSearch; dr++ {
		for dc := -candRadiusSearch; dc <= candRadiusSearch; dc++ {
			rr, cc := r+dr, c+dc
			if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
				continue
			}
			n := rr*config.BoardStride + cc
			if delta > 0 {
				t.count[n]++
				t.set[n/wordBits] |= 1 << (uint(n) % wordBits)
			} else {
				t.count[n]--
				if t.count[n] == 0 {
					t.set[n/wordBits] &^= 1 << (uint(n) % wordBits)
				}
			}
		}
	}
}

func (t *candTracker) make(cell rules.Cell) { t.apply(int(cell), +1) }

func (t *candTracker) unmake(cell rules.Cell) { t.apply(int(cell), -1) }

func (t *candTracker) matches(b *rules.Board) bool {
	want := dilateBB(bb(b.Full), candRadiusSearch)
	for w := range want {
		inSet := t.set[w] & b.Region[w] &^ b.Full[w]
		if inSet != want[w]&b.Region[w]&^b.Full[w] {
			return false
		}
	}
	return true
}
