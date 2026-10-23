package vcf

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func hexGeom(t *testing.T) *geom.Geometry {
	t.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	return g
}

func buildAtFor(t testing.TB, g *geom.Geometry, ar, ac int, side rules.Color, spec []placed) *rules.Board {
	t.Helper()
	b := rules.NewBoardFor(g)
	for _, p := range spec {
		b.Side = rules.Red
		if !p.red {
			b.Side = rules.Blue
		}
		b.Make(cellOf(ar+p.dr, ac+p.dc, g))
	}
	b.Side = side
	return b
}

func checkSolve(t *testing.T, b *rules.Board, kind Kind, wantPlies int) {
	t.Helper()
	s := New(kind)
	var stats SolverStats
	found := s.Solve(b, config.SolverNodeBudget, nil, &stats)
	if wantPlies < 0 {
		if found {
			t.Fatalf("kind %d: expected not found, claimed %d plies", kind, stats.Plies)
		}
		return
	}
	if !found {
		t.Fatalf("kind %d: expected a forced win", kind)
	}
	if stats.Plies != wantPlies {
		t.Fatalf("kind %d: plies %d want %d", kind, stats.Plies, wantPlies)
	}
	verifyForcedPV(t, b, stats)
}

func TestHexTacticalBattery(t *testing.T) {
	g := hexGeom(t)
	cases := []struct {
		name string
		spec []placed
		vcf  int
		vct  int
	}{
		{"win in 1", specWinIn1, 1, 1},
		{"open four", specOpenFour, 3, 3},
		{"four chain", specFourChain, 7, -2},
		{"double four", specDoubleFour, 3, 3},
		{"cross 3-4", specCross34, 5, -2},
		{"counter four refutes", specCounterFour, -1, -1},
		{"far block", specFarBlock, -1, -1},
		{"vct double three", specVCTDoubleThree, -1, 5},
		{"vct counter four refutes", specVCTCounterFour, -1, -1},
		{"phantom three", specPhantomThree, -1, -1},
		{"quiet", specQuiet, -1, -1},
		{"two dead fours", specTwoDeadFours, -1, -2},
		{"phantom four", []placed{o(0, 1), o(0, 2), o(0, 3), x(0, -1), x(0, 5)}, -1, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := buildAtFor(t, g, 11, 11, rules.Red, c.spec)
			checkSolve(t, b, KindVCF, c.vcf)
			switch c.vct {
			case -2:
				s := New(KindVCT)
				var stats SolverStats
				if !s.Solve(b, config.SolverNodeBudget, nil, &stats) {
					t.Fatalf("vct: expected a forced win")
				}
				verifyForcedPV(t, b, stats)
			default:
				checkSolve(t, b, KindVCT, c.vct)
			}
		})
	}
}

func TestHexSoundnessOracle(t *testing.T) {
	g := hexGeom(t)
	cases := []struct {
		name string
		spec []placed
	}{
		{"winin1", specWinIn1},
		{"openfour", specOpenFour},
		{"doublefour", specDoubleFour},
		{"cross34", specCross34},
		{"overlinedecline", specOverlineDecline},
		{"counterfour", specCounterFour},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := buildAtFor(t, g, 11, 11, rules.Red, c.spec)
			p := buildOracleHex(t, b)
			for _, kind := range []Kind{KindVCF, KindVCT} {
				soundnessCheck(t, b, p, kind, config.SolverNodeBudget)
			}
		})
	}
}

func TestHexOverlineDiscipline(t *testing.T) {
	g := hexGeom(t)
	decline := buildAtFor(t, g, 11, 11, rules.Red, specOverlineDecline)
	s := New(KindVCF)
	var stats SolverStats
	if !s.Solve(decline, config.SolverNodeBudget, nil, &stats) {
		t.Fatal("overline decline: expected the near win")
	}
	if stats.Plies != 1 || stats.PV[0] != rules.Move(cellOf(11, 11, g)) {
		t.Fatalf("overline decline: plies %d pv %d want the anchor cell", stats.Plies, stats.PV[0])
	}
	trap := buildAtFor(t, g, 11, 12, rules.Red, specOverlineTrap)
	checkSolve(t, trap, KindVCF, -1)
	checkSolve(t, trap, KindVCT, -1)
}

func TestHexBlueAttacker(t *testing.T) {
	g := hexGeom(t)
	spec := []placed{
		o(0, 1), o(0, 2), o(0, 3), x(1, 1), x(1, 2), x(1, 3), x(1, 4),
	}
	b := buildAtFor(t, g, 11, 11, rules.Blue, spec)
	checkSolve(t, b, KindVCF, 1)
}

