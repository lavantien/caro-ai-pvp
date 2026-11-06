package server

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestSingleSearcherSearchExcludedAvoidsTheExcludedMove(t *testing.T) {
	g, err := geom.ByName(config.DefaultVariant)
	if err != nil {
		t.Fatal(err)
	}
	b := rules.NewBoardFor(g)
	b.Make(rules.Cell(g.Center))
	s := singleSearcher{e: engine.NewEval(0, config.TierEasy.EvalWeights)}
	t.Cleanup(s.Close)
	exclude := rules.Move(0)
	mv, _ := s.SearchExcluded(b, engine.NewNodeBudget(4000), exclude)
	if mv == exclude {
		t.Fatalf("SearchExcluded returned the excluded move %d", mv)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("SearchExcluded returned illegal move %d", mv)
	}
}

func TestTierSearcherSearchExcludedAvoidsTheExcludedMove(t *testing.T) {
	g, err := geom.ByName(config.DefaultVariant)
	if err != nil {
		t.Fatal(err)
	}
	b := rules.NewBoardFor(g)
	b.Make(rules.Cell(g.Center))
	s := tierSearcher{smp: engine.NewTieredEval(config.TierMedium, config.TierMedium.EvalWeights)}
	t.Cleanup(s.Close)
	exclude := rules.Move(0)
	mv, _ := s.SearchExcluded(b, engine.NewNodeBudget(4000), exclude)
	if mv == exclude {
		t.Fatalf("SearchExcluded returned the excluded move %d", mv)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("SearchExcluded returned illegal move %d", mv)
	}
}

func TestBotDisplayNameEmptyNameStaysEmpty(t *testing.T) {
	if got := botDisplayName("", "3"); got != "" {
		t.Fatalf("botDisplayName(\"\", 3) = %q, want empty", got)
	}
	if got := botDisplayName("gm", "3"); got != "gm-3" {
		t.Fatalf("botDisplayName(gm, 3) = %q, want gm-3", got)
	}
}
