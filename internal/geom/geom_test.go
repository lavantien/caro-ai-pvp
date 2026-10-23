package geom

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func popcount(m [config.MaxBoardWordsPerColor]uint64) int {
	n := 0
	for _, w := range m {
		for x := w; x != 0; x &= x - 1 {
			n++
		}
	}
	return n
}

func regionCells(g *Geometry) []int {
	var out []int
	for cell := range g.Cells {
		if g.Contains(cell) {
			out = append(out, cell)
		}
	}
	return out
}

func TestSquareSpecMatchesConfig(t *testing.T) {
	g := Square(config.BoardSize)
	if g.Name != "square" || g.Kind != KindSquare {
		t.Fatalf("square identity: %q %d", g.Name, g.Kind)
	}
	if g.Stride != config.BoardStride || g.Rows != config.BoardSize || g.Cols != config.BoardSize {
		t.Fatalf("square shape: stride %d rows %d cols %d", g.Stride, g.Rows, g.Cols)
	}
	if g.Cells != config.BoardCells {
		t.Fatalf("square cells = %d, want %d", g.Cells, config.BoardCells)
	}
	if g.Words != config.BoardWordsPerColor {
		t.Fatalf("square words = %d, want %d", g.Words, config.BoardWordsPerColor)
	}
	if n := popcount(g.Region); n != config.BoardCells {
		t.Fatalf("square region popcount = %d, want %d", n, config.BoardCells)
	}
	if g.Center != config.SearchEmptyBoardCell {
		t.Fatalf("square center = %d, want %d", g.Center, config.SearchEmptyBoardCell)
	}
	if g.OpeningMin != config.OpeningChebyshevMin {
		t.Fatalf("square opening min = %d, want %d", g.OpeningMin, config.OpeningChebyshevMin)
	}
	if len(g.Dirs) != len(config.PatternDirs) {
		t.Fatalf("square dirs = %d, want %d", len(g.Dirs), len(config.PatternDirs))
	}
	for i := range g.Dirs {
		if g.Dirs[i].Dr != config.PatternDirs[i][0] || g.Dirs[i].Dc != config.PatternDirs[i][1] {
			t.Fatalf("square dir %d = (%d,%d), want %v", i, g.Dirs[i].Dr, g.Dirs[i].Dc, config.PatternDirs[i])
		}
	}
	if len(g.Neighbors) != 8 {
		t.Fatalf("square neighbors = %d, want 8", len(g.Neighbors))
	}
}

func TestHexSpec(t *testing.T) {
	g := Hex(12)
	if g.Name != "hex" || g.Kind != KindHex {
		t.Fatalf("hex identity: %q %d", g.Name, g.Kind)
	}
	if g.Stride != 23 || g.Rows != 23 || g.Cols != 23 {
		t.Fatalf("hex shape: stride %d rows %d cols %d", g.Stride, g.Rows, g.Cols)
	}
	if g.Cells != config.MaxBoardCells {
		t.Fatalf("hex cells = %d, want %d", g.Cells, config.MaxBoardCells)
	}
	if g.Words != config.MaxBoardWordsPerColor {
		t.Fatalf("hex words = %d, want %d", g.Words, config.MaxBoardWordsPerColor)
	}
	if n := popcount(g.Region); n != 397 {
		t.Fatalf("hex region popcount = %d, want 397", n)
	}
	if g.Center != 264 {
		t.Fatalf("hex center = %d, want 264", g.Center)
	}
	if !g.Contains(517) {
		t.Fatal("hex max in-region cell 517 must be playable")
	}
	if g.Contains(518) || g.Contains(528) {
		t.Fatal("cells at or past the capacity bound must not be playable")
	}
	if len(g.Dirs) != 3 {
		t.Fatalf("hex dirs = %d, want 3", len(g.Dirs))
	}
	wantSteps := [3]uint{1, 23, 22}
	for i, d := range g.Dirs {
		if d.Step != wantSteps[i] {
			t.Fatalf("hex dir %d step = %d, want %d", i, d.Step, wantSteps[i])
		}
	}
	if len(g.Neighbors) != 6 {
		t.Fatalf("hex neighbors = %d, want 6", len(g.Neighbors))
	}
}

