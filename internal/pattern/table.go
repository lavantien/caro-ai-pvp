package pattern

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Entry classifies one packed window for the mover: the win-in-1 bitmask over
// the PatternWindowLen positions plus the threat class byte from taxonomy.md.
type Entry struct {
	Win1  uint16
	Class uint8
}

const (
	flagRealizable uint8 = 1 << iota
	flagThree
	flagBroken
)

var tables [config.PatternDirections][config.PatternTableEntries]Entry

// Generation scratch, written only inside buildTables.
var (
	n1Scratch    [config.PatternTableEntries]uint8
	flagsScratch [config.PatternTableEntries]uint8
)

func init() { buildTables(&tables) }

// Lookup returns the entry of a packed own-relative window index.
func Lookup(dir int, idx uint32) Entry { return tables[dir][idx] }

// buildTables fills all 4 direction tables by emulating every realizable
// window on one rules board and reading only rules.Make, rules.Unmake and
// rules.FastLastMoveWin. Single threaded by construction: package init or a
// benchmark call.
func buildTables(dst *[config.PatternDirections][config.PatternTableEntries]Entry) {
	board := rules.NewBoard()
	for d := range config.PatternDirections {
		clear(n1Scratch[:])
		clear(flagsScratch[:])
		sweepWins(board, &config.PatternDirs[d], dst[d][:])
		markThreats()
		finishClasses(dst[d][:])
	}
}

// sweepWins emulates each realizable window on the board, centered at the
// board middle so every window offset lands in region, and records the
// win-in-1 mask plus the n1 count. Out-of-window cells stay empty, which is
// exactly wall semantics: runs stop there and empty never blocks.
func sweepWins(board *rules.Board, delta *[2]int, dst []Entry) {
	anchor := config.BoardSize / 2
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
			board.Make(genCell(anchor, delta, i))
			placed++
		}
		var mask uint16
		for i, s := range states {
			if s != config.PatternStateEmpty {
				continue
			}
			cell := genCell(anchor, delta, i)
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

// markThreats derives Three and BrokenThree from child n1 counts by pure
// index arithmetic: placing Own at empty position i of idx gives the packed
// index idx|Own<<(2*i), always realizable itself.
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

// finishClasses assigns the class byte. In the OpenTwo branch every child has
// n1 = 0, so a child flagged Three is exactly a child classified Three.
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

func genCell(anchor int, delta *[2]int, i int) rules.Cell {
	k := i - windowHalf
	return rules.Cell((anchor+k*delta[0])*config.BoardStride + anchor + k*delta[1])
}
