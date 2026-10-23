package rules

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

func shiftFwdInto(dst *bb, d *geom.Dir, words int) {
	for w := words - 1; w >= 0; w-- {
		var carry uint64
		if w > 0 {
			carry = dst[w-1] >> (wordBits - d.Step)
		}
		dst[w] = dst[w]<<d.Step | carry
		dst[w] &^= d.FwdClr[w]
	}
}

func shiftBwdInto(dst *bb, d *geom.Dir, words int) {
	for w := range words {
		var carry uint64
		if w < words-1 {
			carry = dst[w+1] << (wordBits - d.Step)
		}
		dst[w] = dst[w]>>d.Step | carry
		dst[w] &^= d.BwdClr[w]
	}
}

func andInto(a, b *bb, words int) {
	for w := range words {
		a[w] &= b[w]
	}
}

func orInto(a, b *bb, words int) {
	for w := range words {
		a[w] |= b[w]
	}
}

func andNotInto(a, b *bb, words int) {
	for w := range words {
		a[w] &^= b[w]
	}
}

func nonzeroBB(a *bb, words int) bool {
	for w := range words {
		if a[w] != 0 {
			return true
		}
	}
	return false
}

func winsDir(s, o *bb, d *geom.Dir, words int) bool {
	five, cur := *s, *s
	for range config.WinLength - 1 {
		shiftBwdInto(&cur, d, words)
		andInto(&five, &cur, words)
	}
	if !nonzeroBB(&five, words) {
		return false
	}
	after := cur
	shiftBwdInto(&after, d, words)
	before := *s
	shiftFwdInto(&before, d, words)
	orInto(&after, &before, words)
	cand := five
	andNotInto(&cand, &after, words)
	if !nonzeroBB(&cand, words) {
		return false
	}
	oppShift := *o
	for range config.WinLength {
		shiftBwdInto(&oppShift, d, words)
	}
	blocked := *o
	shiftFwdInto(&blocked, d, words)
	andInto(&blocked, &oppShift, words)
	andNotInto(&cand, &blocked, words)
	return nonzeroBB(&cand, words)
}

func (b *Board) stonesPtr(color Color) *bb {
	if color == Red {
		return &b.Red
	}
	return &b.Blue
}

func (b *Board) Wins(color Color) bool {
	s, o := b.stonesPtr(color), b.stonesPtr(color.Opponent())
	g := b.Geom
	for i := range g.Dirs {
		if winsDir(s, o, &g.Dirs[i], g.Words) {
			return true
		}
	}
	return false
}

func (b *Board) FastLastMoveWin(color Color, cell Cell) bool {
	s, o := b.stonesPtr(color), b.stonesPtr(color.Opponent())
	g := b.Geom
	r := int(cell) / g.Stride
	c := int(cell) % g.Stride
	if !b.bitAt(s, r, c) {
		return false
	}
	for i := range g.Dirs {
		dr, dc := g.Dirs[i].Dr, g.Dirs[i].Dc
		back := 0
		br, bc := r-dr, c-dc
		for k := 0; k < config.WinLength && inGeom(g, br, bc) && b.bitAt(s, br, bc); k++ {
			back++
			br -= dr
			bc -= dc
		}
		fwd := 0
		fr, fc := r+dr, c+dc
		for k := 0; k < config.WinLength && inGeom(g, fr, fc) && b.bitAt(s, fr, fc); k++ {
			fwd++
			fr += dr
			fc += dc
		}
		if back+fwd+1 != config.WinLength {
			continue
		}
		if inGeom(g, br, bc) && b.bitAt(o, br, bc) && inGeom(g, fr, fc) && b.bitAt(o, fr, fc) {
			continue
		}
		return true
	}
	return false
}

func inGeom(g *geom.Geometry, r, c int) bool {
	return r >= 0 && r < g.Rows && c >= 0 && c < g.Cols
}

func (b *Board) bitAt(s *bb, r, c int) bool {
	w, m := bitOf(Cell(r*b.Geom.Stride + c))
	return s[w]&m != 0
}
