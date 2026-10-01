package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The zero allocation contract of the search path, pinned with
// testing.AllocsPerRun the way the rules and pattern packages pin theirs.

func TestZeroAllocsSearch(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	dl := NewFixedBudget(0)
	var sinkMove rules.Move
	var sinkStats SearchStats
	if n := testing.AllocsPerRun(20, func() {
		dl.Reset(time.Millisecond)
		mv, stats := e.Search(b, dl)
		sinkMove, sinkStats = mv, stats
	}); n != 0 {
		t.Fatalf("Search: %v allocs, want 0", n)
	}
	if !b.IsLegal(rules.Cell(sinkMove)) || sinkStats.Nodes == 0 {
		t.Fatal("sink run produced nothing")
	}
}

func TestZeroAllocsSearchNoTT(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := NewFixedBudget(0)
	if n := testing.AllocsPerRun(20, func() {
		dl.Reset(500 * time.Microsecond)
		_, _ = e.Search(b, dl)
	}); n != 0 {
		t.Fatalf("Search without tt: %v allocs, want 0", n)
	}
}

func TestZeroAllocsEvalIncremental(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	e.eval.reset(b)
	cell := mustCell(t, "J10")
	if n := testing.AllocsPerRun(100, func() {
		e.eval.makeMove(rules.Red, cell)
		e.eval.unmakeMove(cell)
	}); n != 0 {
		t.Fatalf("eval make/unmake: %v allocs, want 0", n)
	}
}

func TestZeroAllocsEvalReset(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	if n := testing.AllocsPerRun(10, func() { e.eval.reset(b) }); n != 0 {
		t.Fatalf("eval reset: %v allocs, want 0", n)
	}
}

func TestZeroAllocsGenerate(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	e.eval.reset(b)
	if n := testing.AllocsPerRun(100, func() { e.generate(b, 0, moveNone) }); n != 0 {
		t.Fatalf("generate: %v allocs, want 0", n)
	}
}

func TestZeroAllocsTT(t *testing.T) {
	var tb ttTable
	tb.init(1 << 12)
	if n := testing.AllocsPerRun(100, func() {
		tb.store(99, 15, 3, 6, ttBoundExact, 2, 1)
		_, _, _ = tb.probe(99, 6, -1000, 1000, 2)
		_ = tb.move(99)
		_ = tb.hashFullPermille(1)
	}); n != 0 {
		t.Fatalf("tt round trip: %v allocs, want 0", n)
	}
}

func TestZeroAllocsAppendPV(t *testing.T) {
	var s SearchStats
	s.PVLen = config.SearchMaxPly
	buf := make([]byte, 0, 4*config.SearchMaxPly)
	if n := testing.AllocsPerRun(100, func() { buf = s.AppendPV(buf[:0]) }); n != 0 {
		t.Fatalf("AppendPV: %v allocs, want 0", n)
	}
}

func TestZeroAllocsDeadline(t *testing.T) {
	dl := NewFixedBudget(time.Second)
	if n := testing.AllocsPerRun(1000, func() { _ = dl.Exceeded() }); n != 0 {
		t.Fatalf("FixedBudget.Exceeded: %v allocs, want 0", n)
	}
}
