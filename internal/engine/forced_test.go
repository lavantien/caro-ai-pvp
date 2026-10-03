package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The selective threat restriction of the spec's search design: at a node
// where the opponent of the mover holds a live four, only own win-in-1
// cells and blocks of the opponent's win-in-1 cells can change the value,
// every other move loses at once, so generate keeps only cells whose
// centered window interaction reaches the Four class weight, a superset of
// the forced set that costs one comparison per candidate.

// forcedBoard seats red against a blue four H8-H11 closed at H7, so H12 is
// blue's single win-in-1 cell and red's only forced answer.
func forcedBoard(t testing.TB) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "H7", "P16")
	place(t, b, rules.Blue, "H8", "H9", "H10", "H11")
	return b
}

func TestForcedFourRestrictsCandidates(t *testing.T) {
	b := forcedBoard(t)
	e := New(0)
	e.eval.reset(b)
	if b.Side != rules.Red {
		t.Fatalf("side to move = %d, want red", b.Side)
	}
	n := e.generate(b, 0, moveNone)
	if n != 1 {
		t.Fatalf("forced candidate count = %d, want the single block H12", n)
	}
	if got, want := rules.Cell(e.moves[0][0]), mustCell(t, "H12"); got != want {
		t.Fatalf("forced candidate = %v, want %v", got, want)
	}
}

// TestForcedFourKeepsEveryForcedCell is the soundness property: over random
// playouts, whenever the opponent of the mover holds a four, every win-in-1
// cell of either color (own counters and opponent blocks alike) survives the
// restriction. Dropping any of them could change the node value.
func TestForcedFourKeepsEveryForcedCell(t *testing.T) {
	for seed := range uint64(12) {
		b := playout(t, 911*seed+17, int(8+seed*9))
		e := New(0)
		e.eval.reset(b)
		if e.eval.fours[b.Side^1] == 0 {
			continue
		}
		n := e.generate(b, 0, moveNone)
		if n == 0 {
			t.Fatalf("seed %d: forced candidate set empty", seed)
		}
		set := make(map[rules.Move]bool, n)
		for _, m := range e.moves[0][:n] {
			set[m] = true
		}
		for _, color := range [...]rules.Color{rules.Red, rules.Blue} {
			for _, cell := range winIn1Cells(t, b, color) {
				if !set[rules.Move(cell)] {
					t.Fatalf("seed %d: win-in-1 cell %v of color %d dropped at a forced node", seed, cell, color)
				}
			}
		}
		for _, m := range e.moves[0][:n] {
			static := e.eval.cell[b.Side][m] + e.eval.cell[b.Side^1][m]
			if static < config.PatternWeightFour {
				t.Fatalf("seed %d: candidate %v static %d under the Four weight %d", seed, m, static, config.PatternWeightFour)
			}
		}
	}
}

// TestFallbackMovePicksBestOrdered pins the fallback against the
// enumeration-order regression: a starved search must answer with its best
// known candidate, never the first cell of the A1 scan. Under a live four
// that is the forced block; on a quiet board it is the highest static cell.
func TestFallbackMovePicksBestOrdered(t *testing.T) {
	e := New(0)

	forced := forcedBoard(t)
	e.eval.reset(forced)
	if got, want := rules.Cell(e.fallbackMove(forced)), mustCell(t, "H12"); got != want {
		t.Fatalf("forced fallback = %v, want the block %v", got, want)
	}

	quiet := rules.NewBoard()
	place(t, quiet, rules.Red, "F4", "H12", "H13", "H14")
	place(t, quiet, rules.Blue, "B2")
	if quiet.Side != rules.Red {
		t.Fatalf("quiet board side = %d, want red", quiet.Side)
	}
	e.eval.reset(quiet)
	n := e.generate(quiet, 0, moveNone)
	if n < 2 {
		t.Fatalf("quiet candidate count = %d, want a real choice", n)
	}
	maxStatic := 0
	for _, m := range e.moves[0][:n] {
		if s := e.eval.cell[rules.Red][m] + e.eval.cell[rules.Blue][m]; s > maxStatic {
			maxStatic = s
		}
	}
	mv := e.fallbackMove(quiet)
	if s := e.eval.cell[rules.Red][mv] + e.eval.cell[rules.Blue][mv]; s != maxStatic {
		t.Fatalf("quiet fallback static = %d, want the candidate max %d", s, maxStatic)
	}
}
