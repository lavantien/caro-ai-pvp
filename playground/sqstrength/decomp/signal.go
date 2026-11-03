package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type lineKey struct {
	dir  int
	line int
}

type sideSignals struct {
	CleanWin int
	CleanLn  int
	CleanSt  int
	OpenBld  int
	MaxSt    int
}

func lineIndex(g *geom.Geometry, cell int, d int) int {
	dr, dc := g.Dirs[d].Dr, g.Dirs[d].Dc
	r, c := cell/g.Stride, cell%g.Stride
	if dr == 0 {
		return r
	}
	if dc == 0 {
		return c
	}
	if dc > 0 {
		return r - c + g.Stride
	}
	return r + c
}

func computeSignals(g *geom.Geometry, b *rules.Board) (blue, red sideSignals) {
	for d := range g.Dirs {
		for cell := range g.Cells {
			if !g.Contains(cell) {
				continue
			}
			for _, side := range []rules.Color{rules.Red, rules.Blue} {
				idx := pattern.Index(b, rules.Cell(cell), d, side)
				states := pattern.Unpack(idx)
				own, opp := 0, false
				for _, s := range states {
					switch s {
					case config.PatternStateOwn:
						own++
					case config.PatternStateOpp:
						opp = true
					}
				}
				if own == 0 || opp {
					continue
				}
				sd := &red
				if side == rules.Blue {
					sd = &blue
				}
				sd.CleanWin++
			}
		}
	}
	seen := map[lineKey][2]int{}
	for d := range g.Dirs {
		for cell := range g.Cells {
			if !g.Contains(cell) {
				continue
			}
			k := lineKey{dir: d, line: lineIndex(g, cell, d)}
			s := seen[k]
			s[0]++
			if b.Red[cell/64]&(1<<uint(cell%64)) != 0 {
				s[1] |= 1
			}
			if b.Blue[cell/64]&(1<<uint(cell%64)) != 0 {
				s[1] |= 2
			}
			seen[k] = s
		}
	}
	for k, s := range seen {
		if s[0] == 0 {
			continue
		}
		for _, side := range []rules.Color{rules.Red, rules.Blue} {
			bit := 1
			if side == rules.Blue {
				bit = 2
			}
			if s[1]&bit == 0 || s[1]&(bit^3) != 0 {
				continue
			}
			ownOnLine := 0
			for cell := range g.Cells {
				if !g.Contains(cell) || lineIndex(g, cell, k.dir) != k.line {
					continue
				}
				if side == rules.Red {
					if b.Red[cell/64]&(1<<uint(cell%64)) != 0 {
						ownOnLine++
					}
				} else if b.Blue[cell/64]&(1<<uint(cell%64)) != 0 {
					ownOnLine++
				}
			}
			sd := &red
			if side == rules.Blue {
				sd = &blue
			}
			sd.CleanLn++
			sd.CleanSt += ownOnLine
			if ownOnLine >= 2 {
				sd.OpenBld++
			}
			if ownOnLine > sd.MaxSt {
				sd.MaxSt = ownOnLine
			}
		}
	}
	return blue, red
}

func probeSignal(label string, g *geom.Geometry, moves []string, candidates []string) {
	b := rules.NewBoardFor(g)
	for _, name := range moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("%s: cell %s: %v", label, name, err)
		}
		b.Make(rules.Cell(cell))
	}
	blue, red := computeSignals(g, b)
	sb, sr := staticEval(g, b)
	fmt.Printf("%s ply=%d static b=%d r=%d | sig b=%+v r=%+v\n", label, b.MoveCount, sb, sr, blue, red)
	for _, cand := range candidates {
		cell, err := g.ParseCell(cand)
		if err != nil || !b.IsLegal(rules.Cell(cell)) {
			fmt.Printf("  cand %s illegal\n", cand)
			continue
		}
		b.Make(rules.Cell(cell))
		cb, cr := computeSignals(g, b)
		sb2, sr2 := staticEval(g, b)
		fmt.Printf("  after %s: static b=%d r=%d net=%d | sig b=%+v r=%+v\n", cand, sb2, sr2, sb2-sr2, cb, cr)
		b.Unmake()
	}
}

func runSignalProbes(g *geom.Geometry) {
	probeSignal("g2-easy-tc1 M10 (lost: J9; twin won with L7)", g,
		[]string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10"},
		[]string{"J9", "L7", "K10", "K6", "J10"})
	probeSignal("g24-hard-tc1 M10 (lost: F9)", g,
		[]string{"H8", "G7", "E8", "G8", "G5", "G9", "G11", "G6", "G10"},
		[]string{"F9", "E10", "E9", "F8", "E11"})
	probeSignal("g2-easy-tc1 M16 red open three live (lost: H9)", g,
		[]string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10", "J9", "K9", "F9", "L8", "J10", "K7"},
		[]string{"H9", "K6", "K10", "J6"})
	probeSignal("M16 lost continuation board, blue to move (g2, lost)", g,
		[]string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10", "J9", "K9", "F9", "L8", "J10", "K7"}, nil)
	probeSignal("M16 held continuation board, blue to move (g8 twin, won)", g,
		[]string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10", "L7", "K7", "K6", "J6", "L8", "L5"}, nil)
}
