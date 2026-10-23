package pattern

import (
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func hexBoardFor(t *testing.T) *rules.Board {
	t.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	return rules.NewBoardFor(g)
}

func hexGeomFor(t *testing.T) *geom.Geometry {
	t.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	return g
}

func TestUnifiedTableServesHex(t *testing.T) {
	b := hexBoardFor(t)
	g := b.Geom
	if len(g.Dirs) != 3 {
		t.Fatalf("hex geometry carries %d dirs, want 3", len(g.Dirs))
	}
	dst := make([]Entry, config.PatternTableEntries)
	for d := range g.Dirs {
		delta := [2]int{g.Dirs[d].Dr, g.Dirs[d].Dc}
		buildDirTable(b, &delta, dst)
		for idx := range config.PatternTableEntries {
			if dst[idx] != table[idx] {
				t.Fatalf("hex dir %d idx %#x: rebuilt %+v differs from the unified table %+v", d, idx, dst[idx], table[idx])
			}
		}
	}
}

func TestHexBuildRealizableCount(t *testing.T) {
	b := hexBoardFor(t)
	g := b.Geom
	delta := [2]int{g.Dirs[0].Dr, g.Dirs[0].Dc}
	dst := make([]Entry, config.PatternTableEntries)
	buildDirTable(b, &delta, dst)
	live := 0
	for idx := range config.PatternTableEntries {
		if flagsScratch[idx]&flagRealizable != 0 {
			live++
		}
	}
	if live != 44272 {
		t.Fatalf("hex build realizable entries = %d, want the geometry-independent 44272", live)
	}
}

func seededHexBoard(t *testing.T, seed uint64) *rules.Board {
	t.Helper()
	b := hexBoardFor(t)
	g := b.Geom
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	for range rng.IntN(160) {
		cell := rules.Cell(rng.IntN(g.Cells))
		if !g.Contains(int(cell)) || b.Occupied(cell) {
			continue
		}
		b.Side = rules.Color(rng.IntN(2))
		b.Make(cell)
	}
	for _, rc := range [][2]int{
		{0, 11}, {22, 11}, {11, 0}, {11, 22},
		{1, 10}, {1, 11}, {1, 12}, {21, 10}, {21, 11}, {21, 12},
		{10, 1}, {12, 1}, {10, 21}, {12, 21},
	} {
		cell := rules.Cell(rc[0]*g.Stride + rc[1])
		if !g.Contains(int(cell)) || b.Occupied(cell) {
			continue
		}
		b.Side = rules.Color(rng.IntN(2))
		b.Make(cell)
	}
	return b
}

func manualGeomWindow(b *rules.Board, cell rules.Cell, dir int, mover rules.Color) [config.PatternWindowLen]uint8 {
	var w [config.PatternWindowLen]uint8
	g := b.Geom
	dr, dc := g.Dirs[dir].Dr, g.Dirs[dir].Dc
	r := int(cell) / g.Stride
	c := int(cell) % g.Stride
	for i := range w {
		rr, cc := r+(int(i)-windowHalf)*dr, c+(int(i)-windowHalf)*dc
		if rr < 0 || rr >= g.Rows || cc < 0 || cc >= g.Cols {
			w[i] = config.PatternStateOff
			continue
		}
		ic := rr*g.Stride + cc
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

func TestHexRimWindows(t *testing.T) {
	seeds := uint64(3)
	if testing.Short() {
		seeds = 1
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		b := seededHexBoard(t, seed)
		g := b.Geom
		for cell := 0; cell < g.Cells; cell++ {
			for d := range g.Dirs {
				for _, mover := range [2]rules.Color{rules.Red, rules.Blue} {
					idx := Index(b, rules.Cell(cell), d, mover)
					want := Pack(manualGeomWindow(b, rules.Cell(cell), d, mover))
					if idx != want {
						t.Fatalf("seed %d cell %d dir %d mover %v: Index = %#x want %#x", seed, cell, d, mover, idx, want)
					}
					w := Unpack(idx)
					if !isRealizable(&w) {
						t.Fatalf("seed %d cell %d dir %d mover %v: window %v not realizable", seed, cell, d, mover, w)
					}
					if g.Contains(cell) && w[windowHalf] == config.PatternStateOff {
						t.Fatalf("seed %d cell %d dir %d: region center reads Off", seed, cell, d)
					}
				}
			}
		}
	}
}

func TestHexRimWindowLookupMatchesNaive(t *testing.T) {
	stride := 7
	if testing.Short() {
		stride = 61
	}
	g := hexGeomFor(t)
	for seed := uint64(1); seed <= 2; seed++ {
		b := seededHexBoard(t, seed)
		n := 0
		for cell := 0; cell < g.Cells; cell += stride {
			if !g.Contains(cell) {
				continue
			}
			for d := range g.Dirs {
				idx := Index(b, rules.Cell(cell), d, rules.Red)
				if got, want := Lookup(idx).Win1, naiveWin1For(t, g, Unpack(idx)); got != want {
					t.Fatalf("seed %d cell %d dir %d: Win1 = %#x want %#x", seed, cell, d, got, want)
				}
				n++
			}
		}
		if n == 0 {
			t.Fatal("no hex windows sampled")
		}
	}
}

func TestHexWitnessesFromOracle(t *testing.T) {
	g := hexGeomFor(t)
	cases := []struct {
		name     string
		witness  string
		class    uint8
		win1Bits []int
	}{
		{"four", "xoooo..xx", config.PatternClassFour, []int{5}},
		{"open four", ".oooo.xxx", config.PatternClassOpenFour, []int{0, 5}},
		{"three", "..ooo..xx", config.PatternClassThree, nil},
		{"broken three", "xooo...xx", config.PatternClassBrokenThree, nil},
		{"open two", "..oo...xx", config.PatternClassOpenTwo, nil},
		{"none", "oxoxoxo..", config.PatternClassNone, nil},
		{"split four", "oo.oo....", config.PatternClassFour, []int{2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := win(t, c.witness)
			if got := naiveClassFor(t, g, w); got != c.class {
				t.Fatalf("hex oracle class = %d want %d", got, c.class)
			}
			var wantMask uint16
			for _, bit := range c.win1Bits {
				wantMask |= 1 << uint(bit)
			}
			if got := naiveWin1For(t, g, w); got != wantMask {
				t.Fatalf("hex oracle Win1 = %#x want %#x", got, wantMask)
			}
			entry := Lookup(Pack(w))
			if entry.Class != c.class || entry.Win1 != wantMask {
				t.Fatalf("table entry %+v want class %d win1 %#x", entry, c.class, wantMask)
			}
		})
	}
}

func TestHexOverlineExclusion(t *testing.T) {
	g := hexGeomFor(t)
	for _, s := range []string{
		".ooooo...",
		"oo.ooo...",
		".ooooox..",
		"xooooox..",
		".oooooo..",
		"  .ooooo.",
	} {
		w := win(t, s)
		entry := Lookup(Pack(w))
		if entry.Win1 != 0 {
			t.Fatalf("%q: Win1 = %#x want 0", s, entry.Win1)
		}
		if got, want := entry.Class, naiveClassFor(t, g, w); got != want {
			t.Fatalf("%q: class = %d want %d", s, got, want)
		}
	}
}

func TestHexIndexPanicsOutOfRange(t *testing.T) {
	b := hexBoardFor(t)
	defer func() {
		if recover() == nil {
			t.Fatal("hex Index on out-of-range cell: want panic")
		}
	}()
	_ = Index(b, rules.Cell(b.Geom.Cells), 0, rules.Red)
}

func TestHexIndexPanicsPastDirTable(t *testing.T) {
	b := hexBoardFor(t)
	defer func() {
		if recover() == nil {
			t.Fatal("hex Index on a dir past the geometry table: want panic")
		}
	}()
	_ = Index(b, rules.Cell(b.Geom.Center), len(b.Geom.Dirs), rules.Red)
}

func TestHexDirBoundInterlock(t *testing.T) {
	g := hexGeomFor(t)
	if len(g.Dirs) > config.PatternDirections {
		t.Fatalf("hex carries %d dirs, above the %d fixed-array bound", len(g.Dirs), config.PatternDirections)
	}
	if len(g.Dirs) != 3 {
		t.Fatalf("hex carries %d dirs, want the 3 line axes", len(g.Dirs))
	}
}

func TestWindowEnumerationPerGeometry(t *testing.T) {
	want := map[string]struct{ enumerated, scored int }{
		"square": {1024, 1024},
		"hex":    {1554, 1191},
	}
	for _, v := range config.Variants {
		t.Run(v.Name, func(t *testing.T) {
			g, err := geom.FromSpec(v)
			if err != nil {
				t.Fatalf("variant %s: %v", v.Name, err)
			}
			region := 0
			for _, w := range g.Region {
				region += bits.OnesCount64(w)
			}
			w2 := want[v.Name]
			if total := len(g.Dirs) * g.Cells; total != w2.enumerated {
				t.Fatalf("%s enumerates %d cell-dir windows, want %d", v.Name, total, w2.enumerated)
			}
			if scored := len(g.Dirs) * region; scored != w2.scored {
				t.Fatalf("%s scores %d region windows, want %d", v.Name, scored, w2.scored)
			}
		})
	}
}
