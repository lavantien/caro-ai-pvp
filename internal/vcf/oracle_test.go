package vcf

import (
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Cross-check oracle: full width AND/OR minimax over the naive array board,
// sharing no code with the solvers. Terminals come from rules.NaiveBoard
// through WinsThrough, the array path the solvers never touch. The
// soundness criterion per spec: every solver-claimed win of P plies must
// be confirmed as a real forced win within exactly P plies, since a sound
// P-ply claim is a P-ply strategy and the evaluation below is exact.
// Refutations need no verification, completeness is not claimed.

const (
	oracleNodeBudget = 1_500_000
	fuzzNodeBudget   = 2500
	randomCrossRuns  = 80
	randomFullRuns   = 24
	// oracleClaimPlyMax bounds the forced line length of every solver run
	// handed to the oracle: full width confirmation cost grows
	// exponentially with the claim depth, so the soundness harness keeps
	// every claim inside what the oracle can decide. The production
	// solvers stay unbounded.
	oracleClaimPlyMax = 5
	// oracleCutTwoMaxDepth is the deepest claim for which oracle cut (2),
	// collapsing inert ring-5 defender replies, stays exact: every
	// completing frame holds an existing stone only while each side gets
	// fewer than 5 fresh placements in the horizon, and a side needs
	// ceil(d/2) moves to build an all-fresh five, breaking at d = 9.
	// Deeper clips must revisit the cut before use, else the oracle can
	// confirm false claims by skipping real refutations.
	oracleCutTwoMaxDepth = 8
)

// TestOracleClaimPlyCapStaysWithinCutBounds pins the tie between the claim
// clip and cut (2): raising oracleClaimPlyMax past oracleCutTwoMaxDepth
// without re-proving the cut would let the oracle produce false
// confirmations, the dangerous direction for a soundness oracle.
func TestOracleClaimPlyCapStaysWithinCutBounds(t *testing.T) {
	if oracleClaimPlyMax > oracleCutTwoMaxDepth {
		t.Fatalf("oracleClaimPlyMax = %d exceeds the depth where cut (2) stays exact, %d",
			oracleClaimPlyMax, oracleCutTwoMaxDepth)
	}
}

type opos struct {
	nb     rules.NaiveBoard
	occ    [config.BoardCells]bool
	color  [config.BoardCells]uint8
	region [config.BoardCells]bool
}

type oracleLim struct {
	nodes  int
	capped bool
}

// stoneList collects the cells of one color in one board pass, so the ring
// classification of all empties costs empties times stones instead of
// empties times cells.
func stoneList(p *opos, color rules.Color) [config.BoardCells]int {
	var out [config.BoardCells]int
	n := 0
	want := uint8(rules.Red)
	if color == rules.Blue {
		want = uint8(rules.Blue)
	}
	for cell := range config.BoardCells {
		if p.occ[cell] && p.color[cell] == want {
			out[n] = cell
			n++
		}
	}
	out[config.BoardCells-1] = n
	return out
}

func ringOf(cell int, stones [config.BoardCells]int) int {
	n := stones[config.BoardCells-1]
	r, c := cell/config.BoardStride, cell%config.BoardStride
	best := 5
	for i := 0; i < n; i++ {
		dr := stones[i]/config.BoardStride - r
		dc := stones[i]%config.BoardStride - c
		if dr < 0 {
			dr = -dr
		}
		if dc < 0 {
			dc = -dc
		}
		if dr < dc {
			dr = dc
		}
		if dr < best {
			best = dr
		}
	}
	return best
}

// oracleForced decides whether attacker forces a win within depth plies.
// Confirmation exactness is what the soundness criterion consumes, and it
// survives three cost cuts: (1) a winning placement always sits at
// Chebyshev ring 1 of its own color, since the other four cells of the
// completing five are stones, so depth 1 nodes probe ring 1 only;
// (2) a defender stone at ring 5 of both colors cannot enter any frame
// that completes within the horizon, because every completing frame holds
// an existing stone and spans at most 4 cells around it, so all such
// inert replies evaluate once through a single representative;
// (3) the attacker visits near cells first, where every witness of a
// forced threat line lives. decided is false when the horizon, the budget,
// or a skipped far attacker try left the value unknown.
func oracleForced(p opos, attacker, side rules.Color, depth int, pv []rules.Move, ply int, lim *oracleLim) (bool, bool) {
	lim.nodes--
	if lim.nodes < 0 {
		lim.capped = true
		return false, false
	}
	hint := rules.Move(config.BoardCells)
	if ply < len(pv) {
		hint = pv[ply]
	}
	var ring1, mid, far [config.BoardCells]rules.Cell
	var n1, nmid, nfar int
	moverStones := stoneList(&p, side)
	otherStones := stoneList(&p, side.Opponent())
	for cell := range config.BoardCells {
		if p.occ[cell] || !p.region[cell] {
			continue
		}
		c := rules.Cell(cell)
		switch ringOf(cell, moverStones) {
		case 1:
			ring1[n1] = c
			n1++
		case 2, 3, 4:
			mid[nmid] = c
			nmid++
		default:
			far[nfar] = c
			nfar++
		}
	}
	if n1+nmid+nfar == 0 {
		return false, true
	}
	undecided := false
	tryIdx := rules.Cell(config.BoardCells)
	if hint < rules.Move(config.BoardCells) {
		tryIdx = rules.Cell(hint)
		if tryIdx >= rules.Cell(config.BoardCells) || p.occ[int(tryIdx)] || !p.region[int(tryIdx)] {
			tryIdx = rules.Cell(config.BoardCells)
		}
	}
	try := func(cell rules.Cell) (bool, bool) {
		child := p
		child.nb.Set(cell, side)
		child.occ[cell] = true
		child.color[cell] = uint8(side)
		if child.nb.WinsThrough(side, cell) {
			if side == attacker {
				return true, true
			}
			return false, true
		}
		if depth <= 1 {
			return false, false
		}
		return oracleForced(child, attacker, side.Opponent(), depth-1, pv, ply+1, lim)
	}
	if depth == 1 {
		if tryIdx != rules.Cell(config.BoardCells) {
			if f, d := try(tryIdx); f && d {
				if side == attacker {
					return true, true
				}
				return false, true
			}
		}
		for i := 0; i < n1; i++ {
			if ring1[i] == tryIdx {
				continue
			}
			if f, d := try(ring1[i]); f && d {
				if side == attacker {
					return true, true
				}
				return false, true
			}
		}
		return false, false
	}
	if side == attacker {
		tryHint := func() (bool, bool) {
			if tryIdx == rules.Cell(config.BoardCells) {
				return false, false
			}
			f, d := try(tryIdx)
			if f && d {
				return true, true
			}
			if !d {
				undecided = true
			}
			return false, true
		}
		if hit, res := tryHint(); hit {
			return res, true
		}
		for i := 0; i < n1; i++ {
			if ring1[i] == tryIdx {
				continue
			}
			if f, d := try(ring1[i]); f && d {
				return true, true
			} else if !d {
				undecided = true
			}
		}
		for i := 0; i < nmid; i++ {
			if f, d := try(mid[i]); f && d {
				return true, true
			} else if !d {
				undecided = true
			}
		}
		for i := 0; i < nfar; i++ {
			if f, d := try(far[i]); f && d {
				return true, true
			} else if !d {
				undecided = true
			}
		}
		return false, !undecided
	}
	refuted := false
	for i := 0; i < n1; i++ {
		f, d := try(ring1[i])
		if !f && d {
			refuted = true
			break
		}
		if !d {
			undecided = true
		}
	}
	if !refuted {
		for i := 0; i < nmid; i++ {
			f, d := try(mid[i])
			if !f && d {
				refuted = true
				break
			}
			if !d {
				undecided = true
			}
		}
	}
	if !refuted {
		skippedInert := false
		for i := 0; i < nfar; i++ {
			if ringOf(int(far[i]), otherStones) <= 4 {
				f, d := try(far[i])
				if !f && d {
					refuted = true
					break
				}
				if !d {
					undecided = true
				}
				continue
			}
			if skippedInert {
				continue
			}
			skippedInert = true
			f, d := try(far[i])
			if !f && d {
				refuted = true
				break
			}
			if !d {
				undecided = true
			}
		}
	}
	if refuted {
		return false, true
	}
	return !undecided, !undecided
}

func buildOracleAt(t *testing.T, cross bool, ar, ac int, side rules.Color, spec []placed) (*rules.Board, opos) {
	t.Helper()
	b := buildAt(t, cross, ar, ac, side, spec)
	var p opos
	if cross {
		p.nb = *rules.NewNaiveCrossCheck()
	} else {
		p.nb = *rules.NewNaiveBoard()
	}
	for r := range config.BoardSize {
		for c := range config.BoardSize {
			cell := r*config.BoardStride + c
			w, m := cell/64, uint64(1)<<(uint(cell)%64)
			p.region[cell] = b.Region[w]&m != 0
			switch b.At(rules.Cell(cell)) {
			case rules.Red:
				p.nb.Set(rules.Cell(cell), rules.Red)
				p.occ[cell] = true
				p.color[cell] = uint8(rules.Red)
			case rules.Blue:
				p.nb.Set(rules.Cell(cell), rules.Blue)
				p.occ[cell] = true
				p.color[cell] = uint8(rules.Blue)
			}
		}
	}
	if p.nb.Wins(rules.Red) || p.nb.Wins(rules.Blue) {
		t.Fatalf("construction already holds a win")
	}
	return b, p
}

// soundnessCheck runs one solver and hands every claim to the oracle. A
// claimed win the oracle cannot confirm is a soundness violation. The
// solver's forced line length is clipped to oracleClaimPlyMax so every
// claim stays decidable inside the oracle's node budget.
func soundnessCheck(t *testing.T, b *rules.Board, p opos, kind Kind, budget int) {
	t.Helper()
	s := New(kind)
	s.maxPly = min(s.maxPly, oracleClaimPlyMax)
	var stats SolverStats
	if !s.Solve(b, budget, nil, &stats) {
		return
	}
	verifyForcedPV(t, b, stats)
	lim := &oracleLim{nodes: oracleNodeBudget}
	forced, decided := oracleForced(p, b.Side, b.Side, stats.Plies, stats.PV[:stats.Plies], 0, lim)
	if !forced || !decided {
		t.Fatalf("kind %d: claimed win not confirmed, forced=%v decided=%v capped=%v plies=%d nodes=%d",
			kind, forced, decided, lim.capped, stats.Plies, stats.Nodes)
	}
}

func TestOracleConfirmsConstructions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ar    int
		ac    int
		side  rules.Color
		spec  []placed
		kinds []Kind
	}{
		{"winin1", 1, 1, rules.Red, specWinIn1, []Kind{KindVCF, KindVCT}},
		{"openfour", 1, 1, rules.Red, specOpenFour, []Kind{KindVCF, KindVCT}},
		{"doublefour", 4, 2, rules.Red, specDoubleFour, []Kind{KindVCF, KindVCT}},
		{"cross34", 4, 2, rules.Red, specCross34, []Kind{KindVCF, KindVCT}},
		{"overlinedecline", 1, 1, rules.Red, specOverlineDecline, []Kind{KindVCF, KindVCT}},
		{"counterfour", 4, 1, rules.Red, specCounterFour, []Kind{KindVCF}},
		{"doublethree", 4, 6, rules.Red, specVCTDoubleThree, []Kind{KindVCF, KindVCT}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, p := buildOracleAt(t, true, tc.ar, tc.ac, tc.side, tc.spec)
			for _, kind := range tc.kinds {
				soundnessCheck(t, b, p, kind, config.SolverNodeBudget)
			}
		})
	}
}

