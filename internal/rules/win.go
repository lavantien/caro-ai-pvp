package rules

import "github.com/lavantien/caro-ai-pvp/internal/config"

var (
	fileMask0 uint64
	fileMaskF uint64
	dirStep   [4]uint
	dirFwdClr [4]uint64
	dirBwdClr [4]uint64
)

func init() {
	for col := range wordBits / config.BoardStride {
		fileMask0 |= 1 << (uint(col) * config.BoardStride)
		fileMaskF |= 1 << (uint(col)*config.BoardStride + config.BoardStride - 1)
	}
	for d, delta := range lineDirs {
		dirStep[d] = uint(delta[0]*config.BoardStride + delta[1])
		dirFwdClr[d] = fileClear(delta[1])
		dirBwdClr[d] = fileClear(-delta[1])
	}
}

func fileClear(dc int) uint64 {
	switch dc {
	case 1:
		return fileMask0
	case -1:
		return fileMaskF
	}
	return 0
}

func shiftFwd(s bb, step uint, clear uint64) bb {
	var r bb
	for w := config.BoardWordsPerColor - 1; w >= 0; w-- {
		r[w] = s[w] << step
		if w > 0 {
			r[w] |= s[w-1] >> (wordBits - step)
		}
		if clear != 0 {
			r[w] &^= clear
		}
	}
	return r
}

func shiftBwd(s bb, step uint, clear uint64) bb {
	var r bb
	for w := range config.BoardWordsPerColor {
		r[w] = s[w] >> step
		if w < config.BoardWordsPerColor-1 {
			r[w] |= s[w+1] << (wordBits - step)
		}
		if clear != 0 {
			r[w] &^= clear
		}
	}
	return r
}

func andBB(a, b bb) bb {
	for w := range a {
		a[w] &= b[w]
	}
	return a
}

func orBB(a, b bb) bb {
	for w := range a {
		a[w] |= b[w]
	}
	return a
}

func andNotBB(a, b bb) bb {
	for w := range a {
		a[w] &^= b[w]
	}
	return a
}

func nonzeroBB(a bb) bool {
	for _, x := range a {
		if x != 0 {
			return true
		}
	}
	return false
}

// winsDir reports whether color s has a maximal run of exactly WinLength stones
// in direction d whose two immediate neighbors are not both opponent stones.
func winsDir(s, o bb, d int) bool {
	step, fc, bc := dirStep[d], dirFwdClr[d], dirBwdClr[d]
	five := s
	cur := s
	for range config.WinLength - 1 {
		cur = shiftBwd(cur, step, bc)
		five = andBB(five, cur)
	}
	if !nonzeroBB(five) {
		return false
	}
	after := shiftBwd(cur, step, bc)
	before := shiftFwd(s, step, fc)
	cand := andNotBB(five, orBB(before, after))
	if !nonzeroBB(cand) {
		return false
	}
	oppShift := o
	for range config.WinLength {
		oppShift = shiftBwd(oppShift, step, bc)
	}
	blocked := andBB(shiftFwd(o, step, fc), oppShift)
	return nonzeroBB(andNotBB(cand, blocked))
}

func (b *Board) Wins(color Color) bool {
	s := b.stones(color)
	o := b.stones(color.Opponent())
	for d := range lineDirs {
		if winsDir(s, o, d) {
			return true
		}
	}
	return false
}

func (b *Board) FastLastMoveWin(color Color, cell Cell) bool {
	s := b.stones(color)
	o := b.stones(color.Opponent())
	r := int(cell) / config.BoardStride
	c := int(cell) % config.BoardStride
	for d := range lineDirs {
		dr, dc := lineDirs[d][0], lineDirs[d][1]
		back := 0
		br, bc := r-dr, c-dc
		for k := 0; k < config.WinLength && inBoard(br, bc) && b.bitAt(s, br, bc); k++ {
			back++
			br -= dr
			bc -= dc
		}
		fwd := 0
		fr, fc := r+dr, c+dc
		for k := 0; k < config.WinLength && inBoard(fr, fc) && b.bitAt(s, fr, fc); k++ {
			fwd++
			fr += dr
			fc += dc
		}
		if back+fwd+1 != config.WinLength {
			continue
		}
		if inBoard(br, bc) && b.bitAt(o, br, bc) && inBoard(fr, fc) && b.bitAt(o, fr, fc) {
			continue
		}
		return true
	}
	return false
}

func (b *Board) bitAt(s bb, r, c int) bool {
	w, m := bitOf(Cell(r*config.BoardStride + c))
	return s[w]&m != 0
}
