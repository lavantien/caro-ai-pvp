package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const benchTTBytes = 64 << 20

func BenchmarkSearch50ms(b *testing.B) {
	board := midgameBoard(b)
	e := New(benchTTBytes)
	dl := NewFixedBudget(0)
	dl.Reset(time.Microsecond)
	_, _ = e.Search(board, dl) // warm the deadline itab outside the measured loop
	for b.Loop() {
		dl.Reset(50 * time.Millisecond)
		_, _ = e.Search(board, dl)
	}
}

func BenchmarkSearch1s(b *testing.B) {
	board := midgameBoard(b)
	e := New(benchTTBytes)
	dl := NewFixedBudget(0)
	dl.Reset(time.Microsecond)
	_, _ = e.Search(board, dl) // warm the deadline itab outside the measured loop
	for b.Loop() {
		dl.Reset(time.Second)
		mv, stats := e.Search(board, dl)
		b.StopTimer()
		b.ReportMetric(float64(stats.Depth), "depth")
		b.ReportMetric(float64(stats.Nodes)/1e6, "Mnodes")
		b.ReportMetric(float64(stats.Nps)/1e6, "Mnps")
		b.ReportMetric(float64(stats.EBFMilli)/1000, "ebf")
		b.ReportMetric(float64(stats.TTHitPermille)/10, "tt%")
		b.ReportMetric(float64(stats.HashFullPermille)/10, "hf%")
		b.ReportMetric(float64(stats.FirstMoveFailHighPermille)/10, "fh1%")
		if !board.IsLegal(rules.Cell(mv)) {
			b.Fatalf("illegal move %d", mv)
		}
		b.StartTimer()
	}
}

// BenchmarkSearch50msNoTT is the easy tier shape: no transposition table.
func BenchmarkSearch50msNoTT(b *testing.B) {
	board := midgameBoard(b)
	e := New(0)
	dl := NewFixedBudget(0)
	dl.Reset(time.Microsecond)
	_, _ = e.Search(board, dl) // warm the deadline itab outside the measured loop
	for b.Loop() {
		dl.Reset(50 * time.Millisecond)
		_, _ = e.Search(board, dl)
	}
}

func BenchmarkEvalMakeUnmake(b *testing.B) {
	board := midgameBoard(b)
	e := New(0)
	e.eval.reset(board)
	cell := mustCell(b, "J10")
	for b.Loop() {
		e.eval.makeMove(rules.Blue, cell)
		e.eval.unmakeMove(cell)
	}
}

func BenchmarkEvalReset(b *testing.B) {
	board := midgameBoard(b)
	e := New(0)
	for b.Loop() {
		e.eval.reset(board)
	}
}

func BenchmarkGenerate(b *testing.B) {
	board := midgameBoard(b)
	e := New(0)
	e.eval.reset(board)
	for b.Loop() {
		_ = e.generate(board, 0, moveNone)
	}
}
