package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestZeroAllocs(t *testing.T) {
	b := NewBoard()
	buf := make([]Move, config.BoardCells)
	cell := oneCell(t, "H8")

	if n := testing.AllocsPerRun(100, func() {
		b.Make(cell)
		b.Unmake()
	}); n != 0 {
		t.Fatalf("make+unmake: %v allocs, want 0", n)
	}

	b.Make(cell)
	if n := testing.AllocsPerRun(100, func() {
		_ = b.Wins(Red)
		_ = b.Wins(Blue)
	}); n != 0 {
		t.Fatalf("Wins: %v allocs, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		_ = b.FastLastMoveWin(Red, cell)
	}); n != 0 {
		t.Fatalf("FastLastMoveWin: %v allocs, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		_ = b.LegalMoves(buf)
	}); n != 0 {
		t.Fatalf("LegalMoves: %v allocs, want 0", n)
	}

	a1 := oneCell(t, "A1")
	if n := testing.AllocsPerRun(100, func() {
		_ = b.IsLegal(a1)
	}); n != 0 {
		t.Fatalf("IsLegal: %v allocs, want 0", n)
	}
}

func TestZeroAllocsHex(t *testing.T) {
	board, buf, g := hexBenchBoard(t)
	last, err := g.ParseCell("L12")
	if err != nil {
		t.Fatalf("hex cell: %v", err)
	}
	lastMove := Cell(last)
	open, err := g.ParseCell("A12")
	if err != nil {
		t.Fatalf("hex cell: %v", err)
	}
	openCell := Cell(open)

	if n := testing.AllocsPerRun(100, func() {
		_ = board.FastLastMoveWin(Red, lastMove)
	}); n != 0 {
		t.Fatalf("hex FastLastMoveWin: %v allocs, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		board.Make(openCell)
		board.Unmake()
	}); n != 0 {
		t.Fatalf("hex make+unmake: %v allocs, want 0", n)
	}

	board.Make(openCell)
	if n := testing.AllocsPerRun(100, func() {
		_ = board.Wins(Red)
		_ = board.Wins(Blue)
	}); n != 0 {
		t.Fatalf("hex Wins: %v allocs, want 0", n)
	}

	if n := testing.AllocsPerRun(100, func() {
		_ = board.LegalMoves(buf)
	}); n != 0 {
		t.Fatalf("hex LegalMoves: %v allocs, want 0", n)
	}
}
