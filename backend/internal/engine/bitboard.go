package engine

import (
	"caro-ai-pvp/internal/domain"
	"math/bits"
)

type BitBoard [4]uint64

func bitIndex(x, y int) (int, uint) {
	idx := y*domain.BoardSize + x
	return idx >> 6, uint(idx & 63)
}

func (b *BitBoard) Set(x, y int) {
	i, off := bitIndex(x, y)
	b[i] |= 1 << off
}

func (b *BitBoard) Clear(x, y int) {
	i, off := bitIndex(x, y)
	b[i] &^= 1 << off
}

func (b BitBoard) Get(x, y int) bool {
	i, off := bitIndex(x, y)
	return b[i]&(1<<off) != 0
}

func (b BitBoard) Or(other BitBoard) BitBoard {
	return BitBoard{b[0] | other[0], b[1] | other[1], b[2] | other[2], b[3] | other[3]}
}

func (b BitBoard) And(other BitBoard) BitBoard {
	return BitBoard{b[0] & other[0], b[1] & other[1], b[2] & other[2], b[3] & other[3]}
}

func (b BitBoard) Xor(other BitBoard) BitBoard {
	return BitBoard{b[0] ^ other[0], b[1] ^ other[1], b[2] ^ other[2], b[3] ^ other[3]}
}

func (b BitBoard) Not() BitBoard {
	return BitBoard{^b[0], ^b[1], ^b[2], ^b[3]}
}

func (b BitBoard) IsZero() bool {
	return b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0
}

func (b BitBoard) Count() int {
	return bits.OnesCount64(b[0]) + bits.OnesCount64(b[1]) +
		bits.OnesCount64(b[2]) + bits.OnesCount64(b[3])
}

func (b BitBoard) Dilate() BitBoard {
	const W = domain.BoardSize
	var result BitBoard

	// Iterate over all 256 bits, for each set bit set all 8 neighbors
	for y := range W {
		for x := range W {
			if !b.Get(x, y) {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < W && ny >= 0 && ny < W {
						result.Set(nx, ny)
					}
				}
			}
		}
	}
	return result
}

func BitBoardsFromDomain(b domain.Board) (red, blue BitBoard) {
	rBits := b.BitBoardBits(domain.PlayerRed)
	bBits := b.BitBoardBits(domain.PlayerBlue)
	copy(red[:], rBits[:])
	copy(blue[:], bBits[:])
	return
}
