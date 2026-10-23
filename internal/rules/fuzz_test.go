package rules

import (
	"testing"
)

func FuzzRulesDifferential(f *testing.F) {
	seeds := [][]byte{
		{0, 5, 0, 0, 1, 2, 3, 4},
		{0, 5, 2, 1, 2, 3, 4, 5, 0, 6},
		{0, 6, 0, 0, 1, 2, 3, 4, 5},
		{0, 5, 0, 0, 17, 34, 51, 68},
		{0, 5, 0, 64, 49, 34, 19, 4},
		{0, 5, 0, 0, 1, 2, 3, 4, 119, 5},
		{1, 5, 0, 0, 1, 2, 3, 4},
		{1, 5, 2, 1, 2, 3, 4, 5, 0, 6},
		{1, 5, 0, 3, 19, 35, 51, 67},
		{0, 1, 1, 119, 0, 120, 200},
		{2, 1, 1, 119, 0, 120, 200},
		{0, 10, 10, 0, 1, 2, 3, 4, 16, 32, 48, 64, 17, 33, 255, 254, 253, 128, 127, 126, 125},
		{4, 3, 1, 8, 1, 31, 1, 54, 1, 34, 1, 26, 0, 244, 1},
		{4, 5, 1, 8, 1, 31, 1, 54, 1, 77, 1, 100, 1, 241, 0, 0, 0, 5, 2},
		{4, 6, 0, 8, 1, 31, 1, 54, 1, 77, 1, 100, 1, 123, 1, 0, 0},
		{4, 1, 1, 8, 1, 9, 1, 31, 1, 74, 1},
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 3 {
			return
		}
		hexMode := data[0]&4 != 0
		cross := !hexMode && data[0]&1 == 1
		side := Red
		if data[0]&2 != 0 {
			side = Blue
		}
		reds := int(data[1]) % 33
		blues := int(data[2]) % 33
		body := data[3:]
		width := 1
		if hexMode {
			width = 2
		}
		cells := len(body) / width
		if reds+blues > cells {
			reds = min(reds, cells)
			blues = min(blues, cells-reds)
		}
		var b *Board
		var nb *NaiveBoard
		if hexMode {
			b, nb = NewBoardFor(hexGeometry), NewNaiveBoardFor(hexGeometry)
		} else if cross {
			b, nb = NewCrossCheck(), NewNaiveCrossCheck()
		} else {
			b, nb = NewBoard(), NewNaiveBoard()
		}
		b.Side = side
		cellAt := func(i int) Cell {
			if hexMode {
				return Cell(int(body[2*i]) | int(body[2*i+1])<<8)
			}
			return Cell(body[i])
		}
		limit := Cell(b.Geom.Cells)
		var stones []Cell
		for i := range reds {
			c := cellAt(i)
			if c < limit && b.inRegion(c) && !b.Occupied(c) {
				setStone(b, c, Red)
				nb.Set(c, Red)
				stones = append(stones, c)
			}
		}
		for i := range blues {
			c := cellAt(reds + i)
			if c < limit && b.inRegion(c) && !b.Occupied(c) {
				setStone(b, c, Blue)
				nb.Set(c, Blue)
				stones = append(stones, c)
			}
		}
		for _, color := range [colorCount]Color{Red, Blue} {
			if b.Wins(color) != nb.Wins(color) {
				t.Fatalf("wins disagreement color %v data %x", color, data)
			}
		}
		for _, c := range stones {
			color := b.At(c)
			if b.FastLastMoveWin(color, c) != nb.WinsThrough(color, c) {
				t.Fatalf("through disagreement cell %d data %x", c, data)
			}
		}
		rest := cells - reds - blues
		for i := range rest {
			c := cellAt(reds + blues + i)
			if c >= limit {
				continue
			}
			if b.IsLegal(c) != nb.IsLegal(side, c) {
				t.Fatalf("legality disagreement cell %d data %x", c, data)
			}
		}
		for i := range rest {
			c := cellAt(reds + blues + i)
			if c >= limit || !b.inRegion(c) || b.Occupied(c) {
				continue
			}
			before := snapshot(b)
			b.Make(c)
			b.Unmake()
			if snapshot(b) != before {
				t.Fatalf("make/unmake snapshot drift data %x", data)
			}
			break
		}
	})
}
