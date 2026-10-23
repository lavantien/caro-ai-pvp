package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func BenchmarkHexIndex(b *testing.B) {
	board := hexBoardFor(b)
	cell := rules.Cell(board.Geom.Center)
	for i := range b.N {
		_ = Index(board, cell, i%3, rules.Red)
	}
}

func BenchmarkHexIndexMovers(b *testing.B) {
	board := hexBoardFor(b)
	cell := rules.Cell(board.Geom.Center)
	for i := range b.N {
		_ = Index(board, cell, i%3, rules.Color((i>>2)&1))
	}
}
