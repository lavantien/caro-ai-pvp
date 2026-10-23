package vcf

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64

const (
	windowHalf        = (config.PatternWindowLen - 1) / 2
	windowCenterShift = config.PatternCellBits * windowHalf
)

type bb [config.MaxBoardWordsPerColor]uint64

func stones(b *rules.Board, mover rules.Color) bb {
	if mover == rules.Red {
		return bb(b.Red)
	}
	return bb(b.Blue)
}

func cellOf(r, c int, g *geom.Geometry) rules.Cell { return rules.Cell(r*g.Stride + c) }

func shiftLine(s bb, d *geom.Dir, g *geom.Geometry, forward bool) bb {
	words := g.Words
	var r bb
	if forward {
		for w := words - 1; w >= 0; w-- {
			r[w] = s[w] << d.Step
			if w > 0 {
				r[w] |= s[w-1] >> (wordBits - d.Step)
			}
			r[w] &^= d.FwdClr[w]
		}
		return r
	}
	for w := range words {
		r[w] = s[w] >> d.Step
		if w < words-1 {
			r[w] |= s[w+1] << (wordBits - d.Step)
		}
		r[w] &^= d.BwdClr[w]
	}
	return r
}

func shiftDeltaVCF(s bb, dr, dc int, g *geom.Geometry) bb {
	delta := dr*g.Stride + dc
	var clear *[config.MaxBoardWordsPerColor]uint64
	if dc > 0 {
		clear = &g.Dirs[0].FwdClr
	} else if dc < 0 {
		clear = &g.Dirs[0].BwdClr
	}
	var r bb
	if delta >= 0 {
		us := uint(delta)
		for w := g.Words - 1; w >= 0; w-- {
			r[w] = s[w] << us
			if w > 0 {
				r[w] |= s[w-1] >> (wordBits - us)
			}
			if clear != nil {
				r[w] &^= clear[w]
			}
		}
		return r
	}
	us := uint(-delta)
	for w := range g.Words {
		r[w] = s[w] >> us
		if w < g.Words-1 {
			r[w] |= s[w+1] << (wordBits - us)
		}
		if clear != nil {
			r[w] &^= clear[w]
		}
	}
	return r
}

func orBB(a, b bb, words int) bb {
	for w := range words {
		a[w] |= b[w]
	}
	return a
}

func dirReach(b *rules.Board, mover rules.Color) [config.PatternDirections]bb {
	var out [config.PatternDirections]bb
	g := b.Geom
	for d := range g.Dirs {
		up, down := stones(b, mover), stones(b, mover)
		m := up
		for range windowHalf {
			up = shiftLine(up, &g.Dirs[d], g, true)
			down = shiftLine(down, &g.Dirs[d], g, false)
			m = orBB(m, orBB(up, down, g.Words), g.Words)
		}
		out[d] = m
	}
	return out
}

func dilate(s bb, radius int, g *geom.Geometry) bb {
	for range radius {
		s = dilate1(s, g)
	}
	return s
}

func dilate1(s bb, g *geom.Geometry) bb {
	if g.Kind != geom.KindSquare {
		return dilateNeighbors(s, g)
	}
	words := g.Words
	horiz := &g.Dirs[0]
	var vert, out bb
	for w := range words {
		up := s[w] >> uint(g.Stride)
		if w+1 < words {
			up |= s[w+1] << (wordBits - uint(g.Stride))
		}
		down := s[w] << uint(g.Stride)
		if w > 0 {
			down |= s[w-1] >> (wordBits - uint(g.Stride))
		}
		vert[w] = up | down
		out[w] = s[w] | vert[w] | (s[w]>>1)&^horiz.BwdClr[w] | (s[w]<<1)&^horiz.FwdClr[w] |
			(vert[w]>>1)&^horiz.BwdClr[w] | (vert[w]<<1)&^horiz.FwdClr[w]
	}
	return out
}

func dilateNeighbors(s bb, g *geom.Geometry) bb {
	out := s
	for _, n := range g.Neighbors {
		m := shiftDeltaVCF(s, n[0], n[1], g)
		for w := range g.Words {
			out[w] |= m[w]
		}
	}
	return out
}

func threatRank(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb) uint8 {
	best := config.PatternClassNone
	w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
	for d := range b.Geom.Dirs {
		if reach[d][w]&m == 0 {
			continue
		}
		idx := pattern.Index(b, cell, d, mover)
		cls := pattern.Lookup(idx | uint32(config.PatternStateOwn)<<windowCenterShift).Class
		if cls > best {
			best = cls
		}
	}
	return best
}

func (s *Solver) threatScore(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb) uint8 {
	floor := s.kindFloor()
	rank, dirs := config.PatternClassNone, 0
	w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
	for d := range b.Geom.Dirs {
		if reach[d][w]&m == 0 {
			continue
		}
		idx := pattern.Index(b, cell, d, mover)
		cls := pattern.Lookup(idx | uint32(config.PatternStateOwn)<<windowCenterShift).Class
		if cls > rank {
			rank = cls
		}
		if cls >= floor {
			dirs++
		}
	}
	return rank<<2 | uint8(min(dirs, 3))
}