func TestHexRegionPopcountFormula(t *testing.T) {
	for _, s := range []int{2, 3, 5, 12} {
		want := 3*s*(s-1) + 1
		if got := popcount(Hex(s).Region); got != want {
			t.Fatalf("hex(%d) region popcount = %d, want %d", s, got, want)
		}
	}
}

func TestHexLongestLineSpansSideDoubling(t *testing.T) {
	g := Hex(12)
	step := int(g.Dirs[1].Step)
	length := 1
	for cell := g.Center; g.Contains(cell + step); cell += step {
		length++
	}
	for cell := g.Center; g.Contains(cell - step); cell -= step {
		length++
	}
	if length != 23 {
		t.Fatalf("hex longest line through the center = %d, want 23", length)
	}
}

func TestDirMasksMatchIndependentDerivation(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		for i, d := range g.Dirs {
			if want := uint(d.Dr*g.Stride + d.Dc); d.Step != want {
				t.Fatalf("%s dir %d step = %d, want %d", g.Name, i, d.Step, want)
			}
			if d.Dc == 0 {
				if d.FwdClr != [config.MaxBoardWordsPerColor]uint64{} || d.BwdClr != [config.MaxBoardWordsPerColor]uint64{} {
					t.Fatalf("%s dir %d with dc 0 must carry no clear masks", g.Name, i)
				}
				continue
			}
			fwdMod, bwdMod := 0, g.Stride-1
			if d.Dc < 0 {
				fwdMod, bwdMod = g.Stride-1, 0
			}
			var fwdWant, bwdWant [config.MaxBoardWordsPerColor]uint64
			for flat := range g.Cells {
				w, b := flat/64, uint(flat%64)
				if flat%g.Stride == fwdMod {
					fwdWant[w] |= 1 << b
				}
				if flat%g.Stride == bwdMod {
					bwdWant[w] |= 1 << b
				}
			}
			if d.FwdClr != fwdWant {
				t.Fatalf("%s dir %d fwd clear mask = %x, want %x", g.Name, i, d.FwdClr, fwdWant)
			}
			if d.BwdClr != bwdWant {
				t.Fatalf("%s dir %d bwd clear mask = %x, want %x", g.Name, i, d.BwdClr, bwdWant)
			}
		}
	}
}

func TestParseCellRoundTrip(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		cells := regionCells(g)
		if len(cells) == 0 {
			t.Fatalf("%s: no region cells", g.Name)
		}
		for _, cell := range cells {
			name, err := g.CellName(cell)
			if err != nil {
				t.Fatalf("%s CellName(%d): %v", g.Name, cell, err)
			}
			back, err := g.ParseCell(name)
			if err != nil {
				t.Fatalf("%s ParseCell(%q): %v", g.Name, name, err)
			}
			if back != cell {
				t.Fatalf("%s round trip %q: got %d, want %d", g.Name, name, back, cell)
			}
		}
	}
}

func TestParseCellKnown(t *testing.T) {
	s := Square(config.BoardSize)
	for _, tc := range []struct {
		name string
		cell int
	}{{"A1", 0}, {"P1", 15}, {"A2", 16}, {"H8", 119}, {"P16", 255}} {
		cell, err := s.ParseCell(tc.name)
		if err != nil || cell != tc.cell {
			t.Fatalf("square ParseCell(%q) = (%d, %v), want (%d, nil)", tc.name, cell, err, tc.cell)
		}
	}
	h := Hex(12)
	for _, tc := range []struct {
		name string
		cell int
	}{{"A12", 253}, {"L12", 264}, {"L23", 517}, {"A23", 506}, {"M1", 12}, {"W1", 22}, {"L1", 11}} {
		cell, err := h.ParseCell(tc.name)
		if err != nil || cell != tc.cell {
			t.Fatalf("hex ParseCell(%q) = (%d, %v), want (%d, nil)", tc.name, cell, err, tc.cell)
		}
	}
}

