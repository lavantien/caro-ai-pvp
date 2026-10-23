package pattern

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64

const windowHalf = (config.PatternWindowLen - 1) / 2

func Pack(w [config.PatternWindowLen]uint8) uint32 {
	var idx uint32
	for i, s := range w {
		idx |= uint32(s&3) << (uint(i) * config.PatternCellBits)
	}
	return idx
}

func Unpack(idx uint32) [config.PatternWindowLen]uint8 {
	var w [config.PatternWindowLen]uint8
	for i := range w {
		w[i] = uint8(idx >> (uint(i) * config.PatternCellBits) & 3)
	}
	return w
}

func Index(b *rules.Board, cell rules.Cell, dir int, mover rules.Color) uint32 {
	if uint(cell) >= uint(b.Geom.Cells) {
		panic("pattern: Index cell out of range")
	}
	g := b.Geom
	d := &g.Dirs[dir]
	c0 := int(g.Col[cell])
	own, other := &b.Red, &b.Blue
	if mover == rules.Blue {
		own, other = &b.Blue, &b.Red
	}
	var idx uint32
	for i := range config.PatternWindowLen {
		k := int(i) - windowHalf
		cc := c0 + k*d.Dc
		s := config.PatternStateOff
		if cc >= 0 && cc < g.Cols {
			ic := int(cell) + k*int(d.Step)
			if ic >= 0 && ic < g.Rows*g.Stride {
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
		}
		idx |= uint32(s) << (uint(i) * config.PatternCellBits)
	}
	return idx
}
