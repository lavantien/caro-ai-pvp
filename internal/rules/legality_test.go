package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func constrainedBoard(t *testing.T) (*Board, *NaiveBoard) {
	t.Helper()
	b, nb := NewBoard(), NewNaiveBoard()
	h8, a1 := oneCell(t, "H8"), oneCell(t, "A1")
	b.Make(h8)
	nb.Set(h8, Red)
	b.Make(a1)
	nb.Set(a1, Blue)
	return b, nb
}

func TestIsLegalOpeningOffsets(t *testing.T) {
	b, nb := constrainedBoard(t)
	h8r, h8c := 7, 7
	for dr := -4; dr <= 4; dr++ {
		for dc := -4; dc <= 4; dc++ {
			cheb := max(absInt(dr), absInt(dc))
			cell := Cell((h8r+dr)*config.BoardStride + h8c + dc)
			want := cheb >= config.OpeningChebyshevMin
			if got := b.IsLegal(cell); got != want {
				t.Fatalf("offset (%d,%d) cheb %d: IsLegal = %v, want %v", dr, dc, cheb, got, want)
			}
			if got := nb.IsLegal(Red, cell); got != want {
				t.Fatalf("offset (%d,%d): naive IsLegal = %v, want %v", dr, dc, got, want)
			}
		}
	}
	if b.IsLegal(oneCell(t, "I8")) {
		t.Fatal("distance 1 must be illegal during opening constraint")
	}
	if !b.IsLegal(oneCell(t, "K11")) {
		t.Fatal("pure diagonal distance 3 must be legal")
	}
	if !b.IsLegal(oneCell(t, "H11")) {
		t.Fatal("straight distance 3 must be legal")
	}
	if b.IsLegal(oneCell(t, "J10")) {
		t.Fatal("pure diagonal distance 2 must be illegal")
	}
}

func TestIsLegalConstraintExpires(t *testing.T) {
	b, _ := constrainedBoard(t)
	b.Make(oneCell(t, "P16"))
	if b.Side != Blue {
		t.Fatal("setup: blue to move")
	}
	if !b.IsLegal(oneCell(t, "I8")) {
		t.Fatal("blue is never constrained")
	}
	b.Make(oneCell(t, "B2"))
	if b.Side != Red || popcountBB(b.Red) != 2 {
		t.Fatal("setup: red to move with two stones")
	}
	if !b.IsLegal(oneCell(t, "I8")) {
		t.Fatal("constraint must be absent from red's third move on")
	}
}

func TestIsLegalUnusableCells(t *testing.T) {
	b, _ := constrainedBoard(t)
	if b.IsLegal(oneCell(t, "H8")) {
		t.Fatal("occupied cell must be illegal")
	}
	if b.IsLegal(oneCell(t, "A1")) {
		t.Fatal("occupied cell must be illegal")
	}
	if b.IsLegal(Cell(config.BoardCells)) {
		t.Fatal("out-of-bounds cell must be illegal")
	}
	cb := NewCrossCheck()
	cb.Make(0)
	if cb.IsLegal(oneCell(t, "A9")) {
		t.Fatal("out-of-region cell must be illegal")
	}
}

func TestIsLegalFirstMoveUnconstrained(t *testing.T) {
	b := NewBoard()
	if !b.IsLegal(oneCell(t, "A1")) || !b.IsLegal(oneCell(t, "P16")) {
		t.Fatal("red first move with zero red stones must be unconstrained")
	}
}

func TestLegalMovesEmptyBoard(t *testing.T) {
	buf := make([]Move, config.BoardCells)
	b := NewBoard()
	n := b.LegalMoves(buf)
	if n != config.BoardCells {
		t.Fatalf("empty full board: %d legal moves, want %d", n, config.BoardCells)
	}
	seen := make(map[Move]bool)
	for _, m := range buf[:n] {
		seen[m] = true
	}
	for cell := range config.BoardCells {
		if !seen[Move(cell)] {
			t.Fatalf("cell %d missing from legal moves", cell)
		}
	}
}

func TestLegalMovesOpeningConstraint(t *testing.T) {
	buf := make([]Move, config.BoardCells)
	b, _ := constrainedBoard(t)
	n := b.LegalMoves(buf)
	want := 0
	for cell := range config.BoardCells {
		dr := absInt(cell/config.BoardStride - 7)
		dc := absInt(cell%config.BoardStride - 7)
		if max(dr, dc) >= config.OpeningChebyshevMin && !b.Occupied(Cell(cell)) {
			want++
		}
	}
	if n != want {
		t.Fatalf("constrained: %d legal moves, want %d", n, want)
	}
	for _, m := range buf[:n] {
		if !b.IsLegal(Cell(m)) {
			t.Fatalf("LegalMoves returned illegal move %d", m)
		}
	}
	seen := make(map[Move]bool)
	for _, m := range buf[:n] {
		seen[m] = true
	}
	if seen[Move(oneCell(t, "I8"))] {
		t.Fatal("distance 1 cell must be filtered out")
	}
	if !seen[Move(oneCell(t, "K11"))] {
		t.Fatal("distance 3 cell must be present")
	}

	cb := NewCrossCheck()
	cb.Make(oneCell(t, "D4"))
	cb.Make(oneCell(t, "A1"))
	n = cb.LegalMoves(buf)
	want = 0
	for cell := range config.BoardCells {
		if !cb.inRegion(Cell(cell)) || cb.Occupied(Cell(cell)) {
			continue
		}
		dr := absInt(cell/config.BoardStride - 3)
		dc := absInt(cell%config.BoardStride - 3)
		if max(dr, dc) >= config.OpeningChebyshevMin {
			want++
		}
	}
	if n != want {
		t.Fatalf("constrained cross region: %d legal moves, want %d", n, want)
	}
}

func TestLegalMovesConstraintExpires(t *testing.T) {
	buf := make([]Move, config.BoardCells)
	b, _ := constrainedBoard(t)
	b.Make(oneCell(t, "P16"))
	b.Make(oneCell(t, "B2"))
	n := b.LegalMoves(buf)
	if n != config.BoardCells-4 {
		t.Fatalf("after constraint expires: %d moves, want %d", n, config.BoardCells-4)
	}
	seen := make(map[Move]bool)
	for _, m := range buf[:n] {
		seen[m] = true
	}
	if !seen[Move(oneCell(t, "I8"))] {
		t.Fatal("I8 must be legal once constraint expired")
	}
}
