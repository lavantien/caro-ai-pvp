package pattern

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type Entry struct {
	Win1  uint16
	Class uint8
}

const (
	flagRealizable uint8 = 1 << iota
	flagThree
	flagBroken
)

var table [config.PatternTableEntries]Entry

var (
	n1Scratch    [config.PatternTableEntries]uint8
	flagsScratch [config.PatternTableEntries]uint8
)

func init() {
	board := rules.NewBoard()
	buildDirTable(board, &config.PatternDirs[0], table[:])
}

func Lookup(idx uint32) Entry { return table[idx] }

func buildDirTable(board *rules.Board, delta *[2]int, dst []Entry) {
	clear(n1Scratch[:])
	clear(flagsScratch[:])
	sweepWins(board, delta, dst)
	markThreats()
	finishClasses(dst)
}

func sweepWins(board *rules.Board, delta *[2]int, dst []Entry) {
	anchor := board.Geom.Rows / 2
	for idx := range config.PatternTableEntries {
		states := Unpack(uint32(idx))
		if !isRealizable(&states) {
			dst[idx] = Entry{}
			continue
		}
		flagsScratch[idx] |= flagRealizable
		placed := 0
		for i, s := range states {
			if s == config.PatternStateEmpty || s == config.PatternStateOff {
				continue
			}
			board.Side = genColor(s)
			board.Make(genCell(board.Geom, anchor, delta, i))
			placed++
		}
		var mask uint16
		for i, s := range states {
			if s != config.PatternStateEmpty {
				continue
			}
			cell := genCell(board.Geom, anchor, delta, i)
			board.Side = rules.Red
			board.Make(cell)
			if board.FastLastMoveWin(rules.Red, cell) {
				mask |= 1 << uint(i)
			}
			board.Unmake()
		}
		for ; placed > 0; placed-- {
			board.Unmake()
		}
		dst[idx] = Entry{Win1: mask}
		n1Scratch[idx] = uint8(bits.OnesCount16(mask))
	}
}

func markThreats() {
	for idx := range config.PatternTableEntries {
		if flagsScratch[idx]&flagRealizable == 0 {
			continue
		}
		var f uint8
		for i, s := range Unpack(uint32(idx)) {
			if s != config.PatternStateEmpty {
				continue
			}
			cn := n1Scratch[idx|int(config.PatternStateOwn)<<(i*config.PatternCellBits)]
			if cn >= 2 {
				f |= flagThree
			} else if cn == 1 {
				f |= flagBroken
			}
		}
		flagsScratch[idx] |= f
	}
}

func finishClasses(dst []Entry) {
	for idx := range config.PatternTableEntries {
		if flagsScratch[idx]&flagRealizable == 0 {
			continue
		}
		cls := config.PatternClassNone
		switch n1Scratch[idx] {
		case 0:
			switch {
			case flagsScratch[idx]&flagThree != 0:
				cls = config.PatternClassThree
			case flagsScratch[idx]&flagBroken != 0:
				cls = config.PatternClassBrokenThree
			case openTwo(idx):
				cls = config.PatternClassOpenTwo
			}
		case 1:
			cls = config.PatternClassFour
		default:
			cls = config.PatternClassOpenFour
		}
		dst[idx].Class = cls
	}
}

func openTwo(idx int) bool {
	for i, s := range Unpack(uint32(idx)) {
		if s != config.PatternStateEmpty {
			continue
		}
		if flagsScratch[idx|int(config.PatternStateOwn)<<(i*config.PatternCellBits)]&flagThree != 0 {
			return true
		}
	}
	return false
}

func isRealizable(w *[config.PatternWindowLen]uint8) bool {
	first, last := -1, -1
	for i, s := range *w {
		if s != config.PatternStateOff {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i := first + 1; i < last; i++ {
		if w[i] == config.PatternStateOff {
			return false
		}
	}
	return true
}

func genColor(s uint8) rules.Color {
	if s == config.PatternStateOwn {
		return rules.Red
	}
	return rules.Blue
}

func genCell(g *geom.Geometry, anchor int, delta *[2]int, i int) rules.Cell {
	k := i - windowHalf
	return rules.Cell((anchor+k*delta[0])*g.Stride + anchor + k*delta[1])
}
