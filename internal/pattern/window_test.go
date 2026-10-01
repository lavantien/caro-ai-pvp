package pattern

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func seededBoard(t *testing.T, seed uint64, cross bool) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	size := config.BoardSize
	if cross {
		b = rules.NewCrossCheck()
		size = config.CrossCheckSize
	}
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	count := rng.IntN(2 * size * size / 3)
	edge := [...]int{0, 1, size - 2, size - 1}
	for _, r := range edge {
		for _, c := range edge {
			b.Side = rules.Color(rng.IntN(2))
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	for i := 0; i < count; i++ {
		cell := rules.Cell(rng.IntN(size))*config.BoardStride + rules.Cell(rng.IntN(size))
		if b.Occupied(cell) {
			i--
			continue
		}
		b.Side = rules.Color(rng.IntN(2))
		b.Make(cell)
	}
	return b
}

// manualWindow re-derives the window through rules.At and a direct region
// bit read, a separate code path from Index.
func manualWindow(b *rules.Board, cell rules.Cell, dir int, mover rules.Color) [config.PatternWindowLen]uint8 {
	var w [config.PatternWindowLen]uint8
	dr, dc := config.PatternDirs[dir][0], config.PatternDirs[dir][1]
	r := int(cell) / config.BoardStride
	c := int(cell) % config.BoardStride
	for i := range w {
		rr, cc := r+(int(i)-windowHalf)*dr, c+(int(i)-windowHalf)*dc
		if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
			w[i] = config.PatternStateOff
			continue
		}
		ic := rr*config.BoardStride + cc
		if b.Region[ic/wordBits]&(uint64(1)<<(uint(ic)%wordBits)) == 0 {
			w[i] = config.PatternStateOff
			continue
		}
		switch b.At(rules.Cell(ic)) {
		case rules.Empty:
			w[i] = config.PatternStateEmpty
		case mover:
			w[i] = config.PatternStateOwn
		default:
			w[i] = config.PatternStateOpp
		}
	}
	return w
}

func TestPackUnpackRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 22))
	for range 10000 {
		var w [config.PatternWindowLen]uint8
		for i := range w {
			w[i] = uint8(rng.IntN(4))
		}
		if got := Unpack(Pack(w)); got != w {
			t.Fatalf("round trip: got %v want %v", got, w)
		}
	}
}

func TestIndexMatchesManual(t *testing.T) {
	seeds := uint64(4)
	if testing.Short() {
		seeds = 1
	}
	for _, region := range []struct {
		name  string
		cross bool
	}{{"full", false}, {"crosscheck", true}} {
		for seed := uint64(1); seed <= seeds; seed++ {
			b := seededBoard(t, seed, region.cross)
			size := config.BoardSize
			if region.cross {
				size = config.CrossCheckSize
			}
			for r := range size {
				for c := range size {
					cell := rules.Cell(r*config.BoardStride + c)
					for dir := range config.PatternDirections {
						for _, mover := range [2]rules.Color{rules.Red, rules.Blue} {
							idx := Index(b, cell, dir, mover)
							if want := Pack(manualWindow(b, cell, dir, mover)); idx != want {
								t.Fatalf("%s seed %d cell %d dir %d mover %v: Index = %#x want %#x", region.name, seed, cell, dir, mover, idx, want)
							}
							w := Unpack(idx)
							if !isRealizable(&w) {
								t.Fatalf("%s seed %d cell %d dir %d mover %v: window %v not realizable", region.name, seed, cell, dir, mover, w)
							}
							if w[windowHalf] == config.PatternStateOff {
								t.Fatalf("%s seed %d cell %d dir %d mover %v: center off", region.name, seed, cell, dir, mover)
							}
						}
					}
				}
			}
		}
	}
}

func TestExtractedEntryMatchesNaive(t *testing.T) {
	stride := 1
	if testing.Short() {
		stride = 17
	}
	for _, region := range []struct {
		name  string
		cross bool
	}{{"full", false}, {"crosscheck", true}} {
		b := seededBoard(t, 7, region.cross)
		size := config.BoardSize
		if region.cross {
			size = config.CrossCheckSize
		}
		n := 0
		for r := range size {
			for c := range size {
				if (r*size+c)%stride != 0 {
					continue
				}
				cell := rules.Cell(r*config.BoardStride + c)
				for dir := range config.PatternDirections {
					idx := Index(b, cell, dir, rules.Red)
					if got, want := Lookup(dir, idx).Win1, naiveWin1(t, Unpack(idx)); got != want {
						t.Fatalf("%s cell %d dir %d: Win1 = %#x want %#x", region.name, cell, dir, got, want)
					}
					n++
				}
			}
		}
		if n == 0 {
			t.Fatalf("%s: no windows sampled", region.name)
		}
	}
}

func TestIndexPanicsOutOfRange(t *testing.T) {
	b := rules.NewBoard()
	defer func() {
		if recover() == nil {
			t.Fatal("Index on out-of-range cell: want panic")
		}
	}()
	_ = Index(b, rules.Cell(config.BoardCells), 0, rules.Red)
}

func TestLookupPanicsBadDir(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Lookup on dir 4: want panic")
		}
	}()
	_ = Lookup(config.PatternDirections, 0)
}

func TestLookupPanicsOversizedIndex(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Lookup on oversized index: want panic")
		}
	}()
	_ = Lookup(0, uint32(config.PatternTableEntries))
}
