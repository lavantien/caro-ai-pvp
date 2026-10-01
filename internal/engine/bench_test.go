package engine

import (
	"fmt"
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

// BenchmarkSMPSearch50ms measures lazy SMP scaling on the fixed midgame
// position: 1, 2, and 4 workers over one shared 64MiB table. Two warmup
// searches start the pool and grow worker stacks to the iteration depth the
// budget reaches, so the measured loop reports the search's own cost: zero
// allocation.
func BenchmarkSMPSearch50ms(b *testing.B) {
	board := midgameBoard(b)
	for _, workers := range [...]int{1, 2, 4} {
		b.Run(fmt.Sprintf("workers-%d", workers), func(b *testing.B) {
			s := newSMP(workers, benchTTBytes)
			dl := NewFixedBudget(0)
			for range 3 {
				dl.Reset(50 * time.Millisecond)
				_, _ = s.Search(board, dl)
			}
			var depth, ebfSum, nps, mvLast uint64
			var iters int
			for b.Loop() {
				dl.Reset(50 * time.Millisecond)
				mv, stats := s.Search(board, dl)
				mvLast = uint64(mv)
				depth = uint64(stats.Depth)
				ebfSum += uint64(stats.EBFMilli)
				nps += stats.Nps
				iters++
			}
			// Metrics report once after the loop: harness formatting inside
			// the measured region would stretch the gap past the worker
			// park delay and pollute the allocation reading.
			if !board.IsLegal(rules.Cell(mvLast)) {
				b.Fatalf("illegal move %d", mvLast)
			}
			b.ReportMetric(float64(depth), "depth")
			b.ReportMetric(float64(ebfSum)/1000/float64(iters), "ebf")
			b.ReportMetric(float64(nps)/1e6/float64(iters), "Mnps")
		})
	}
}
