package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func windowAt(t *testing.T, b *rules.Board, name string, dir int) uint32 {
	t.Helper()
	return pattern.Index(b, rules.Cell(mustCell(t, name)), dir, rules.Red)
}

func boxedWalk(b *rules.Board, side rules.Color) int {
	n := 0
	for d := range config.PatternDirections {
		for cell := range b.Geom.Cells {
			idx := pattern.Index(b, rules.Cell(cell), d, side)
			if pattern.Lookup(idx).Class != config.PatternClassThree {
				continue
			}
			states := pattern.Unpack(idx)
			engaged := false
			for i, si := range states {
				if si != config.PatternStateOwn {
					continue
				}
				for j, sj := range states {
					if sj == config.PatternStateOpp && j >= i-2 && j <= i+2 {
						engaged = true
					}
				}
			}
			if engaged {
				n++
			}
		}
	}
	return n
}

func TestEvalBoxedClassificationWitnesses(t *testing.T) {
	buildStateTables()
	ladder := boardFromMoves(t, geom.Default(), "H8", "I9", "K8", "I8", "I11", "I7")
	for _, name := range [...]string{"I7", "I8", "I9"} {
		if got := boxedPairs[windowAt(t, ladder, name, 1)]; got&2 == 0 {
			t.Fatalf("window at %s holds the boxed blue ladder, boxed bit missing (%02b)", name, got)
		}
	}
	if got := boxedPairs[windowAt(t, ladder, "I6", 1)]; got&2 != 0 {
		t.Fatalf("window at I6 sees no red stone, must stay unboxed (%02b)", got)
	}
	broken := boardFromMoves(t, geom.Default(), "F6", "F7", "G6", "F8", "H6", "F9")
	if got := boxedPairs[windowAt(t, broken, "F8", 1)]; got&2 != 0 {
		t.Fatalf("the adjacent blocked broken three is a healthy forcing shape and must stay uncut (%02b)", got)
	}
	clean := boardFromMoves(t, geom.Default(), "D1", "F6", "D2", "F7", "D3", "F8")
	if got := boxedPairs[windowAt(t, clean, "F7", 1)]; got&2 != 0 {
		t.Fatalf("lone open three far from every red stone must stay unboxed (%02b)", got)
	}
}

func TestEvalBoxedCountsMatchWindowWalk(t *testing.T) {
	for seed := range uint64(5) {
		b := playout(t, 900*seed+31, int(11+seed*9))
		var ev evaluator
		ev.reset(b)
		for _, side := range [...]rules.Color{rules.Red, rules.Blue} {
			if want := boxedWalk(b, side); ev.boxedT[side] != want {
				t.Fatalf("seed %d side %d: boxed threes %d want %d from the window walk", seed, side, ev.boxedT[side], want)
			}
		}
	}
}

func TestEvalBoxedIncrementalMatchesReset(t *testing.T) {
	for trial := range 8 {
		b := playout(t, uint64(1000*trial+17), 6+trial*7)
		var ev evaluator
		ev.reset(b)
		var buf [config.BoardCells]rules.Move
		n := b.LegalMoves(buf[:])
		for step := range n {
			m := rules.Cell(buf[step])
			side := b.Side
			ev.makeMove(side, m)
			b.Make(m)
			var fresh evaluator
			fresh.reset(b)
			if ev.boxedT != fresh.boxedT {
				t.Fatalf("trial %d step %d: boxed threes %v drift from %v", trial, step, ev.boxedT, fresh.boxedT)
			}
			b.Unmake()
			ev.unmakeMove(m)
		}
	}
}

func TestEvalBoxedColorSymmetry(t *testing.T) {
	for seed := range uint64(4) {
		b := playout(t, 1100*seed+37, int(15+seed*11))
		var a, s evaluator
		a.reset(b)
		s.reset(swappedColors(b))
		if a.boxedT[rules.Red] != s.boxedT[rules.Blue] || a.boxedT[rules.Blue] != s.boxedT[rules.Red] {
			t.Fatalf("seed %d: boxed threes must swap with colors", seed)
		}
		if evA, evB := a.eval(rules.Red), s.eval(rules.Blue); evA != evB {
			t.Fatalf("seed %d: mover evals %d %d must match across the swap", seed, evA, evB)
		}
	}
}

func TestEvalBoxedHexCutInactive(t *testing.T) {
	hex, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	b := boardFromMoves(t, hex, "L12", "L10", "K11", "M10", "K9")
	var ev evaluator
	ev.reset(b)
	if ev.cutT != 0 {
		t.Fatalf("hex cut %d, want 0 so the hex calibration stays untouched", ev.cutT)
	}
	if got := ev.eval(b.Side); got != ev.score[b.Side]-ev.score[b.Side^1]+config.EvalTempo {
		t.Fatalf("hex eval %d must reduce to the class sum plus tempo", got)
	}
}

func TestEvalBoxedLadderCountsAndFormulaOnRecordedBoard(t *testing.T) {
	b := boardFromMoves(t, geom.Default(), "H8", "I9", "K8", "I8", "I11", "I7")
	var ev evaluator
	ev.reset(b)
	if ev.boxedT[rules.Blue] != 3 {
		t.Fatalf("blue boxed threes %d, want 3 from the recorded ladder decomposition", ev.boxedT[rules.Blue])
	}
	if ev.boxedT[rules.Red] != 0 {
		t.Fatalf("red boxed threes %d, the ring stones hold no ladder windows", ev.boxedT[rules.Red])
	}
	want := ev.score[rules.Blue] - ev.score[rules.Red] + config.EvalTempo +
		ev.cleanW*(ev.clean[rules.Blue]-ev.clean[rules.Red]) -
		ev.cutT*(ev.boxedT[rules.Blue]-ev.boxedT[rules.Red])
	if got := ev.eval(rules.Blue); got != want {
		t.Fatalf("eval %d, want the boxed cut formula value %d", got, want)
	}
}

func TestEvalBoxedThreePricesBelowOpenAndBroken(t *testing.T) {
	cut := config.EvalBoxedCutThree("square")
	if config.PatternWeightThree-cut >= config.PatternWeightThree {
		t.Fatalf("boxed three cut %d does not lower the price", cut)
	}
	if config.PatternWeightThree-cut > config.PatternWeightBrokenThree {
		t.Fatalf("boxed three prices at %d above the broken three %d, a one-side-engaged three converts worse than a clean broken three", config.PatternWeightThree-cut, config.PatternWeightBrokenThree)
	}
	if cut > 900 {
		t.Fatalf("square boxed three cut %d outside the calibrated stability envelope", cut)
	}
	if got := config.EvalBoxedCutThree("hex"); got != 0 {
		t.Fatalf("hex boxed cut %d, want parity with the shipped calibration", got)
	}
	if config.EvalCleanWindowWeightSquare != 50 {
		t.Fatalf("clean window weight %d, the shipped clean term must stay untouched", config.EvalCleanWindowWeightSquare)
	}
}
