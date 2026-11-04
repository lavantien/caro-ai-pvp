package engine

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var classPairs [config.PatternTableEntries]uint8

var cleanBits [config.PatternTableEntries]uint8

var boxedPairs [config.PatternTableEntries]uint8

func init() {
	for idx := range config.PatternTableEntries {
		swapped := swapStates(uint32(idx))
		red := pattern.Lookup(uint32(idx)).Class
		blue := pattern.Lookup(swapped).Class
		classPairs[idx] = red | blue<<3
		var hasOwn, hasOpp bool
		for _, s := range pattern.Unpack(uint32(idx)) {
			switch s {
			case config.PatternStateOwn:
				hasOwn = true
			case config.PatternStateOpp:
				hasOpp = true
			}
		}
		var bits uint8
		if hasOwn && !hasOpp {
			bits |= 1
		}
		if hasOpp && !hasOwn {
			bits |= 2
		}
		cleanBits[idx] = bits
		var bx uint8
		if boxedEngaged(uint32(idx), red) {
			bx |= 1
		}
		if boxedEngaged(swapped, blue) {
			bx |= 2
		}
		boxedPairs[idx] = bx
	}
}

func boxedEngaged(idx uint32, cls uint8) bool {
	if cls != config.PatternClassThree {
		return false
	}
	states := pattern.Unpack(idx)
	for i, si := range states {
		if si != config.PatternStateOwn {
			continue
		}
		for j, sj := range states {
			if sj == config.PatternStateOpp && j >= i-2 && j <= i+2 {
				return true
			}
		}
	}
	return false
}

const cellStateMask = 1<<config.PatternCellBits - 1

const windowHalf = (config.PatternWindowLen - 1) / 2

type evaluator struct {
	geom   *geom.Geometry
	win    [config.PatternDirections][config.MaxBoardCells]uint32
	wpair  [config.PatternDirections][config.MaxBoardCells]uint8
	wboxed [config.PatternDirections][config.MaxBoardCells]uint8
	score  [2]int
	cell   [2][config.MaxBoardCells]int
	fours  [2]int
	clean  [2]int
	cleanW int
	boxedT [2]int
	cutT   int
	w      *[config.PatternClassCount]int
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
	*ev = evaluator{
		w:      w,
		geom:   b.Geom,
		cleanW: config.EvalCleanWindowWeight(b.Geom.Name),
		cutT:   config.EvalBoxedCutThree(b.Geom.Name),
	}
	g := b.Geom
	cw := ev.weights()
	for d := range g.Dirs {
		for cell := range g.Cells {
			red := pattern.Index(b, rules.Cell(cell), d, rules.Red)
			pair := classPairs[red]
			bx := boxedPairs[red]
			ev.win[d][cell] = red
			ev.wpair[d][cell] = pair
			ev.wboxed[d][cell] = bx
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
			bits := cleanBits[red]
			if bits&1 != 0 {
				ev.clean[rules.Red]++
			}
			if bits&2 != 0 {
				ev.clean[rules.Blue]++
			}
			if bx&1 != 0 {
				ev.boxedT[rules.Red]++
			}
			if bx&2 != 0 {
				ev.boxedT[rules.Blue]++
			}
		}
	}
}

func (ev *evaluator) eval(side rules.Color) int {
	return ev.score[side] - ev.score[side^1] + config.EvalTempo +
		ev.cleanW*(ev.clean[side]-ev.clean[side^1]) -
		ev.cutT*(ev.boxedT[side]-ev.boxedT[side^1])
}

func (ev *evaluator) applyBoxed(side rules.Color, beforeCls, afterCls uint8, beforeOn, afterOn bool) {
	beforeThree := beforeOn && beforeCls == config.PatternClassThree
	afterThree := afterOn && afterCls == config.PatternClassThree
	if afterThree != beforeThree {
		if afterThree {
			ev.boxedT[side]++
		} else {
			ev.boxedT[side]--
		}
	}
}

func (ev *evaluator) applyWindow(center int, beforePair, afterPair, beforeBoxed, afterBoxed uint8) {
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
	ev.applyBoxed(rules.Red, beforePair&7, afterPair&7, beforeBoxed&1 != 0, afterBoxed&1 != 0)
	ev.applyBoxed(rules.Blue, beforePair>>3, afterPair>>3, beforeBoxed&2 != 0, afterBoxed&2 != 0)
}

func (ev *evaluator) applyClean(before, after uint32) {
	db, da := cleanBits[before], cleanBits[after]
	if db == da {
		return
	}
	if db&1 != da&1 {
		if da&1 != 0 {
			ev.clean[rules.Red]++
		} else {
			ev.clean[rules.Red]--
		}
	}
	if db&2 != da&2 {
		if da&2 != 0 {
			ev.clean[rules.Blue]++
		} else {
			ev.clean[rules.Blue]--
		}
	}
}

func (ev *evaluator) makeMove(color rules.Color, cell rules.Cell) {
	state := uint32(config.PatternStateOpp)
	if color == rules.Red {
		state = uint32(config.PatternStateOwn)
	}
	g := ev.geom
	r, c := int(cell)/g.Stride, int(cell)%g.Stride
	for d := range g.Dirs {
		dr, dc := g.Dirs[d].Dr, g.Dirs[d].Dc
		for k := -windowHalf; k <= windowHalf; k++ {
			rr, cc := r+k*dr, c+k*dc
			if rr < 0 || rr >= g.Rows || cc < 0 || cc >= g.Cols {
				continue
			}
			center := rr*g.Stride + cc
			if center >= config.MaxBoardCells {
				continue
			}
			before := ev.win[d][center]
			beforePair := ev.wpair[d][center]
			beforeBoxed := ev.wboxed[d][center]
			after := before | state<<(uint(windowHalf-k)*config.PatternCellBits)
			ev.win[d][center] = after
			afterPair := classPairs[after]
			afterBoxed := boxedPairs[after]
			ev.wpair[d][center] = afterPair
			ev.wboxed[d][center] = afterBoxed
			ev.applyWindow(center, beforePair, afterPair, beforeBoxed, afterBoxed)
			ev.applyClean(before, after)
		}
	}
}

func (ev *evaluator) unmakeMove(cell rules.Cell) {
	g := ev.geom
	r, c := int(cell)/g.Stride, int(cell)%g.Stride
	for d := range g.Dirs {
		dr, dc := g.Dirs[d].Dr, g.Dirs[d].Dc
		for k := -windowHalf; k <= windowHalf; k++ {
			rr, cc := r+k*dr, c+k*dc
			if rr < 0 || rr >= g.Rows || cc < 0 || cc >= g.Cols {
				continue
			}
			center := rr*g.Stride + cc
			if center >= config.MaxBoardCells {
				continue
			}
			after := ev.win[d][center]
			afterPair := ev.wpair[d][center]
			afterBoxed := ev.wboxed[d][center]
			before := after &^ (cellStateMask << (uint(windowHalf-k) * config.PatternCellBits))
			ev.win[d][center] = before
			beforePair := classPairs[before]
			beforeBoxed := boxedPairs[before]
			ev.wpair[d][center] = beforePair
			ev.wboxed[d][center] = beforeBoxed
			ev.applyWindow(center, afterPair, beforePair, afterBoxed, beforeBoxed)
			ev.applyClean(after, before)
		}
	}
}
