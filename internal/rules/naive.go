package rules

import "github.com/lavantien/caro-ai-pvp/internal/config"

type naiveColor uint8

const (
	naiveEmpty naiveColor = iota
	naiveRed
	naiveBlue
)

type NaiveBoard struct {
	cells  [config.BoardCells]naiveColor
	region [config.BoardCells]bool
}

func newNaiveRegion(size int) *NaiveBoard {
	nb := &NaiveBoard{}
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			nb.region[r*config.BoardStride+c] = true
		}
	}
	return nb
}

func NewNaiveBoard() *NaiveBoard      { return newNaiveRegion(config.BoardSize) }
func NewNaiveCrossCheck() *NaiveBoard { return newNaiveRegion(config.CrossCheckSize) }

func naiveOf(color Color) naiveColor {
	if color == Red {
		return naiveRed
	}
	return naiveBlue
}

func (nb *NaiveBoard) Set(cell Cell, color Color) {
	if int(cell) >= config.BoardCells || !nb.region[cell] || nb.cells[cell] != naiveEmpty {
		panic("rules: naive Set on unusable cell")
	}
	nb.cells[cell] = naiveOf(color)
}

func (nb *NaiveBoard) at(r, c int) naiveColor {
	return nb.cells[r*config.BoardStride+c]
}

func (nb *NaiveBoard) blocks(r, c int, opp naiveColor) bool {
	return inBoard(r, c) && nb.region[r*config.BoardStride+c] && nb.at(r, c) == opp
}

func (nb *NaiveBoard) Wins(color Color) bool {
	me := naiveOf(color)
	opp := naiveOf(color.Opponent())
	for cell := 0; cell < config.BoardCells; cell++ {
		if nb.cells[cell] != me {
			continue
		}
		r, c := cell/config.BoardStride, cell%config.BoardStride
		for d := range lineDirs {
			dr, dc := lineDirs[d][0], lineDirs[d][1]
			if inBoard(r-dr, c-dc) && nb.at(r-dr, c-dc) == me {
				continue
			}
			length := 0
			rr, cc := r, c
			for inBoard(rr, cc) && nb.at(rr, cc) == me {
				length++
				rr += dr
				cc += dc
			}
			if length != config.WinLength {
				continue
			}
			if !(nb.blocks(r-dr, c-dc, opp) && nb.blocks(rr, cc, opp)) {
				return true
			}
		}
	}
	return false
}

func (nb *NaiveBoard) WinsThrough(color Color, cell Cell) bool {
	me := naiveOf(color)
	opp := naiveOf(color.Opponent())
	if int(cell) >= config.BoardCells || nb.cells[cell] != me {
		return false
	}
	r, c := int(cell)/config.BoardStride, int(cell)%config.BoardStride
	for d := range lineDirs {
		dr, dc := lineDirs[d][0], lineDirs[d][1]
		back := 0
		br, bc := r-dr, c-dc
		for inBoard(br, bc) && nb.at(br, bc) == me {
			back++
			br -= dr
			bc -= dc
		}
		fwd := 0
		fr, fc := r+dr, c+dc
		for inBoard(fr, fc) && nb.at(fr, fc) == me {
			fwd++
			fr += dr
			fc += dc
		}
		if back+fwd+1 != config.WinLength {
			continue
		}
		if !(nb.blocks(br, bc, opp) && nb.blocks(fr, fc, opp)) {
			return true
		}
	}
	return false
}

func (nb *NaiveBoard) IsLegal(side Color, cell Cell) bool {
	if int(cell) >= config.BoardCells || !nb.region[cell] || nb.cells[cell] != naiveEmpty {
		return false
	}
	if side != Red {
		return true
	}
	redStones := 0
	anchor := -1
	for i, v := range nb.cells {
		if v == naiveRed {
			redStones++
			anchor = i
		}
	}
	if redStones != 1 {
		return true
	}
	dr := anchor/config.BoardStride - int(cell)/config.BoardStride
	dc := anchor%config.BoardStride - int(cell)%config.BoardStride
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}
	if dr < dc {
		dr = dc
	}
	return dr >= config.OpeningChebyshevMin
}
