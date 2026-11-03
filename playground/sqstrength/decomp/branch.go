package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func staticEval(g *geom.Geometry, b *rules.Board) (blue, red int) {
	for d := range g.Dirs {
		for cell := range g.Cells {
			if !g.Contains(cell) {
				continue
			}
			blue += config.PatternClassWeights[pattern.Lookup(pattern.Index(b, rules.Cell(cell), d, rules.Blue)).Class]
			red += config.PatternClassWeights[pattern.Lookup(pattern.Index(b, rules.Cell(cell), d, rules.Red)).Class]
		}
	}
	return blue, red
}

func probeBranch(label string, g *geom.Geometry, moves []string, depth int, candidates []string) {
	b := rules.NewBoardFor(g)
	for _, name := range moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("%s: cell %s: %v", label, name, err)
		}
		b.Make(rules.Cell(cell))
	}
	blue, red := staticEval(g, b)
	fmt.Printf("%s side=%v ply=%d staticBlue=%d staticRed=%d net=%d\n", label, b.Side, b.MoveCount, blue, red, blue-red)
	e := engine.New(0)
	dl := engine.NewFixedBudget(30e9)
	mv, st := e.SearchDepth(b, dl, depth)
	fmt.Printf("  engine d=%d picks %s score=%d pv=%v\n", depth, g.AppendName(nil, int(mv)), st.Score, st.PV[:min(st.PVLen, 6)])
	for _, cand := range candidates {
		cell, err := g.ParseCell(cand)
		if err != nil || !b.IsLegal(rules.Cell(cell)) {
			fmt.Printf("  cand %s illegal\n", cand)
			continue
		}
		b.Make(rules.Cell(cell))
		cb, cr := staticEval(g, b)
		e2 := engine.New(0)
		_, st2 := e2.SearchDepth(b, engine.NewFixedBudget(30e9), depth-1)
		e2.Close()
		fmt.Printf("  cand %s childStatic blue=%d red=%d net=%d replyScoreForBlue=%d pv0=%s\n", cand, cb, cr, cb-cr, -st2.Score, cellNameOrNone(g, st2.PV, st2.PVLen))
		b.Unmake()
	}
	e.Close()
}

func cellNameOrNone(g *geom.Geometry, pv [config.SearchMaxPly]rules.Move, n int) string {
	if n < 1 || pv[0] == rules.MoveNone {
		return "none"
	}
	return string(g.AppendName(nil, int(pv[0])))
}

func runBranchProbes(g *geom.Geometry) {
	g2m10 := []string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10"}
	probeBranch("g2-easy-tc1 M10 blue to move (lost: played J9)", g, g2m10, 8, []string{"J9", "L7", "K10", "K6", "J10"})
	g2m16 := []string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10", "J9", "K9", "F9", "L8", "J10", "K7"}
	probeBranch("g2-easy-tc1 M16 blue to move, red open three K7-K9 (lost: played H9)", g, g2m16, 8, []string{"H9", "K6", "K10", "J6"})
	g8m10 := []string{"H8", "I9", "K8", "I8", "I11", "I7", "I5", "I6", "I10"}
	probeBranch("g8-easy-tc1 M10 same board (won: played L7)", g, g8m10, 8, []string{"J9", "L7"})
	g24m10 := []string{"H8", "G7", "E8", "G8", "G5", "G9", "G11", "G6", "G10"}
	probeBranch("g24-hard-tc1 M10 blue to move (lost: played F9)", g, g24m10, 8, []string{"F9", "E10", "E9", "F8", "E11"})
}
