package main

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func tableFours(b *rules.Board, mover rules.Color) []bool {
	out := make([]bool, config.BoardCells)
	reach := dirReachBB(b, mover)
	for cell := range config.BoardCells {
		c := rules.Cell(cell)
		w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
		if b.Full[w]&m != 0 {
			continue
		}
		for d := range config.PatternDirs {
			if reach[d][w]&m == 0 {
				continue
			}
			idx := pattern.Index(b, c, d, mover)
			if pattern.Lookup(d, idx|uint32(config.PatternStateOwn)<<windowCenterShift).Class >= config.PatternClassFour {
				out[cell] = true
				break
			}
		}
	}
	return out
}

func planesOf(b *rules.Board, mover rules.Color) (own, free [config.BoardSize][config.BoardSize]uint8) {
	s := &b.Red
	if mover == rules.Blue {
		s = &b.Blue
	}
	for cell := range config.BoardCells {
		r, c := cell/config.BoardStride, cell%config.BoardStride
		if s[cell/wordBits]&(1<<(uint(cell)%wordBits)) != 0 {
			own[r][c] = 1
		} else if b.Full[cell/wordBits]&(1<<(uint(cell)%wordBits)) == 0 {
			free[r][c] = 1
		}
	}
	return own, free
}

func convFours(b *rules.Board, mover rules.Color) []bool {
	own, free := planesOf(b, mover)
	out := make([]bool, config.BoardCells)
	for d := range config.PatternDirs {
		dr, dc := config.PatternDirs[d][0], config.PatternDirs[d][1]
		for start := range config.BoardCells {
			r0, c0 := start/config.BoardStride, start%config.BoardStride
			er, ec := r0+dr*(config.BoardSize-1), c0+dc*(config.BoardSize-1)
			if er < 0 || er >= config.BoardSize || ec < 0 || ec >= config.BoardSize {
				continue
			}
			ownSum, freeSum := 0, 0
			for j := range config.WinLength {
				ownSum += int(own[r0+j*dr][c0+j*dc])
				freeSum += int(free[r0+j*dr][c0+j*dc])
			}
			markSeg(out, b, r0, c0, dr, dc, 0, ownSum, freeSum)
			for j := 1; j+config.WinLength <= config.BoardSize; j++ {
				ownSum += int(own[r0+(j+config.WinLength-1)*dr][c0+(j+config.WinLength-1)*dc]) - int(own[r0+(j-1)*dr][c0+(j-1)*dc])
				freeSum += int(free[r0+(j+config.WinLength-1)*dr][c0+(j+config.WinLength-1)*dc]) - int(free[r0+(j-1)*dr][c0+(j-1)*dc])
				markSeg(out, b, r0, c0, dr, dc, j, ownSum, freeSum)
			}
		}
	}
	return out
}

func markSeg(out []bool, b *rules.Board, r0, c0, dr, dc, j0, ownSum, freeSum int) {
	if ownSum < config.WinLength-2 || freeSum < 2 || ownSum+freeSum != config.WinLength {
		return
	}
	for j := j0; j < j0+config.WinLength; j++ {
		cell := (r0+j*dr)*config.BoardStride + c0 + j*dc
		w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
		if b.Full[w]&m == 0 && !out[cell] {
			out[cell] = true
		}
	}
}

func screenTightness(truth, screen []bool) (int, int) {
	s, both := 0, 0
	for i := range truth {
		if screen[i] {
			s++
			if truth[i] {
				both++
			}
		}
	}
	return s, s - both
}
