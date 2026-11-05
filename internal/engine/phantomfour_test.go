package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var phantomFourMoves = [...]string{
	"H8", "H6", "H5", "I6", "J6", "I7", "J8", "J5", "H7", "K8", "I4", "G6",
	"I9", "K7", "J10", "K11", "J9", "J7", "K9", "L9", "M10", "H9", "L10", "K10",
	"M11", "N12", "F6", "G7", "M12", "M7", "L7", "M8", "M13", "M14",
}

func phantomFourBoard(t testing.TB, extra ...string) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, name := range phantomFourMoves {
		b.Make(mustCell(t, name))
	}
	for _, name := range extra {
		b.Make(mustCell(t, name))
	}
	return b
}

func TestPhantomFourLeavesDefenseUnrestricted(t *testing.T) {
	b := phantomFourBoard(t, "I11")
	if b.Side != rules.Blue {
		t.Fatalf("side to move = %d, want blue", b.Side)
	}
	e := New(testTTBytes)
	e.eval.reset(b)
	if e.eval.fours[rules.Red] == 0 {
		t.Fatalf("setup: red four windows = 0, want the dead M-file four counted")
	}
	if cells := winIn1Cells(t, b, rules.Red); len(cells) != 0 {
		t.Fatalf("setup: red real win-in-1 cells %v, want none", cells)
	}
	n := e.generate(b, 0, moveNone)
	n7 := rules.Move(mustCell(t, "N7"))
	for i := 0; i < n; i++ {
		if e.moves[0][i] == n7 {
			return
		}
	}
	t.Fatalf("blue candidate set of %d moves drops N7, the recorded killing reply", n)
}

func TestSearchFindsKillBehindPhantomFour(t *testing.T) {
	b := phantomFourBoard(t, "I11")
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(30*time.Second), 4)
	if stats.Score < config.EvalMateFloor {
		t.Fatalf("depth 6 score %d on move %d, want a mate score for blue's forced N7-J11-O6 kill", stats.Score, mv)
	}
}
