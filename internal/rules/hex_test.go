package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

func mustHexGeometry() *geom.Geometry {
	g, err := geom.ByName("hex")
	if err != nil {
		panic(err)
	}
	return g
}

var hexGeometry = mustHexGeometry()

func hexCellsOf(t testing.TB, names ...string) []Cell {
	t.Helper()
	out := make([]Cell, len(names))
	for i, name := range names {
		c, err := hexGeometry.ParseCell(name)
		if err != nil {
			t.Fatalf("hex ParseCell(%q): %v", name, err)
		}
		out[i] = Cell(c)
	}
	return out
}

func hexOne(t testing.TB, name string) Cell {
	t.Helper()
	return hexCellsOf(t, name)[0]
}

func hexBoard(t testing.TB, red, blue []string) (*Board, *NaiveBoard) {
	t.Helper()
	b, nb := NewBoardFor(hexGeometry), NewNaiveBoardFor(hexGeometry)
	for _, c := range hexCellsOf(t, red...) {
		setStone(b, c, Red)
		nb.Set(c, Red)
	}
	for _, c := range hexCellsOf(t, blue...) {
		setStone(b, c, Blue)
		nb.Set(c, Blue)
	}
	return b, nb
}

func TestHexWinsFixtures(t *testing.T) {
	cases := []struct {
		name    string
		red     []string
		blue    []string
		redWin  bool
		blueWin bool
	}{
		{"empty", nil, nil, false, false},
		{"exact five row open", []string{"I12", "J12", "K12", "L12", "M12"}, nil, true, false},
		{"exact five row one end blocked", []string{"G12", "H12", "I12", "J12", "K12"}, []string{"F12"}, true, false},
		{"exact five row both ends blocked dead", []string{"H12", "I12", "J12", "K12", "L12"}, []string{"G12", "M12"}, false, false},
		{"rim five other end open", []string{"A12", "B12", "C12", "D12", "E12"}, nil, true, false},
		{"rim five wall side plus blocked side win", []string{"A12", "B12", "C12", "D12", "E12"}, []string{"F12"}, true, false},
		{"rim overline six never wins", []string{"A12", "B12", "C12", "D12", "E12", "F12"}, nil, false, false},
		{"column five", []string{"L8", "L9", "L10", "L11", "L12"}, nil, true, false},
		{"rim column five", []string{"L1", "L2", "L3", "L4", "L5"}, nil, true, false},
		{"slope five", []string{"N8", "M9", "L10", "K11", "J12"}, nil, true, false},
		{"rim slope five", []string{"W1", "V2", "U3", "T4", "S5"}, nil, true, false},
		{"gap no win", []string{"I12", "J12", "K12", "M12", "N12"}, nil, false, false},
		{"four no win", []string{"I12", "J12", "K12", "L12"}, nil, false, false},
		{"blue wins red does not", []string{"H8", "H9"}, []string{"N8", "M9", "L10", "K11", "J12"}, false, true},
	}
	for _, tc := range cases {
		b, nb := hexBoard(t, tc.red, tc.blue)
		if got := nb.Wins(Red); got != tc.redWin {
			t.Fatalf("%s: naive red Wins = %v", tc.name, got)
		}
		if got := b.Wins(Red); got != tc.redWin {
			t.Fatalf("%s: board red Wins = %v", tc.name, got)
		}
		if got := nb.Wins(Blue); got != tc.blueWin {
			t.Fatalf("%s: naive blue Wins = %v", tc.name, got)
		}
		if got := b.Wins(Blue); got != tc.blueWin {
			t.Fatalf("%s: board blue Wins = %v", tc.name, got)
		}
	}
}

func TestHexBoardGoldens(t *testing.T) {
	b := NewBoardFor(hexGeometry)
	if got := popcountBB(b.Region); got != 397 {
		t.Fatalf("hex region popcount = %d, want 397", got)
	}
	if !b.inRegion(517) {
		t.Fatal("hex max in-region cell 517 must be in region")
	}
	if hexGeometry.Contains(518) || b.IsLegal(Cell(518)) {
		t.Fatal("cell 518 at the capacity bound must be out of range")
	}
	buf := make([]Move, config.MaxBoardCells)
	if n := b.LegalMoves(buf); n != 397 {
		t.Fatalf("empty hex legal moves = %d, want 397", n)
	}
	for cell := range hexGeometry.Cells {
		if hexGeometry.Contains(cell) {
			b.Make(Cell(cell))
		}
	}
	if !b.IsFull() {
		t.Fatal("filled hex region must report full")
	}
	if b.MoveCount != 397 {
		t.Fatalf("hex fill move count = %d, want 397", b.MoveCount)
	}
}

func TestHexOpeningRule(t *testing.T) {
	anchor := hexOne(t, "L12")
	b, nb := NewBoardFor(hexGeometry), NewNaiveBoardFor(hexGeometry)
	b.Make(anchor)
	nb.Set(anchor, Red)
	blue := hexOne(t, "A12")
	b.Make(blue)
	nb.Set(blue, Blue)
	for _, tc := range []struct {
		name     string
		hexLegal bool
	}{
		{"M13", false},
		{"M12", false},
		{"N12", false},
		{"K13", false},
		{"J14", false},
		{"M14", true},
		{"N13", true},
		{"L15", true},
		{"O12", true},
		{"M15", true},
		{"I15", true},
		{"L1", true},
	} {
		c := hexOne(t, tc.name)
		if got := b.IsLegal(c); got != tc.hexLegal {
			t.Fatalf("hex IsLegal(%s) = %v, want %v", tc.name, got, tc.hexLegal)
		}
		if got := nb.IsLegal(Red, c); got != tc.hexLegal {
			t.Fatalf("naive hex IsLegal(%s) = %v, want %v", tc.name, got, tc.hexLegal)
		}
	}
	for q := range hexGeometry.Rows {
		for r := range hexGeometry.Cols {
			cell := q*hexGeometry.Stride + r
			if !hexGeometry.Contains(cell) || q >= config.BoardSize || r >= config.BoardSize {
				continue
			}
			cheb := max(absInt(q-11), absInt(r-11))
			if cheb >= config.OpeningChebyshevMin && hexGeometry.Distance(int(anchor), cell) < config.OpeningChebyshevMin {
				t.Fatalf("square legal cell (%d,%d) cheb %d must stay hex legal, hex distance %d", q, r, cheb, hexGeometry.Distance(int(anchor), cell))
			}
		}
	}
	second := hexOne(t, "M14")
	b.Make(second)
	nb.Set(second, Red)
	if !b.IsLegal(hexOne(t, "M13")) {
		t.Fatal("hex constraint must expire once red holds two stones")
	}
}

