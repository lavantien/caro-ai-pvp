package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

var hexBenchNames = [...]string{"L12", "L13", "M12", "M13", "G7", "G8", "H7", "P17", "P18", "Q17"}

func hexBenchBoard(tb testing.TB) (*Board, []Move, *geom.Geometry) {
	tb.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		tb.Fatalf("hex geometry: %v", err)
	}
	board := NewBoardFor(g)
	buf := make([]Move, config.MaxBoardCells)
	for _, name := range hexBenchNames {
		cell, err := g.ParseCell(name)
		if err != nil {
			tb.Fatalf("bench cell %q: %v", name, err)
		}
		board.Make(Cell(cell))
	}
	return board, buf, g
}

func BenchmarkHexMakeUnmake(b *testing.B) {
	board, _, g := hexBenchBoard(b)
	cell, err := g.ParseCell("A12")
	if err != nil {
		b.Fatalf("bench cell: %v", err)
	}
	for b.Loop() {
		board.Make(Cell(cell))
		board.Unmake()
	}
}

func BenchmarkHexWins(b *testing.B) {
	board, _, _ := hexBenchBoard(b)
	for b.Loop() {
		board.Wins(Red)
		board.Wins(Blue)
	}
}

func BenchmarkHexFastLastMoveWin(b *testing.B) {
	board, _, g := hexBenchBoard(b)
	cell, err := g.ParseCell("L12")
	if err != nil {
		b.Fatalf("bench cell: %v", err)
	}
	for b.Loop() {
		board.FastLastMoveWin(Red, Cell(cell))
	}
}

func BenchmarkHexLegalMoves(b *testing.B) {
	board, buf, _ := hexBenchBoard(b)
	for b.Loop() {
		board.LegalMoves(buf)
	}
}
