package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func boardFromMoves(t *testing.T, g *geom.Geometry, moves ...string) *rules.Board {
	t.Helper()
	b := rules.NewBoardFor(g)
	for _, name := range moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			t.Fatalf("cell %s: %v", name, err)
		}
		if !b.Geom.Contains(cell) || b.Occupied(rules.Cell(cell)) {
			t.Fatalf("cell %s unusable at ply %d", name, b.MoveCount+1)
		}
		b.Make(rules.Cell(cell))
	}
	return b
}

func TestEvalCleanWindowRingOutlivesBoxedLadder(t *testing.T) {
	moves := []string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10"}
	b := boardFromMoves(t, geom.Default(), moves...)
	var ev evaluator
	ev.reset(b)
	if ev.clean[rules.Red] <= ev.clean[rules.Blue] {
		t.Fatalf("clean windows red %d blue %d, the live ring must outcount the boxed ladder", ev.clean[rules.Red], ev.clean[rules.Blue])
	}
}

func TestEvalCleanWindowHexInactive(t *testing.T) {
	hex, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	b := boardFromMoves(t, hex, "L12", "L10", "K11", "M10", "K9")
	var ev evaluator
	ev.reset(b)
	if ev.cleanW != 0 {
		t.Fatalf("hex clean weight %d, want 0 so the hex calibration stays untouched", ev.cleanW)
	}
	if got := ev.eval(b.Side); got != ev.score[b.Side]-ev.score[b.Side^1]+config.EvalTempo {
		t.Fatalf("hex eval %d must reduce to the class sum plus tempo", got)
	}
}

func TestEvalCleanWindowCountsMatchWindowWalk(t *testing.T) {
	for seed := range uint64(5) {
		b := playout(t, 600*seed+7, int(9+seed*8))
		var ev evaluator
		ev.reset(b)
		var red, blue int
		for d := range config.PatternDirections {
			for cell := range b.Geom.Cells {
				var hasOwn, hasOpp bool
				for _, s := range pattern.Unpack(pattern.Index(b, rules.Cell(cell), d, rules.Red)) {
					switch s {
					case config.PatternStateOwn:
						hasOwn = true
					case config.PatternStateOpp:
						hasOpp = true
					}
				}
				if hasOwn && !hasOpp {
					red++
				}
				if hasOwn != hasOpp && !hasOwn {
					blue++
				}
			}
		}
		if ev.clean[rules.Red] != red || ev.clean[rules.Blue] != blue {
			t.Fatalf("seed %d: clean counters %v want %d %d from the window walk", seed, ev.clean, red, blue)
		}
	}
}

func TestEvalCleanWindowIncrementalMatchesReset(t *testing.T) {
	for trial := range 8 {
		b := playout(t, uint64(700*trial+13), 6+trial*7)
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
			if ev.clean != fresh.clean {
				t.Fatalf("trial %d step %d: clean counters %v drift from %v", trial, step, ev.clean, fresh.clean)
			}
			b.Unmake()
			ev.unmakeMove(m)
		}
		var fresh evaluator
		fresh.reset(b)
		if ev.clean != fresh.clean {
			t.Fatalf("trial %d: clean counters drift after full unmake", trial)
		}
	}
}

func TestEvalCleanWindowColorSymmetry(t *testing.T) {
	for seed := range uint64(4) {
		b := playout(t, 800*seed+29, int(15+seed*11))
		var a, s evaluator
		a.reset(b)
		s.reset(swappedColors(b))
		if a.clean[rules.Red] != s.clean[rules.Blue] || a.clean[rules.Blue] != s.clean[rules.Red] {
			t.Fatalf("seed %d: clean counters must swap with colors", seed)
		}
		if evA, evB := a.eval(rules.Red), s.eval(rules.Blue); evA != evB {
			t.Fatalf("seed %d: mover evals %d %d must match across the swap", seed, evA, evB)
		}
	}
}

func TestSearchAvoidsBoxedLadderOpening(t *testing.T) {
	if testing.Short() {
		t.Skip("depth-6 opening search")
	}
	b := boardFromMoves(t, geom.Default(), "H8", "H6", "H5")
	e := New(0)
	defer e.Close()
	dl := NewStoppable()
	mv, _ := e.SearchDepth(b, dl, 6)
	name := string(geom.Default().AppendName(nil, int(mv)))
	for _, boxed := range [...]string{"I6", "G6", "H7", "I7"} {
		if name == boxed {
			t.Fatalf("blue move 4 enters the boxed ladder with %s, every recorded square loss grew from this family", name)
		}
	}
}

func TestSearchScoresBoxedLadderBelowEven(t *testing.T) {
	if testing.Short() {
		t.Skip("depth-6 midgame search")
	}
	b := boardFromMoves(t, geom.Default(), "H8", "H6", "H5", "I6", "K6")
	e := New(0)
	defer e.Close()
	_, st := e.SearchDepth(b, NewStoppable(), 6)
	if st.Score >= 0 {
		t.Fatalf("depth-6 blue score %d on the boxed ladder board, the in-game engine scored plus 2600 here and every game from it lost", st.Score)
	}
}
