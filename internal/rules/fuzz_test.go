package rules

import (
	"testing"
)

// Encoding: data[0] bit0 region (0 full, 1 cross check), bit1 side to move
// (0 red, 1 blue). data[1] red stone count, data[2] blue stone count, both
// capped. Then red cells followed by blue cells, one byte per cell index.
// Remaining bytes are legality and make/unmake candidates.
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
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 3 {
			return
		}
		cross := data[0]&1 == 1
		side := Red
		if data[0]&2 != 0 {
			side = Blue
		}
		reds := int(data[1]) % 33
		blues := int(data[2]) % 33
		body := data[3:]
		if reds+blues > len(body) {
			reds = min(reds, len(body))
			blues = min(blues, len(body)-reds)
		}
		var b *Board
		var nb *NaiveBoard
		if cross {
			b, nb = NewCrossCheck(), NewNaiveCrossCheck()
		} else {
			b, nb = NewBoard(), NewNaiveBoard()
		}
		b.Side = side
		var stones []Cell
		for _, v := range body[:reds] {
			c := Cell(v)
			if b.inRegion(c) && !b.Occupied(c) {
				setStone(b, c, Red)
				nb.Set(c, Red)
				stones = append(stones, c)
			}
		}
		for _, v := range body[reds : reds+blues] {
			c := Cell(v)
			if b.inRegion(c) && !b.Occupied(c) {
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
		rest := body[reds+blues:]
		for _, v := range rest {
			if b.IsLegal(Cell(v)) != nb.IsLegal(side, Cell(v)) {
				t.Fatalf("legality disagreement cell %d data %x", v, data)
			}
		}
		for _, v := range rest {
			c := Cell(v)
			if !b.inRegion(c) || b.Occupied(c) {
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