func TestHexLegalMovesMatchOracle(t *testing.T) {
	buf := make([]Move, config.MaxBoardCells)
	for _, phase := range []int{0, 1, 2} {
		b, nb := NewBoardFor(hexGeometry), NewNaiveBoardFor(hexGeometry)
		if phase >= 1 {
			for _, m := range []struct {
				name  string
				color Color
			}{{"L12", Red}, {"A12", Blue}} {
				c := hexOne(t, m.name)
				b.Make(c)
				nb.Set(c, m.color)
			}
		}
		if phase == 2 {
			for _, m := range []struct {
				name  string
				color Color
			}{{"M14", Red}, {"A13", Blue}} {
				c := hexOne(t, m.name)
				b.Make(c)
				nb.Set(c, m.color)
			}
		}
		n := b.LegalMoves(buf)
		want := make(map[Cell]bool)
		for cell := range hexGeometry.Cells {
			if nb.IsLegal(b.Side, Cell(cell)) {
				want[Cell(cell)] = true
			}
		}
		if n != len(want) {
			t.Fatalf("phase %d: %d legal moves, oracle count %d", phase, n, len(want))
		}
		for _, m := range buf[:n] {
			if !want[Cell(m)] {
				t.Fatalf("phase %d: LegalMoves yielded %d which the oracle rejects", phase, m)
			}
		}
	}
}

func recomputeHashHex(b *Board) uint64 {
	var h uint64
	for cell := range b.Geom.Cells {
		w, m := bitOf(Cell(cell))
		if b.Red[w]&m != 0 {
			h ^= zobristPieces[Red][cell]
		}
		if b.Blue[w]&m != 0 {
			h ^= zobristPieces[Blue][cell]
		}
	}
	if b.MoveCount%2 == 1 {
		h ^= zobristSide
	}
	return h
}

func TestHexRandomPlayDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x5EED_0BE5, 0x0DD_5EED))
	buf := make([]Move, config.MaxBoardCells)
	plies := 0
	for game := range 60 {
		b := NewBoardFor(hexGeometry)
		nb := NewNaiveBoardFor(hexGeometry)
		for b.MoveCount < 397 {
			for _, color := range [colorCount]Color{Red, Blue} {
				if got, want := b.Wins(color), nb.Wins(color); got != want {
					t.Fatalf("game %d ply %d: Wins(%v) = %v, oracle %v", game, b.MoveCount, color, got, want)
				}
			}
			n := b.LegalMoves(buf)
			if n == 0 {
				if !b.IsFull() {
					t.Fatalf("game %d: stuck with region not full", game)
				}
				break
			}
			mover := b.Side
			cell := Cell(buf[rng.IntN(n)])
			before := snapshot(b)
			b.Make(cell)
			nb.Set(cell, mover)
			plies++
			if b.Hash != recomputeHashHex(b) {
				t.Fatalf("game %d ply %d: incremental hash %016x, recomputed %016x", game, b.MoveCount, b.Hash, recomputeHashHex(b))
			}
			for _, color := range [colorCount]Color{Red, Blue} {
				if got, want := b.Wins(color), nb.Wins(color); got != want {
					t.Fatalf("game %d ply %d after move: Wins(%v) = %v, oracle %v", game, b.MoveCount, color, got, want)
				}
			}
			if got, want := b.FastLastMoveWin(mover, cell), nb.WinsThrough(mover, cell); got != want {
				t.Fatalf("game %d ply %d: FastLastMoveWin = %v, WinsThrough %v", game, b.MoveCount, got, want)
			}
			for _, probe := range []Cell{cell, 0, 517, 518} {
				if got, want := b.IsLegal(probe), nb.IsLegal(b.Side, probe); got != want {
					t.Fatalf("game %d ply %d: IsLegal(%d) = %v, oracle %v", game, b.MoveCount, probe, got, want)
				}
			}
			b.Unmake()
			assertSnapEq(t, snapshot(b), before)
			b.Make(cell)
			if b.Wins(Red) || b.Wins(Blue) {
				break
			}
		}
	}
	if plies < 500 {
		t.Fatalf("differential covered only %d plies", plies)
	}
}

func TestHexUnmakeDeepRestore(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 33))
	b := NewBoardFor(hexGeometry)
	buf := make([]Move, config.MaxBoardCells)
	for range 30 {
		n := b.LegalMoves(buf)
		if n == 0 {
			break
		}
		b.Make(Cell(buf[rng.IntN(n)]))
	}
	if b.MoveCount == 0 {
		t.Fatal("setup: expected moves on the board")
	}
	fresh := snapshot(NewBoardFor(hexGeometry))
	for b.MoveCount > 0 {
		b.Unmake()
	}
	assertSnapEq(t, snapshot(b), fresh)
}
