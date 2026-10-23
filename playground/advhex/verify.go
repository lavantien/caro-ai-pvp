package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	hexStride = 23
	hexBox    = hexStride * hexStride
	emptyCell = 0
	redCell   = 1
	blueCell  = 2
	winLen    = 5
)

var hexAxes = [3][2]int{{0, 1}, {1, 0}, {1, -1}}

type vboard struct {
	cells  [hexBox]int8
	region [hexBox]bool
}

func newVBoard() *vboard {
	b := &vboard{}
	for q := range hexStride {
		for r := range hexStride {
			dq, dr, ds := q-11, r-11, q+r-22
			if dq < 0 {
				dq = -dq
			}
			if dr < 0 {
				dr = -dr
			}
			if ds < 0 {
				ds = -ds
			}
			b.region[q*hexStride+r] = dq <= 11 && dr <= 11 && ds <= 11
		}
	}
	return b
}

func (b *vboard) inBox(q, r int) bool { return q >= 0 && q < hexStride && r >= 0 && r < hexStride }

func (b *vboard) stone(q, r int) int8 {
	if !b.inBox(q, r) {
		return emptyCell
	}
	return b.cells[q*hexStride+r]
}

func (b *vboard) place(f int, col int8) { b.cells[f] = col }

func (b *vboard) unplace(f int) { b.cells[f] = emptyCell }

func (b *vboard) winThrough(f int, col int8) bool {
	q, r := f/hexStride, f%hexStride
	opp := int8(3 - col)
	for _, ax := range hexAxes {
		back := 0
		bq, br := q-ax[0], r-ax[1]
		for back < winLen && b.stone(bq, br) == col {
			back++
			bq -= ax[0]
			br -= ax[1]
		}
		fwd := 0
		fq, fr := q+ax[0], r+ax[1]
		for fwd < winLen && b.stone(fq, fr) == col {
			fwd++
			fq += ax[0]
			fr += ax[1]
		}
		if back+fwd+1 != winLen {
			continue
		}
		blockedBack := b.stone(bq, br) == opp
		blockedFwd := b.stone(fq, fr) == opp
		if !blockedBack || !blockedFwd {
			return true
		}
	}
	return false
}

func (b *vboard) reach(col int8) []int {
	marks := make([]bool, hexBox)
	n := 0
	for f := range hexBox {
		if b.cells[f] != col {
			continue
		}
		q, r := f/hexStride, f%hexStride
		for _, ax := range hexAxes {
			for k := -4; k <= 4; k++ {
				if k == 0 {
					continue
				}
				qq, rr := q+k*ax[0], r+k*ax[1]
				if !b.inBox(qq, rr) {
					continue
				}
				g := qq*hexStride + rr
				if b.region[g] && b.cells[g] == emptyCell && !marks[g] {
					marks[g] = true
					n++
				}
			}
		}
	}
	out := make([]int, 0, n)
	for f := range hexBox {
		if marks[f] {
			out = append(out, f)
		}
	}
	return out
}

func (b *vboard) empties() []int {
	out := make([]int, 0, 397)
	for f := range hexBox {
		if b.region[f] && b.cells[f] == emptyCell {
			out = append(out, f)
		}
	}
	return out
}

func (b *vboard) anyWinCell(col int8) (int, bool) {
	for _, f := range b.reach(col) {
		b.place(f, col)
		w := b.winThrough(f, col)
		b.unplace(f)
		if w {
			return f, true
		}
	}
	return 0, false
}

func (b *vboard) winCells(col int8) []int {
	var out []int
	for _, f := range b.reach(col) {
		b.place(f, col)
		w := b.winThrough(f, col)
		b.unplace(f)
		if w {
			out = append(out, f)
		}
	}
	return out
}

type pathMove struct {
	f   int
	att bool
}

type verifier struct {
	b             *vboard
	memo          map[string]bool
	forcedCalls   int
	defendCalls   int
	defenderMoves int
	memoHits      int
	threatsOnly   bool
	foursOnly     bool
	survivingLine []pathMove
	path          []pathMove
	attacker      int8
}

func (v *verifier) key(plies int) string {
	buf := make([]byte, 0, 400)
	buf = append(buf, byte(plies))
	for f := range hexBox {
		if v.b.region[f] && v.b.cells[f] != emptyCell {
			buf = append(buf, byte(f>>8), byte(f), byte(v.b.cells[f]))
		}
	}
	return string(buf)
}

func (v *verifier) forced(plies int) bool {
	if plies <= 0 {
		return false
	}
	k := v.key(plies)
	if r, ok := v.memo[k]; ok {
		v.memoHits++
		return r
	}
	res := v.forcedCompute(plies)
	v.memo[k] = res
	return res
}