func TestParseCellInvalid(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		for _, name := range []string{"", "A", "A123", "A16x", "a1", "@1", "A1b", "A-1", "A 1", "A0", "A00", "A01"} {
			if cell, err := g.ParseCell(name); err == nil {
				t.Fatalf("%s ParseCell(%q) = %d, want error", g.Name, name, cell)
			}
		}
	}
	s := Square(config.BoardSize)
	for _, name := range []string{"Q1", "Z16", "A17", "A99"} {
		if _, err := s.ParseCell(name); err == nil {
			t.Fatalf("square ParseCell(%q): want error", name)
		}
	}
	h := Hex(12)
	for _, name := range []string{"X1", "A24", "L24", "W23", "M23", "A1", "W12x"} {
		if _, err := h.ParseCell(name); err == nil {
			t.Fatalf("hex ParseCell(%q): want error", name)
		}
	}
}

func TestCellNameInvalid(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		for _, cell := range []int{-1, g.Cells, g.Cells + 1, 1 << 20} {
			if name, err := g.CellName(cell); err == nil {
				t.Fatalf("%s CellName(%d) = %q, want error", g.Name, cell, name)
			}
		}
	}
	if _, err := Hex(12).CellName(0); err == nil {
		t.Fatal("hex CellName(0) is off region, want error")
	}
}

func TestAppendNameMatchesCellName(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		for _, cell := range regionCells(g) {
			want, err := g.CellName(cell)
			if err != nil {
				t.Fatalf("%s CellName(%d): %v", g.Name, cell, err)
			}
			if got := string(g.AppendName(nil, cell)); got != want {
				t.Fatalf("%s AppendName(%d) = %q, want %q", g.Name, cell, got, want)
			}
		}
	}
}

