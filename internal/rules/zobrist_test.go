package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestZobristKeysDistinctNonzero(t *testing.T) {
	seen := make(map[uint64]struct{})
	add := func(k uint64) {
		if k == 0 {
			t.Fatal("zobrist key must not be zero")
		}
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate zobrist key %016x", k)
		}
		seen[k] = struct{}{}
	}
	for color := range colorCount {
		for cell := range config.BoardCells {
			add(zobristPieces[color][cell])
		}
	}
	add(zobristSide)
}

// Golden pins derived once from the pinned splitmix64 constants and
// config.ZobristSeed: any mixer, seed-chain, or colorCount mutation changes
// at least one of these exact values.
func TestZobristGoldenPins(t *testing.T) {
	if len(zobristPieces) != 2 {
		t.Fatalf("len(zobristPieces) = %d, want 2 (red and blue keys only)", len(zobristPieces))
	}
	golden := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"zobristPieces[Red][0]", zobristPieces[Red][0], 0x6e789e6aa1b965f4},
		{"zobristPieces[Red][1]", zobristPieces[Red][1], 0x46b73e79f0c37c00},
		{"zobristPieces[Blue][0]", zobristPieces[Blue][0], 0xeaeb6be0867a5fcc},
		{"zobristSide", zobristSide, 0x6e091eb2c7957492},
	}
	for _, g := range golden {
		if g.got != g.want {
			t.Errorf("%s = %#016x, want %#016x", g.name, g.got, g.want)
		}
	}
	if got := NewBoard().Hash; got != 0 {
		t.Errorf("empty board hash = %#016x, want 0", got)
	}
	b := NewBoard()
	b.Make(oneCell(t, "A1"))
	if got := b.Hash; got != 0x007180d8662c1166 {
		t.Errorf("hash after Make(A1) = %#016x, want 0x007180d8662c1166", got)
	}
	b.Make(oneCell(t, "P16"))
	if got := b.Hash; got != 0xf63fcb26f01f7430 {
		t.Errorf("hash after Make(A1),Make(P16) = %#016x, want 0xf63fcb26f01f7430", got)
	}
	b2 := NewBoard()
	for _, name := range []string{"H8", "A1", "P16"} {
		b2.Make(oneCell(t, name))
	}
	if got := b2.Hash; got != 0xdd88e0135ff27c4b {
		t.Errorf("hash after H8,A1,P16 = %#016x, want 0xdd88e0135ff27c4b", got)
	}
}

func TestZobristManualExpected(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x5EED_5EED, 1))
	for game := range 50 {
		b := NewBoard()
		want := uint64(0)
		n := rng.IntN(config.BoardCells)
		for i := 0; i < n; i++ {
			cell := Cell(rng.IntN(config.BoardCells))
			if b.Occupied(cell) {
				i--
				continue
			}
			want ^= zobristPieces[b.Side][cell] ^ zobristSide
			b.Make(cell)
		}
		if b.Hash != want {
			t.Fatalf("game %d: hash %016x want %016x", game, b.Hash, want)
		}
	}
}

func TestZobristUnmakeRestoresHash(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for range 100 {
		b := NewBoard()
		for m := rng.IntN(64); m > 0; m-- {
			cell := Cell(rng.IntN(config.BoardCells))
			if b.Occupied(cell) {
				continue
			}
			h := b.Hash
			b.Make(cell)
			if b.Hash == h {
				t.Fatal("make must change hash")
			}
			b.Unmake()
			if b.Hash != h {
				t.Fatalf("unmake must restore hash exactly: %016x want %016x", b.Hash, h)
			}
		}
	}
}

func TestZobristOrderIndependence(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 17))
	for range 100 {
		red := rng.IntN(30) + 1
		perm := rng.Perm(config.BoardCells)
		redCells := perm[:red]
		blueCells := perm[red : red+red-1]

		play := func() (uint64, Color, int) {
			b := NewBoard()
			ri, bi := 0, 0
			for ri < len(redCells) || bi < len(blueCells) {
				if bi >= len(blueCells) || (ri < len(redCells) && b.Side == Red) {
					b.Make(Cell(redCells[ri]))
					ri++
				} else {
					b.Make(Cell(blueCells[bi]))
					bi++
				}
			}
			return b.Hash, b.Side, b.MoveCount
		}

		h1, s1, m1 := play()
		rng.Shuffle(len(redCells), func(i, j int) { redCells[i], redCells[j] = redCells[j], redCells[i] })
		rng.Shuffle(len(blueCells), func(i, j int) { blueCells[i], blueCells[j] = blueCells[j], blueCells[i] })
		h2, s2, m2 := play()
		if h1 != h2 || s1 != s2 || m1 != m2 {
			t.Fatalf("same position must hash identically: %016x vs %016x", h1, h2)
		}
	}
}
