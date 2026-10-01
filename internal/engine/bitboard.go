package engine

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64

type bitmask [config.BoardWordsPerColor]uint64

var (
	fileMaskFirst uint64
	fileMaskLast  uint64
)

func init() {
	for row := range config.BoardWordsPerColor {
		fileMaskFirst |= 1 << (row * config.BoardStride)
		fileMaskLast |= 1 << (row*config.BoardStride + config.BoardStride - 1)
	}
}

// dilate1 grows a stone set by one Chebyshev ring. Vertical shifts move by
// BoardStride inside each word with cross-word carries, horizontal shifts
// clear the wrapped file bits so columns never leak into neighbors.
func dilate1(s bitmask) bitmask {
	var vert bitmask
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
	}
	var horiz, diag, out bitmask
	for w := range s {
		horiz[w] = (s[w]>>1)&^fileMaskLast | (s[w]<<1)&^fileMaskFirst
		diag[w] = (vert[w]>>1)&^fileMaskLast | (vert[w]<<1)&^fileMaskFirst
		out[w] = s[w] | vert[w] | horiz[w] | diag[w]
	}
	return out
}

// dilate grows a stone set by radius Chebyshev rings; Chebyshev dilation
// composes, so radius r is r single steps.
func dilate(s bitmask, radius int) bitmask {
	for range radius {
		s = dilate1(s)
	}
	return s
}

func chebyshevCell(a, b uint16) int {
	dr := int(a/config.BoardStride) - int(b/config.BoardStride)
	dc := int(a%config.BoardStride) - int(b%config.BoardStride)
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}
	if dr < dc {
		return dc
	}
	return dr
}

// openingAnchor reports red's first stone when the opening rule constrains
// red's second move: red to move with exactly one red stone on the board.
func openingAnchor(b *rules.Board) (uint16, bool) {
	if b.Side != rules.Red {
		return 0, false
	}
	stones, anchor := 0, 0
	for w := range b.Red {
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
