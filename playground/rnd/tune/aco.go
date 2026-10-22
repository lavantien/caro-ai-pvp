package main

import (
	"math"
	"math/rand/v2"
)

type acoState struct {
	dims   []dim
	levels []int
	pher   [][]float64
}

func newAcoState(dims []dim) *acoState {
	s := &acoState{dims: dims}
	s.levels = make([]int, len(dims))
	s.pher = make([][]float64, len(dims))
	for i, d := range dims {
		if d.integer {
			s.levels[i] = dimLevels(d)
		} else {
			s.levels[i] = acoLevels
		}
		s.pher[i] = make([]float64, s.levels[i])
		for j := range s.pher[i] {
			s.pher[i][j] = 1
		}
	}
	return s
}

func (s *acoState) construct(rng *rand.Rand) []float64 {
	x := make([]float64, len(s.dims))
	for i, d := range s.dims {
		sum := 0.0
		for _, p := range s.pher[i] {
			sum += p
		}
		r := rng.Float64() * sum
		lvl := len(s.pher[i]) - 1
		acc := 0.0
		for j, p := range s.pher[i] {
			acc += p
			if r <= acc {
				lvl = j
				break
			}
		}
		x[i] = levelValue(d, lvl, s.levels[i])
	}
	return x
}

func (s *acoState) evaporate() {
	for i := range s.pher {
		for j := range s.pher[i] {
			s.pher[i][j] *= 1 - acoEvap
			if s.pher[i][j] < acoPherFloor {
				s.pher[i][j] = acoPherFloor
			}
		}
	}
}

func (s *acoState) deposit(x []float64, w float64) {
	for i := range s.dims {
		s.pher[i][s.nearestLevel(i, x[i])] += w
	}
}

func (s *acoState) nearestLevel(i int, v float64) int {
	d := s.dims[i]
	levels := s.levels[i]
	if levels < 2 {
		return 0
	}
	frac := (v - d.lo) / (d.hi - d.lo)
	lvl := int(math.Round(frac * float64(levels-1)))
	if lvl < 0 {
		lvl = 0
	}
	if lvl > levels-1 {
		lvl = levels - 1
	}
	return lvl
}

func runACO(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult {
	rng := newSeededRng(seed)
	s := newAcoState(dims)
	best := runResult{bestF: math.Inf(1)}
	n := 0
	for n < budget {
		iterF := math.Inf(1)
		var iterX []float64
		for a := 0; a < acoAnts && n < budget; a++ {
			x := s.construct(rng)
			f := obj(x)
			n++
			if f < best.bestF {
				best.bestF = f
				best.bestX = append([]float64(nil), x...)
			}
			if f < iterF {
				iterF = f
				iterX = append([]float64(nil), x...)
			}
			if f <= target {
				best.trialsToTarget = n
				return best
			}
		}
		s.evaporate()
		s.deposit(iterX, acoDeposit)
		s.deposit(best.bestX, acoGlobalBoost*acoDeposit)
	}
	return best
}
