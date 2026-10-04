package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// mate1: red E9..H9 with the left end closed by D9, I9 open. I9 completes the
// exact five E9..I9 whose only blocked end is D9.
func mate1Board(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H9")
	place(t, b, rules.Blue, append([]string{"D9"}, scatterBlue[:]...)...)
	b.Side = rules.Red
	return b
}

// mate2: an open three F9..H9. Red extends to an open four, blue cannot hold
// both ends: forced in 3 plies.
func mate2Board(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "F9", "G9", "H9")
	place(t, b, rules.Blue, scatterBlue[:]...)
	b.Side = rules.Red
	return b
}

// mate3: the four plus open three cross. H9 makes the simple four E9..H9
// (win cell I9) and the open column three H7..H9 at once. Blue must spend
// its move on I9, then H6 or H10 turns the column into an open four:
// forced in 5 plies.
func mate3Board(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H7", "H8")
	place(t, b, rules.Blue, append([]string{"D9"}, scatterBlue[:]...)...)
	b.Side = rules.Red
	return b
}

// overlineTrap: F9..I9 plus K9. J9 completes six in a row and wins nothing,
// E9 is the only win-in-1. The engine must decline the trap.
func overlineTrapBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "F9", "G9", "H9", "I9", "K9")
	place(t, b, rules.Blue, scatterBlue[:]...)
	b.Side = rules.Red
	return b
}

// deadLine: E9..H9 plus J9 with D9 closed. Every extension of the row
// overlines or hits the blue block, no win exists on this line.
func deadLineBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H9", "J9")
	place(t, b, rules.Blue, append([]string{"D9"}, scatterBlue[:3]...)...)
	b.Side = rules.Red
	return b
}

func TestSearchFindsMateInOne(t *testing.T) {
	b := mate1Board(t)
	if wins := winIn1Cells(t, b, rules.Red); len(wins) != 1 || wins[0] != mustCell(t, "I9") {
		t.Fatalf("setup: win-in-1 cells %v, want exactly I9", wins)
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 4)
	if mv != rules.Move(mustCell(t, "I9")) {
		t.Fatalf("mate in 1 move %d, want I9", mv)
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("mate in 1 score = %d, want %d", stats.Score, want)
	}
	if stats.PVLen != 1 || stats.PV[0] != mv {
		t.Errorf("mate in 1 pv len %d pv0 %d, want 1 and the move", stats.PVLen, stats.PV[0])
	}
}

func TestSearchDeclinesOverlineTrap(t *testing.T) {
	b := overlineTrapBoard(t)
	wins := winIn1Cells(t, b, rules.Red)
	if len(wins) != 1 || wins[0] != mustCell(t, "E9") {
		t.Fatalf("setup: win-in-1 cells %v, want exactly E9", wins)
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 4)
	if mv != rules.Move(mustCell(t, "E9")) {
		t.Fatalf("trap move %d returned, want E9: J9 overlines into a dead six", mv)
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("score = %d, want mate in 1 %d", stats.Score, want)
	}
}

func TestSearchFindsMateInTwo(t *testing.T) {
	b := mate2Board(t)
	if wins := winIn1Cells(t, b, rules.Red); len(wins) != 0 {
		t.Fatalf("setup: unexpected win-in-1 cells %v", wins)
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 6)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("mate in 2 move %d illegal", mv)
	}
	if want := config.EvalMateMax - 3*config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("mate in 2 score = %d, want %d", stats.Score, want)
	}
	if mv != rules.Move(mustCell(t, "E9")) && mv != rules.Move(mustCell(t, "I9")) {
		t.Errorf("mate in 2 move %d, want an end of the open three", mv)
	}
}

func TestSearchFindsMateInThree(t *testing.T) {
	b := mate3Board(t)
	if wins := winIn1Cells(t, b, rules.Red); len(wins) != 0 {
		t.Fatalf("setup: unexpected win-in-1 cells %v", wins)
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(2*time.Second), 8)
	if mv != rules.Move(mustCell(t, "H9")) {
		t.Fatalf("mate in 3 move %d, want the cross point H9", mv)
	}
	if want := config.EvalMateMax - 5*config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("mate in 3 score = %d, want %d", stats.Score, want)
	}
}

func TestSearchNoPhantomMateOnDeadLine(t *testing.T) {
	b := deadLineBoard(t)
	if wins := winIn1Cells(t, b, rules.Red); len(wins) != 0 {
		t.Fatalf("setup: dead row still has win-in-1 cells %v", wins)
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(300*time.Millisecond), 5)
	if stats.Score >= config.EvalMateMax-config.SearchMaxPly*config.EvalMateScoreStep {
		t.Errorf("dead line scored as mate: %d", stats.Score)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("dead line move %d illegal", mv)
	}
}

func TestMateDistanceMonotonicity(t *testing.T) {
	e := New(testTTBytes)
	_, s1 := e.SearchDepth(mate1Board(t), NewFixedBudget(time.Second), 4)
	_, s2 := e.SearchDepth(mate2Board(t), NewFixedBudget(time.Second), 6)
	_, s3 := e.SearchDepth(mate3Board(t), NewFixedBudget(2*time.Second), 8)
	if s1.Score <= s2.Score || s2.Score <= s3.Score {
		t.Errorf("mate scores not monotone in distance: %d %d %d", s1.Score, s2.Score, s3.Score)
	}
}

