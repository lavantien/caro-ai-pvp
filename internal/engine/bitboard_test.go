package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func bruteDilate(cells map[uint16]bool, radius int) map[uint16]bool {
	out := make(map[uint16]bool, len(cells)*(2*radius+1)*(2*radius+1))
	for c := range cells {
		r, co := int(c/config.BoardStride), int(c%config.BoardStride)
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius; dc++ {
				rr, cc := r+dr, co+dc
				if rr >= 0 && rr < config.BoardSize && cc >= 0 && cc < config.BoardSize {
					out[uint16(rr*config.BoardStride+cc)] = true
				}
			}
		}
	}
	return out
}

func TestDilateMatchesBruteForce(t *testing.T) {
	g := geom.Default()
	rng := rand.New(rand.NewPCG(7, 13))
	for radius := 1; radius <= 3; radius++ {
		for trial := range 30 {
			cells := make(map[uint16]bool)
			for range 1 + rng.IntN(20) {
				cells[uint16(rng.IntN(config.BoardCells))] = true
			}
			var s bitmask
			for c := range cells {
				s[c/64] |= 1 << (c % 64)
			}
			got := dilate(s, radius, g)
			want := bruteDilate(cells, radius)
			for w := range want {
				if got[w/64]>>(w%64)&1 == 0 {
					t.Fatalf("radius %d trial %d: cell %d missing", radius, trial, w)
				}
			}
			for w := range config.BoardCells {
				u16 := uint16(w)
				if !want[u16] && got[w/64]>>(w%64)&1 != 0 {
					t.Fatalf("radius %d trial %d: stray cell %d", radius, trial, w)
				}
			}
		}
	}
}

func TestDilate1NeighborsMatchesFused(t *testing.T) {
	g := geom.Default()
	rng := rand.New(rand.NewPCG(21, 34))
	for trial := range 50 {
		var s bitmask
		for range 1 + rng.IntN(24) {
			c := uint16(rng.IntN(config.BoardCells))
			s[c/64] |= 1 << (c % 64)
		}
		fused := dilate1(s, g)
		byNeighbors := dilate1Neighbors(s, g)
		if fused != byNeighbors {
			t.Fatalf("trial %d: neighbor dilation %x differs from the fused ring %x", trial, byNeighbors, fused)
		}
	}
}

func TestDilateHexMatchesBruteForce(t *testing.T) {
	g := geom.Hex(12)
	box := g.Rows * g.Cols
	rng := rand.New(rand.NewPCG(5, 83))
	for radius := 1; radius <= 3; radius++ {
		for trial := range 30 {
			var s bitmask
			stones := make(map[int]bool)
			for range 1 + rng.IntN(16) {
				for {
					cell := regionCell(g, rng)
					if !stones[cell] {
						stones[cell] = true
						s[cell/64] |= 1 << (uint(cell) % 64)
						break
					}
				}
			}
			want := make(map[int]bool)
			for range radius {
				next := make(map[int]bool)
				for c := range want {
					next[c] = true
				}
				for c := range stones {
					next[c] = true
				}
				for c := range next {
					q, r := c/g.Stride, c%g.Stride
					for _, n := range g.Neighbors {
						qq, rr := q+n[0], r+n[1]
						if qq >= 0 && qq < g.Rows && rr >= 0 && rr < g.Cols {
							want[qq*g.Stride+rr] = true
						}
					}
				}
			}
			for c := range stones {
				want[c] = true
			}
			got := dilate(s, radius, g)
			for c := range want {
				if got[c/64]>>(uint(c)%64)&1 == 0 {
					t.Fatalf("radius %d trial %d: cell %d missing", radius, trial, c)
				}
			}
			for c := range box {
				if !want[c] && got[c/64]>>(uint(c)%64)&1 != 0 {
					var missing []int
					for w := range want {
						missing = append(missing, w)
					}
					t.Fatalf("radius %d trial %d: stray cell %d, want has %v, stones %v", radius, trial, c, missing, stones)
				}
			}
		}
	}
}

func regionCell(g *geom.Geometry, rng *rand.Rand) int {
	for {
		cell := rng.IntN(g.Cells)
		if g.Contains(cell) {
			return cell
		}
	}
}

func TestDilateCornerDoesNotWrap(t *testing.T) {
	var s bitmask
	s[0] = 1
	got := dilate1(s, geom.Default())
	want := []uint16{0, 1, 16, 17}
	for _, c := range want {
		if got[c/64]>>(c%64)&1 == 0 {
			t.Errorf("corner dilation missing cell %d", c)
		}
	}
	if got[0] != 0b11|(1<<16)|(1<<17) {
		t.Errorf("corner dilation = %#x, want exactly A1 A2 B1 B2", got[0])
	}
	if got[0]>>2&1 != 0 {
		t.Error("column wrap leaked C1 into the dilation")
	}
}

func TestChebyshevCell(t *testing.T) {
	g := rules.NewBoard().Geom
	cases := [...]struct {
		a, b uint16
		want int
	}{
		{0, 0, 0},
		{0, 1, 1},
		{0, 16, 1},
		{0, 17, 1},
		{0, 32, 2},
		{3, 3 + 3*config.BoardStride + 2, 3},
	}
	for _, c := range cases {
		if got := g.Distance(int(c.a), int(c.b)); got != c.want {
			t.Errorf("distance(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestOpeningAnchor(t *testing.T) {
	b := rules.NewBoard()
	if _, ok := openingAnchor(b); ok {
		t.Error("empty board must not constrain")
	}
	place(t, b, rules.Red, "H8")
	b.Side = rules.Red
	anchor, ok := openingAnchor(b)
	if !ok || anchor != uint16(mustCell(t, "H8")) {
		t.Errorf("one red stone side red: anchor %d ok %v", anchor, ok)
	}
	b.Side = rules.Blue
	if _, ok := openingAnchor(b); ok {
		t.Error("blue to move must not constrain")
	}
	place(t, b, rules.Blue, "I9")
	place(t, b, rules.Red, "E5")
	if _, ok := openingAnchor(b); ok {
		t.Error("two red stones must not constrain")
	}
}
