package rules

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Color uint8

const (
	Red Color = iota
	Blue
	Empty
)

const colorCount = 2

func (c Color) Opponent() Color { return c ^ 1 }

var lineDirs = [4][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

func inBoard(r, c int) bool {
	return r >= 0 && r < config.BoardSize && c >= 0 && c < config.BoardSize
}

type Move uint16

const wordBits = 64

type bb [config.BoardWordsPerColor]uint64

type undoEntry struct {
	cell  Cell
	color Color
	hash  uint64
}

type Board struct {
	Red       bb
	Blue      bb
	Side      Color
	MoveCount int
	Hash      uint64
	Region    bb
	Full      bb
	undo      [config.BoardCells]undoEntry
}

func regionBB(rows, cols int) bb {
	var r bb
	for row := range rows {
		for col := range cols {
			cell := row*config.BoardStride + col
			r[cell/wordBits] |= 1 << (uint(cell) % wordBits)
		}
	}
	return r
}

func NewBoard() *Board {
	return &Board{Side: Red, Region: regionBB(config.BoardSize, config.BoardSize)}
}

func NewCrossCheck() *Board {
	return &Board{Side: Red, Region: regionBB(config.CrossCheckSize, config.CrossCheckSize)}
}

func bitOf(cell Cell) (int, uint64) {
	return int(cell) / wordBits, 1 << (uint(cell) % wordBits)
}

func popcountBB(w bb) int {
	n := 0
	for _, x := range w {
		n += bits.OnesCount64(x)
	}
	return n
}

func (b *Board) stones(color Color) bb {
	if color == Red {
		return b.Red
	}
	return b.Blue
}

func (b *Board) At(cell Cell) Color {
	w, m := bitOf(cell)
	if b.Red[w]&m != 0 {
		return Red
	}
	if b.Blue[w]&m != 0 {
		return Blue
	}
	return Empty
}

func (b *Board) Occupied(cell Cell) bool {
	w, m := bitOf(cell)
	return b.Full[w]&m != 0
}

func (b *Board) inRegion(cell Cell) bool {
	w, m := bitOf(cell)
	return b.Region[w]&m != 0
}

func (b *Board) IsFull() bool {
	return b.Full == b.Region
}

func (b *Board) Make(cell Cell) {
	if int(cell) >= config.BoardCells {
		panic("rules: Make out of bounds")
	}
	w, m := bitOf(cell)
	if b.Full[w]&m != 0 || b.Region[w]&m == 0 {
		panic("rules: Make on occupied or out-of-region cell")
	}
	color := b.Side
	b.undo[b.MoveCount] = undoEntry{cell: cell, color: color, hash: b.Hash}
	if color == Red {
		b.Red[w] |= m
	} else {
		b.Blue[w] |= m
	}
	b.Full[w] |= m
	b.Hash ^= zobristPieces[color][cell] ^ zobristSide
	b.Side = color.Opponent()
	b.MoveCount++
}

func (b *Board) Unmake() {
	if b.MoveCount == 0 {
		panic("rules: Unmake on empty stack")
	}
	b.MoveCount--
	e := b.undo[b.MoveCount]
	b.Hash = e.hash
	w, m := bitOf(e.cell)
	if e.color == Red {
		b.Red[w] &^= m
	} else {
		b.Blue[w] &^= m
	}
	b.Full[w] &^= m
	b.Side = e.color
}
