package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func benchBoard(b *testing.B) (*Board, []Move) {
	board := NewBoard()
	buf := make([]Move, config.BoardCells)
	for _, c := range cellsOf(b, "H8", "H9", "I8", "I9", "C3", "C4", "D3", "M12", "M13", "N12") {
		board.Make(c)
	}
	return board, buf
}

func BenchmarkMakeUnmake(b *testing.B) {
	board, _ := benchBoard(b)
	cell := cellsOf(b, "A1")[0]
	for range b.N {
		board.Make(cell)
		board.Unmake()
	}
}

func BenchmarkWins(b *testing.B) {
	board, _ := benchBoard(b)
	for range b.N {
		board.Wins(Red)
		board.Wins(Blue)
	}
}

func BenchmarkFastLastMoveWin(b *testing.B) {
	board, _ := benchBoard(b)
	cell := cellsOf(b, "H8")[0]
	for range b.N {
		board.FastLastMoveWin(Red, cell)
	}
}

func BenchmarkLegalMoves(b *testing.B) {
	board, buf := benchBoard(b)
	for range b.N {
		board.LegalMoves(buf)
	}
}