func (v *verifier) forcedCompute(plies int) bool {
	v.forcedCalls++
	att, def := v.attacker, int8(3-v.attacker)
	if _, ok := v.b.anyWinCell(att); ok {
		return true
	}
	if plies == 1 {
		return false
	}
	if dCells := v.b.winCells(def); len(dCells) > 0 {
		if len(dCells) >= 2 {
			return false
		}
		block := dCells[0]
		v.path = append(v.path, pathMove{block, true})
		v.b.place(block, att)
		r := v.forced(plies - 1)
		v.b.unplace(block)
		v.path = v.path[:len(v.path)-1]
		return r
	}
	var cands []int
	if v.threatsOnly {
		cands = v.threatCands(att, plies)
	} else {
		cands = v.b.empties()
	}
	for _, a := range cands {
		v.path = append(v.path, pathMove{a, true})
		v.b.place(a, att)
		r := v.defend(plies - 1)
		v.b.unplace(a)
		v.path = v.path[:len(v.path)-1]
		if r {
			return true
		}
	}
	return false
}

func (v *verifier) threatCands(att int8, plies int) []int {
	fours := v.fourMakers(att)
	if plies <= 3 || v.foursOnly {
		return fours
	}
	fourSet := make(map[int]bool, len(fours))
	for _, f := range fours {
		fourSet[f] = true
	}
	out := append([]int(nil), fours...)
	for _, a := range v.b.reach(att) {
		if fourSet[a] {
			continue
		}
		if v.makesThree(a, att) {
			out = append(out, a)
		}
	}
	return out
}

func (v *verifier) fourMakers(att int8) []int {
	var out []int
	for _, a := range v.b.reach(att) {
		v.b.place(a, att)
		_, w := v.b.anyWinCell(att)
		v.b.unplace(a)
		if w {
			out = append(out, a)
		}
	}
	return out
}

func (v *verifier) makesThree(a int, att int8) bool {
	v.b.place(a, att)
	defer v.b.unplace(a)
	for _, e := range v.b.reach(att) {
		v.b.place(e, att)
		_, w := v.b.anyWinCell(att)
		v.b.unplace(e)
		if w {
			return true
		}
	}
	return false
}

func (v *verifier) defend(plies int) bool {
	v.defendCalls++
	if plies <= 0 {
		v.recordSurvival()
		return false
	}
	def := int8(3 - v.attacker)
	if _, ok := v.b.anyWinCell(def); ok {
		v.recordSurvival()
		return false
	}
	for _, d := range v.b.empties() {
		v.defenderMoves++
		v.path = append(v.path, pathMove{d, false})
		v.b.place(d, def)
		lost := v.b.winThrough(d, def)
		var r bool
		if lost {
			r = false
		} else {
			r = v.forced(plies - 1)
		}
		v.b.unplace(d)
		v.path = v.path[:len(v.path)-1]
		if !r {
			v.recordSurvival()
			return false
		}
	}
	return true
}

func (v *verifier) recordSurvival() {
	if v.survivingLine == nil {
		v.survivingLine = append([]pathMove(nil), v.path...)
	}
}

func replayPv(pv []rules.Move) string {
	v := newFixtureVerifier(false)
	att := int8(redCell)
	notes := []string{}
	for i, m := range pv {
		f := int(m)
		if !v.b.region[f] || v.b.cells[f] != emptyCell {
			return fmt.Sprintf("illegal pv entry %d", i)
		}
		v.b.place(f, att)
		w := v.b.winThrough(f, att)
		if w && i != len(pv)-1 {
			return "intermediate win on the pv"
		}
		if w && i == len(pv)-1 {
			notes = append(notes, "final move wins")
		}
		att = 3 - att
	}
	if len(notes) == 1 {
		return "final move completes a five, all earlier entries legal and non-winning"
	}
	return "final move does not win on the independent model"
}

func runVcfSweep(outDir string) {
	path := filepath.Join(outDir, "twodeadfours-vcf-sweep.txt")
	fh, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer func() { _ = fh.Close() }()
	p := func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		fmt.Print(s)
		_, _ = fmt.Fprint(fh, s)
	}
	for plies := 1; plies <= 15; plies += 2 {
		v := newFixtureVerifier(true)
		v.foursOnly = true
		res := v.forced(plies)
		p("four-only forced-win-in-%d: %v (forcedCalls=%d defenderMoves=%d)\n", plies, res, v.forcedCalls, v.defenderMoves)
		if res {
			p("four-only sweep: a fours-only forced win exists, solver VCF not-found is a false negative\n")
			return
		}
	}
	p("four-only sweep: no forced win through ply 15 with attacker restricted to four makers, solver VCF not-found consistent\n")
}

func cellName(f int) string {
	q, r := f/hexStride, f%hexStride
	return string(rune('A'+r)) + fmt.Sprint(q+1)
}

