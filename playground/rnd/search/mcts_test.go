package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestMctsRestoreSweep(t *testing.T) {
	for _, n := range []int{500, 1000, 2000, 5000, 20000} {
		b := craftBoard(t, []int{119, 87, 71}, []int{73, 120})
		hash, side, moves := b.Hash, b.Side, b.MoveCount
		e := newMcts(b, mctsSeed)
		ok := true
		for i := range n {
			e.playout()
			if b.Hash != hash || b.Side != side || b.MoveCount != moves {
				t.Fatalf("n=%d broken at playout %d: hash %v side %v moves %d", n, i, b.Hash == hash, b.Side == side, b.MoveCount)
			}
			_ = ok
		}
	}
}

func TestMctsTreeGrows(t *testing.T) {
	b := craftBoard(t, []int{119, 87, 71}, []int{73, 120})
	e := newMcts(b, mctsSeed)
	for range 3000 {
		e.playout()
	}
	if len(e.root.children) < 10 {
		t.Fatalf("root children %d, tree barely expanded", len(e.root.children))
	}
	depth := 0
	for n := e.root; len(n.children) > 0; n = n.children[0] {
		depth++
	}
	if depth < 2 {
		t.Fatalf("tree depth %d, children never expand", depth)
	}
	var _ = rules.Red
}
