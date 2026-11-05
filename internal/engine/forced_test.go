package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

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

func TestForcedFourKeepsEveryForcedCell(t *testing.T) {
	for seed := range uint64(12) {
		b := playout(t, 911*seed+17, int(8+seed*9))
		e := New(0)
		e.eval.reset(b)
		if len(winIn1Cells(t, b, b.Side^1)) == 0 {
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

func TestPhantomFourRestrictionStaysOff(t *testing.T) {
	boards := []*rules.Board{phantomFourBoard(t, "I11")}
	for seed := range uint64(24) {
		b := playout(t, 4177*seed+31, int(6+seed*7))
		e := New(0)
		e.eval.reset(b)
		if e.eval.fours[b.Side^1] == 0 || len(winIn1Cells(t, b, b.Side^1)) != 0 {
			continue
		}
		boards = append(boards, b)
	}
	if len(boards) < 2 {
		t.Fatalf("phantom-only boards found %d, want the recorded board plus at least one playout hit", len(boards))
	}
	for i, b := range boards {
		e := New(0)
		e.eval.reset(b)
		n := e.generate(b, 0, moveNone)
		unconstrained := 0
		for _, m := range e.moves[0][:n] {
			static := e.eval.cell[b.Side][m] + e.eval.cell[b.Side^1][m]
			if static < config.PatternWeightFour {
				unconstrained++
			}
		}
		if unconstrained == 0 {
			t.Fatalf("board %d: restriction fired on phantom fours alone, %d candidates all at the four weight", i, n)
		}
	}
}

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
