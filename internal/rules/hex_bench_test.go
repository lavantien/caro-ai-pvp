package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

var hexBenchNames = [...]string{"L12", "L13", "M12", "M13", "G7", "G8", "H7", "P17", "P18", "Q17"}

func hexBenchBoard(b *testing.B) (*Board, []Move, *geom.Geometry) {
	b.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		b.Fatalf("hex geometry: %v", err)
	}
	board := NewBoardFor(g)
	buf := make([]Move, config.MaxBoardCells)
	for _, name := range hexBenchNames {
		cell, err := g.ParseCell(name)
		if err != nil {
			b.Fatalf("bench cell %q: %v", name, err)
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
	for range b.N {
		board.Make(Cell(cell))
		board.Unmake()
	}
}

func BenchmarkHexWins(b *testing.B) {
	board, _, _ := hexBenchBoard(b)
	for range b.N {
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
	for range b.N {
		board.FastLastMoveWin(Red, Cell(cell))
	}
}

func BenchmarkHexLegalMoves(b *testing.B) {
	board, buf, _ := hexBenchBoard(b)
	for range b.N {
		board.LegalMoves(buf)
	}
}
