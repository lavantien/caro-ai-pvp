package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var benchSink [config.PatternDirections][config.PatternTableEntries]Entry

func benchBoard(b *testing.B) *rules.Board {
	b.Helper()
	board := rules.NewBoard()
	for _, name := range [...]string{"H8", "H9", "I8", "I9", "C3", "C4", "D3", "M12", "M13", "N12"} {
		cell, err := rules.ParseCell(name)
		if err != nil {
			b.Fatalf("bench cell %q: %v", name, err)
		}
		board.Make(cell)
	}
	return board
}

func BenchmarkInit(b *testing.B) {
	for range b.N {
		buildTables(&benchSink)
	}
}

func BenchmarkIndex(b *testing.B) {
	board := benchBoard(b)
	cell := rules.Cell(config.BoardSize/2*config.BoardStride + config.BoardSize/2)
	for i := range b.N {
		_ = Index(board, cell, i&3, rules.Red)
	}
}

func BenchmarkIndexMovers(b *testing.B) {
	board := benchBoard(b)
	cell := rules.Cell(config.BoardSize/2*config.BoardStride + config.BoardSize/2)
	for i := range b.N {
		_ = Index(board, cell, i&3, rules.Color((i>>2)&1))
	}
}

func BenchmarkLookup(b *testing.B) {
	board := benchBoard(b)
	cell := rules.Cell(config.BoardSize/2*config.BoardStride + config.BoardSize/2)
	idx := Index(board, cell, 0, rules.Red)
	for range b.N {
		_ = Lookup(0, idx)
	}
}
