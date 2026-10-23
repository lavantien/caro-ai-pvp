package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func reverseIdx(idx uint32) uint32 {
	var out uint32
	for i := range config.PatternWindowLen {
		out |= (idx >> (uint(i) * config.PatternCellBits) & 3) << (uint(config.PatternWindowLen-1-i) * config.PatternCellBits)
	}
	return out
}

func reverseMask(m uint16) uint16 {
	var out uint16
	for i := range config.PatternWindowLen {
		if m&(1<<uint(i)) != 0 {
			out |= 1 << uint(config.PatternWindowLen-1-i)
		}
	}
	return out
}

func TestAntiDiagonalReversalMapsTable(t *testing.T) {
	anti := dirTable(t, 3)
	for idx := range config.PatternTableEntries {
		e, rev := anti[idx], anti[reverseIdx(uint32(idx))]
		if rev.Win1 != reverseMask(e.Win1) || rev.Class != e.Class {
			t.Fatalf("idx %#x: reversed {%#x %d} want {%#x %d}", idx, rev.Win1, rev.Class, reverseMask(e.Win1), e.Class)
		}
	}
}

func TestReversalInvariance(t *testing.T) {
	for dir := range config.PatternDirections {
		dt := dirTable(t, dir)
		for idx := range config.PatternTableEntries {
			e, rev := dt[idx], dt[reverseIdx(uint32(idx))]
			if rev.Win1 != reverseMask(e.Win1) || rev.Class != e.Class {
				t.Fatalf("dir %d idx %#x: reversed {%#x %d} want {%#x %d}", dir, idx, rev.Win1, rev.Class, reverseMask(e.Win1), e.Class)
			}
		}
	}
}