// genRandom derives a clustered random position: stones only inside a
// small window so the oracle's full width stays tractable, both colors
// present, no pre-existing five, at least four stones so the opening rule
// constrains nothing.
func genRandom(seedA, seedB uint64, cross bool) (*rules.Board, opos, bool) {
	rng := rand.New(rand.NewPCG(seedA, seedB))
	region := config.BoardSize
	if cross {
		region = config.CrossCheckSize
	}
	win := 4 + rng.IntN(4)
	baseR := rng.IntN(region - win + 1)
	baseC := rng.IntN(region - win + 1)
	want := 4 + rng.IntN(7)
	var b *rules.Board
	var p opos
	if cross {
		b = rules.NewCrossCheck()
		p.nb = *rules.NewNaiveCrossCheck()
		for r := range config.CrossCheckSize {
			for c := range config.CrossCheckSize {
				p.region[r*config.BoardStride+c] = true
			}
		}
	} else {
		b = rules.NewBoard()
		p.nb = *rules.NewNaiveBoard()
		for r := range config.BoardSize {
			for c := range config.BoardSize {
				p.region[r*config.BoardStride+c] = true
			}
		}
	}
	side := rules.Red
	if rng.IntN(2) == 1 {
		side = rules.Blue
	}
	for placed, cells := 0, 0; placed < want && cells < win*win; cells++ {
		r := baseR + rng.IntN(win)
		c := baseC + rng.IntN(win)
		cell := r*config.BoardStride + c
		if p.occ[cell] {
			continue
		}
		color := rules.Red
		if placed%2 == 1 {
			color = rules.Blue
		}
		b.Side = color
		b.Make(rules.Cell(cell))
		p.nb.Set(rules.Cell(cell), color)
		p.occ[cell] = true
		p.color[cell] = uint8(color)
		placed++
	}
	if p.nb.Wins(rules.Red) || p.nb.Wins(rules.Blue) {
		return nil, p, false
	}
	b.Side = side
	return b, p, true
}

