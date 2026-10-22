package main

import "math"

func dimLevels(d dim) int {
	if d.integer {
		n := int(math.Floor(d.hi)) - int(math.Ceil(d.lo)) + 1
		if n < 1 {
			n = 1
		}
		if n > gridLevels {
			n = gridLevels
		}
		return n
	}
	return gridLevels
}

func levelValue(d dim, lvl, levels int) float64 {
	if levels == 1 {
		return clampScalar(d, (d.lo+d.hi)/2)
	}
	return clampScalar(d, d.lo+float64(lvl)/float64(levels-1)*(d.hi-d.lo))
}

func gridStream(dims []dim) [][]float64 {
	levels := make([]int, len(dims))
	for i, d := range dims {
		levels[i] = dimLevels(d)
	}
	idx := make([]int, len(dims))
	total := 1
	for _, n := range levels {
		total *= n
	}
	out := make([][]float64, 0, total)
	for {
		x := make([]float64, len(dims))
		for i, d := range dims {
			x[i] = levelValue(d, idx[i], levels[i])
		}
		out = append(out, x)
		done := true
		for i := len(idx) - 1; i >= 0; i-- {
			idx[i]++
			if idx[i] < levels[i] {
				done = false
				break
			}
			idx[i] = 0
		}
		if done {
			break
		}
	}
	return out
}

func runGrid(dims []dim, obj func([]float64) float64, budget int, target float64) runResult {
	best := runResult{bestF: math.Inf(1)}
	n := 0
	for _, x := range gridStream(dims) {
		if n >= budget {
			break
		}
		n++
		f := obj(x)
		if f < best.bestF {
			best.bestF = f
			best.bestX = append([]float64(nil), x...)
		}
		if f <= target {
			best.trialsToTarget = n
			return best
		}
	}
	return best
}

func runRandom(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult {
	rng := newSeededRng(seed)
	best := runResult{bestF: math.Inf(1)}
	for i := 1; i <= budget; i++ {
		x := randomPoint(dims, rng)
		f := obj(x)
		if f < best.bestF {
			best.bestF = f
			best.bestX = append([]float64(nil), x...)
		}
		if f <= target {
			best.trialsToTarget = i
			return best
		}
	}
	return best
}
