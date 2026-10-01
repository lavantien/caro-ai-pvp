package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// reverseIdx applies the window position permutation i -> PatternWindowLen-1-i.
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

// Transposition maps a horizontal window to the vertical window with identical
// position order: the induced permutation is the identity.
func TestTranspositionHorizontalVertical(t *testing.T) {
	for idx := range config.PatternTableEntries {
		if tables[0][idx] != tables[1][idx] {
			t.Fatalf("idx %#x: horizontal %+v vertical %+v", idx, tables[0][idx], tables[1][idx])
		}
	}
}

// Transposition fixes the main diagonal with the identity permutation, a
// trivial self map witnessed by TestAllDirectionsEqual. It maps the
// anti-diagonal to itself with the position order reversed (i -> 8-i), so the
// anti-diagonal table maps a reversed window to the reversed mask and the
// same class.
func TestDiagonalsUnderTransposition(t *testing.T) {
	for idx := range config.PatternTableEntries {
		e, rev := tables[3][idx], tables[3][reverseIdx(uint32(idx))]
		if rev.Win1 != reverseMask(e.Win1) || rev.Class != e.Class {
			t.Fatalf("anti diag idx %#x: reversed {%#x %d} want {%#x %d}", idx, rev.Win1, rev.Class, reverseMask(e.Win1), e.Class)
		}
	}
}

// The window-world semantics is direction free, so all 4 tables coincide as
// functions of the packed index.
func TestAllDirectionsEqual(t *testing.T) {
	for idx := range config.PatternTableEntries {
		for dir := 1; dir < config.PatternDirections; dir++ {
			if tables[0][idx] != tables[dir][idx] {
				t.Fatalf("idx %#x: dir 0 %+v dir %d %+v", idx, tables[0][idx], dir, tables[dir][idx])
			}
		}
	}
}

// The win predicate is invariant under reversing a line, so every table maps
// a reversed window to the reversed mask and the same class.
func TestReversalInvariance(t *testing.T) {
	for dir := range config.PatternDirections {
		for idx := range config.PatternTableEntries {
			e, rev := tables[dir][idx], tables[dir][reverseIdx(uint32(idx))]
			if rev.Win1 != reverseMask(e.Win1) || rev.Class != e.Class {
				t.Fatalf("dir %d idx %#x: reversed {%#x %d} want {%#x %d}", dir, idx, rev.Win1, rev.Class, reverseMask(e.Win1), e.Class)
			}
		}
	}
}
