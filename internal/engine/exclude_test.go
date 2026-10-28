package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestSearchExcludedSkipsTheCell(t *testing.T) {
	b := mate1Board(t)
	e := New(testTTBytes)
	mv, _ := e.SearchDepth(b, NewFixedBudget(time.Second), 4)
	if mv != rules.Move(mustCell(t, "I9")) {
		t.Fatalf("setup: unexcluded move %d, want the mate I9", mv)
	}
	mv2, stats := e.SearchExcluded(b, NewFixedBudget(time.Second), mv)
	if mv2 == mv {
		t.Fatalf("excluded move %d returned again", mv2)
	}
	if !b.IsLegal(rules.Cell(mv2)) {
		t.Fatalf("excluded search move %d illegal", mv2)
	}
	if stats.Score >= config.EvalMateFloor {
		t.Fatalf("excluded search still claims mate, score %d", stats.Score)
	}
}

func TestSMPSearchExcludedSkipsTheCell(t *testing.T) {
	b := mate1Board(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	mv, _ := s.SearchDepth(b, NewFixedBudget(2*time.Second), 4)
	if mv != rules.Move(mustCell(t, "I9")) {
		t.Fatalf("setup: unexcluded move %d, want the mate I9", mv)
	}
	mv2, stats := s.SearchExcluded(b, NewFixedBudget(2*time.Second), mv)
	if mv2 == mv {
		t.Fatalf("excluded move %d returned again", mv2)
	}
	if !b.IsLegal(rules.Cell(mv2)) {
		t.Fatalf("excluded search move %d illegal", mv2)
	}
	if stats.Score >= config.EvalMateFloor {
		t.Fatalf("excluded smp search still claims mate, score %d", stats.Score)
	}
}
