package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestTTOldGenMateBoundDistrusted(t *testing.T) {
	tb := newTT(1 << 10)
	tb.gen = 4
	mated := -(config.EvalMateMax - 4*config.EvalMateScoreStep)
	tb.store(501, mated, 9, 12, ttBoundUpper, 0)
	if _, _, cutoff := tb.probe(501, 10, mated, config.EvalMateMax, 0); !cutoff {
		t.Fatal("current generation mated bound must cut")
	}
	tb.bumpGen()
	if _, move, cutoff := tb.probe(501, 10, mated, config.EvalMateMax, 0); cutoff {
		t.Error("old generation mated bound must not cut")
	} else if move != 9 {
		t.Errorf("old generation move hint = %d, want the stored ordering move 9", move)
	}
	mating := config.EvalMateMax - 5*config.EvalMateScoreStep
	tb.store(502, mating, 3, 12, ttBoundLower, 0)
	if _, _, cutoff := tb.probe(502, 10, -config.EvalMateMax, mating, 0); !cutoff {
		t.Fatal("current generation mating bound must cut")
	}
	tb.bumpGen()
	if _, _, cutoff := tb.probe(502, 10, -config.EvalMateMax, mating, 0); cutoff {
		t.Error("old generation mating bound must not cut")
	}
	tb.gen = 7
	tb.store(503, 700, 5, 9, ttBoundExact, 0)
	tb.bumpGen()
	if score, _, cutoff := tb.probe(503, 8, -1000, 1000, 0); !cutoff || score != 700 {
		t.Errorf("old generation milliunit bound must keep cutting, got cutoff=%v score=%d", cutoff, score)
	}
}

func TestSearchStaleMateClaimIgnored(t *testing.T) {
	b := midgameBoard(t)
	gen := New(testTTBytes)
	gen.beginSearch(b)
	n := gen.generate(b, 0, moveNone)
	if n < 2 {
		t.Fatalf("midgame board yields %d candidates, want a choice", n)
	}
	poison := rules.Cell(gen.moves[0][0])
	e := New(testTTBytes)
	e.tt.bumpGen()
	cb := *b
	cb.Make(poison)
	e.tt.store(cb.Hash, -(config.EvalMateMax - 4*config.EvalMateScoreStep), rules.Move(poison), 12, ttBoundExact, 1)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 6)
	if stats.Score >= config.EvalMateFloor {
		t.Fatalf("warm stale mate claim survived the probe, score %d", stats.Score)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("honest move %d illegal", mv)
	}
}
