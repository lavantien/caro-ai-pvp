package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestNodeBudgetBoundaries(t *testing.T) {
	zero := NewNodeBudget(0)
	if !zero.Exceeded() {
		t.Fatal("limit 0 must exceed before any node feeds")
	}
	one := NewNodeBudget(1)
	if one.Exceeded() {
		t.Fatal("limit 1 with no feed must not exceed")
	}
	one.Add(1)
	if !one.Exceeded() {
		t.Fatal("limit 1 after one node must exceed")
	}
	quant := NewNodeBudget(2049)
	quant.Add(2048)
	if quant.Exceeded() {
		t.Fatal("limit 2049 after one interval batch must not exceed")
	}
	quant.Add(2048)
	if !quant.Exceeded() {
		t.Fatal("limit 2049 after two interval batches must exceed")
	}
}

func TestNodeBudgetStopAndReset(t *testing.T) {
	nb := NewNodeBudget(1 << 40)
	nb.Stop()
	if !nb.Exceeded() {
		t.Fatal("Stop must trip regardless of count")
	}
	nb.Reset(1 << 40)
	if nb.Exceeded() {
		t.Fatal("Reset must clear stop and count")
	}
	nb.Add(5)
	if got := nb.Count(); got != 5 {
		t.Fatalf("count = %d, want 5", got)
	}
	nb.Reset(10)
	if got := nb.Count(); got != 0 {
		t.Fatalf("count after reset = %d, want 0", got)
	}
}

func TestNodeBudgetZeroLimitReturnsFallback(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewNodeBudget(0), 8)
	if stats.Depth != 0 || stats.RootIters != 0 {
		t.Fatalf("zero budget completed depth %d iters %d, want 0 0", stats.Depth, stats.RootIters)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("fallback move %v illegal", mv)
	}
}

func TestNodeBudgetComposesWithStop(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	dl := NewNodeBudget(1 << 40)
	dl.Stop()
	mv, stats := e.SearchDepth(b, dl, 8)
	if stats.Depth != 0 {
		t.Fatalf("stopped budget completed depth %d, want 0", stats.Depth)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("fallback move %v illegal", mv)
	}
}

func TestNodeBudgetSearchStopsAndRepeats(t *testing.T) {
	b := midgameBoard(t)
	var first struct {
		mv    rules.Move
		score int
		depth int
		nodes uint64
	}
	for run := range 2 {
		e := New(testTTBytes)
		mv, stats := e.SearchDepth(b, NewNodeBudget(2048), 8)
		if stats.Depth < 1 {
			t.Fatalf("run %d: no iteration completed before the trip", run)
		}
		if stats.Nodes < 2048 {
			t.Fatalf("run %d: nodes %d under one check interval, trip impossible", run, stats.Nodes)
		}
		if run == 0 {
			first.mv, first.score, first.depth, first.nodes = mv, stats.Score, stats.Depth, stats.Nodes
			continue
		}
		if mv != first.mv || stats.Score != first.score || stats.Depth != first.depth || stats.Nodes != first.nodes {
			t.Fatalf("nondeterministic node budget search: (%v %d %d %d) versus (%v %d %d %d)",
				mv, stats.Score, stats.Depth, stats.Nodes, first.mv, first.score, first.depth, first.nodes)
		}
	}
}

func TestNodeBudgetNoStopAtExactConsumption(t *testing.T) {
	b := midgameBoard(t)
	ref := New(testTTBytes)
	refMv, refStats := ref.SearchDepth(b, NewFixedBudget(30*time.Second), 4)
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewNodeBudget(refStats.Nodes), 4)
	if mv != refMv || stats.Score != refStats.Score || stats.Depth != refStats.Depth || stats.Nodes != refStats.Nodes {
		t.Fatalf("exact budget diverged: (%v %d %d %d) versus (%v %d %d %d)",
			mv, stats.Score, stats.Depth, stats.Nodes, refMv, refStats.Score, refStats.Depth, refStats.Nodes)
	}
}

func TestZeroAllocsNodeBudget(t *testing.T) {
	dl := NewNodeBudget(1 << 20)
	if n := testing.AllocsPerRun(1000, func() { dl.Add(2048); _ = dl.Exceeded() }); n != 0 {
		t.Fatalf("NodeBudget Add plus Exceeded: %v allocs, want 0", n)
	}
	b := midgameBoard(t)
	e := New(testTTBytes)
	if n := testing.AllocsPerRun(20, func() {
		dl.Reset(3000)
		_, _ = e.SearchDepth(b, dl, 8)
	}); n != 0 {
		t.Fatalf("SearchDepth under NodeBudget: %v allocs, want 0", n)
	}
}

func TestNodeBudgetGovernsSMP(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	mv, stats := s.SearchDepth(b, NewNodeBudget(1<<40), 3)
	if stats.Depth != 3 {
		t.Fatalf("huge budget smp depth = %d, want 3", stats.Depth)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("smp move %v illegal", mv)
	}
	s2 := newSMP(2, testTTBytes)
	defer s2.Close()
	mv2, stats2 := s2.SearchDepth(b, NewNodeBudget(2048), 8)
	if stats2.Depth < 1 || stats2.Depth > 7 {
		t.Fatalf("budgeted smp depth = %d, want a partial completion inside 1 to 7", stats2.Depth)
	}
	if !b.IsLegal(rules.Cell(mv2)) {
		t.Fatalf("budgeted smp move %v illegal", mv2)
	}
	if stats2.Nodes < 2048 {
		t.Fatalf("budgeted smp nodes %d under the limit, trip impossible", stats2.Nodes)
	}
}
