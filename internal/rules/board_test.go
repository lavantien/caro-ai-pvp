package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type boardSnap struct {
	red, blue, region, full bb
	side                    Color
	moves                   int
	hash                    uint64
}

func snapshot(b *Board) boardSnap {
	return boardSnap{b.Red, b.Blue, b.Region, b.Full, b.Side, b.MoveCount, b.Hash}
}

func assertSnapEq(t *testing.T, got, want boardSnap) {
	t.Helper()
	if got != want {
		t.Fatalf("snapshot mismatch: got %+v want %+v", got, want)
	}
}

func TestNewBoard(t *testing.T) {
	b := NewBoard()
	if b.Side != Red {
		t.Fatal("fresh board: side to move must be red")
	}
	if b.MoveCount != 0 || b.Hash != 0 {
		t.Fatal("fresh board: want zero moves and zero hash")
	}
	if popcountBB(b.Region) != config.BoardCells {
		t.Fatalf("full region popcount = %d, want %d", popcountBB(b.Region), config.BoardCells)
	}
	if b.IsFull() {
		t.Fatal("fresh board must not be full")
	}
	if b.At(0) != Empty {
		t.Fatal("fresh board A1 must be empty")
	}
	if b.Occupied(config.BoardCells - 1) {
		t.Fatal("fresh board P16 must be unoccupied")
	}
}

func TestNewCrossCheck(t *testing.T) {
	b := NewCrossCheck()
	want := config.CrossCheckSize * config.CrossCheckSize
	if got := popcountBB(b.Region); got != want {
		t.Fatalf("cross region popcount = %d, want %d", got, want)
	}
	if b.inRegion(0) != true || b.inRegion(Cell(config.CrossCheckSize*config.BoardStride)) {
		t.Fatal("cross region must cover rows 0..7 cols 0..7 only")
	}
}

func TestMakeAtOccupiedSideMoveCount(t *testing.T) {
	b := NewBoard()
	a1, p16, h8 := oneCell(t, "A1"), oneCell(t, "P16"), oneCell(t, "H8")
	b.Make(a1)
	if b.At(a1) != Red {
		t.Fatal("after red A1: At(A1) must be red")
	}
	if !b.Occupied(a1) {
		t.Fatal("after red A1: A1 occupied")
	}
	if b.Side != Blue {
		t.Fatal("after one move: blue to move")
	}
	if b.MoveCount != 1 {
		t.Fatal("after one move: MoveCount 1")
	}
	if b.Hash == 0 {
		t.Fatal("hash must change after a move")
	}
	b.Make(p16)
	if b.At(p16) != Blue || b.At(a1) != Red {
		t.Fatal("colors mixed up after second move")
	}
	after2 := snapshot(b)
	b.Make(h8)
	if b.At(h8) != Red {
		t.Fatal("third move must be red")
	}
	before := snapshot(b)
	b.Unmake()
	if b.At(h8) != Empty || b.Occupied(h8) {
		t.Fatal("after unmake H8 empty")
	}
	assertSnapEq(t, snapshot(b), after2)
	b.Make(h8)
	assertSnapEq(t, snapshot(b), before)

	for b.MoveCount > 0 {
		b.Unmake()
	}
	fresh := snapshot(NewBoard())
	assertSnapEq(t, snapshot(b), fresh)
}

func TestMakePanics(t *testing.T) {
	b := NewBoard()
	a1 := oneCell(t, "A1")
	b.Make(a1)
	for name, fn := range map[string]func(){
		"occupied":      func() { b.Make(a1) },
		"out of region": func() { NewCrossCheck().Make(Cell(config.CrossCheckSize * config.BoardStride)) },
		"out of bounds": func() { b.Make(Cell(config.BoardCells)) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("Make %s: want panic", name)
				}
			}()
			fn()
		}()
	}
}

func TestUnmakePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Unmake on empty stack: want panic")
		}
	}()
	NewBoard().Unmake()
}

func TestIsFullAfterFillingRegion(t *testing.T) {
	for _, b := range []*Board{NewCrossCheck(), NewBoard()} {
		if b.IsFull() {
			t.Fatal("empty region cannot be full")
		}
		for cell := 0; cell < config.BoardCells; cell++ {
			c := Cell(cell)
			if b.inRegion(c) && !b.Occupied(c) {
				b.Make(c)
			}
		}
		if !b.IsFull() {
			t.Fatal("filled region must report full")
		}
		if b.MoveCount != popcountBB(b.Region) {
			t.Fatalf("MoveCount %d != region size %d", b.MoveCount, popcountBB(b.Region))
		}
	}
}

func TestUnmakeDeepRestore(t *testing.T) {
	b := NewBoard()
	cells := cellsOf(t, "H8", "A1", "P16", "C3", "L12", "F16", "B2")
	for _, c := range cells {
		b.Make(c)
	}
	for range cells {
		b.Unmake()
	}
	assertSnapEq(t, snapshot(b), snapshot(NewBoard()))
}
