package engine

import (
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func hexGeometry(t *testing.T) *geom.Geometry {
	t.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	return g
}

func regionPopcount(g *geom.Geometry) int {
	n := 0
	for _, w := range g.Region {
		n += bits.OnesCount64(w)
	}
	return n
}

func TestInterlockPerGeometry(t *testing.T) {
	maxWeight := config.PatternClassWeights[config.PatternClassOpenFour]
	for _, v := range config.Variants {
		t.Run(v.Name, func(t *testing.T) {
			g, err := geom.FromSpec(v)
			if err != nil {
				t.Fatalf("variant %s: %v", v.Name, err)
			}
			region := regionPopcount(g)
			if region > config.SearchMaxMovesPerPly {
				t.Fatalf("%s region %d exceeds the %d move buffer", v.Name, region, config.SearchMaxMovesPerPly)
			}
			maxStatic := len(g.Dirs) * 2 * maxWeight
			if config.SearchOrderKiller2 <= maxStatic+config.SearchHistoryMax {
				t.Fatalf("%s killer2 %d not above max order %d", v.Name, config.SearchOrderKiller2, maxStatic+config.SearchHistoryMax)
			}
			if config.SearchOrderKiller1 <= maxStatic+config.SearchHistoryMax {
				t.Fatalf("%s killer1 %d not above max order %d", v.Name, config.SearchOrderKiller1, maxStatic+config.SearchHistoryMax)
			}
			maxTotal := len(g.Dirs) * region * maxWeight
			if config.EvalMateMax <= maxTotal {
				t.Fatalf("%s EvalMateMax %d not above max eval sum %d", v.Name, config.EvalMateMax, maxTotal)
			}
		})
	}
}

func compareEvalStates(t *testing.T, ev *evaluator, b *rules.Board, step int) {
	t.Helper()
	var fresh evaluator
	fresh.reset(b)
	if ev.score != fresh.score {
		t.Fatalf("step %d: incremental score %v want %v", step, ev.score, fresh.score)
	}
	if ev.fours != fresh.fours {
		t.Fatalf("step %d: incremental fours %v want %v", step, ev.fours, fresh.fours)
	}
	if ev.cell != fresh.cell {
		t.Fatalf("step %d: incremental cell table drifted", step)
	}
	if ev.win != fresh.win || ev.wpair != fresh.wpair {
		t.Fatalf("step %d: incremental window tables drifted", step)
	}
	for _, c := range [2]rules.Color{rules.Red, rules.Blue} {
		if got, want := ev.eval(c), fresh.eval(c); got != want {
			t.Fatalf("step %d: eval %v = %d want %d", step, c, got, want)
		}
	}
}

func hexEmptyCells(b *rules.Board) []rules.Cell {
	g := b.Geom
	out := make([]rules.Cell, 0, 64)
	for cell := 0; cell < g.Cells; cell++ {
		if !g.Contains(cell) {
			continue
		}
		if b.Full[cell/64]&(1<<(uint(cell)%64)) == 0 {
			out = append(out, rules.Cell(cell))
		}
	}
	return out
}

func TestEvalIncrementalMatchesResetHex(t *testing.T) {
	g := hexGeometry(t)
	games := 8
	if testing.Short() {
		games = 3
	}
	for game := range games {
		rng := rand.New(rand.NewPCG(uint64(game+1), uint64(game+1)*2+1))
		b := rules.NewBoardFor(g)
		e := New(0)
		e.eval.reset(b)
		type played struct {
			cell rules.Cell
			side rules.Color
		}
		var seq []played
		steps := rng.IntN(140)
		for range steps {
			empty := hexEmptyCells(b)
			if len(empty) == 0 {
				break
			}
			cell := empty[rng.IntN(len(empty))]
			side := b.Side
			e.eval.makeMove(side, cell)
			b.Make(cell)
			seq = append(seq, played{cell: cell, side: side})
			compareEvalStates(t, &e.eval, b, game*1000+len(seq))
			if b.FastLastMoveWin(side, cell) {
				break
			}
		}
		for i := len(seq) - 1; i >= 0; i-- {
			e.eval.unmakeMove(seq[i].cell)
			b.Unmake()
			compareEvalStates(t, &e.eval, b, game*1000-i)
		}
		compareEvalStates(t, &e.eval, b, game*1000-999)
	}
}

func nearByDistance(b *rules.Board, radius int) map[int]bool {
	g := b.Geom
	stones := make([]int, 0, 64)
	for cell := 0; cell < g.Cells; cell++ {
		if g.Contains(cell) && b.Full[cell/64]&(1<<(uint(cell)%64)) != 0 {
			stones = append(stones, cell)
		}
	}
	out := make(map[int]bool, len(stones)*8)
	for cell := 0; cell < g.Cells; cell++ {
		if !g.Contains(cell) {
			continue
		}
		for _, s := range stones {
			if g.Distance(cell, s) <= radius {
				out[cell] = true
				break
			}
		}
	}
	return out
}

func referenceMoveSet(b *rules.Board, radius int) map[rules.Move]bool {
	g := b.Geom
	var fresh evaluator
	fresh.reset(b)
	near := nearByDistance(b, radius)
	anchor, constrained := openingAnchor(b)
	side := int(b.Side)
	passes := []struct {
		requireLive bool
		minStatic   int
	}{
		{true, 0},
		{true, 0},
		{false, 0},
	}
	if fresh.fours[side^1] > 0 {
		passes[0].minStatic = config.PatternClassWeights[config.PatternClassFour]
	}
	for _, p := range passes {
		out := make(map[rules.Move]bool)
		for cell := range near {
			if b.Full[cell/64]&(1<<(uint(cell)%64)) != 0 {
				continue
			}
			if constrained && g.Distance(int(anchor), cell) < g.OpeningMin {
				continue
			}
			static := fresh.cell[side][cell] + fresh.cell[side^1][cell]
			if p.requireLive && static == 0 {
				continue
			}
			if static < p.minStatic {
				continue
			}
			out[rules.Move(cell)] = true
		}
		if len(out) > 0 {
			return out
		}
	}
	return map[rules.Move]bool{}
}

func generatedMoveSet(t *testing.T, e *Engine, b *rules.Board) map[rules.Move]bool {
	t.Helper()
	e.eval.reset(b)
	n := e.generate(b, 0, moveNone)
	if n > config.SearchMaxMovesPerPly {
		t.Fatalf("generate returned %d moves above the %d buffer", n, config.SearchMaxMovesPerPly)
	}
	out := make(map[rules.Move]bool, n)
	for i := range n {
		out[e.moves[0][i]] = true
	}
	return out
}

func seededHexPosition(t *testing.T, seed uint64, stones int) *rules.Board {
	t.Helper()
	g := hexGeometry(t)
	b := rules.NewBoardFor(g)
	rng := rand.New(rand.NewPCG(seed, seed*3+7))
	for i := 0; i < stones; i++ {
		empty := hexEmptyCells(b)
		if len(empty) == 0 {
			break
		}
		b.Side = rules.Color(i % 2)
		b.Make(empty[rng.IntN(len(empty))])
	}
	b.Side = rules.Red
	return b
}

func TestGenerateMatchesReferenceHex(t *testing.T) {
	g := hexGeometry(t)
	e := New(0)
	b := rules.NewBoardFor(g)
	got := generatedMoveSet(t, e, b)
	if len(got) != 1 || !got[rules.Move(g.Center)] {
		t.Fatalf("empty hex board generate = %v, want the center alone", got)
	}
	b2 := rules.NewBoardFor(g)
	b2.Side = rules.Red
	b2.Make(rules.Cell(g.Center))
	b2.Side = rules.Blue
	edgeCell := rules.Cell(0*g.Stride + 11)
	b2.Make(edgeCell)
	if got, want := generatedMoveSet(t, e, b2), referenceMoveSet(b2, e.radius); !equalMoveSets(got, want) {
		t.Fatalf("corner blue reply: generate %d moves want %d", len(got), len(want))
	}
	positions := 12
	if testing.Short() {
		positions = 5
	}
	for i := range positions {
		b := seededHexPosition(t, uint64(i+90), 1+i*7)
		got := generatedMoveSet(t, e, b)
		want := referenceMoveSet(b, e.radius)
		if !equalMoveSets(got, want) {
			t.Fatalf("position %d stones %d: generate set size %d want %d", i, 1+i*7, len(got), len(want))
		}
	}
}

func equalMoveSets(a, b map[rules.Move]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for m := range a {
		if !b[m] {
			return false
		}
	}
	return true
}

func TestGenerateAllLegalHex(t *testing.T) {
	g := hexGeometry(t)
	e := New(0)
	for i := range 8 {
		b := seededHexPosition(t, uint64(i+31), 3+i*5)
		side := b.Side
		nb := rules.NewNaiveBoardFor(g)
		for cell := 0; cell < g.Cells; cell++ {
			if g.Contains(cell) && b.Full[cell/64]&(1<<(uint(cell)%64)) != 0 {
				w := b.At(rules.Cell(cell))
				nb.Set(rules.Cell(cell), w)
			}
		}
		for m := range generatedMoveSet(t, e, b) {
			if !nb.IsLegal(side, rules.Cell(m)) {
				t.Fatalf("position %d: generated move %d fails the oracle legality", i, m)
			}
		}
	}
}

func naiveWinInOne(g *geom.Geometry, b *rules.Board, side rules.Color) map[rules.Cell]bool {
	out := map[rules.Cell]bool{}
	for cell := 0; cell < g.Cells; cell++ {
		if !g.Contains(cell) || b.Full[cell/64]&(1<<(uint(cell)%64)) != 0 {
			continue
		}
		nb := rules.NewNaiveBoardFor(g)
		for other := 0; other < g.Cells; other++ {
			if g.Contains(other) && b.Full[other/64]&(1<<(uint(other)%64)) != 0 {
				nb.Set(rules.Cell(other), b.At(rules.Cell(other)))
			}
		}
		nb.Set(rules.Cell(cell), side)
		if nb.WinsThrough(side, rules.Cell(cell)) {
			out[rules.Cell(cell)] = true
		}
	}
	return out
}

func TestWinInOneInsideGeneratedHex(t *testing.T) {
	g := hexGeometry(t)
	e := New(0)
	ctr := func(dr, dc int) int { return (11+dr)*g.Stride + 11 + dc }
	mkBoard := func(red, blue []int) *rules.Board {
		b := rules.NewBoardFor(g)
		for _, c := range red {
			b.Side = rules.Red
			b.Make(rules.Cell(c))
		}
		for _, c := range blue {
			b.Side = rules.Blue
			b.Make(rules.Cell(c))
		}
		b.Side = rules.Red
		return b
	}
	far := []int{ctr(3, 3)}
	fixtures := []struct {
		name string
		red  []int
		blue []int
	}{
		{"gap four", []int{ctr(0, 2), ctr(0, 3), ctr(0, 4), ctr(0, 6)}, far},
		{"rim half-dead four", []int{ctr(0, -11), ctr(0, -10), ctr(0, -9), ctr(0, -8)}, far},
		{"open vertical four", []int{ctr(1, 0), ctr(2, 0), ctr(3, 0), ctr(4, 0)}, far},
	}
	for _, f := range fixtures {
		b := mkBoard(f.red, f.blue)
		side := b.Side
		wins := naiveWinInOne(g, b, side)
		if len(wins) == 0 {
			t.Fatalf("%s: expected at least one win-in-1", f.name)
		}
		generated := generatedMoveSet(t, e, b)
		for c := range wins {
			if !generated[rules.Move(c)] {
				t.Fatalf("%s: win-in-1 cell %d missing from the generated set of %d", f.name, c, len(generated))
			}
		}
	}
	boards := 10
	if testing.Short() {
		boards = 4
	}
	for i := range boards {
		b := seededHexPosition(t, uint64(i+700), 20+i*9)
		side := b.Side
		wins := naiveWinInOne(g, b, side)
		if len(wins) == 0 {
			continue
		}
		generated := generatedMoveSet(t, e, b)
		for c := range wins {
			if !generated[rules.Move(c)] {
				t.Fatalf("board %d: win-in-1 cell %d missing from the generated set of %d", i, c, len(generated))
			}
		}
	}
}

func TestOpeningAnchorHex(t *testing.T) {
	g := hexGeometry(t)
	e := New(0)
	b := rules.NewBoardFor(g)
	b.Side = rules.Red
	b.Make(rules.Cell(g.Center))
	b.Side = rules.Blue
	anchor := g.Center
	for dr := -2; dr <= 2; dr++ {
		for dc := -2; dc <= 2; dc++ {
			cell := (11+dr)*g.Stride + 11 + dc
			if cell == anchor || !g.Contains(cell) {
				continue
			}
			b.Side = rules.Blue
			b.Make(rules.Cell(cell))
			b.Side = rules.Red
			generated := generatedMoveSet(t, e, b)
			for m := range generated {
				if g.Distance(anchor, int(m)) < g.OpeningMin {
					t.Fatalf("blue at (%d,%d): generated %d within opening distance of the red anchor", dr, dc, m)
				}
			}
			b.Unmake()
		}
	}
}
