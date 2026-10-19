package server

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func zeroClassWeights() *[config.PatternClassCount]int {
	return &[config.PatternClassCount]int{}
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
	if st.Score != config.EvalTempo {
		t.Fatalf("override score = %d, want the even-depth constant tempo %d", st.Score, config.EvalTempo)
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
	if st.Score != config.EvalTempo {
		t.Fatalf("smp override score = %d, want the constant tempo %d", st.Score, config.EvalTempo)
	}
	if st.Threads != 2 {
		t.Fatalf("smp override threads = %d, want 2", st.Threads)
	}
}
