package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func craftBoard(t *testing.T, red []int, blue []int) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for i := range max(len(red), len(blue)) {
		if i < len(red) {
			b.Side = rules.Red
			b.Make(rules.Cell(red[i]))
		}
		if i < len(blue) {
			b.Side = rules.Blue
			b.Make(rules.Cell(blue[i]))
		}
	}
	return b
}

func TestCandTrackerParity(t *testing.T) {
	red := []int{119, 87, 71, 88, 104}
	blue := []int{73, 120, 122, 100, 63}
	b := craftBoard(t, red, blue)
	tr := &candTracker{}
	for i := range red {
		tr.make(rules.Cell(red[i]))
		tr.make(rules.Cell(blue[i]))
	}
	if !tr.matches(b) {
		t.Fatal("tracker set diverges from dilate after makes")
	}
	for i := len(red) - 1; i >= 0; i-- {
		tr.unmake(rules.Cell(blue[i]))
		tr.unmake(rules.Cell(red[i]))
	}
	if !tr.matches(rules.NewBoard()) {
		t.Fatal("tracker set not empty after full unmake")
	}
}

func TestSolverMirrorMatchesShipped(t *testing.T) {
	boards := []*rules.Board{
		craftBoard(t, []int{119, 121, 137, 153}, []int{120, 136, 105, 87}),
		craftBoard(t, []int{87, 88, 89, 90}, []int{120, 136, 152, 71}),
		craftBoard(t, []int{119, 135, 151, 100}, []int{104, 105, 106, 107}),
	}
	shipped := vcf.New(vcf.KindVCF)
	s := newSolver()
	for i, b := range boards {
		var out vcf.SolverStats
		want := shipped.Solve(b, solveBudget, nil, &out)
		wave := wavefront(b, b.Side)
		got, nodes, _ := s.solve(b, solveBudget, orderShipped, 0, wave)
		if got != want {
			t.Fatalf("board %d mirror found %v shipped %v", i, got, want)
		}
		if nodes != int(out.Nodes) {
			t.Fatalf("board %d mirror nodes %d shipped %d", i, nodes, out.Nodes)
		}
	}
}

func TestKeyRoundTrip(t *testing.T) {
	b := craftBoard(t, []int{119, 87, 71, 88}, []int{73, 120, 122, 100})
	k := keyOfBoard(b)
	b2, err := boardFromKey(k)
	if err != nil {
		t.Fatal(err)
	}
	if keyOfBoard(b2) != k {
		t.Fatal("board from key does not reproduce key")
	}
	if b2.Hash != b.Hash || b2.Side != b.Side {
		t.Fatal("reconstructed hash or side mismatch")
	}
}

func TestConvScreenSuperset(t *testing.T) {
	boards := []*rules.Board{
		craftBoard(t, []int{119, 121, 137, 153, 87}, []int{120, 136, 105, 87 + 16}),
		craftBoard(t, []int{87, 88, 89, 90, 91}, []int{120, 136, 152, 71}),
		craftBoard(t, []int{119, 135, 151, 100, 84}, []int{104, 105, 106, 107}),
	}
	for i, b := range boards {
		for _, mover := range []rules.Color{rules.Red, rules.Blue} {
			truth := tableFours(b, mover)
			screen := convFours(b, mover)
			for c := range truth {
				if truth[c] && !screen[c] {
					t.Fatalf("board %d mover %d cell %d truth four missed by screen", i, mover, c)
				}
			}
		}
	}
}

func TestMctsRestoreAndLegality(t *testing.T) {
	b := craftBoard(t, []int{119, 87, 71}, []int{73, 120})
	hash := b.Hash
	redCount, blueCount := b.Red, b.Blue
	e := newMcts(b, mctsSeed)
	for range 500 {
		e.playout()
	}
	if b.Hash != hash || b.MoveCount != 5 {
		t.Fatalf("board not restored after playouts: hash equal %v moves %d", b.Hash == hash, b.MoveCount)
	}
	if b.Red != redCount || b.Blue != blueCount {
		t.Fatal("stone sets changed after playouts")
	}
}
