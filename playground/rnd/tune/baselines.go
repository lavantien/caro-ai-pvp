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

func fittedLevels(dims []dim, budget int) int {
	continuous := 0
	cells := 1
	for _, d := range dims {
		if d.integer {
			cells *= dimLevels(d)
		} else {
			continuous++
		}
	}
	if continuous == 0 {
		return gridLevels
	}
	levels := 1
	for l := 1; l <= gridLevels; l++ {
		c := cells
		for range continuous {
			c *= l
		}
		if c > budget {
			break
		}
		levels = l
	}
	return levels
}

func gridLattice(dims []dim, levels int, seed uint64) [][]float64 {
	rng := newSeededRng(seed)
	axes := make([][]float64, len(dims))
	for i, d := range dims {
		if d.integer {
			n := dimLevels(d)
			axis := make([]float64, n)
			for lvl := range n {
				axis[lvl] = levelValue(d, lvl, n)
			}
			axes[i] = axis
			continue
		}
		spacing := (d.hi - d.lo) / float64(levels)
		offset := rng.Float64() * spacing
		axis := make([]float64, levels)
		for k := range levels {
			axis[k] = clampScalar(d, d.lo+offset+float64(k)*spacing)
		}
		axes[i] = axis
	}
	idx := make([]int, len(dims))
	total := 1
	for _, a := range axes {
		total *= len(a)
	}
	out := make([][]float64, 0, total)
	for {
		x := make([]float64, len(dims))
		for i := range x {
			x[i] = axes[i][idx[i]]
		}
		out = append(out, x)
		done := true
		for i := len(idx) - 1; i >= 0; i-- {
			idx[i]++
			if idx[i] < len(axes[i]) {
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

func runGridAt(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64, levels int) runResult {
	best := runResult{bestF: math.Inf(1)}
	n := 0
	for _, x := range gridLattice(dims, levels, seed) {
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

func runGrid(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult {
	return runGridAt(dims, obj, seed, budget, target, gridLevels)
}

func runGridCoarse(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult {
	return runGridAt(dims, obj, seed, budget, target, fittedLevels(dims, budget))
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
