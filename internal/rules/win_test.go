package rules

import (
	"fmt"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func setStone(b *Board, cell Cell, color Color) {
	w, m := bitOf(cell)
	if color == Red {
		b.Red[w] |= m
	} else {
		b.Blue[w] |= m
	}
	b.Full[w] |= m
}

func unsetStone(b *Board, cell Cell, color Color) {
	w, m := bitOf(cell)
	if color == Red {
		b.Red[w] &^= m
	} else {
		b.Blue[w] &^= m
	}
	b.Full[w] &^= m
}

type winCase struct {
	name    string
	cross   bool
	red     []string
	blue    []string
	redWin  bool
	blueWin bool
}

func winCases() []winCase {
	return []winCase{
		{"exact five open", false, []string{"A1", "B1", "C1", "D1", "E1"}, nil, true, false},
		{"exact five one end blocked still win", false, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"G1"}, true, false},
		{"exact five both ends blocked dead", false, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1", "G1"}, false, false},
		{"wall end plus blocked end win", false, []string{"A1", "B1", "C1", "D1", "E1"}, []string{"F1"}, true, false},
		{"wall both ends via out of region", true, []string{"A1", "B1", "C1", "D1", "E1"}, nil, true, false},
		{"overline six never wins", false, []string{"A1", "B1", "C1", "D1", "E1", "F1"}, nil, false, false},
		{"overline seven never wins", false, []string{"B2", "C2", "D2", "E2", "F2", "G2", "H2"}, nil, false, false},
		{"own stone beyond kills line but other direction wins", false,
			[]string{"B1", "C1", "D1", "E1", "F1", "G1", "E2", "E3", "E4", "E5"}, nil, true, false},
		{"vertical five", false, []string{"K10", "K11", "K12", "K13", "K14"}, nil, true, false},
		{"diag southeast five", false, []string{"A1", "B2", "C3", "D4", "E5"}, nil, true, false},
		{"diag northeast five", false, []string{"A5", "B4", "C3", "D2", "E1"}, nil, true, false},
		{"blue wins red does not", false, []string{"H8", "H9"}, []string{"A16", "B16", "C16", "D16", "E16"}, false, true},
		{"gap in line no win", false, []string{"A1", "B1", "C1", "E1", "F1"}, nil, false, false},
		{"cross region five at region edge", true, []string{"D1", "E1", "F1", "G1", "H1"}, nil, true, false},
		{"cross region both ends blocked dead", true, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1", "G1"}, false, false},
		{"cross region one end blocked win", true, []string{"B1", "C1", "D1", "E1", "F1"}, []string{"A1"}, true, false},
		{"cross region vertical wall end win", true, []string{"A1", "A2", "A3", "A4", "A5"}, nil, true, false},
		{"cross region diag blocked both ends dead", true, []string{"C3", "D4", "E5", "F6", "G7"}, []string{"B2", "H8"}, false, false},
		{"cross region diag win", true, []string{"A2", "B3", "C4", "D5", "E6"}, nil, true, false},
	}
}

func TestWinSpecCases(t *testing.T) {
	for _, tc := range winCases() {
		var b *Board
		var nb *NaiveBoard
		if tc.cross {
			b, nb = NewCrossCheck(), NewNaiveCrossCheck()
		} else {
			b, nb = NewBoard(), NewNaiveBoard()
		}
		for _, name := range tc.red {
			c := oneCell(t, name)
			setStone(b, c, Red)
			nb.Set(c, Red)
		}
		for _, name := range tc.blue {
			c := oneCell(t, name)
			setStone(b, c, Blue)
			nb.Set(c, Blue)
		}
		for _, color := range [colorCount]Color{Red, Blue} {
			want := tc.redWin
			if color == Blue {
				want = tc.blueWin
			}
			if got := b.Wins(color); got != want {
				t.Fatalf("%s: bitboard Wins(%v) = %v, want %v", tc.name, color, got, want)
			}
			if got := nb.Wins(color); got != want {
				t.Fatalf("%s: naive Wins(%v) = %v, want %v", tc.name, color, got, want)
			}
			for _, name := range tc.red {
				c := oneCell(t, name)
				if got := b.FastLastMoveWin(color, c); got != nb.WinsThrough(color, c) {
					t.Fatalf("%s: FastLastMoveWin(%v, %s) = %v, naive %v", tc.name, color, name, got, nb.WinsThrough(color, c))
				}
			}
			for _, name := range tc.blue {
				c := oneCell(t, name)
				if got := b.FastLastMoveWin(color, c); got != nb.WinsThrough(color, c) {
					t.Fatalf("%s: FastLastMoveWin(%v, %s) = %v, naive %v", tc.name, color, name, got, nb.WinsThrough(color, c))
				}
			}
		}
	}
}

func TestWinOverlineSubWindowsDead(t *testing.T) {
	b, nb := NewBoard(), NewNaiveBoard()
	for _, name := range []string{"A1", "B1", "C1", "D1", "E1", "F1"} {
		c := oneCell(t, name)
		setStone(b, c, Red)
		nb.Set(c, Red)
	}
	for _, name := range []string{"A1", "B1", "C1", "D1", "E1", "F1"} {
		c := oneCell(t, name)
		if b.FastLastMoveWin(Red, c) {
			t.Fatalf("overline member %s must not report a fast win", name)
		}
		if nb.WinsThrough(Red, c) {
			t.Fatalf("naive overline member %s must not win through", name)
		}
	}
}

func TestWinDirectionThroughSameStone(t *testing.T) {
	b, nb := NewBoard(), NewNaiveBoard()
	for _, name := range []string{"B1", "C1", "D1", "E1", "F1", "G1", "E2", "E3", "E4", "E5"} {
		c := oneCell(t, name)
		setStone(b, c, Red)
		nb.Set(c, Red)
	}
	if !b.Wins(Red) {
		t.Fatal("vertical exact five through E1 must win")
	}
	if !b.FastLastMoveWin(Red, oneCell(t, "E1")) {
		t.Fatal("fast win through E1 (vertical) must hold")
	}
	if b.FastLastMoveWin(Red, oneCell(t, "C1")) {
		t.Fatal("C1 sits only in the dead six run, fast win must not hold")
	}
	if !nb.WinsThrough(Red, oneCell(t, "E1")) || nb.WinsThrough(Red, oneCell(t, "C1")) {
		t.Fatal("naive through checks disagree with expectations")
	}
}

func TestShiftImage(t *testing.T) {
	patterns := make(map[string]bb)
	var full bb
	for cell := 0; cell < config.BoardCells; cell++ {
		w, m := bitOf(Cell(cell))
		full[w] |= m
	}
	patterns["full"] = full
	for row := 0; row < config.BoardSize; row++ {
		var r bb
		for col := 0; col < config.BoardSize; col++ {
			w, m := bitOf(Cell(row*config.BoardStride + col))
			r[w] |= m
		}
		patterns[fmt.Sprintf("row%d", row)] = r
	}
	for col := 0; col < config.BoardSize; col++ {
		var c bb
		for row := 0; row < config.BoardSize; row++ {
			w, m := bitOf(Cell(row*config.BoardStride + col))
			c[w] |= m
		}
		patterns[fmt.Sprintf("col%d", col)] = c
	}

	image := func(p bb, d int, fwd bool) bb {
		dr, dc := lineDirs[d][0], lineDirs[d][1]
		if !fwd {
			dr, dc = -dr, -dc
		}
		var out bb
		for cell := 0; cell < config.BoardCells; cell++ {
			w, m := bitOf(Cell(cell))
			if p[w]&m == 0 {
				continue
			}
			r := cell/config.BoardStride + dr
			c := cell%config.BoardStride + dc
			if inBoard(r, c) {
				w2, m2 := bitOf(Cell(r*config.BoardStride + c))
				out[w2] |= m2
			}
		}
		return out
	}

	for name, p := range patterns {
		for d := range lineDirs {
			want := image(p, d, true)
			if got := shiftFwd(p, dirStep[d], dirFwdClr[d]); got != want {
				t.Fatalf("%s fwd dir %d: got %x want %x", name, d, got, want)
			}
			want = image(p, d, false)
			if got := shiftBwd(p, dirStep[d], dirBwdClr[d]); got != want {
				t.Fatalf("%s bwd dir %d: got %x want %x", name, d, got, want)
			}
		}
	}
}
