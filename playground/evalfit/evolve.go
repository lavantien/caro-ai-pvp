package main

import (
	"math"
	"math/rand"
	"sort"
)

const (
	shadePop       = 48
	shadeGens      = 400
	shadeUBound    = 12.0
	shadeULo       = -4.0
	shadeSLo       = 200.0
	shadeSHi       = 20000.0
	shadeMemSize   = 6
	shadePBestFrac = 0.15
	shadeNMin      = 4
	shadeStallGens = 25
	shadeDivEps    = 1e-6
)

type deOptions struct {
	Pop, Gens int
	Lo, Hi    [6]float64
}

func defaultDEOptions() deOptions {
	var lo, hi [6]float64
	for j := 0; j < 5; j++ {
		lo[j], hi[j] = shadeULo, shadeUBound
	}
	lo[5], hi[5] = math.Log(shadeSLo), math.Log(shadeSHi)
	return deOptions{Pop: shadePop, Gens: shadeGens, Lo: lo, Hi: hi}
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

func cauchy(rng *rand.Rand, loc, scale float64) float64 {
	u := rng.Float64()
	return loc + scale*math.Tan(math.Pi*(u-0.5))
}

func clampF(v float64) float64 {
	if v <= 0 {
		v = -v
	}
	if v >= 1 {
		return 1
	}
	if v == 0 {
		return 0.05
	}
	return v
}

func clampCR(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func lehmerMean(v, w []float64) float64 {
	num, den := 0.0, 0.0
	for i := range v {
		num += w[i] * v[i] * v[i]
		den += w[i] * v[i]
	}
	if den == 0 {
		return 0.5
	}
	return num / den
}

func weightedMean(v, w []float64) float64 {
	num, den := 0.0, 0.0
	for i := range v {
		num += w[i] * v[i]
		den += w[i]
	}
	if den == 0 {
		return 0.5
	}
	return num / den
}

type shadeState struct {
	opts          deOptions
	initPop       int
	memF, memCR   [shadeMemSize]float64
	memIdx        int
	archive       [][6]float64
	lastF, lastCR []float64
	succF, succCR []float64
	succW         []float64
	stall         int
	prevBest      float64
}

func newShadeState(pop int, opts deOptions) *shadeState {
	s := &shadeState{opts: opts, initPop: pop, prevBest: math.Inf(1)}
	for i := range s.memF {
		s.memF[i], s.memCR[i] = 0.5, 0.5
	}
	s.lastF = make([]float64, pop)
	s.lastCR = make([]float64, pop)
	return s
}

func (s *shadeState) record(i int, delta float64, accept bool) {
	if accept && delta < 0 {
		s.succF = append(s.succF, s.lastF[i])
		s.succCR = append(s.succCR, s.lastCR[i])
		s.succW = append(s.succW, -delta)
	}
}

func (s *shadeState) afterSuccesses() {
	if len(s.succF) == 0 {
		return
	}
	s.memF[s.memIdx] = clampF(lehmerMean(s.succF, s.succW))
	s.memCR[s.memIdx] = clampCR(weightedMean(s.succCR, s.succW))
	s.memIdx = (s.memIdx + 1) % shadeMemSize
	s.succF, s.succCR, s.succW = nil, nil, nil
}

func (s *shadeState) pushArchive(x [6]float64) {
	s.archive = append(s.archive, x)
}

func (s *shadeState) trimArchive(rng *rand.Rand) {
	for len(s.archive) > s.initPop {
		j := rng.Intn(len(s.archive))
		s.archive = append(s.archive[:j], s.archive[j+1:]...)
	}
}

func (s *shadeState) pickPBest(loss []float64, rng *rand.Rand) int {
	order := make([]int, len(loss))
	for i := range order {
		order[i] = i
	}
	for i := len(order) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	k := int(math.Ceil(shadePBestFrac * float64(len(loss))))
	if k < 1 {
		k = 1
	}
	best := order[0]
	for _, i := range order[:k] {
		if loss[i] < loss[best] {
			best = i
		}
	}
	return best
}

func (s *shadeState) trial(pop [][6]float64, loss []float64, i int, rng *rand.Rand) [6]float64 {
	mi := rng.Intn(shadeMemSize)
	f := clampF(cauchy(rng, s.memF[mi], 0.1))
	cr := clampCR(s.memCR[mi] + rng.NormFloat64()*0.1)
	s.lastF[i], s.lastCR[i] = f, cr
	pb := s.pickPBest(loss, rng)
	pool := len(pop) + len(s.archive)
	r1 := i
	for r1 == i || r1 == pb {
		r1 = rng.Intn(len(pop))
	}
	r2 := i
	for r2 == i || r2 == r1 || r2 == pb {
		r2 = rng.Intn(pool)
	}
	var r2X [6]float64
	if r2 < len(pop) {
		r2X = pop[r2]
	} else {
		r2X = s.archive[r2-len(pop)]
	}
	forced := rng.Intn(6)
	var out [6]float64
	copy(out[:], pop[i][:])
	for j := 0; j < 6; j++ {
		if j == forced || rng.Float64() < cr {
			out[j] = pop[i][j] + f*(pop[pb][j]-pop[i][j]) + f*(pop[r1][j]-r2X[j])
			if out[j] < s.opts.Lo[j] {
				out[j] = s.opts.Lo[j]
			}
			if out[j] > s.opts.Hi[j] {
				out[j] = s.opts.Hi[j]
			}
		}
	}
	return out
}

func acceptShade(pop [][6]float64, loss []float64, best *int, i int, trial [6]float64, tloss float64) bool {
	if tloss > loss[i] {
		return false
	}
	pop[i], loss[i] = trial, tloss
	if tloss < loss[*best] {
		*best = i
	}
	return true
}

func shadeReduce(pop [][6]float64, loss []float64, best, n int) ([][6]float64, []float64, int) {
	if n >= len(pop) {
		return pop, loss, best
	}
	order := make([]int, len(pop))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return loss[order[a]] < loss[order[b]] })
	keep := order[:n]
	keptBest := false
	for _, idx := range keep {
		if idx == best {
			keptBest = true
		}
	}
	if !keptBest {
		keep[len(keep)-1] = best
	}
	np := make([][6]float64, len(keep))
	nl := make([]float64, len(keep))
	nb := 0
	for k, idx := range keep {
		np[k], nl[k] = pop[idx], loss[idx]
		if idx == best {
			nb = k
		}
	}
	return np, nl, nb
}

