package main

import (
	"math"
	"math/rand"
)

const (
	dePop       = 48
	deGens      = 400
	deWeight    = 0.7
	deCrossover = 0.9
	deUBound    = 12.0
	deULo       = -4.0
	deSLo       = 200.0
	deSHi       = 20000.0
)

type deOptions struct {
	Pop, Gens int
	F, CR     float64
	Lo, Hi    [6]float64
}

func defaultDEOptions() deOptions {
	var lo, hi [6]float64
	for j := 0; j < 5; j++ {
		lo[j], hi[j] = deULo, deUBound
	}
	lo[5], hi[5] = math.Log(deSLo), math.Log(deSHi)
	return deOptions{Pop: dePop, Gens: deGens, F: deWeight, CR: deCrossover, Lo: lo, Hi: hi}
}

type deResult struct {
	W    [5]float64
	S    float64
	Loss float64
	Gen  int
}

func deCandidate(x [6]float64) ([5]float64, float64) {
	var u [5]float64
	copy(u[:], x[:5])
	return weightsFromU(u), math.Exp(x[5])
}

func deLoss(d fitData, x [6]float64) float64 {
	w, s := deCandidate(x)
	return d.lossAt(w, s)
}

func fitDE(d fitData, opts deOptions, rng *rand.Rand) deResult {
	pop := make([][6]float64, opts.Pop)
	loss := make([]float64, opts.Pop)
	for i := range pop {
		for j := range pop[i] {
			pop[i][j] = opts.Lo[j] + rng.Float64()*(opts.Hi[j]-opts.Lo[j])
		}
		loss[i] = deLoss(d, pop[i])
	}
	bestIdx := 0
	for i := range loss {
		if loss[i] < loss[bestIdx] {
			bestIdx = i
		}
	}
	best := deResult{Loss: loss[bestIdx]}
	best.W, best.S = deCandidate(pop[bestIdx])
	for gen := 1; gen <= opts.Gens; gen++ {
		for i := range pop {
			r1, r2, r3 := dePick3(rng, opts.Pop, i)
			var trial [6]float64
			forced := rng.Intn(6)
			for j := range trial {
				trial[j] = pop[i][j]
				if j == forced || rng.Float64() < opts.CR {
					trial[j] = pop[r1][j] + opts.F*(pop[r2][j]-pop[r3][j])
					if trial[j] < opts.Lo[j] {
						trial[j] = opts.Lo[j]
					}
					if trial[j] > opts.Hi[j] {
						trial[j] = opts.Hi[j]
					}
				}
			}
			tloss := deLoss(d, trial)
			if tloss <= loss[i] {
				pop[i], loss[i] = trial, tloss
				if tloss < best.Loss {
					best.Loss = tloss
					best.W, best.S = deCandidate(trial)
					best.Gen = gen
				}
			}
		}
	}
	return best
}

func dePick3(rng *rand.Rand, n, skip int) (int, int, int) {
	used := map[int]bool{skip: true}
	pick := func() int {
		for {
			c := rng.Intn(n)
			if !used[c] {
				used[c] = true
				return c
			}
		}
	}
	return pick(), pick(), pick()
}
