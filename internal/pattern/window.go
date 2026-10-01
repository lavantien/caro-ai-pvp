package pattern

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = config.BoardCells / config.BoardWordsPerColor

const windowHalf = (config.PatternWindowLen - 1) / 2

// Pack packs window states into the table index, position 0 in the two
// lowest bits.
func Pack(w [config.PatternWindowLen]uint8) uint32 {
	var idx uint32
	for i, s := range w {
		idx |= uint32(s&3) << (uint(i) * config.PatternCellBits)
	}
	return idx
}

// Unpack decodes a packed index into window states.
func Unpack(idx uint32) [config.PatternWindowLen]uint8 {
	var w [config.PatternWindowLen]uint8
	for i := range w {
		w[i] = uint8(idx >> (uint(i) * config.PatternCellBits) & 3)
	}
	return w
}

// Index packs the own-relative window through cell for one direction and the
// mover. Off-board and out-of-region cells map to PatternStateOff.
func Index(b *rules.Board, cell rules.Cell, dir int, mover rules.Color) uint32 {
	if uint(cell) >= config.BoardCells {
		panic("pattern: Index cell out of range")
	}
	delta := &config.PatternDirs[dir]
	r := int(cell) / config.BoardStride
	c := int(cell) % config.BoardStride
	own, other := &b.Red, &b.Blue
	if mover == rules.Blue {
		own, other = &b.Blue, &b.Red
	}
	var idx uint32
	for i := range config.PatternWindowLen {
		k := int(i) - windowHalf
		rr := r + k*delta[0]
		cc := c + k*delta[1]
		s := config.PatternStateOff
		if rr >= 0 && rr < config.BoardSize && cc >= 0 && cc < config.BoardSize {
			ic := rr*config.BoardStride + cc
			m := uint64(1) << (uint(ic) % wordBits)
			w := ic / wordBits
			switch {
			case b.Region[w]&m == 0:
				s = config.PatternStateOff
			case own[w]&m != 0:
				s = config.PatternStateOwn
			case other[w]&m != 0:
				s = config.PatternStateOpp
			default:
				s = config.PatternStateEmpty
			}
		}
		idx |= uint32(s) << (uint(i) * config.PatternCellBits)
	}
	return idx
}