func lpsrNext(gen, maxGens, initPop, nMin int) int {
	if maxGens <= 0 {
		return initPop
	}
	t := float64(gen) / float64(maxGens)
	n := int(math.Round(float64(nMin) + float64(initPop-nMin)*(1-t)))
	if n < nMin {
		n = nMin
	}
	if n > initPop {
		n = initPop
	}
	return n
}

func (s *shadeState) diversity(pop [][6]float64) float64 {
	if len(pop) < 2 {
		return 0
	}
	span := 0.0
	for i := range s.opts.Lo {
		span += (s.opts.Hi[i] - s.opts.Lo[i]) * (s.opts.Hi[i] - s.opts.Lo[i])
	}
	if span == 0 {
		return 0
	}
	sum, cnt := 0.0, 0
	for i := 0; i < len(pop); i++ {
		for j := i + 1; j < len(pop); j++ {
			for k := range pop[i] {
				t := pop[i][k] - pop[j][k]
				sum += t * t
			}
			cnt++
		}
	}
	return (sum / float64(cnt)) / span
}

func (s *shadeState) restartAround(pop [][6]float64, loss []float64, best int, rng *rand.Rand) {
	for i := range pop {
		if i == best {
			continue
		}
		for j := range pop[i] {
			pop[i][j] = math.Max(s.opts.Lo[j], math.Min(s.opts.Hi[j], pop[best][j]+rng.NormFloat64()*0.1*(s.opts.Hi[j]-s.opts.Lo[j])))
		}
		loss[i] = math.Inf(1)
	}
	s.archive = nil
	s.stall = 0
}

func (s *shadeState) afterGeneration(rng *rand.Rand, gen int, pop [][6]float64, loss []float64, best int) ([][6]float64, []float64, int) {
	s.afterSuccesses()
	s.trimArchive(rng)
	if loss[best] >= s.prevBest-shadeDivEps {
		s.stall++
	} else {
		s.stall = 0
	}
	s.prevBest = loss[best]
	if s.stall >= shadeStallGens && s.diversity(pop) < shadeDivEps {
		s.restartAround(pop, loss, best, rng)
		return pop, loss, best
	}
	return shadeReduce(pop, loss, best, lpsrNext(gen, s.opts.Gens, s.initPop, shadeNMin))
}

func fitShade(d fitData, opts deOptions, rng *rand.Rand) deResult {
	pop := make([][6]float64, opts.Pop)
	loss := make([]float64, opts.Pop)
	for i := range pop {
		for j := range pop[i] {
			pop[i][j] = opts.Lo[j] + rng.Float64()*(opts.Hi[j]-opts.Lo[j])
		}
		loss[i] = deLoss(d, pop[i])
	}
	best := 0
	for i := range loss {
		if loss[i] < loss[best] {
			best = i
		}
	}
	s := newShadeState(opts.Pop, opts)
	result := deResult{Loss: loss[best]}
	result.W, result.S = deCandidate(pop[best])
	for gen := 1; gen <= opts.Gens; gen++ {
		for i := range pop {
			trial := s.trial(pop, loss, i, rng)
			tloss := deLoss(d, trial)
			delta := tloss - loss[i]
			parent := pop[i]
			accepted := acceptShade(pop, loss, &best, i, trial, tloss)
			s.record(i, delta, accepted)
			if accepted {
				s.pushArchive(parent)
				if tloss < result.Loss {
					result.Loss = tloss
					result.W, result.S = deCandidate(trial)
					result.Gen = gen
				}
			}
		}
		pop, loss, best = s.afterGeneration(rng, gen, pop, loss, best)
		if best >= len(pop) {
			best = 0
			for i := range loss {
				if loss[i] < loss[best] {
					best = i
				}
			}
		}
	}
	return result
}
