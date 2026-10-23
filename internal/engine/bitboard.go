package engine

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64

type bitmask [config.MaxBoardWordsPerColor]uint64

func shiftDelta(s bitmask, dr, dc int, g *geom.Geometry) bitmask {
	step := dr*g.Stride + dc
	var clear *[config.MaxBoardWordsPerColor]uint64
	if dc > 0 {
		clear = &g.Dirs[0].FwdClr
	} else if dc < 0 {
		clear = &g.Dirs[0].BwdClr
	}
	var r bitmask
	if step >= 0 {
		us := uint(step)
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
	us := uint(-step)
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

func dilate1(s bitmask, g *geom.Geometry) bitmask {
	if g.Kind != geom.KindSquare {
		return dilate1Neighbors(s, g)
	}
	words := g.Words
	horiz := &g.Dirs[0]
	var vert bitmask
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
	}
	var out bitmask
	for w := range words {
		h := (s[w]>>1)&^horiz.BwdClr[w] | (s[w]<<1)&^horiz.FwdClr[w]
		d := (vert[w]>>1)&^horiz.BwdClr[w] | (vert[w]<<1)&^horiz.FwdClr[w]
		out[w] = s[w] | vert[w] | h | d
	}
	return out
}

func dilate1Neighbors(s bitmask, g *geom.Geometry) bitmask {
	out := s
	for _, n := range g.Neighbors {
		m := shiftDelta(s, n[0], n[1], g)
		for w := range g.Words {
			out[w] |= m[w]
		}
	}
	return out
}

func dilate(s bitmask, radius int, g *geom.Geometry) bitmask {
	for range radius {
		s = dilate1(s, g)
	}
	return s
}

func openingAnchor(b *rules.Board) (uint16, bool) {
	if b.Side != rules.Red {
		return 0, false
	}
	stones, anchor := 0, 0
	for w := range b.Geom.Words {
		if b.Red[w] != 0 {
			stones += bits.OnesCount64(b.Red[w])
			if stones > 1 {
				return 0, false
			}
			anchor = w*wordBits + bits.TrailingZeros64(b.Red[w])
		}
	}
	return uint16(anchor), stones == 1
}
