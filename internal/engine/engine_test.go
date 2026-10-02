package engine

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const testTTBytes = 1 << 22

func mustCell(t testing.TB, name string) rules.Cell {
	t.Helper()
	cell, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("cell %q: %v", name, err)
	}
	return cell
}

// place sets stones of one color by name, bypassing turn order the way the
// pattern package's generator does.
func place(t testing.TB, b *rules.Board, color rules.Color, names ...string) {
	t.Helper()
	for _, name := range names {
		b.Side = color
		b.Make(mustCell(t, name))
	}
}

var scatterBlue = [...]string{"A1", "A3", "C1", "P16"}

func winIn1Cells(t testing.TB, b *rules.Board, color rules.Color) []rules.Cell {
	t.Helper()
	side := b.Side
	b.Side = color
	defer func() { b.Side = side }()
	var out []rules.Cell
	var buf [config.BoardCells]rules.Move
	legal := b.LegalMoves(buf[:])
	for _, m := range buf[:legal] {
		cell := rules.Cell(m)
		b.Make(cell)
		won := b.FastLastMoveWin(color, cell)
		b.Unmake()
		if won {
			out = append(out, cell)
		}
	}
	return out
}

// playout plays count random legal moves, skipping the opening constraint
// once passed, and returns the board ready for further play.
func playout(t testing.TB, seed uint64, count int) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	var buf [config.BoardCells]rules.Move
	for range count {
		n := b.LegalMoves(buf[:])
		if n == 0 {
			break
		}
		b.Make(rules.Cell(buf[rng.IntN(n)]))
	}
	return b
}

func midgameBoard(t testing.TB) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, name := range [...]string{"H8", "H9", "I8", "I9", "C3", "C4", "D3", "M12", "M13", "N12"} {
		b.Make(mustCell(t, name))
	}
	return b
}

func TestSearchReturnsLegalMove(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	budget := scaledBudget(50 * time.Millisecond)
	mv, stats := e.Search(b, NewFixedBudget(budget))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("search returned illegal move %d", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2 in %v", stats.Depth, budget)
	}
	if stats.Nodes == 0 || stats.Nps == 0 {
		t.Errorf("nodes %d nps %d, both must be positive", stats.Nodes, stats.Nps)
	}
	if stats.Threads != 1 {
		t.Errorf("threads = %d, want 1 for the single threaded core", stats.Threads)
	}
	if stats.PVLen == 0 || stats.PV[0] != mv {
		t.Errorf("pv must start with the best move, got pv0 %d move %d len %d", stats.PV[0], mv, stats.PVLen)
	}
	if stats.AllocNs != int64(budget) {
		t.Errorf("alloc budget = %d, want %d", stats.AllocNs, budget)
	}
}

func TestSearchEmptyBoardPlaysCenter(t *testing.T) {
	e := New(testTTBytes)
	mv, stats := e.Search(rules.NewBoard(), NewFixedBudget(scaledBudget(20*time.Millisecond)))
	if mv != rules.Move(config.SearchEmptyBoardCell) {
		t.Errorf("empty board move = %d, want center %d", mv, config.SearchEmptyBoardCell)
	}
	if !rules.NewBoard().IsLegal(rules.Cell(mv)) {
		t.Errorf("center cell %d must be legal", mv)
	}
	if stats.Depth == 0 {
		t.Errorf("depth 0 on the empty board, want the shallow iteration done")
	}
}

func TestSearchFullBoardReturnsNoMove(t *testing.T) {
	b := rules.NewCrossCheck()
	var buf [config.BoardCells]rules.Move
	for {
		n := b.LegalMoves(buf[:])
		if n == 0 {
			break
		}
		b.Make(rules.Cell(buf[0]))
	}
	if !b.IsFull() {
		t.Fatal("setup failed to fill the board")
	}
	e := New(testTTBytes)
	mv, stats := e.Search(b, NewFixedBudget(time.Millisecond))
	if mv != moveNone {
		t.Errorf("full board move = %d, want moveNone", mv)
	}
	if stats.Depth != 0 {
		t.Errorf("full board depth = %d, want 0", stats.Depth)
	}
}

func TestSearchTinyBudgetStillLegal(t *testing.T) {
	for _, budget := range []time.Duration{0, 1, 50} {
		b := midgameBoard(t)
		e := New(0)
		mv, _ := e.Search(b, NewFixedBudget(budget*time.Microsecond))
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("budget %d: illegal move %d", budget, mv)
		}
	}
}

func TestSearchStoppedDeadlineReturnsFallback(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := NewFixedBudget(time.Second)
	dl.Stop()
	mv, _ := e.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("stopped deadline returned illegal move %d", mv)
	}
}

func TestSearchGenerationWraps(t *testing.T) {
	e := New(0)
	b := midgameBoard(t)
	for range 64 {
		mv, _ := e.Search(b, NewFixedBudget(200*time.Microsecond))
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("generation wrap search returned illegal move %d", mv)
		}
	}
	if e.tt.gen == 0 || e.tt.gen > 63 {
		t.Errorf("generation = %d, want in [1, 63] after wrapping", e.tt.gen)
	}
}

func TestSearchOpeningRuleSecondMove(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	place(t, b, rules.Blue, "I9")
	e := New(testTTBytes)
	mv, _ := e.Search(b, NewFixedBudget(30*time.Millisecond))
	cell := rules.Cell(mv)
	if !b.IsLegal(cell) {
		t.Fatalf("move %d violates the opening rule", mv)
	}
	anchor := mustCell(t, "H8")
	dr := int(cell/config.BoardStride) - int(anchor/config.BoardStride)
	dc := int(cell%config.BoardStride) - int(anchor%config.BoardStride)
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}
	if dr < config.OpeningChebyshevMin && dc < config.OpeningChebyshevMin {
		t.Errorf("move %v at Chebyshev %d from the anchor", cell, max(dr, dc))
	}
}
