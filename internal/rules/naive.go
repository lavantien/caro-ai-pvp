package rules

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

type naiveColor uint8

const (
	naiveEmpty naiveColor = iota
	naiveRed
	naiveBlue
)

type NaiveBoard struct {
	cells  [config.MaxBoardBoxCells]naiveColor
	region [config.MaxBoardBoxCells]bool
	g      *geom.Geometry
}

func NewNaiveBoardFor(g *geom.Geometry) *NaiveBoard {
	nb := &NaiveBoard{g: g}
	for cell := range g.Cells {
		nb.region[cell] = g.Contains(cell)
	}
	return nb
}

func NewNaiveBoard() *NaiveBoard { return NewNaiveBoardFor(squareGeometry) }

func NewNaiveCrossCheck() *NaiveBoard {
	nb := NewNaiveBoard()
	for cell := range config.BoardCells {
		nb.region[cell] = cell/config.BoardStride < config.CrossCheckSize && cell%config.BoardStride < config.CrossCheckSize
	}
	return nb
}

func naiveOf(color Color) naiveColor {
	if color == Red {
		return naiveRed
	}
	return naiveBlue
}

func (nb *NaiveBoard) inGeom(r, c int) bool {
	return r >= 0 && r < nb.g.Rows && c >= 0 && c < nb.g.Cols
}

func (nb *NaiveBoard) Set(cell Cell, color Color) {
	if int(cell) >= nb.g.Cells || !nb.region[cell] || nb.cells[cell] != naiveEmpty {
		panic("rules: naive Set on unusable cell")
	}
	nb.cells[cell] = naiveOf(color)
}

func (nb *NaiveBoard) at(r, c int) naiveColor {
	return nb.cells[r*nb.g.Stride+c]
}

func (nb *NaiveBoard) blocks(r, c int, opp naiveColor) bool {
	return nb.inGeom(r, c) && nb.region[r*nb.g.Stride+c] && nb.at(r, c) == opp
}

func (nb *NaiveBoard) Wins(color Color) bool {
	me := naiveOf(color)
	opp := naiveOf(color.Opponent())
	for cell := range nb.g.Cells {
		if nb.cells[cell] != me {
			continue
		}
		r, c := cell/nb.g.Stride, cell%nb.g.Stride
		for _, dir := range nb.g.Dirs {
			dr, dc := dir.Dr, dir.Dc
			if nb.inGeom(r-dr, c-dc) && nb.at(r-dr, c-dc) == me {
				continue
			}
			length := 0
			rr, cc := r, c
			for nb.inGeom(rr, cc) && nb.at(rr, cc) == me {
				length++
				rr += dr
				cc += dc
			}
			if length != config.WinLength {
				continue
			}
			beforeBlocked := nb.blocks(r-dr, c-dc, opp)
			afterBlocked := nb.blocks(rr, cc, opp)
			if !beforeBlocked || !afterBlocked {
				return true
			}
		}
	}
	return false
}

func (nb *NaiveBoard) WinsThrough(color Color, cell Cell) bool {
	me := naiveOf(color)
	opp := naiveOf(color.Opponent())
	if int(cell) >= nb.g.Cells || nb.cells[cell] != me {
		return false
	}
	r, c := int(cell)/nb.g.Stride, int(cell)%nb.g.Stride
	for _, dir := range nb.g.Dirs {
		dr, dc := dir.Dr, dir.Dc
		back := 0
		br, bc := r-dr, c-dc
		for nb.inGeom(br, bc) && nb.at(br, bc) == me {
			back++
			br -= dr
			bc -= dc
		}
		fwd := 0
		fr, fc := r+dr, c+dc
		for nb.inGeom(fr, fc) && nb.at(fr, fc) == me {
			fwd++
			fr += dr
			fc += dc
		}
		if back+fwd+1 != config.WinLength {
			continue
		}
		beforeBlocked := nb.blocks(br, bc, opp)
		afterBlocked := nb.blocks(fr, fc, opp)
		if !beforeBlocked || !afterBlocked {
			return true
		}
	}
	return false
}

func (nb *NaiveBoard) IsLegal(side Color, cell Cell) bool {
	if int(cell) >= nb.g.Cells || !nb.region[cell] || nb.cells[cell] != naiveEmpty {
		return false
	}
	if side != Red {
		return true
	}
	redStones := 0
	anchor := -1
	for i := range nb.g.Cells {
		if nb.cells[i] == naiveRed {
			redStones++
			anchor = i
		}
	}
	if redStones != 1 {
		return true
	}
	return nb.g.Distance(anchor, int(cell)) >= nb.g.OpeningMin
}
