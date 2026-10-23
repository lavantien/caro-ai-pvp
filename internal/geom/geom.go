package geom

import (
	"fmt"
	"strconv"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Kind uint8

const (
	KindSquare Kind = iota
	KindHex
)

type Dir struct {
	Dr, Dc int
	Step   uint
	FwdClr [config.MaxBoardWordsPerColor]uint64
	BwdClr [config.MaxBoardWordsPerColor]uint64
}

type Geometry struct {
	Name       string
	Kind       Kind
	Stride     int
	Rows       int
	Cols       int
	Cells      int
	Words      int
	Region     [config.MaxBoardWordsPerColor]uint64
	Dirs       []Dir
	Neighbors  [][2]int
	Col        []uint8
	Center     int
	OpeningMin int
}

type NameError struct {
	Text string
}

func (e *NameError) Error() string { return "geom: " + e.Text }

func wordCap(cells int) int { return (cells + 63) / 64 }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (g *Geometry) setRegion(cell int) {
	g.Region[cell/64] |= 1 << uint(cell%64)
}

func (g *Geometry) Contains(cell int) bool {
	return cell >= 0 && cell < g.Cells && g.Region[cell/64]&(1<<uint(cell%64)) != 0
}

func clearMasks(stride, cells, mod int) [config.MaxBoardWordsPerColor]uint64 {
	var m [config.MaxBoardWordsPerColor]uint64
	for flat := range cells {
		if flat%stride == mod {
			m[flat/64] |= 1 << uint(flat%64)
		}
	}
	return m
}

func newDir(dr, dc, stride, cells int) Dir {
	d := Dir{Dr: dr, Dc: dc, Step: uint(dr*stride + dc)}
	if dc == 0 {
		return d
	}
	first := clearMasks(stride, cells, 0)
	last := clearMasks(stride, cells, stride-1)
	if dc > 0 {
		d.FwdClr, d.BwdClr = first, last
	} else {
		d.FwdClr, d.BwdClr = last, first
	}
	return d
}

func Square(size int) *Geometry {
	cells := size * size
	g := &Geometry{
		Name:       "square",
		Kind:       KindSquare,
		Stride:     size,
		Rows:       size,
		Cols:       size,
		Cells:      cells,
		Words:      wordCap(cells),
		Center:     (size-1)/2*size + (size-1)/2,
		OpeningMin: config.OpeningChebyshevMin,
	}
	g.Col = make([]uint8, cells)
	for r := range size {
		for c := range size {
			g.setRegion(r*size + c)
			g.Col[r*size+c] = uint8(c)
		}
	}
	for _, d := range [4][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}} {
		g.Dirs = append(g.Dirs, newDir(d[0], d[1], size, cells))
	}
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr != 0 || dc != 0 {
				g.Neighbors = append(g.Neighbors, [2]int{dr, dc})
			}
		}
	}
	return g
}

func Hex(side int) *Geometry {
	w := 2*side - 1
	c := side - 1
	cells := (w-1)*w + c + 1
	g := &Geometry{
		Name:       "hex",
		Kind:       KindHex,
		Stride:     w,
		Rows:       w,
		Cols:       w,
		Cells:      cells,
		Words:      wordCap(cells),
		Center:     c*w + c,
		OpeningMin: config.OpeningChebyshevMin,
	}
	g.Col = make([]uint8, w*w)
	for q := range w {
		for r := range w {
			if abs(q-c) <= c && abs(r-c) <= c && abs(q+r-2*c) <= c {
				g.setRegion(q*w + r)
			}
			g.Col[q*w+r] = uint8(r)
		}
	}
	for _, d := range [3][2]int{{0, 1}, {1, 0}, {1, -1}} {
		g.Dirs = append(g.Dirs, newDir(d[0], d[1], w, cells))
	}
	for _, n := range [6][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, -1}, {-1, 1}} {
		g.Neighbors = append(g.Neighbors, n)
	}
	return g
}

