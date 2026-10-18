package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestTTMoveMissesBeforeAnySearch(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	if mv := s.TTMove(midgameBoard(t)); mv != moveNone {
		t.Fatalf("TTMove on a never-searched root = %d, want moveNone", mv)
	}
}

func TestTTMoveDisabledTableAlwaysMisses(t *testing.T) {
	s := newSMP(1, 0)
	defer s.Close()
	if mv := s.TTMove(midgameBoard(t)); mv != moveNone {
		t.Fatalf("TTMove with the table disabled = %d, want moveNone", mv)
	}
}

func TestTTMoveDepthOneSearchStoresNoChildEntry(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	b := midgameBoard(t)
	mv, _ := s.SearchDepth(b, NewFixedBudget(scaledBudget(time.Minute)), 1)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("depth-1 search move %d illegal", mv)
	}
	child := *b
	child.Make(rules.Cell(mv))
	if reply := s.TTMove(&child); reply != moveNone {
		t.Fatalf("TTMove after a depth-1 search = %d, want moveNone: leaf probes store nothing", reply)
	}
}

func TestTTMoveReturnsLegalReplyAfterChildSearch(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	b := midgameBoard(t)
	mv, _ := s.SearchDepth(b, NewFixedBudget(scaledBudget(time.Minute)), 2)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("depth-2 search move %d illegal", mv)
	}
	child := *b
	child.Make(rules.Cell(mv))
	reply := s.TTMove(&child)
	if reply == moveNone {
		t.Fatal("TTMove on the searched child = moveNone, want the stored reply")
	}
	if !child.IsLegal(rules.Cell(reply)) {
		t.Fatalf("TTMove reply %d illegal on the child position", reply)
	}
}

func TestTTMoveProbesTheSharedTable(t *testing.T) {
	s := newSMP(2, testTTBytes)
	defer s.Close()
	b := midgameBoard(t)
	mv, _ := s.Search(b, NewFixedBudget(scaledBudget(60*time.Millisecond)))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("search move %d illegal", mv)
	}
	child := *b
	child.Make(rules.Cell(mv))
	if reply := s.TTMove(&child); reply != moveNone && !child.IsLegal(rules.Cell(reply)) {
		t.Fatalf("shared-table TTMove reply %d illegal on the child position", reply)
	}
}
