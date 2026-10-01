package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// TestCandidateSetCoversAllWinInOneCells is the tactical half of the radius
// justification: every cell where either color would win by moving sits in
// the candidate set. Every such cell is line-adjacent to a stone of its run,
// so any radius >= 1 covers them; radius 2 keeps quiet developing moves too.
func TestCandidateSetCoversAllWinInOneCells(t *testing.T) {
	for seed := range uint64(10) {
		b := playout(t, 700*seed+29, int(6+seed*7))
		e := New(0)
		e.eval.reset(b)
		n := e.generate(b, 0, moveNone)
		if n == 0 {
			t.Fatalf("seed %d: candidate set empty on a non full board", seed)
		}
		set := make(map[rules.Move]bool, n)
		for _, m := range e.moves[0][:n] {
			set[m] = true
			if !b.IsLegal(rules.Cell(m)) {
				t.Fatalf("seed %d: generated illegal move %d", seed, m)
			}
		}
		for _, color := range [...]rules.Color{rules.Red, rules.Blue} {
			for _, cell := range winIn1Cells(t, b, color) {
				if !set[rules.Move(cell)] {
					t.Fatalf("seed %d: win-in-1 cell %v for color %d outside candidates", seed, cell, color)
				}
			}
		}
	}
}

func TestGenerateRespectsOpeningRule(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	place(t, b, rules.Blue, "I9")
	e := New(0)
	e.eval.reset(b)
	n := e.generate(b, 0, moveNone)
	if n == 0 {
		t.Fatal("constrained candidate set empty")
	}
	anchor := mustCell(t, "H8")
	for _, m := range e.moves[0][:n] {
		if chebyshevCell(uint16(anchor), uint16(m)) < config.OpeningChebyshevMin {
			t.Fatalf("move %v within Chebyshev %d of the anchor", m, chebyshevCell(uint16(anchor), uint16(m)))
		}
		if !b.IsLegal(rules.Cell(m)) {
			t.Fatalf("move %v illegal", m)
		}
	}
}

func TestOrderingLayers(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	e.eval.reset(b)
	// Pick live cells so the pruning in generate keeps them.
	n := e.generate(b, 0, moveNone)
	if n < 3 {
		t.Fatalf("need 3 live candidates, got %d", n)
	}
	ttCell, killerA, killerB := e.moves[0][0], e.moves[0][1], e.moves[0][2]
	e.killers[0][0], e.killers[0][1] = killerA, killerB
	n = e.generate(b, 0, ttCell)
	sawTT, sawK1, sawK2 := false, false, false
	for i, m := range e.moves[0][:n] {
		switch e.scores[0][i] {
		case config.SearchOrderTT:
			if m != ttCell {
				t.Fatalf("TT layer on wrong move %v", m)
			}
			sawTT = true
		case config.SearchOrderKiller1:
			if m != killerA {
				t.Fatalf("killer1 layer on wrong move %v", m)
			}
			sawK1 = true
		case config.SearchOrderKiller2:
			if m != killerB {
				t.Fatalf("killer2 layer on wrong move %v", m)
			}
			sawK2 = true
		default:
			if e.scores[0][i] >= config.SearchOrderKiller2 {
				t.Fatalf("static layer %d leaks into the killer layer", e.scores[0][i])
			}
		}
	}
	if !sawTT || !sawK1 || !sawK2 {
		t.Errorf("layers hit tt %v k1 %v k2 %v", sawTT, sawK1, sawK2)
	}
	pickMax(e.moves[0][:n], e.scores[0][:n], 0, n)
	if e.moves[0][0] != ttCell {
		t.Errorf("first ordered move %v, want the tt move", e.moves[0][0])
	}
}

func TestNoOrderingFlattensScores(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	e.noOrder = true
	e.eval.reset(b)
	n := e.generate(b, 0, moveNone)
	for i := range n {
		if e.scores[0][i] != 0 {
			t.Fatalf("noOrder score %d at %d, want 0", e.scores[0][i], i)
		}
	}
}

func TestRecordCutoffHistoryOverflowHalves(t *testing.T) {
	e := New(0)
	e.history[rules.Red][10] = config.SearchHistoryMax
	e.recordCutoff(10, rules.Red, 3, 4)
	if e.history[rules.Red][10] > config.SearchHistoryMax {
		t.Fatalf("history %d not capped", e.history[rules.Red][10])
	}
	if e.history[rules.Blue][10] != 0 {
		t.Errorf("halving must touch every entry, blue %d", e.history[rules.Blue][10])
	}
	if e.killers[3][0] != 10 {
		t.Errorf("killer not recorded: %d", e.killers[3][0])
	}
	e.recordCutoff(11, rules.Red, 3, 4)
	if e.killers[3][0] != 11 || e.killers[3][1] != 10 {
		t.Errorf("killers %d %d, want 11 then 10", e.killers[3][0], e.killers[3][1])
	}
}

// TestRingRadiusMeasurement is the documented measurement behind
// SearchRingRadius: candidate counts and depth reached per radius on a fixed
// midgame position under one 150ms budget. Run with -v to read the table.
func TestRingRadiusMeasurement(t *testing.T) {
	if testing.Short() {
		t.Skip("measurement needs wall clock budget")
	}
	for _, radius := range [...]int{1, 2, 3} {
		b := midgameBoard(t)
		e := New(testTTBytes)
		e.radius = radius
		mv, stats := e.Search(b, NewFixedBudget(150*time.Millisecond))
		e2 := New(0)
		e2.radius = radius
		e2.eval.reset(b)
		n := e2.generate(b, 0, moveNone)
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("radius %d illegal move", radius)
		}
		t.Logf("radius %d: candidates %d, depth %d, nodes %d, ebf %d.%03d",
			radius, n, stats.Depth, stats.Nodes, stats.EBFMilli/1000, stats.EBFMilli%1000)
	}
}
