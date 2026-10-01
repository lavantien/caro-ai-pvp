package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// bruteDilate computes the Chebyshev radius neighborhood cell set directly.
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
			got := dilate(s, radius)
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

func TestDilateCornerDoesNotWrap(t *testing.T) {
	var s bitmask
	s[0] = 1 // A1
	got := dilate1(s)
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
		if got := chebyshevCell(c.a, c.b); got != c.want {
			t.Errorf("chebyshev(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
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
