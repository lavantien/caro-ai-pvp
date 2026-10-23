package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var redRules = rules.Red

func rulesCell(f int) rules.Cell { return rules.Cell(f) }

func newHexRulesBoard(g *geom.Geometry) *rules.Board { return rules.NewBoardFor(g) }

type geomReport struct {
	name              string
	regionPopcount    int
	maxInRegion       int
	cells             int
	words             int
	parseRoundTripBad int
	centerName        string
	centerFlat        int
	distanceBad       int
	fwdMaskBad        int
	bwdMaskBad        int
	winsDiffBad       int
}

func regionOf(g *geom.Geometry) []int {
	var out []int
	for f := range g.Cells {
		if g.Contains(f) {
			out = append(out, f)
		}
	}
	return out
}

func bfsDistances(g *geom.Geometry, from int) []int {
	dist := make([]int, g.Cells)
	for i := range dist {
		dist[i] = -1
	}
	dist[from] = 0
	queue := []int{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		q, r := cur/g.Stride, cur%g.Stride
		for _, n := range g.Neighbors {
			qq, rr := q+n[0], r+n[1]
			if qq < 0 || qq >= g.Rows || rr < 0 || rr >= g.Cols {
				continue
			}
			nc := qq*g.Stride + rr
			if nc >= g.Cells || !g.Contains(nc) || dist[nc] >= 0 {
				continue
			}
			dist[nc] = dist[cur] + 1
			queue = append(queue, nc)
		}
	}
	return dist
}

func maskPropertyBad(g *geom.Geometry, forward bool) int {
	bad := 0
	box := g.Rows * g.Stride
	for s := range box {
		q, r := s/g.Stride, s%g.Stride
		for di := range g.Dirs {
			d := &g.Dirs[di]
			var target int
			var mask *[9]uint64
			var qq, rr int
			if forward {
				target = s + int(d.Step)
				mask = &d.FwdClr
				qq, rr = q+d.Dr, r+d.Dc
			} else {
				target = s - int(d.Step)
				mask = &d.BwdClr
				qq, rr = q-d.Dr, r-d.Dc
			}
			truth := qq >= 0 && qq < g.Rows && rr >= 0 && rr < g.Cols
			var repo bool
			if target >= 0 && target < box {
				m := uint64(1) << uint(target%64)
				w := target / 64
				if w < len(g.BoxClear) {
					repo = mask[w]&m == 0 && g.BoxClear[w]&m == 0
				}
			}
			if repo != truth {
				bad++
			}
		}
	}
	return bad
}

func winsDiffBad(g *geom.Geometry, boards int) int {
	bad := 0
	for i := range boards {
		rb := rules.NewBoardFor(g)
		mb := newVBoardFor(g)
		rng := rand.New(rand.NewPCG(uint64(i+31), uint64(i*7+1)))
		pool := mb.empties()
		col := int8(redCell)
		for m := 0; m < 50 && len(pool) > 0; m++ {
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
			for _, side := range [2]int8{redCell, blueCell} {
				want := rb.Wins(colorOf(side))
				got := naiveWins(mb, g, side)
				if want != got {
					bad++
				}
			}
			col = 3 - col
		}
	}
	return bad
}

func colorOf(c int8) rules.Color {
	if c == redCell {
		return rules.Red
	}
	return rules.Blue
}

func naiveWins(b *vboard, g *geom.Geometry, col int8) bool {
	for f := range g.Cells {
		if b.cells[f] == col && b.winThrough(f, col) {
			return true
		}
	}
	return false
}

func newVBoardFor(g *geom.Geometry) *vboard {
	b := &vboard{}
	for q := range g.Rows {
		for r := range g.Cols {
			f := q*g.Stride + r
			b.cells[f] = emptyCell
			if f < len(b.region) {
				b.region[f] = g.Contains(f)
			}
		}
	}
	for f := range hexBox {
		b.region[f] = f < g.Cells && g.Contains(f)
	}
	return b
}

func checkGeom(g *geom.Geometry) geomReport {
	rep := geomReport{name: g.Name, cells: g.Cells, words: g.Words}
	reg := regionOf(g)
	rep.regionPopcount = len(reg)
	rep.maxInRegion = reg[len(reg)-1]
	for _, f := range reg {
		name, err := g.CellName(f)
		if err != nil {
			rep.parseRoundTripBad++
			continue
		}
		back, err := g.ParseCell(name)
		if err != nil || back != f {
			rep.parseRoundTripBad++
		}
	}
	rep.centerName, _ = g.CellName(g.Center)
	rep.centerFlat = g.Center
	for _, from := range reg {
		dist := bfsDistances(g, from)
		for _, to := range reg {
			if g.Distance(from, to) != dist[to] {
				rep.distanceBad++
			}
		}
	}
	rep.fwdMaskBad = maskPropertyBad(g, true)
	rep.bwdMaskBad = maskPropertyBad(g, false)
	if g.Name == "hex" {
		rep.winsDiffBad = winsDiffBad(g, 60)
	}
	return rep
}

func runGeom(outDir string) {
	path := filepath.Join(outDir, "geom-battery.txt")
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
	for _, name := range []string{"square", "hex"} {
		g, err := geom.ByName(name)
		if err != nil {
			fail(err)
		}
		rep := checkGeom(g)
		p("%s: cells=%d words=%d region=%d maxInRegion=%d center=%s(%d)\n", rep.name, rep.cells, rep.words, rep.regionPopcount, rep.maxInRegion, rep.centerName, rep.centerFlat)
		p("%s: parseRoundTripBad=%d distanceBad=%d fwdMaskBad=%d bwdMaskBad=%d winsDiffBad=%d\n", rep.name, rep.parseRoundTripBad, rep.distanceBad, rep.fwdMaskBad, rep.bwdMaskBad, rep.winsDiffBad)
	}
	hex, _ := geom.ByName("hex")
	sq, _ := geom.ByName("square")
	edge := [][2]string{{"L12", "L11"}, {"L12", "M12"}, {"L12", "M11"}, {"L12", "K13"}, {"A1", "W1"}, {"A1", "A23"}, {"L12", "A1"}, {"L12", "W23"}}
	for _, e := range edge {
		a, err1 := hex.ParseCell(e[0])
		b, err2 := hex.ParseCell(e[1])
		if err1 == nil && err2 == nil {
			p("hex distance %s-%s = %d\n", e[0], e[1], hex.Distance(a, b))
		} else {
			p("hex distance %s-%s parse error %v %v\n", e[0], e[1], err1, err2)
		}
	}
	a, _ := sq.ParseCell("A1")
	b, _ := sq.ParseCell("P16")
	p("square distance A1-P16 = %d\n", sq.Distance(a, b))
}
