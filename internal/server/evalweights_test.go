package server

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func zeroClassWeights() *[config.PatternClassCount]int {
	return &[config.PatternClassCount]int{}
}

func cleanWindowCount(b *rules.Board, side rules.Color) int {
	n := 0
	for d := range len(b.Geom.Dirs) {
		for cell := range b.Geom.Cells {
			var hasOwn, hasOpp bool
			for _, s := range pattern.Unpack(pattern.Index(b, rules.Cell(cell), d, side)) {
				switch s {
				case config.PatternStateOwn:
					hasOwn = true
				case config.PatternStateOpp:
					hasOpp = true
				}
			}
			if hasOwn && !hasOpp {
				n++
			}
		}
	}
	return n
}

func flattenedDepth2Expectation(t *testing.T, b *rules.Board, st engine.SearchStats) int {
	t.Helper()
	if st.PVLen != 2 {
		t.Fatalf("depth-2 probe returned pvlen %d, want the full 2-move line", st.PVLen)
	}
	leaf := *b
	leaf.Make(rules.Cell(st.PV[0]))
	leaf.Make(rules.Cell(st.PV[1]))
	delta := cleanWindowCount(&leaf, rules.Red) - cleanWindowCount(&leaf, rules.Blue)
	return config.EvalTempo + config.EvalCleanWindowWeight(b.Geom.Name)*delta
}

func searchDepthStats(t *testing.T, s searcher, b *rules.Board, depth int) engine.SearchStats {
	t.Helper()
	dl := engine.NewFixedBudget(10 * time.Second)
	switch inner := s.(type) {
	case singleSearcher:
		_, st := inner.e.SearchDepth(b, dl, depth)
		return st
	case tierSearcher:
		_, st := inner.smp.SearchDepth(b, dl, depth)
		return st
	default:
		t.Fatalf("searcher %T carries no depth probe", s)
		return engine.SearchStats{}
	}
}

func TestNewBotSearcherThreadsEvalWeightsSingleCore(t *testing.T) {
	b := quietBoard(t)
	over := newBotSearcher(config.Tier{Name: "weights-single", Cores: 1, TTBytes: 1 << 20, EvalWeights: zeroClassWeights()})
	st := searchDepthStats(t, over, b, 2)
	over.Close()
	if want := flattenedDepth2Expectation(t, b, st); st.Score != want {
		t.Fatalf("override score = %d, want the flattened clean-only value %d", st.Score, want)
	}

	easy := newBotSearcher(config.TierEasy)
	def := searchDepthStats(t, easy, b, 2)
	easy.Close()
	bare := engine.New(config.TierEasy.TTBytes)
	_, bareSt := bare.SearchDepth(b, engine.NewFixedBudget(10*time.Second), 2)
	if def.Score != bareSt.Score || def.Depth != bareSt.Depth {
		t.Fatalf("easy nil-path score %d depth %d diverged from the bare engine %d depth %d", def.Score, def.Depth, bareSt.Score, bareSt.Depth)
	}
	if def.Score == st.Score {
		t.Fatalf("default score %d equals the flattened override probe, the quiet board cannot pin the override", def.Score)
	}
}

func TestNewBotSearcherThreadsEvalWeightsSMP(t *testing.T) {
	b := quietBoard(t)
	s := newBotSearcher(config.Tier{Name: "weights-smp", Cores: 2, TTBytes: 1 << 20, EvalWeights: zeroClassWeights()})
	st := searchDepthStats(t, s, b, 2)
	s.Close()
	d := newBotSearcher(config.Tier{Name: "weights-smp-default", Cores: 2, TTBytes: 1 << 20})
	def := searchDepthStats(t, d, b, 2)
	d.Close()
	if st.Score == def.Score {
		t.Fatalf("smp flattened score %d equals the default %d, the override does not reach the workers", st.Score, def.Score)
	}
	if st.Threads != 2 {
		t.Fatalf("smp override threads = %d, want 2", st.Threads)
	}
}
