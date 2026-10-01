package engine

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// classPairs packs both mover views of one packed window into a byte: the
// red-relative class in the low 3 bits, the blue-relative class (the state
// swapped view) in the high bits. Built once at package init from the pattern
// tables, which are direction independent by the pinned reversal and
// transposition invariants of internal/pattern.
var classPairs [config.PatternTableEntries]uint8

func init() {
	for idx := range config.PatternTableEntries {
		red := pattern.Lookup(0, uint32(idx)).Class
		blue := pattern.Lookup(0, swapStates(uint32(idx))).Class
		classPairs[idx] = red | blue<<3
	}
}

const cellStateMask = 1<<config.PatternCellBits - 1

const windowHalf = (config.PatternWindowLen - 1) / 2

// evaluator keeps the pattern table view of the position in sync with the
// board incrementally. A stone placed at cell x changes exactly the windows
// containing x: PatternDirections lines through x, PatternWindowLen centers
// each, 36 windows total. For every such window the evaluator stores the
// red-relative packed index and its packed class pair, so a make derives the
// new index by setting the placed stone's two state bits and reads a single
// class pair load, and an unmake clears the bits again, with no pattern
// calls and no per-ply save buffer.
//
// Three aggregates ride the same 36-window pass:
//   - score per color: sum of window class weights, the leaf evaluation
//   - cell per color: the same sum restricted to the 4 windows centered on a
//     cell, the static move ordering score (threat plus denial value)
//   - fours per color: window count at class Four or above, the forced
//     defense trigger of the threat extension
type evaluator struct {
	win   [config.PatternDirections][config.BoardCells]uint32
	wpair [config.PatternDirections][config.BoardCells]uint8
	score [2]int
	cell  [2][config.BoardCells]int
	fours [2]int
}

// swapStates maps a red-relative packed index to the blue-relative one:
// Own and Opp exchange their two-bit codes, Empty and Off stay fixed.
func swapStates(idx uint32) uint32 {
	const evenBits = 0b010101010101010101
	return idx&evenBits<<1 | idx>>1&evenBits
}

func (ev *evaluator) reset(b *rules.Board) {
	*ev = evaluator{}
	for d := range config.PatternDirections {
		for cell := range config.BoardCells {
			red := pattern.Index(b, rules.Cell(cell), d, rules.Red)
			pair := classPairs[red]
			ev.win[d][cell] = red
			ev.wpair[d][cell] = pair
			ev.score[rules.Red] += int(config.PatternClassWeights[pair&7])
			ev.score[rules.Blue] += int(config.PatternClassWeights[pair>>3])
			ev.cell[rules.Red][cell] += int(config.PatternClassWeights[pair&7])
			ev.cell[rules.Blue][cell] += int(config.PatternClassWeights[pair>>3])
			if pair&7 >= config.PatternClassFour {
				ev.fours[rules.Red]++
			}
			if pair>>3 >= config.PatternClassFour {
				ev.fours[rules.Blue]++
			}
		}
	}
}

func (ev *evaluator) eval(side rules.Color) int {
	return ev.score[side] - ev.score[side^1] + config.EvalTempo
}

func (ev *evaluator) applyWindow(center int, beforePair, afterPair uint8) {
	redDelta := int(config.PatternClassWeights[afterPair&7]) - int(config.PatternClassWeights[beforePair&7])
	blueDelta := int(config.PatternClassWeights[afterPair>>3]) - int(config.PatternClassWeights[beforePair>>3])
	if redDelta != 0 {
		ev.score[rules.Red] += redDelta
		ev.cell[rules.Red][center] += redDelta
	}
	if blueDelta != 0 {
		ev.score[rules.Blue] += blueDelta
		ev.cell[rules.Blue][center] += blueDelta
	}
	if rb, ra := beforePair&7 >= config.PatternClassFour, afterPair&7 >= config.PatternClassFour; rb != ra {
		if ra {
			ev.fours[rules.Red]++
		} else {
			ev.fours[rules.Red]--
		}
	}
	if blueBefore, blueAfter := beforePair>>3 >= config.PatternClassFour, afterPair>>3 >= config.PatternClassFour; blueBefore != blueAfter {
		if blueAfter {
			ev.fours[rules.Blue]++
		} else {
			ev.fours[rules.Blue]--
		}
	}
}

// makeMove folds a stone into the window tables. The state bits are set
// relative to red: Own for a red stone, Opp for a blue one.
func (ev *evaluator) makeMove(color rules.Color, cell rules.Cell) {
	state := uint32(config.PatternStateOpp)
	if color == rules.Red {
		state = uint32(config.PatternStateOwn)
	}
	r, c := int(cell)/config.BoardStride, int(cell)%config.BoardStride
	for d := range config.PatternDirections {
		dr, dc := config.PatternDirs[d][0], config.PatternDirs[d][1]
		for k := -windowHalf; k <= windowHalf; k++ {
			rr, cc := r+k*dr, c+k*dc
			if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
				continue
			}
			center := rr*config.BoardStride + cc
			beforePair := ev.wpair[d][center]
			after := ev.win[d][center] | state<<(uint(windowHalf-k)*config.PatternCellBits)
			ev.win[d][center] = after
			afterPair := classPairs[after]
			ev.wpair[d][center] = afterPair
			ev.applyWindow(center, beforePair, afterPair)
		}
	}
}

// unmakeMove is the exact inverse: the placed field returns to Empty and the
// window deltas reverse. Color independent, clearing works for both codes.
func (ev *evaluator) unmakeMove(cell rules.Cell) {
	r, c := int(cell)/config.BoardStride, int(cell)%config.BoardStride
	for d := range config.PatternDirections {
		dr, dc := config.PatternDirs[d][0], config.PatternDirs[d][1]
		for k := -windowHalf; k <= windowHalf; k++ {
			rr, cc := r+k*dr, c+k*dc
			if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
				continue
			}
			center := rr*config.BoardStride + cc
			afterPair := ev.wpair[d][center]
			before := ev.win[d][center] &^ (cellStateMask << (uint(windowHalf-k) * config.PatternCellBits))
			ev.win[d][center] = before
			beforePair := classPairs[before]
			ev.wpair[d][center] = beforePair
			ev.applyWindow(center, afterPair, beforePair)
		}
	}
}
