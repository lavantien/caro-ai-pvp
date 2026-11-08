package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func reproProfile(mv rules.Move, st SearchStats) (rules.Move, SearchStats) {
	st.ElapsedNs = 0
	st.Nps = 0
	return mv, st
}

func TestSearchReproducibleAcrossRuns(t *testing.T) {
	b := midgameBoard(t)
	const depth = 6
	dl := cappedNodeBudget(1_000_000)
	wantMv, want := reproProfile(New(testTTBytes).SearchDepth(b, dl, depth))
	if want.Depth != depth {
		t.Fatalf("completed depth %d, want the capped %d (budget starved?)", want.Depth, depth)
	}
	if want.Nodes == 0 || want.PVLen == 0 {
		t.Fatalf("reference run produced nothing: nodes %d pvLen %d", want.Nodes, want.PVLen)
	}
	for range 2 {
		e := New(testTTBytes)
		mv, st := reproProfile(e.SearchDepth(b, dl, depth))
		if mv != wantMv || st != want {
			t.Fatalf("rerun diverged: move %d stats %+v, want move %d stats %+v", mv, st, wantMv, want)
		}
	}
}

func TestSMPWidthOneReproducibleAcrossRuns(t *testing.T) {
	b := midgameBoard(t)
	const depth = 6
	var wantMv rules.Move
	var want SearchStats
	for run := range 3 {
		s := newSMP(1, testTTBytes)
		dl := cappedNodeBudget(1_000_000)
		mv, st := s.SearchDepth(b, dl, depth)
		s.Close()
		if st.Depth != depth {
			s2 := newSMP(1, testTTBytes)
			_, probe := s2.SearchDepth(b, cappedNodeBudget(1_000_000), depth)
			s2.Close()
			t.Fatalf("run %d completed depth %d, want the capped %d (starved; probe reached %d)",
				run, st.Depth, depth, probe.Depth)
		}
		mv, st = reproProfile(mv, st)
		if run == 0 {
			wantMv, want = mv, st
			continue
		}
		if mv != wantMv || st != want {
			t.Fatalf("run %d diverged: move %d stats %+v, want move %d stats %+v", run, mv, st, wantMv, want)
		}
	}
}
