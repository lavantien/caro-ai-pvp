package main

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64
const windowHalf = (config.PatternWindowLen - 1) / 2
const windowCenterShift = config.PatternCellBits * windowHalf

type bb [config.BoardWordsPerColor]uint64

var fileMaskFirst, fileMaskLast uint64

func init() {
	for row := range config.BoardWordsPerColor {
		fileMaskFirst |= 1 << (row * config.BoardStride)
		fileMaskLast |= 1 << (row*config.BoardStride + config.BoardStride - 1)
	}
}

func stonesOf(b *rules.Board, mover rules.Color) bb {
	var out bb
	if mover == rules.Red {
		copy(out[:], b.Red[:config.BoardWordsPerColor])
		return out
	}
	copy(out[:], b.Blue[:config.BoardWordsPerColor])
	return out
}

func cellOfRC(r, c int) rules.Cell { return rules.Cell(r*config.BoardStride + c) }

func fwdClear(dc int) uint64 {
	switch dc {
	case 1:
		return fileMaskFirst
	case -1:
		return fileMaskLast
	}
	return 0
}

func shiftLineBB(s bb, dir int, forward bool) bb {
	dr, dc := config.PatternDirs[dir][0], config.PatternDirs[dir][1]
	step := dr*config.BoardStride + dc
	clear := fwdClear(dc)
	if !forward {
		step, clear = -step, fwdClear(-dc)
	}
	var r bb
	if step >= 0 {
		us := uint(step)
		for w := config.BoardWordsPerColor - 1; w >= 0; w-- {
			r[w] = s[w] << us
			if w > 0 {
				r[w] |= s[w-1] >> (wordBits - us)
			}
			r[w] &^= clear
		}
		return r
	}
	us := uint(-step)
	for w := range s {
		r[w] = s[w] >> us
		if w < config.BoardWordsPerColor-1 {
			r[w] |= s[w+1] << (wordBits - us)
		}
		r[w] &^= clear
	}
	return r
}

func orBB(a, b bb) bb {
	for w := range a {
		a[w] |= b[w]
	}
	return a
}

func dirReachBB(b *rules.Board, mover rules.Color) [config.PatternDirections]bb {
	var out [config.PatternDirections]bb
	for d := range config.PatternDirs {
		up, down := stonesOf(b, mover), stonesOf(b, mover)
		m := up
		for range windowHalf {
			up = shiftLineBB(up, d, true)
			down = shiftLineBB(down, d, false)
			m = orBB(m, orBB(up, down))
		}
		out[d] = m
	}
	return out
}

func dilateBB(s bb, radius int) bb {
	for range radius {
		var vert, out bb
		for w := range s {
			up := s[w] >> config.BoardStride
			if w+1 < len(s) {
				up |= s[w+1] << (wordBits - config.BoardStride)
			}
			down := s[w] << config.BoardStride
			if w > 0 {
				down |= s[w-1] >> (wordBits - config.BoardStride)
			}
			vert[w] = up | down
			out[w] = s[w] | vert[w] | (s[w]>>1)&^fileMaskLast | (s[w]<<1)&^fileMaskFirst |
				(vert[w]>>1)&^fileMaskLast | (vert[w]<<1)&^fileMaskFirst
		}
		s = out
	}
	return s
}

func threatRank(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb) uint8 {
	best := config.PatternClassNone
	for d := range config.PatternDirs {
		w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
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

func threatScore(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb, floor uint8) uint8 {
	rank, dirs := config.PatternClassNone, 0
	for d := range config.PatternDirs {
		w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
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

func freeInRegion(b *rules.Board, cands bb) []rules.Cell {
	var out []rules.Cell
	for w := range cands {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			out = append(out, rules.Cell(w*wordBits+bits.TrailingZeros64(bit)))
		}
	}
	return out
}
