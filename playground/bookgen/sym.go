package main

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type cellTransform func(r, c int) (int, int)

var dihedral = [8]cellTransform{
	func(r, c int) (int, int) { return r, c },
	func(r, c int) (int, int) { return c, config.BoardStride - 1 - r },
	func(r, c int) (int, int) { return config.BoardStride - 1 - r, config.BoardStride - 1 - c },
	func(r, c int) (int, int) { return config.BoardStride - 1 - c, r },
	func(r, c int) (int, int) { return r, config.BoardStride - 1 - c },
	func(r, c int) (int, int) { return config.BoardStride - 1 - r, c },
	func(r, c int) (int, int) { return c, r },
	func(r, c int) (int, int) { return config.BoardStride - 1 - c, config.BoardStride - 1 - r },
}

const dihedralOrder = len(dihedral)

func transformCell(t int, cell int) int {
	r, c := cell/config.BoardStride, cell%config.BoardStride
	nr, nc := dihedral[t](r, c)
	return nr*config.BoardStride + nc
}

func transformMove(t int, mv rules.Move) rules.Move {
	return rules.Move(transformCell(t, int(mv)))
}

func transformMask(t int, words [config.BoardWordsPerColor]uint64) [config.BoardWordsPerColor]uint64 {
	var out [config.BoardWordsPerColor]uint64
	for w, mask := range words {
		rest := mask
		for rest != 0 {
			bit := rest & (-rest)
			rest ^= bit
			cell := w*64 + bits.TrailingZeros64(bit)
			nc := transformCell(t, cell)
			out[nc/64] |= 1 << uint(nc%64)
		}
	}
	return out
}

func transformKey(t int, k [config.BookKeyBytes]byte) [config.BookKeyBytes]byte {
	var out [config.BookKeyBytes]byte
	for color := 0; color < 2; color++ {
		var words [config.BoardWordsPerColor]uint64
		for w := range words {
			words[w] = binary.LittleEndian.Uint64(k[(color*config.BoardWordsPerColor+w)*8:])
		}
		tw := transformMask(t, words)
		for w := range tw {
			binary.LittleEndian.PutUint64(out[(color*config.BoardWordsPerColor+w)*8:], tw[w])
		}
	}
	return out
}

func keyBefore(a, b [config.BookKeyBytes]byte) bool {
	return bytes.Compare(a[:], b[:]) < 0
}

func canonicalKey(k [config.BookKeyBytes]byte) [config.BookKeyBytes]byte {
	min := k
	for t := 1; t < dihedralOrder; t++ {
		if c := transformKey(t, k); keyBefore(c, min) {
			min = c
		}
	}
	return min
}

func canonicalBoardKey(b *rules.Board) [config.BookKeyBytes]byte {
	return canonicalKey(keyOf(b))
}

func cellOrbit(cell int) []int {
	seen := make(map[int]struct{}, dihedralOrder)
	out := make([]int, 0, dihedralOrder)
	for t := range dihedral {
		c := transformCell(t, cell)
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

func sameOrbit(a, b int) bool {
	for _, c := range cellOrbit(a) {
		if c == b {
			return true
		}
	}
	return false
}

func keyVariants(k [config.BookKeyBytes]byte) [][config.BookKeyBytes]byte {
	seen := make(map[[config.BookKeyBytes]byte]struct{}, dihedralOrder)
	out := make([][config.BookKeyBytes]byte, 0, dihedralOrder)
	for t := range dihedral {
		v := transformKey(t, k)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return keyBefore(out[i], out[j]) })
	return out
}