func TestSoundnessRandomPositions(t *testing.T) {
	for i := range randomCrossRuns {
		b, p, ok := genRandom(uint64(i)*2+1, 0x9E3779B9, true)
		if !ok {
			continue
		}
		soundnessCheck(t, b, p, KindVCF, fuzzNodeBudget)
		soundnessCheck(t, b, p, KindVCT, fuzzNodeBudget)
	}
	for i := range randomFullRuns {
		b, _, ok := genRandom(uint64(i)*2+2, 0x5851F42D4C957F2D, false)
		if !ok {
			continue
		}
		for _, kind := range []Kind{KindVCF, KindVCT} {
			s := New(kind)
			var stats SolverStats
			if s.Solve(b, fuzzNodeBudget, nil, &stats) {
				verifyForcedPV(t, b, stats)
			}
		}
	}
}

func fuzzSeeds(data []byte) (uint64, uint64) {
	var buf [16]byte
	for i, d := range data {
		buf[i%16] ^= d
	}
	return binary.LittleEndian.Uint64(buf[:8]), binary.LittleEndian.Uint64(buf[8:])
}

func FuzzSoundness(f *testing.F) {
	f.Add([]byte("cross-doublethree"))
	f.Add([]byte("full-counterfour"))
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		a, b := fuzzSeeds(data)
		for _, cross := range [...]bool{true, false} {
			board, p, ok := genRandom(a^uint64(len(data)), b, cross)
			if !ok {
				continue
			}
			if cross {
				soundnessCheck(t, board, p, KindVCF, fuzzNodeBudget)
				soundnessCheck(t, board, p, KindVCT, fuzzNodeBudget)
				continue
			}
			for _, kind := range []Kind{KindVCF, KindVCT} {
				s := New(kind)
				var stats SolverStats
				if s.Solve(board, fuzzNodeBudget, nil, &stats) {
					verifyForcedPV(t, board, stats)
				}
			}
		}
	})
}
