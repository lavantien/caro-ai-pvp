package geom

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func FuzzParseCell(f *testing.F) {
	for _, s := range []string{"A1", "P16", "H8", "L12", "W1", "A23", "Q1", "a1", "A01", "A17", "X23", "", "A", "A123", "A12", "L23", "M14", "N13", "O15", "A24", "L0", "L24", "W23", "W12", "G33"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
			cell, err := g.ParseCell(name)
			if err != nil {
				continue
			}
			if cell < 0 || cell >= g.Cells || !g.Contains(cell) {
				t.Fatalf("%s ParseCell(%q) = %d, must land in region", g.Name, name, cell)
			}
			back, err := g.CellName(cell)
			if err != nil || back != name {
				t.Fatalf("%s CellName(ParseCell(%q)) = (%q, %v), want %q", g.Name, name, back, err, name)
			}
		}
	})
}

func FuzzFromSpec(f *testing.F) {
	f.Add("square", 16)
	f.Add("hex", 12)
	f.Add("square", 1)
	f.Add("hex", 2)
	f.Add("square", 64)
	f.Add("hex", 31)
	f.Fuzz(func(t *testing.T, kind string, size int) {
		spec := config.VariantSpec{Name: kind, Kind: kind, Size: size % 64}
		g, err := FromSpec(spec)
		if err != nil {
			return
		}
		if g.Words != (g.Cells+63)/64 {
			t.Fatalf("%s(%d): Words %d does not cover Cells %d", kind, spec.Size, g.Words, g.Cells)
		}
		if g.Center < 0 || g.Center >= g.Cells || !g.Contains(g.Center) {
			t.Fatalf("%s(%d): Center %d not a region cell of %d cells", kind, spec.Size, g.Center, g.Cells)
		}
		want := spec.Size * spec.Size
		if g.Kind == KindHex {
			want = 3*spec.Size*(spec.Size-1) + 1
		}
		if n := popcount(g.Region); n != want {
			t.Fatalf("%s(%d): region popcount %d, want %d", kind, spec.Size, n, want)
		}
		if len(g.Dirs) == 0 || len(g.Neighbors) == 0 {
			t.Fatalf("%s(%d): empty dir or neighbor table", kind, spec.Size)
		}
	})
}