func (s *Solver) probeWins(b *rules.Board, mover rules.Color, out []rules.Cell) (int, rules.Cell) {
	n := 0
	var first rules.Cell
	g := b.Geom
	cands := dilate(stones(b, mover), config.SolverCandidateRadius, g)
	cur := b.Side
	b.Side = mover
	for w := range g.Words {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := rules.Cell(w*wordBits + bits.TrailingZeros64(bit))
			b.Make(c)
			won := b.FastLastMoveWin(mover, c)
			b.Unmake()
			if won {
				if n == 0 {
					first = c
				}
				if n < len(out) {
					out[n] = c
				}
				n++
			}
		}
	}
	b.Side = cur
	return n, first
}

func cellAt(b *rules.Board, r, c int, color rules.Color) bool {
	g := b.Geom
	if r < 0 || r >= g.Rows || c < 0 || c >= g.Cols {
		return false
	}
	s := &b.Red
	if color == rules.Blue {
		s = &b.Blue
	}
	cell := r*g.Stride + c
	return s[cell/wordBits]&(1<<(uint(cell)%wordBits)) != 0
}

func playable(b *rules.Board, r, c int) bool {
	g := b.Geom
	if r < 0 || r >= g.Rows || c < 0 || c >= g.Cols {
		return false
	}
	cell := r*g.Stride + c
	w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
	return b.Region[w]&m != 0 && b.Full[w]&m == 0
}

func windowCell(cell rules.Cell, g *geom.Geometry, dir, i int) rules.Cell {
	k := i - windowHalf
	dr, dc := g.Dirs[dir].Dr, g.Dirs[dir].Dc
	r := int(cell)/g.Stride + k*dr
	c := int(cell)%g.Stride + k*dc
	return cellOf(r, c, g)
}

type fiveFrame struct{ br, bc, fr, fc int }

func fiveFrames(b *rules.Board, color rules.Color, cell rules.Cell, out []fiveFrame) int {
	g := b.Geom
	r, c := int(cell)/g.Stride, int(cell)%g.Stride
	n := 0
	for d := range g.Dirs {
		dr, dc := g.Dirs[d].Dr, g.Dirs[d].Dc
		back := 0
		xr, xc := r-dr, c-dc
		for back < config.WinLength && cellAt(b, xr, xc, color) {
			back++
			xr -= dr
			xc -= dc
		}
		fwd := 0
		yr, yc := r+dr, c+dc
		for fwd < config.WinLength && cellAt(b, yr, yc, color) {
			fwd++
			yr += dr
			yc += dc
		}
		if back+fwd+1 == config.WinLength {
			if n < len(out) {
				out[n] = fiveFrame{xr, xc, yr, yc}
			}
			n++
		}
	}
	return n
}

func (s *Solver) defusing(b *rules.Board, attacker rules.Color, nA, ply int) int {
	defender := attacker.Opponent()
	n := 0
	g := b.Geom
	var frames [config.PatternDirections]fiveFrame
	for i := 0; i < nA; i++ {
		w := s.winA[i]
		n = addCell(s.defends[ply][:], n, w)
		for j, nf := 0, fiveFrames(b, attacker, w, frames[:]); j < nf; j++ {
			f := frames[j]
			if cellAt(b, f.br, f.bc, defender) && playable(b, f.fr, f.fc) {
				n = addCell(s.defends[ply][:], n, cellOf(f.fr, f.fc, g))
			}
			if cellAt(b, f.fr, f.fc, defender) && playable(b, f.br, f.bc) {
				n = addCell(s.defends[ply][:], n, cellOf(f.br, f.bc, g))
			}
		}
	}
	return n
}

func addCell(buf []rules.Cell, n int, c rules.Cell) int {
	for i := 0; i < n; i++ {
		if buf[i] == c {
			return n
		}
	}
	buf[n] = c
	return n + 1
}

func realThree(b *rules.Board, attacker rules.Color, last rules.Cell) bool {
	g := b.Geom
	for d := range g.Dirs {
		idx := pattern.Index(b, last, d, attacker)
		switch pattern.Lookup(idx).Class {
		case config.PatternClassThree, config.PatternClassBrokenThree:
		default:
			continue
		}
		w := pattern.Unpack(idx)
		for i, st := range w {
			if st != config.PatternStateEmpty {
				continue
			}
			child := pattern.Lookup(idx | uint32(config.PatternStateOwn)<<(uint(i)*config.PatternCellBits)).Win1
			if child == 0 {
				continue
			}
			a := windowCell(last, g, d, i)
			for j := range config.PatternWindowLen {
				if child&(uint16(1)<<uint(j)) == 0 {
					continue
				}
				v := windowCell(last, g, d, j)
				cur := b.Side
				b.Side = attacker
				b.Make(a)
				b.Side = attacker
				b.Make(v)
				won := b.FastLastMoveWin(attacker, v)
				b.Unmake()
				b.Unmake()
				b.Side = cur
				if won {
					return true
				}
			}
		}
	}
	return false
}
