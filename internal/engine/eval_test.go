package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestSwapStates(t *testing.T) {
	cases := [...]struct {
		in, want uint32
	}{
		{0, 0},
		{1, 2},
		{2, 1},
		{3, 3},
		{0b00_01_10_11, 0b00_10_01_11},
		{1<<(2*config.PatternWindowLen) - 1, 1<<(2*config.PatternWindowLen) - 1},
	}
	for _, c := range cases {
		if got := swapStates(c.in); got != c.want {
			t.Errorf("swapStates(%#b) = %#b, want %#b", c.in, got, c.want)
		}
	}
}

// TestClassPairsMatchAllDirections guards the direction independence the
// packed class pair table relies on: every direction must classify both
// mover views identically for any index.
func TestClassPairsMatchAllDirections(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 55))
	probes := []uint32{0, 1, 2, 3, 0xFFFF, config.PatternTableEntries - 1}
	for range 1000 {
		probes = append(probes, rng.Uint32N(config.PatternTableEntries))
	}
	for _, idx := range probes {
		want := pattern.Lookup(0, idx).Class | pattern.Lookup(0, swapStates(idx)).Class<<3
		for d := 1; d < config.PatternDirections; d++ {
			red := pattern.Lookup(d, idx).Class
			blue := pattern.Lookup(d, swapStates(idx)).Class
			if got := red | blue<<3; got != want {
				t.Fatalf("dir %d idx %d: pair %d differs from dir 0 pair %d", d, idx, got, want)
			}
		}
	}
}

func TestSwapStatesMatchesIndex(t *testing.T) {
	for seed := range uint64(8) {
		b := playout(t, 101*seed+3, int(30+seed*10))
		cell := rules.Cell(80 + seed*20)
		if b.Occupied(cell) {
			b = playout(t, 101*seed+3, int(20+seed*5))
			cell = rules.Cell(90 + seed*15)
		}
		for d := range config.PatternDirections {
			red := pattern.Index(b, cell, d, rules.Red)
			blue := pattern.Index(b, cell, d, rules.Blue)
			if swapStates(red) != blue {
				t.Fatalf("seed %d dir %d: swapStates mismatch", seed, d)
			}
		}
	}
}

func TestEvalEmptyBoard(t *testing.T) {
	var ev evaluator
	ev.reset(rules.NewBoard())
	if ev.score[rules.Red] != 0 || ev.score[rules.Blue] != 0 {
		t.Errorf("empty board scores %d %d, want 0 0", ev.score[rules.Red], ev.score[rules.Blue])
	}
	if ev.fours[rules.Red] != 0 || ev.fours[rules.Blue] != 0 {
		t.Errorf("empty board fours %d %d, want 0 0", ev.fours[rules.Red], ev.fours[rules.Blue])
	}
	if got := ev.eval(rules.Red); got != config.EvalTempo {
		t.Errorf("empty board eval = %d, want tempo %d", got, config.EvalTempo)
	}
}

func TestEvalFourWindowCounted(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H9")
	place(t, b, rules.Blue, "D9")
	var ev evaluator
	ev.reset(b)
	if ev.fours[rules.Red] == 0 {
		t.Error("red four must raise the red four window count")
	}
	if ev.fours[rules.Blue] != 0 {
		t.Error("blue has no four windows")
	}
	if ev.cell[rules.Red][mustCell(t, "I9")] < config.PatternWeightFour {
		t.Errorf("I9 red cell score %d, must price the four completion", ev.cell[rules.Red][mustCell(t, "I9")])
	}
}

func TestEvalIncrementalMatchesReset(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 43))
	for trial := range 12 {
		b := playout(t, 500*uint64(trial)+11, int(4+trial*9))
		e := New(0)
		e.eval.reset(b)
		var buf [config.BoardCells]rules.Move
		for step := range 30 {
			n := b.LegalMoves(buf[:])
			if n == 0 {
				break
			}
			m := rules.Cell(buf[rng.IntN(n)])
			side := b.Side
			e.eval.makeMove(side, m)
			b.Make(m)

			var fresh evaluator
			fresh.reset(b)
			if e.eval.score != fresh.score || e.eval.fours != fresh.fours {
				t.Fatalf("trial %d step %d: score/fours drift %v %v vs %v %v",
					trial, step, e.eval.score, e.eval.fours, fresh.score, fresh.fours)
			}
			for c := range config.BoardCells {
				if e.eval.cell[rules.Red][c] != fresh.cell[rules.Red][c] ||
					e.eval.cell[rules.Blue][c] != fresh.cell[rules.Blue][c] {
					t.Fatalf("trial %d step %d: cell %d drift", trial, step, c)
				}
			}
			for d := range config.PatternDirections {
				if e.eval.win[d] != fresh.win[d] || e.eval.wpair[d] != fresh.wpair[d] {
					t.Fatalf("trial %d step %d: window table dir %d drift", trial, step, d)
				}
			}

			b.Unmake()
			e.eval.unmakeMove(m)
			var back evaluator
			back.reset(b)
			if e.eval.score != back.score || e.eval.fours != back.fours || e.eval.win != back.win || e.eval.wpair != back.wpair {
				t.Fatalf("trial %d step %d: unmake drift", trial, step)
			}
		}
	}
}

func swappedColors(b *rules.Board) *rules.Board {
	n := rules.NewBoard()
	n.Red, n.Blue = b.Blue, b.Red
	n.Full = b.Full
	n.Region = b.Region
	n.Side = b.Side
	n.MoveCount = b.MoveCount
	return n
}

func TestEvalSymmetryUnderColorSwap(t *testing.T) {
	for seed := range uint64(6) {
		b := playout(t, 900*seed+5, int(25+seed*13))
		var a, s evaluator
		a.reset(b)
		s.reset(swappedColors(b))
		evA := a.eval(rules.Red)
		evB := s.eval(rules.Red)
		if evA+evB != 2*config.EvalTempo {
			t.Errorf("seed %d: %d + %d != 2*tempo", seed, evA, evB)
		}
		if evA-config.EvalTempo != -(evB - config.EvalTempo) {
			t.Errorf("seed %d: mover relative parts must negate", seed)
		}
		if a.score[rules.Red] != s.score[rules.Blue] || a.score[rules.Blue] != s.score[rules.Red] {
			t.Errorf("seed %d: color scores must swap", seed)
		}
	}
}

func TestEvalForcingClassesDominate(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "F9", "G9", "H9")
	var ev evaluator
	ev.reset(b)
	if got := ev.eval(rules.Red); got <= 0 {
		t.Errorf("open three eval = %d, want positive", got)
	}
	if got := ev.eval(rules.Blue); got >= 0 {
		t.Errorf("defender eval = %d, want negative", got)
	}
}