func FromSpec(spec config.VariantSpec) (*Geometry, error) {
	switch spec.Kind {
	case "square":
		if spec.Size < 1 {
			return nil, fmt.Errorf("geom: square size %d, want at least 1", spec.Size)
		}
		if spec.Size*spec.Size > config.MaxBoardCells {
			return nil, fmt.Errorf("geom: square size %d exceeds the %d cell capacity", spec.Size, config.MaxBoardCells)
		}
		return Square(spec.Size), nil
	case "hex":
		if spec.Size < 2 {
			return nil, fmt.Errorf("geom: hex side %d, want at least 2", spec.Size)
		}
		w := 2*spec.Size - 1
		if cells := (w-1)*w + spec.Size; cells > config.MaxBoardCells {
			return nil, fmt.Errorf("geom: hex side %d exceeds the %d cell capacity", spec.Size, config.MaxBoardCells)
		}
		return Hex(spec.Size), nil
	}
	return nil, fmt.Errorf("geom: unknown kind %q", spec.Kind)
}

func ByName(name string) (*Geometry, error) {
	for i := range config.Variants {
		if config.Variants[i].Name == name {
			return FromSpec(config.Variants[i])
		}
	}
	return nil, fmt.Errorf("geom: unknown variant %q", name)
}

func Default() *Geometry { return defaultGeometry }

var defaultGeometry = mustDefault()

func mustDefault() *Geometry {
	g, err := ByName(config.DefaultVariant)
	if err != nil {
		panic(err)
	}
	return g
}

func (g *Geometry) Distance(a, b int) int {
	if g.Kind == KindHex {
		dq := a/g.Stride - b/g.Stride
		dr := a%g.Stride - b%g.Stride
		return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
	}
	dr := abs(a/g.Stride - b/g.Stride)
	dc := abs(a%g.Stride - b%g.Stride)
	if dr < dc {
		return dc
	}
	return dr
}

func (g *Geometry) CellName(cell int) (string, error) {
	if cell < 0 || cell >= g.Cells {
		return "", &NameError{Text: fmt.Sprintf("cell %d out of 0..%d", cell, g.Cells-1)}
	}
	if !g.Contains(cell) {
		return "", &NameError{Text: fmt.Sprintf("cell %d out of region", cell)}
	}
	return string(g.AppendName(nil, cell)), nil
}

func (g *Geometry) AppendName(dst []byte, cell int) []byte {
	if cell >= 0 && cell < len(g.Col) {
		dst = append(dst, 'A'+g.Col[cell])
		return strconv.AppendInt(dst, int64(cell/g.Stride)+1, 10)
	}
	dst = append(dst, byte('A'+cell%g.Stride))
	return strconv.AppendInt(dst, int64(cell/g.Stride)+1, 10)
}

func (g *Geometry) ParseCell(name string) (int, error) {
	bad := func(text string) (int, error) {
		return 0, &NameError{Text: fmt.Sprintf("cell name %q: %s", name, text)}
	}
	if n := len(name); n < 2 || n > 3 {
		return bad("want 2 or 3 characters")
	}
	col := int(name[0]) - 'A'
	if col < 0 || col >= g.Cols {
		return bad(fmt.Sprintf("column out of A..%c", rune('A'+g.Cols-1)))
	}
	row := 0
	for _, ch := range name[1:] {
		if ch < '0' || ch > '9' {
			return bad(fmt.Sprintf("stray character %q", ch))
		}
		row = row*10 + int(ch-'0')
	}
	if len(name) == 3 && name[1] == '0' {
		return bad("leading zero")
	}
	if row < 1 || row > g.Rows {
		return bad(fmt.Sprintf("row out of 1..%d", g.Rows))
	}
	cell := (row-1)*g.Stride + col
	if !g.Contains(cell) {
		return bad("cell out of region")
	}
	return cell, nil
}