func TestPVStaysLegal(t *testing.T) {
	for seed := range uint64(12) {
		b := playout(t, 300*seed+1, int(12+seed*4))
		e := New(testTTBytes)
		mv, stats := e.Search(b, NewFixedBudget(60*time.Millisecond))
		if stats.PVLen == 0 {
			t.Fatalf("seed %d: empty pv", seed)
		}
		if stats.PV[0] != mv {
			t.Fatalf("seed %d: pv head %d differs from best move %d", seed, stats.PV[0], mv)
		}
		for i := range stats.PVLen {
			cell := rules.Cell(stats.PV[i])
			if !b.IsLegal(cell) {
				t.Fatalf("seed %d: pv move %d at index %d illegal", seed, cell, i)
			}
			b.Make(cell)
		}
		for range stats.PVLen {
			b.Unmake()
		}
		if stats.Score < -config.EvalMateMax || stats.Score > config.EvalMateMax {
			t.Fatalf("seed %d: score %d outside the mate band", seed, stats.Score)
		}
	}
}

func TestIterativeDeepeningProgression(t *testing.T) {
	b := midgameBoard(t)
	var prevNodes uint64
	for depth := 1; depth <= 5; depth++ {
		e := New(testTTBytes)
		_, stats := e.SearchDepth(b, NewFixedBudget(time.Second), depth)
		if stats.Depth != depth {
			t.Errorf("depth %d: completed depth = %d", depth, stats.Depth)
		}
		if stats.Nodes <= prevNodes {
			t.Errorf("depth %d: nodes %d not above previous %d", depth, stats.Nodes, prevNodes)
		}
		prevNodes = stats.Nodes
		if stats.EBFMilli < 1000 {
			t.Errorf("depth %d: ebf %d below 1.0", depth, stats.EBFMilli)
		}
	}
}

// TestOrderingPrunesMoreAtEqualDepth pins ordering's value the
// deterministic way: two fresh single-threaded engines search the same
// midgame board to the same completed depth, one with move ordering off,
// and the ordered one must visit strictly fewer nodes (fewer nodes per
// depth is what reaches deeper under any fixed budget). Node counts at a
// completed depth over a fresh table are a pure function of the position,
// so no wall-clock margin can flake the comparison; the generous budget is
// only the runner-starvation backstop, the depth cap ends both searches.
func TestOrderingPrunesMoreAtEqualDepth(t *testing.T) {
	b := midgameBoard(t)
	const depth = 5
	ordered := New(testTTBytes)
	_, statsOrdered := ordered.SearchDepth(b, NewFixedBudget(10*time.Second), depth)
	bare := New(testTTBytes)
	bare.noOrder = true
	_, statsBare := bare.SearchDepth(b, NewFixedBudget(10*time.Second), depth)
	if statsOrdered.Depth != depth || statsBare.Depth != depth {
		t.Fatalf("completed depths = %d and %d, want both %d (budget starved?)",
			statsOrdered.Depth, statsBare.Depth, depth)
	}
	if statsOrdered.Nodes >= statsBare.Nodes {
		t.Errorf("ordering nodes %d not below unordered %d at depth %d",
			statsOrdered.Nodes, statsBare.Nodes, depth)
	}
	if statsOrdered.FirstMoveFailHighPermille <= 0 {
		t.Errorf("ordered fh1 = %d, want positive", statsOrdered.FirstMoveFailHighPermille)
	}
}

func TestStatsTTCountersPopulated(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	_, stats := e.Search(b, NewFixedBudget(200*time.Millisecond))
	if stats.TTHitPermille < 0 || stats.TTHitPermille > 1000 {
		t.Errorf("tt hit permille %d out of range", stats.TTHitPermille)
	}
	if stats.HashFullPermille < 0 || stats.HashFullPermille > 1000 {
		t.Errorf("hash full permille %d out of range", stats.HashFullPermille)
	}
	if stats.HashFullPermille == 0 {
		t.Error("hash full permille 0 after a 200ms search with a small table")
	}
	if stats.TTHitPermille == 0 {
		t.Error("tt hit permille 0 with transpositions enabled")
	}
}

// TestRootFillingMoveDraws covers the root draw path: a checkerboarded 8x8
// region with one corner empty. The only move fills the board without a win,
// so the root scores the draw and still answers legally.
func TestRootFillingMoveDraws(t *testing.T) {
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			if r == 0 && c == 0 {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	if b.IsFull() {
		t.Fatal("setup filled the board")
	}
	e := New(0)
	mv, stats := e.Search(b, NewFixedBudget(10*time.Millisecond))
	if mv != rules.Move(0) {
		t.Fatalf("move %d, want the last empty cell A1", mv)
	}
	if stats.Score != 0 {
		t.Errorf("filling draw score = %d, want 0", stats.Score)
	}
	b.Make(rules.Cell(mv))
	if !b.IsFull() {
		t.Fatal("move did not fill the board")
	}
}

// TestInteriorFillingMoveDraws drives negamax itself into the last empty
// cell: two holes on a checkerboarded region, depth 2, so the interior node
// takes its own draw branch.
func TestInteriorFillingMoveDraws(t *testing.T) {
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			if r == 0 && c == 0 || r == config.CrossCheckSize-1 && c == config.CrossCheckSize-1 {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	e := New(0)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 2)
	if stats.Depth != 2 {
		t.Fatalf("depth %d, want the capped 2", stats.Depth)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("move %d illegal on the two hole board", mv)
	}
}
