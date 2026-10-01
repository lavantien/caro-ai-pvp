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
	for color := 0; color < colorCount; color++ {
		for cell := 0; cell < config.BoardCells; cell++ {
			add(zobristPieces[color][cell])
		}
	}
	add(zobristSide)
}

func TestZobristManualExpected(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x5EED_5EED, 1))
	for game := 0; game < 50; game++ {
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
	for iter := 0; iter < 100; iter++ {
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
	for iter := 0; iter < 100; iter++ {
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
			t.Fatalf("iter %d: same position must hash identically: %016x vs %016x", iter, h1, h2)
		}
	}
}