func TestDistance(t *testing.T) {
	s := Square(config.BoardSize)
	for _, tc := range []struct {
		a, b, want int
	}{{0, 0, 0}, {0, 1, 1}, {0, 16, 1}, {0, 17, 1}, {0, 32, 2}, {3, 53, 3}} {
		if got := s.Distance(tc.a, tc.b); got != tc.want {
			t.Fatalf("square Distance(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	h := Hex(12)
	at := func(q, r int) int { return q*h.Stride + r }
	for _, tc := range []struct {
		a, b, want int
	}{
		{264, 264, 0},
		{264, at(11, 12), 1},
		{264, at(12, 11), 1},
		{264, at(12, 10), 1},
		{264, at(11, 10), 1},
		{264, at(10, 12), 1},
		{264, at(22, 11), 11},
		{at(0, 11), at(22, 11), 22},
		{at(0, 11), at(0, 22), 11},
		{at(11, 0), at(11, 22), 22},
		{at(22, 0), at(0, 22), 22},
	} {
		if got := h.Distance(tc.a, tc.b); got != tc.want {
			t.Fatalf("hex Distance(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNeighborClosure(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		for _, cell := range regionCells(g) {
			q, r := cell/g.Stride, cell%g.Stride
			for _, n := range g.Neighbors {
				qq, rr := q+n[0], r+n[1]
				if qq < -1 || qq > g.Rows || rr < -1 || rr > g.Cols {
					t.Fatalf("%s neighbor (%+d,%+d) of (%d,%d) steps further than one cell outside", g.Name, n[0], n[1], q, r)
				}
				if qq < 0 || qq >= g.Rows || rr < 0 || rr >= g.Cols {
					continue
				}
				if !g.Contains(qq*g.Stride + rr) {
					continue
				}
				if g.Distance(cell, qq*g.Stride+rr) != 1 {
					t.Fatalf("%s in-region neighbor (%d,%d) of (%d,%d) is not at distance 1", g.Name, qq, rr, q, r)
				}
				back := false
				for _, m := range g.Neighbors {
					if qq+m[0] == q && rr+m[1] == r {
						back = true
						break
					}
				}
				if !back {
					t.Fatalf("%s neighbor relation not symmetric at (%d,%d)", g.Name, q, r)
				}
			}
		}
	}
}

func TestHexCornerNeighborCounts(t *testing.T) {
	h := Hex(12)
	for _, tc := range []struct {
		q, r, want int
	}{{0, 11, 3}, {22, 11, 3}, {11, 0, 3}, {11, 22, 3}, {0, 22, 3}, {22, 0, 3}, {11, 11, 6}} {
		n := 0
		for _, d := range h.Neighbors {
			qq, rr := tc.q+d[0], tc.r+d[1]
			if qq < 0 || qq >= h.Rows || rr < 0 || rr >= h.Cols {
				continue
			}
			if h.Contains(qq*h.Stride + rr) {
				n++
			}
		}
		if n != tc.want {
			t.Fatalf("hex neighbor count at (%d,%d) = %d, want %d", tc.q, tc.r, n, tc.want)
		}
	}
}

func TestVariantConstructors(t *testing.T) {
	d := Default()
	if d.Name != config.DefaultVariant || d.Kind != KindSquare || d.Stride != config.BoardStride {
		t.Fatalf("Default = %q stride %d, want the square spec", d.Name, d.Stride)
	}
	h, err := ByName("hex")
	if err != nil || h.Cells != config.MaxBoardCells {
		t.Fatalf("ByName(hex) = (%d cells, %v)", h.Cells, err)
	}
	if _, err := ByName("tri"); err == nil {
		t.Fatal("ByName(tri): want error")
	}
	if _, err := FromSpec(config.VariantSpec{Name: "x", Kind: "triangle", Size: 9}); err == nil {
		t.Fatal("FromSpec(triangle): want error")
	}
	if _, err := FromSpec(config.VariantSpec{Name: "x", Kind: "square", Size: 0}); err == nil {
		t.Fatal("FromSpec(square 0): want error")
	}
	if _, err := FromSpec(config.VariantSpec{Name: "x", Kind: "hex", Size: 1}); err == nil {
		t.Fatal("FromSpec(hex 1): want error")
	}
}

func TestVariantRegistryLockstep(t *testing.T) {
	if len(config.Variants) != 2 {
		t.Fatalf("Variants = %d entries, want 2", len(config.Variants))
	}
	sq := config.Variants[0]
	if sq.Name != "square" || sq.Kind != "square" || sq.Size*sq.Size != config.BoardCells {
		t.Fatalf("square variant %+v does not derive the pinned board", sq)
	}
	hx := config.Variants[1]
	wantCells := (2*hx.Size-2)*(2*hx.Size-1) + hx.Size
	if hx.Name != "hex" || hx.Kind != "hex" || wantCells != config.MaxBoardCells {
		t.Fatalf("hex variant %+v does not derive MaxBoardCells %d", hx, config.MaxBoardCells)
	}
	if wantWords := (config.MaxBoardCells + 63) / 64; wantWords != config.MaxBoardWordsPerColor {
		t.Fatalf("MaxBoardWordsPerColor = %d, want %d", config.MaxBoardWordsPerColor, wantWords)
	}
}

func TestContainsBounds(t *testing.T) {
	for _, g := range []*Geometry{Square(config.BoardSize), Hex(12)} {
		if g.Contains(-1) || g.Contains(g.Cells) {
			t.Fatalf("%s Contains bounds broken", g.Name)
		}
		for _, cell := range regionCells(g) {
			if !g.Contains(cell) {
				t.Fatalf("%s Contains(%d) false for region cell", g.Name, cell)
			}
		}
	}
}

func TestDistanceSymmetry(t *testing.T) {
	h := Hex(12)
	cells := regionCells(h)
	for _, a := range cells {
		for _, b := range cells {
			if h.Distance(a, b) != h.Distance(b, a) {
				t.Fatalf("hex Distance asymmetric for %d %d", a, b)
			}
		}
	}
}
