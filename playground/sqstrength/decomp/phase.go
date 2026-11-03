package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func searchPick(g *geom.Geometry, moves []string, depth int) (string, int) {
	b := rules.NewBoardFor(g)
	for _, name := range moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("cell %s: %v", name, err)
		}
		b.Make(rules.Cell(cell))
	}
	e := engine.New(0)
	mv, st := e.SearchDepth(b, engine.NewFixedBudget(120e9), depth)
	e.Close()
	return string(g.AppendName(nil, int(mv))), st.Score
}

func runPhaseProbes(g *geom.Geometry) {
	cases := []struct {
		label string
		moves []string
	}{
		{"m2 after H8", []string{"H8"}},
		{"m4 after H8 H6 H5", []string{"H8", "H6", "H5"}},
		{"m6 board H8 H6 H5 I6 K6", []string{"H8", "H6", "H5", "I6", "K6"}},
		{"m6 board H8 H6 H5 G6 E6", []string{"H8", "H6", "H5", "G6", "E6"}},
		{"m6 board H8 G7 E8 G8 G5", []string{"H8", "G7", "E8", "G8", "G5"}},
		{"m6 board H8 I9 K8 I8 I11", []string{"H8", "I9", "K8", "I8", "I11"}},
		{"m6 board H8 F8 E8 F7 F5", []string{"H8", "F8", "E8", "F7", "F5"}},
		{"m8 board H8 H6 H5 I6 K6 G6 F6", []string{"H8", "H6", "H5", "I6", "K6", "G6", "F6"}},
	}
	for _, c := range cases {
		for _, depth := range []int{6, 8, 10} {
			mv, score := searchPick(g, c.moves, depth)
			fmt.Printf("%s d=%d picks %s score=%d\n", c.label, depth, mv, score)
		}
	}
}
