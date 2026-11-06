package engine_test

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

var g36Moves = []string{
	"H8", "G9", "H11", "H10", "F8", "G8", "G7", "F6", "H6", "I5",
	"H7", "H5", "I7", "F7", "G5", "J8", "G6", "I10", "H9", "J10",
	"G10", "K7", "I9", "K10", "L10", "K9", "K8", "J9", "G4", "G3",
	"F4", "E3", "E9", "D10", "J6", "J12", "J11", "I11", "K13", "L8",
}

func g36Board(t *testing.T, plies int) *rules.Board {
	t.Helper()
	g := geom.Default()
	b := rules.NewBoardFor(g)
	for _, name := range g36Moves[:plies] {
		cell, err := g.ParseCell(name)
		if err != nil {
			t.Fatalf("cell %s: %v", name, err)
		}
		b.Make(rules.Cell(cell))
	}
	return b
}

type seatSearcher interface {
	Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string)
	StartPonder(b *rules.Board)
	StopPonder() (rules.Move, engine.SearchStats, string)
	Close()
}

func TestHardSeatSearchReturnsUnderNodeBudgetOnRecordedM41Board(t *testing.T) {
	b := g36Board(t, len(g36Moves))
	if b.Side != rules.Red {
		t.Fatalf("side to move = %v, want Red", b.Side)
	}
	if b.MoveCount != 40 {
		t.Fatalf("move count = %d, want 40", b.MoveCount)
	}
	s := server.NewBotSearcherFor("square", config.TierHard)
	defer s.Close()
	mv, st, _ := s.Search(b, engine.NewNodeBudget(4_000_000))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("returned move %v illegal", mv)
	}
	if st.Nodes == 0 {
		t.Fatalf("search returned zero nodes")
	}
}

func TestPonderCadenceKeepsSeatSearchBoundedOnRecordedBoard(t *testing.T) {
	g := geom.Default()
	var master, hard seatSearcher
	master = server.NewBotSearcherFor("square", config.TierMaster)
	hard = server.NewBotSearcherFor("square", config.TierHard)
	defer master.Close()
	defer hard.Close()
	armed := map[rules.Color]seatSearcher{}
	b := rules.NewBoardFor(g)
	for i, name := range g36Moves {
		side := b.Side
		seat := master
		if side == rules.Red {
			seat = hard
		}
		if lane := armed[side]; lane != nil {
			lane.StopPonder()
			armed[side] = nil
		}
		seat.Search(b, engine.NewFixedBudget(20*time.Millisecond))
		cell, err := g.ParseCell(name)
		if err != nil {
			t.Fatalf("cell %s: %v", name, err)
		}
		b.Make(rules.Cell(cell))
		if i+1 == len(g36Moves) {
			break
		}
		next, err := g.ParseCell(g36Moves[i+1])
		if err != nil {
			t.Fatalf("next cell %s: %v", g36Moves[i+1], err)
		}
		pb := *b
		if !pb.IsLegal(rules.Cell(next)) {
			continue
		}
		pb.Make(rules.Cell(next))
		seat.StartPonder(&pb)
		armed[side] = seat
	}
	if lane := armed[rules.Red]; lane != nil {
		lane.StopPonder()
		armed[rules.Red] = nil
	}
	mv, st, _ := hard.Search(b, engine.NewNodeBudget(3_000_000))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("returned move %v illegal", mv)
	}
	if st.Nodes == 0 {
		t.Fatalf("search returned zero nodes")
	}
}
