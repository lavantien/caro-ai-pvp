package main

import (
	"slices"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func boardWithMoves(t *testing.T, cells ...rules.Cell) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, c := range cells {
		if !b.IsLegal(c) {
			t.Fatalf("move %d illegal on %d stones", c, b.MoveCount)
		}
		b.Make(c)
	}
	return b
}

func TestDihedralTransformsAreInvolutivePermutations(t *testing.T) {
	for ti := range dihedral {
		seen := make(map[int]bool, config.BoardCells)
		for cell := 0; cell < config.BoardCells; cell++ {
			out := transformCell(ti, cell)
			if out < 0 || out >= config.BoardCells {
				t.Fatalf("transform %d maps cell %d out of board: %d", ti, cell, out)
			}
			if seen[out] {
				t.Fatalf("transform %d maps two cells to %d", ti, out)
			}
			seen[out] = true
		}
	}
}

func TestTransformPreservesChebyshevPairs(t *testing.T) {
	cases := []struct {
		r1, b1, r2 int
		legal      bool
	}{
		{7, 4, 200, true},
		{7, 4, 9, false},
		{119, 60, 0, true},
		{119, 60, 120, false},
	}
	for _, tc := range cases {
		pre := boardWithMoves(t, rules.Cell(tc.r1), rules.Cell(tc.b1))
		if got := pre.IsLegal(rules.Cell(tc.r2)); got != tc.legal {
			t.Fatalf("case %v: base legality %t contradicts expectation %t", tc, got, tc.legal)
		}
		for ti := range dihedral {
			tp, err := boardFromKey(transformKey(ti, keyOf(pre)))
			if err != nil {
				t.Fatalf("case %v transform %d key: %v", tc, ti, err)
			}
			tm := rules.Cell(transformMove(ti, rules.Move(tc.r2)))
			if got := tp.IsLegal(tm); got != tc.legal {
				t.Fatalf("case %v transform %d: legality %t diverges from base %t", tc, ti, got, tc.legal)
			}
		}
	}
}

func TestCanonicalKeyIsInvariantAndMinimal(t *testing.T) {
	b := boardWithMoves(t, rules.Cell(119), rules.Cell(72), rules.Cell(10))
	k := keyOf(b)
	c := canonicalKey(k)
	for ti := range dihedral {
		got := canonicalKey(transformKey(ti, k))
		if got != c {
			t.Fatalf("canonical key of transform %d differs from canonical of identity", ti)
		}
	}
	want := k
	for ti := range dihedral {
		v := transformKey(ti, k)
		if keyBefore(v, want) {
			want = v
		}
	}
	if c != want {
		t.Fatalf("canonical key %s differs from the transform minimum %s", hexKey(c[:])[:12], hexKey(want[:])[:12])
	}
}

func TestOpeningCellsTrueOrbitStructure(t *testing.T) {
	center := cellOrbit(119)
	if !slices.Equal(center, []int{119, 120, 135, 136}) {
		t.Errorf("center orbit = %v, want [119 120 135 136]", center)
	}
	diag := cellOrbit(102)
	if !slices.Equal(diag, []int{102, 105, 150, 153}) {
		t.Errorf("diagonal orbit = %v, want [102 105 150 153]", diag)
	}
	if got := len(cellOrbit(116)); got != 8 {
		t.Errorf("cell 116 orbit size = %d, want 8: the opening set is not symmetry derived", got)
	}
	if !slices.Contains(cellOrbit(116), 123) || slices.Contains(cellOrbit(116), 87) {
		t.Errorf("cell 116 orbit %v must contain 123 and must not contain 87", cellOrbit(116))
	}
	if sameOrbit(116, 87) {
		t.Error("cells 116 and 87 sit at different radii from center and share no orbit")
	}
	opening := config.BookOpeningCells[:]
	for _, c := range opening {
		if !slices.Contains(center, c) && !slices.Contains(diag, c) && !slices.Contains(cellOrbit(116), c) && !slices.Contains(cellOrbit(87), c) {
			t.Errorf("opening cell %d outside every documented orbit", c)
		}
	}
}

func TestSymmetricDuplicatesMergeUnderCanonicalKey(t *testing.T) {
	a := boardWithMoves(t, rules.Cell(119), rules.Cell(72))
	b := boardWithMoves(t, rules.Cell(120), rules.Cell(71))
	if canonicalBoardKey(a) != canonicalBoardKey(b) {
		t.Fatal("reflected position pair must merge under the canonical key")
	}
	c := boardWithMoves(t, rules.Cell(87), rules.Cell(4))
	if canonicalBoardKey(a) == canonicalBoardKey(c) {
		t.Fatal("unrelated position must not merge")
	}
}

func TestKeyVariantsSingleStoneFourDistinct(t *testing.T) {
	b := boardWithMoves(t, rules.Cell(102))
	if got := len(keyVariants(keyOf(b))); got != 4 {
		t.Errorf("single diagonal-stone variants = %d, want 4", got)
	}
	deep := boardWithMoves(t, rules.Cell(119), rules.Cell(72), rules.Cell(10), rules.Cell(200), rules.Cell(50))
	if got := len(keyVariants(keyOf(deep))); got != 8 {
		t.Errorf("generic position variants = %d, want 8", got)
	}
	if got := len(keyVariants(keyOf(rules.NewBoard()))); got != 1 {
		t.Errorf("empty board variants = %d, want 1", got)
	}
}

func TestTransformKeyRoundTripsThroughBoardLegality(t *testing.T) {
	b := boardWithMoves(t, rules.Cell(116), rules.Cell(153), rules.Cell(0))
	for ti := range dihedral {
		tb, err := boardFromKey(transformKey(ti, keyOf(b)))
		if err != nil {
			t.Fatalf("transform %d: %v", ti, err)
		}
		if tb.MoveCount != b.MoveCount || tb.Side != b.Side {
			t.Fatalf("transform %d changed parity or side", ti)
		}
	}
}
