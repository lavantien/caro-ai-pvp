package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func cellsOf(t *testing.T, names ...string) []Cell {
	t.Helper()
	out := make([]Cell, len(names))
	for i, name := range names {
		c, err := ParseCell(name)
		if err != nil {
			t.Fatalf("ParseCell(%q): %v", name, err)
		}
		out[i] = c
	}
	return out
}

func setAll(t *testing.T, nb *NaiveBoard, color Color, names ...string) {
	t.Helper()
	for _, c := range cellsOf(t, names...) {
		nb.Set(c, color)
	}
}

func naiveFull(t *testing.T, red, blue []string) *NaiveBoard {
	t.Helper()
	nb := NewNaiveBoard()
	setAll(t, nb, Red, red...)
	setAll(t, nb, Blue, blue...)
	return nb
}

func naiveCross(t *testing.T, red, blue []string) *NaiveBoard {
	t.Helper()
	nb := NewNaiveCrossCheck()
	setAll(t, nb, Red, red...)
	setAll(t, nb, Blue, blue...)
	return nb
}

func TestNaiveWinsFullBoard(t *testing.T) {
	cases := []struct {
		name    string
		red     []string
		blue    []string
		redWin  bool
		blueWin bool
	}{
		{"empty", nil, nil, false, false},
		{"exact five open horizontal", []string{"A1", "B1", "C1", "D1", "E1"}, nil, true, false},
		{"exact five one end blocked", []string{"B1", "C1", "D1", "E1", "F1"}, []string{"G1"}, true, false},
		{"exact five both ends blocked dead", []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1", "G1"}, false, false},
		{"wall end plus blocked end win", []string{"A1", "B1", "C1", "D1", "E1"}, []string{"F1"}, true, false},
		{"wall both ends via board edge empty side", []string{"A1", "B1", "C1", "D1", "E1"}, nil, true, false},
		{"overline six never wins", []string{"A1", "B1", "C1", "D1", "E1", "F1"}, nil, false, false},
		{"seven long never wins", []string{"B2", "C2", "D2", "E2", "F2", "G2", "H2"}, nil, false, false},
		{"vertical five", []string{"K10", "K11", "K12", "K13", "K14"}, nil, true, false},
		{"diag southeast five", []string{"A1", "B2", "C3", "D4", "E5"}, nil, true, false},
		{"diag northeast five", []string{"A5", "B4", "C3", "D2", "E1"}, nil, true, false},
		{"broken three no win", []string{"A1", "B1", "C1", "A3", "B3", "C3"}, nil, false, false},
		{"four no win", []string{"A1", "B1", "C1", "D1"}, nil, false, false},
		{"blue wins red does not", []string{"H8", "H9"}, []string{"A16", "B16", "C16", "D16", "E16"}, false, true},
		{"gap in line no win", []string{"A1", "B1", "C1", "E1", "F1"}, nil, false, false},
		{"five blocked by own extension one side other side opponent", []string{"A1", "B1", "C1", "D1", "E1", "F1"}, []string{"P16"}, false, false},
	}
	for _, tc := range cases {
		nb := naiveFull(t, tc.red, tc.blue)
		if got := nb.Wins(Red); got != tc.redWin {
			t.Fatalf("%s: red Wins = %v", tc.name, got)
		}
		if got := nb.Wins(Blue); got != tc.blueWin {
			t.Fatalf("%s: blue Wins = %v", tc.name, got)
		}
	}
}

func TestNaiveWinsCrossCheckRegion(t *testing.T) {
	nb := naiveCross(t, []string{"A1", "B1", "C1", "D1", "E1"}, nil)
	if !nb.Wins(Red) {
		t.Fatal("region 8x8: five along top with out-of-region F1 acting as wall: want red win")
	}
	nb = naiveCross(t, []string{"D1", "E1", "F1", "G1", "H1"}, nil)
	if !nb.Wins(Red) {
		t.Fatal("region 8x8: five ending at region edge H1: want red win")
	}
	nb = naiveCross(t, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1"})
	if !nb.Wins(Red) {
		t.Fatal("region 8x8: one end blocked by blue, other end open: want red win")
	}
	nb = naiveCross(t, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1", "G1"})
	if nb.Wins(Red) {
		t.Fatal("region 8x8: both ends blocked inside region: want dead")
	}
}

func TestNaiveWinsThrough(t *testing.T) {
	nb := naiveFull(t, []string{"A1", "B1", "C1", "D1", "E1", "H8"}, nil)
	through := cellsOf(t, "A1", "B1", "C1", "D1", "E1")
	for _, c := range through {
		if !nb.WinsThrough(Red, c) {
			t.Fatalf("WinsThrough(red, %d): want true for run member", c)
		}
	}
	if nb.WinsThrough(Red, oneCell(t, "H8")) {
		t.Fatal("WinsThrough(red, H8): want false for stone outside the run")
	}
	if nb.WinsThrough(Blue, oneCell(t, "H8")) {
		t.Fatal("WinsThrough(blue, H8): want false, H8 is red")
	}
	nb = naiveFull(t, []string{"A1", "B1", "C1", "D1", "E1", "F1"}, nil)
	for _, name := range []string{"B1", "C1", "D1", "E1"} {
		c := cellsOf(t, name)[0]
		if nb.WinsThrough(Red, c) {
			t.Fatalf("WinsThrough(red, %s): overline member never wins", name)
		}
	}
	nb = naiveFull(t, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1", "G1"})
	c := cellsOf(t, "D1")[0]
	if nb.WinsThrough(Red, c) {
		t.Fatal("WinsThrough(red, D1): both ends blocked, want false")
	}
}

func oneCell(t *testing.T, name string) Cell {
	t.Helper()
	return cellsOf(t, name)[0]
}

func TestNaiveWinsThroughDiagonals(t *testing.T) {
	nb := naiveFull(t, []string{"A1", "B2", "C3", "D4", "E5"}, nil)
	if !nb.WinsThrough(Red, oneCell(t, "C3")) {
		t.Fatal("WinsThrough(red, C3) on southeast diagonal: want true")
	}
	nb = naiveFull(t, []string{"A5", "B4", "C3", "D2", "E1"}, nil)
	if !nb.WinsThrough(Red, oneCell(t, "C3")) {
		t.Fatal("WinsThrough(red, C3) on northeast diagonal: want true")
	}
}

func TestNaiveIsLegal(t *testing.T) {
	nb := naiveFull(t, []string{"H8"}, []string{"A1"})
	for _, tc := range []struct {
		name string
		side Color
		want bool
	}{
		{"K11", Red, true},
		{"H11", Red, true},
		{"K8", Red, true},
		{"J10", Red, false},
		{"I9", Red, false},
		{"I8", Red, false},
		{"I10", Red, false},
		{"H8", Red, false},
		{"A1", Red, false},
		{"P16", Blue, true},
	} {
		c := cellsOf(t, tc.name)[0]
		if got := nb.IsLegal(tc.side, c); got != tc.want {
			t.Fatalf("IsLegal(%v, %s) = %v, want %v", tc.side, tc.name, got, tc.want)
		}
	}
	nb = naiveFull(t, nil, nil)
	if !nb.IsLegal(Red, oneCell(t, "A1")) {
		t.Fatal("red first move with zero red stones: want legal")
	}
	nb = naiveFull(t, []string{"H8", "H9"}, nil)
	if !nb.IsLegal(Red, oneCell(t, "I8")) {
		t.Fatal("red third move with two red stones: constraint absent, want legal")
	}
	nc := naiveCross(t, []string{"H8"}, nil)
	if nc.IsLegal(Red, Cell(8*config.BoardStride)) {
		t.Fatal("out-of-region cell A9 on 8x8 region: want illegal")
	}
	if nc.IsLegal(Red, Cell(config.BoardCells)) {
		t.Fatal("out-of-bounds cell: want illegal")
	}
}

func TestNaiveSetPanics(t *testing.T) {
	nb := naiveFull(t, []string{"A1"}, nil)
	for name, fn := range map[string]func(){
		"occupied":      func() { nb.Set(0, Blue) },
		"out of region": func() { nc := NewNaiveCrossCheck(); nc.Set(config.CrossCheckSize*config.BoardStride, Red) },
		"out of bounds": func() { nb.Set(Cell(config.BoardCells), Red) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("Set %s: want panic", name)
				}
			}()
			fn()
		}()
	}
}
