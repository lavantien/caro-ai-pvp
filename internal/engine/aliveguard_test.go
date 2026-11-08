package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestMoveGenerationTerminates(t *testing.T) {
	quiet := rules.NewBoard()
	place(t, quiet, rules.Red, "F4", "H12", "H13", "H14")
	place(t, quiet, rules.Blue, "B2")
	boards := []*rules.Board{redWinIn1Board(t), forcedBoard(t), quiet}
	for seed := range uint64(12) {
		boards = append(boards, playout(t, 613*seed+5, int(9+seed*11)))
	}
	e := New(0)
	done := make(chan struct{})
	panicked := make(chan any, 1)
	go func() {
		defer func() {
			close(done)
			if r := recover(); r != nil {
				panicked <- r
			}
		}()
		for _, b := range boards {
			e.eval.reset(b)
			near := e.nearMask(b)
			e.eval.liveWin1(b, rules.Red, near)
			e.eval.liveWin1(b, rules.Blue, near)
			e.generate(b, 0, moveNone)
		}
		e.Search(boards[0], cappedNodeBudget(2048))
		fresh := New(0)
		fresh.StopPonder()
		fresh.Close()
	}()
	select {
	case <-done:
	case r := <-panicked:
		t.Fatalf("move generation panicked on a guard board: %v", r)
	case <-time.After(scaledBudget(2 * time.Second)):
		t.Fatal("move generation did not terminate across the guard boards")
	}
	select {
	case r := <-panicked:
		t.Fatalf("move generation panicked on a guard board: %v", r)
	default:
	}
	for _, pin := range []struct {
		b    *rules.Board
		side rules.Color
		want bool
	}{{boards[0], rules.Red, true}, {boards[1], rules.Blue, true}, {boards[2], rules.Red, false}} {
		e.eval.reset(pin.b)
		if got := e.eval.liveWin1(pin.b, pin.side, e.nearMask(pin.b)); got != pin.want {
			t.Fatalf("liveWin1 on a guard board = %v, want %v", got, pin.want)
		}
	}
	for _, b := range boards {
		e.eval.reset(b)
		if n := e.generate(b, 0, moveNone); n < 1 {
			t.Fatal("move generation returned no candidates on a guard board")
		}
	}
	searchE := New(0)
	mv, stats := searchE.Search(boards[0], cappedNodeBudget(2048))
	if mv != rules.Move(mustCell(t, "H12")) || stats.Nodes == 0 {
		t.Fatalf("search on the win in 1 board returned %v with %d nodes, want H12 after a real search", mv, stats.Nodes)
	}
	smp := newSMP(1, 0)
	defer smp.Close()
	if mv, _ := smp.Search(boards[0], cappedNodeBudget(2048)); mv != rules.Move(mustCell(t, "H12")) {
		t.Fatalf("smp search on the win in 1 board returned %v, want the winning cell H12", mv)
	}
}
