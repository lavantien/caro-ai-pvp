package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

type scriptedSearcher struct {
	moves []rules.Move
	next  int
}

func (s *scriptedSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	mv := s.moves[len(s.moves)-1]
	if s.next < len(s.moves) {
		mv = s.moves[s.next]
	}
	s.next++
	return mv, engine.SearchStats{}, ""
}

func (s *scriptedSearcher) StartPonder(b *rules.Board) {}

func (s *scriptedSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (s *scriptedSearcher) Close() {}

func TestIllegalFallbackRecordsNoVote(t *testing.T) {
	red := &scriptedSearcher{moves: []rules.Move{130, 131, 132, 133, 134}}
	blue := &scriptedSearcher{moves: []rules.Move{119, 140, 141, 142, 143}}
	var mlines bytes.Buffer
	res := playGame([2]server.Searcher{red, blue}, 0, 0, 0, &mlines)
	if res.illegal != 1 {
		t.Fatalf("illegal count = %d over %d moves, want exactly the one occupied blue reply", res.illegal, res.moves)
	}
	if res.winner != "red" {
		t.Fatalf("winner = %q, want red closing the scripted five", res.winner)
	}
	opened := rules.NewBoard()
	opened.Make(rules.Cell(119))
	openKey := keyOf(opened)
	for _, rec := range res.records {
		if rec.key == openKey {
			t.Fatalf("the illegal-fallback position still recorded a vote for move %d", rec.move)
		}
	}
	if len(res.records) == 0 || res.records[0].key != corpusZeroKey || res.records[0].move != 119 {
		t.Fatalf("first record = %+v, want the forced opening on the empty board", res.records[0])
	}
	if !strings.Contains(mlines.String(), "illegal 119 from seat 1") {
		t.Errorf("m-lines miss the illegal line: %q", mlines.String())
	}
}

func TestOpeningFallbackLegalityIsPinned(t *testing.T) {
	b := rules.NewBoard()
	for _, c := range config.BookOpeningCells {
		if !b.IsLegal(rules.Cell(c)) {
			t.Fatalf("rotation cell %d is illegal on the empty board", c)
		}
	}
}