func TestHexBudgetExhaustionNeverWins(t *testing.T) {
	g := hexGeom(t)
	b := buildAtFor(t, g, 11, 11, rules.Red, specFourChain)
	for _, budget := range []int{0, 1, 2, 3, 5, 9} {
		checkSolveBudget(t, b, KindVCF, budget)
		checkSolveBudget(t, b, KindVCT, budget)
	}
}

func checkSolveBudget(t *testing.T, b *rules.Board, kind Kind, budget int) {
	t.Helper()
	s := New(kind)
	var stats SolverStats
	if s.Solve(b, budget, nil, &stats) {
		t.Fatalf("kind %d budget %d: unexpected win", kind, budget)
	}
}

func TestHexMaxPlyAbort(t *testing.T) {
	g := hexGeom(t)
	b := buildAtFor(t, g, 11, 11, rules.Red, specFourChain)
	s := New(KindVCF)
	s.maxPly = 4
	var stats SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &stats) {
		t.Fatal("max ply 4: unexpected win on a 7-ply chain")
	}
}

func TestHexMemoReuse(t *testing.T) {
	g := hexGeom(t)
	b := buildAtFor(t, g, 11, 11, rules.Red, specTwoDeadFours)
	s := New(KindVCF)
	var first, second SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &first) {
		t.Fatal("memo: two dead fours must stay unfound")
	}
	s.Solve(b, config.SolverNodeBudget, nil, &second)
	if second.Found || second.Nodes >= first.Nodes {
		t.Fatalf("memo: verdict flipped or nodes not reduced, %d then %d", first.Nodes, second.Nodes)
	}
}

func TestHexBoardRestored(t *testing.T) {
	g := hexGeom(t)
	b := buildAtFor(t, g, 11, 11, rules.Red, specFourChain)
	hash, count, side := b.Hash, b.MoveCount, b.Side
	s := New(KindVCT)
	var stats SolverStats
	s.Solve(b, config.SolverNodeBudget, nil, &stats)
	if b.Hash != hash || b.MoveCount != count || b.Side != side {
		t.Fatalf("board not restored: hash=%d/%d moves=%d/%d side=%d/%d", b.Hash, hash, b.MoveCount, count, b.Side, side)
	}
}

func TestHexRimHalfDeadFour(t *testing.T) {
	g := hexGeom(t)
	spec := []placed{o(0, -11), o(0, -10), o(0, -9), o(0, -8), x(3, 3)}
	b := buildAtFor(t, g, 11, 11, rules.Red, spec)
	s := New(KindVCF)
	nA, first := s.probeWins(b, rules.Red, s.winA[:])
	if nA != 1 || first != cellOf(11, 4, g) {
		t.Fatalf("rim four: win cells n=%d first=%v, want only the live end (11,4)", nA, first)
	}
	n := s.defusing(b, rules.Red, nA, 0)
	for i := range n {
		if !g.Contains(int(s.defends[0][i])) {
			t.Fatalf("rim four: defusing proposed off-region cell %d", s.defends[0][i])
		}
	}
	checkSolve(t, b, KindVCF, 1)
}

func TestHexRimFiveFramesCrossOffBox(t *testing.T) {
	g := hexGeom(t)
	spec := []placed{
		o(0, -11), o(0, -10), o(0, -9), o(0, -8), o(0, -7),
	}
	b := buildAtFor(t, g, 11, 11, rules.Red, spec)
	var frames [config.PatternDirections]fiveFrame
	n := fiveFrames(b, rules.Red, cellOf(11, 4, g), frames[:])
	if n != 1 {
		t.Fatalf("rim five: frames %d want 1", n)
	}
	want := fiveFrame{br: 11, bc: -1, fr: 11, fc: 5}
	if frames[0] != want {
		t.Fatalf("rim five: frame %+v want %+v", frames[0], want)
	}
	s := New(KindVCF)
	s.winA[0] = cellOf(11, 4, g)
	nD := s.defusing(b, rules.Red, 1, 0)
	for i := range nD {
		if !g.Contains(int(s.defends[0][i])) {
			t.Fatalf("rim five: defusing proposed off-region cell %d", s.defends[0][i])
		}
	}
}

func buildOracleHex(t *testing.T, b *rules.Board) opos {
	t.Helper()
	g := b.Geom
	var p opos
	p.g = g
	p.nb = *rules.NewNaiveBoardFor(g)
	for cell := range g.Cells {
		w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
		p.region[cell] = b.Region[w]&m != 0
		if b.Full[w]&m == 0 {
			continue
		}
		color := b.At(rules.Cell(cell))
		p.nb.Set(rules.Cell(cell), color)
		p.occ[cell] = true
		p.color[cell] = uint8(color)
	}
	return p
}