func lineNames(line []pathMove) string {
	var sb strings.Builder
	for i, m := range line {
		if i > 0 {
			sb.WriteString(" ")
		}
		side := "D"
		if m.att {
			side = "A"
		}
		sb.WriteString(side)
		sb.WriteString(":")
		sb.WriteString(cellName(m.f))
	}
	return sb.String()
}

func newFixtureVerifier(threatsOnly bool) *verifier {
	v := &verifier{b: newVBoard(), memo: map[string]bool{}, attacker: redCell, threatsOnly: threatsOnly}
	for _, s := range hexFixture() {
		col := int8(redCell)
		if !s.red {
			col = blueCell
		}
		v.b.place(s.q*hexStride+s.r, col)
	}
	return v
}

func crossCheckWinLogic(g *geom.Geometry, boards int) error {
	for i := range boards {
		rb := rules.NewBoardFor(g)
		mb := newVBoard()
		rng := rand.New(rand.NewPCG(uint64(i+900), uint64(i*17+5)))
		pool := mb.empties()
		col := int8(redCell)
		for m := range 60 {
			if len(pool) == 0 {
				break
			}
			k := rng.IntN(len(pool))
			f := pool[k]
			pool = append(pool[:k], pool[k+1:]...)
			ruleCol := rules.Red
			if col == blueCell {
				ruleCol = rules.Blue
			}
			rb.Side = ruleCol
			rb.Make(rules.Cell(f))
			mb.place(f, col)
			want := rb.FastLastMoveWin(ruleCol, rules.Cell(f))
			got := mb.winThrough(f, col)
			if want != got {
				return fmt.Errorf("board %d move %d cell %s: rules win %v mine %v", i, m, cellName(f), want, got)
			}
			if want {
				break
			}
			col = 3 - col
		}
	}
	return nil
}

func runVerify(outDir string) {
	defer func() { runVcfSweep(outDir) }()
	g, err := geom.ByName("hex")
	if err != nil {
		fail(err)
	}
	found, plies, pv, nodes := solverClaim()
	path := filepath.Join(outDir, "twodeadfours-verdict.txt")
	fh, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer func() { _ = fh.Close() }()
	p := func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		fmt.Print(s)
		_, _ = fmt.Fprint(fh, s)
	}
	if err := crossCheckWinLogic(g, 150); err != nil {
		p("cross-check FAILED: %v\n", err)
		fail(err)
	}
	p("cross-check: 150 random hex games, winThrough agrees with rules.FastLastMoveWin at every move\n")
	p("solver claim: found=%v plies=%d nodes=%d\n", found, plies, nodes)
	if !found {
		p("verdict: solver reports NO vct win; nothing to verify\n")
		return
	}
	v := newFixtureVerifier(true)
	p("verifier: attacker moves restricted to four and three makers, defender tries all %d legal cells, exact five rules (overline dead per axis, both-ends-stone-blocked lines dead)\n", len(v.b.empties()))
	res := v.forced(plies)
	p("forced-win-in-%d: %v (forcedCalls=%d defendCalls=%d defenderMoves=%d memoHits=%d memo=%d)\n", plies, res, v.forcedCalls, v.defendCalls, v.defenderMoves, v.memoHits, len(v.memo))
	if res {
		shorter := newFixtureVerifier(true)
		p("minimality: forced-win-in-%d = %v\n", plies-2, shorter.forced(plies-2))
		p("verdict: PROVEN, red forces a win in %d plies against every legal defense\n", plies)
		p("solver pv: %s\n", pvNames(g, pv))
		p("pv replay on independent model: %v\n", replayPv(pv))
		return
	}
	p("surviving line under threat-restricted attacker: %s\n", lineNames(v.survivingLine))
	full := newFixtureVerifier(false)
	p("escalating to unrestricted attacker moves\n")
	res2 := full.forced(plies)
	p("forced-win-in-%d unrestricted attacker: %v (forcedCalls=%d defendCalls=%d defenderMoves=%d)\n", plies, res2, full.forcedCalls, full.defendCalls, full.defenderMoves)
	if res2 {
		p("verdict: PROVEN once quiet attacker moves are allowed\n")
		return
	}
	p("verdict: REFUTED, exhaustive search finds a surviving defense\n")
	printRootDefenses(p, plies)
}

func printRootDefenses(p func(string, ...any), plies int) {
	for _, a := range newFixtureVerifier(false).b.empties() {
		v := newFixtureVerifier(false)
		v.b.place(a, redCell)
		v.path = []pathMove{{a, true}}
		if !v.defend(plies - 1) {
			p("root attacker move %s refuted by defense line %s\n", cellName(a), lineNames(v.survivingLine))
		}
	}
}

func pvNames(g *geom.Geometry, pv []rules.Move) string {
	var sb strings.Builder
	for i, m := range pv {
		if i > 0 {
			sb.WriteString(" ")
		}
		name, _ := g.CellName(int(m))
		sb.WriteString(name)
	}
	return sb.String()
}
