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