func seededHexSparse(t *testing.T, seed uint64, stones int) *rules.Board {
	t.Helper()
	g := hexGeom(t)
	b := rules.NewBoardFor(g)
	rng := rand.New(rand.NewPCG(seed, seed*5+3))
	empties := make([]rules.Cell, 0, g.Cells)
	for cell := 0; cell < g.Cells; cell++ {
		if g.Contains(cell) {
			empties = append(empties, rules.Cell(cell))
		}
	}
	for i := 0; i < stones && len(empties) > 0; i++ {
		k := rng.IntN(len(empties))
		b.Side = rules.Color(i % 2)
		b.Make(empties[k])
		empties = append(empties[:k], empties[k+1:]...)
	}
	b.Side = rules.Red
	return b
}

func TestHexProbeWinsSupersetOracle(t *testing.T) {
	g := hexGeom(t)
	s := New(KindVCF)
	fixtures := []*rules.Board{
		buildAtFor(t, g, 11, 11, rules.Red, []placed{o(0, 2), o(0, 3), o(0, 4), o(0, 6), x(3, 3)}),
		buildAtFor(t, g, 11, 11, rules.Red, []placed{o(1, 0), o(2, 0), o(3, 0), o(4, 0), x(3, 3)}),
		buildAtFor(t, g, 11, 11, rules.Red, []placed{o(0, -11), o(0, -10), o(0, -9), o(0, -8), x(3, 3)}),
	}
	sawWin := false
	for i, b := range fixtures {
		oracle := hexOracleWinInOne(t, g, b, rules.Red)
		if len(oracle) == 0 {
			t.Fatalf("fixture %d: expected oracle wins", i)
		}
		sawWin = true
		n, _ := s.probeWins(b, rules.Red, s.winA[:])
		found := make(map[rules.Cell]bool, n)
		for k := range n {
			found[s.winA[k]] = true
		}
		for c := range oracle {
			if !found[c] {
				t.Fatalf("fixture %d: oracle win-in-1 %d missing from probeWins", i, c)
			}
		}
	}
	boards := 6
	if testing.Short() {
		boards = 3
	}
	for i := range boards {
		b := seededHexSparse(t, uint64(i+40), 24+i*6)
		for _, side := range [2]rules.Color{rules.Red, rules.Blue} {
			oracle := hexOracleWinInOne(t, g, b, side)
			n, _ := s.probeWins(b, side, s.winA[:])
			found := make(map[rules.Cell]bool, n)
			for k := range n {
				found[s.winA[k]] = true
			}
			for c := range oracle {
				if !found[c] {
					t.Fatalf("board %d side %v: oracle win-in-1 %d missing from probeWins", i, side, c)
				}
			}
		}
	}
	if !sawWin {
		t.Fatal("no fixture produced an oracle win, the superset check ran vacuous")
	}
}

func hexOracleWinInOne(t *testing.T, g *geom.Geometry, b *rules.Board, side rules.Color) map[rules.Cell]bool {
	t.Helper()
	out := map[rules.Cell]bool{}
	for cell := 0; cell < g.Cells; cell++ {
		if !g.Contains(cell) || b.Full[cell/wordBits]&(1<<(uint(cell)%wordBits)) != 0 {
			continue
		}
		nb := rules.NewNaiveBoardFor(g)
		for other := 0; other < g.Cells; other++ {
			if g.Contains(other) && b.Full[other/wordBits]&(1<<(uint(other)%wordBits)) != 0 {
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

func TestHexDirReachMatchesWalk(t *testing.T) {
	g := hexGeom(t)
	b := seededHexSparse(t, 7, 30)
	reach := dirReach(b, rules.Red)
	for d := range g.Dirs {
		dr, dc := g.Dirs[d].Dr, g.Dirs[d].Dc
		want := make(map[rules.Cell]bool)
		for cell := 0; cell < g.Cells; cell++ {
			if !g.Contains(cell) || b.Red[cell/wordBits]&(1<<(uint(cell)%wordBits)) == 0 {
				continue
			}
			r, c := cell/g.Stride, cell%g.Stride
			want[rules.Cell(cell)] = true
			for k := 1; k <= windowHalf; k++ {
				for _, sign := range [2]int{k, -k} {
					rr, cc := r+sign*dr, c+sign*dc
					if rr < 0 || rr >= g.Rows || cc < 0 || cc >= g.Cols {
						continue
					}
					want[rules.Cell(rr*g.Stride+cc)] = true
				}
			}
		}
		for cell := 0; cell < g.Cells; cell++ {
			if !g.Contains(cell) {
				continue
			}
			m := uint64(1) << (uint(cell) % wordBits)
			got := reach[d][cell/wordBits]&m != 0
			if got != want[rules.Cell(cell)] {
				t.Fatalf("dir %d cell %d: reach %v want %v", d, cell, got, want[rules.Cell(cell)])
			}
		}
	}
}
