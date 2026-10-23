package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func hexMidgame(b *testing.B) (*rules.Board, *geom.Geometry) {
	b.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		b.Fatalf("hex geometry: %v", err)
	}
	board := rules.NewBoardFor(g)
	for _, name := range [...]string{"L12", "L13", "M12", "M13", "G7", "G8", "H7", "P17", "P18", "Q17"} {
		cell, err := g.ParseCell(name)
		if err != nil {
			b.Fatalf("bench cell %q: %v", name, err)
		}
		board.Make(rules.Cell(cell))
	}
	return board, g
}

func BenchmarkHexEvalMakeUnmake(b *testing.B) {
	board, g := hexMidgame(b)
	e := New(0)
	e.eval.reset(board)
	cell, err := g.ParseCell("J12")
	if err != nil {
		b.Fatalf("bench cell: %v", err)
	}
	for range b.N {
		e.eval.makeMove(rules.Blue, rules.Cell(cell))
		e.eval.unmakeMove(rules.Cell(cell))
	}
}

func BenchmarkHexEvalReset(b *testing.B) {
	board, _ := hexMidgame(b)
	e := New(0)
	for range b.N {
		e.eval.reset(board)
	}
}

func BenchmarkHexGenerate(b *testing.B) {
	board, _ := hexMidgame(b)
	e := New(0)
	e.eval.reset(board)
	for range b.N {
		_ = e.generate(board, 0, moveNone)
	}
}
