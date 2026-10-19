package engine

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

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

type evaluator struct {
	win   [config.PatternDirections][config.BoardCells]uint32
	wpair [config.PatternDirections][config.BoardCells]uint8
	score [2]int
	cell  [2][config.BoardCells]int
	fours [2]int
	w     *[config.PatternClassCount]int
}

func (ev *evaluator) weights() *[config.PatternClassCount]int {
	if ev.w != nil {
		return ev.w
	}
	return &config.PatternClassWeights
}

func swapStates(idx uint32) uint32 {
	const evenBits = 0b010101010101010101
	return idx&evenBits<<1 | idx>>1&evenBits
}

func (ev *evaluator) reset(b *rules.Board) {
	w := ev.w
	*ev = evaluator{w: w}
	cw := ev.weights()
	for d := range config.PatternDirections {
		for cell := range config.BoardCells {
			red := pattern.Index(b, rules.Cell(cell), d, rules.Red)
			pair := classPairs[red]
			ev.win[d][cell] = red
			ev.wpair[d][cell] = pair
			ev.score[rules.Red] += int(cw[pair&7])
			ev.score[rules.Blue] += int(cw[pair>>3])
			ev.cell[rules.Red][cell] += int(cw[pair&7])
			ev.cell[rules.Blue][cell] += int(cw[pair>>3])
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
	cw := ev.weights()
	redDelta := int(cw[afterPair&7]) - int(cw[beforePair&7])
	blueDelta := int(cw[afterPair>>3]) - int(cw[beforePair>>3])
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
