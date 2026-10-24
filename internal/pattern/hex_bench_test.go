package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func BenchmarkHexIndex(b *testing.B) {
	board := hexBoardFor(b)
	cell := rules.Cell(board.Geom.Center)
	i := 0
	for b.Loop() {
		_ = Index(board, cell, i%3, rules.Red)
		i++
	}
}

func BenchmarkHexIndexMovers(b *testing.B) {
	board := hexBoardFor(b)
	cell := rules.Cell(board.Geom.Center)
	i := 0
	for b.Loop() {
		_ = Index(board, cell, i%3, rules.Color((i>>2)&1))
		i++
	}
}
