package main

import (
	"math"
	"math/rand"
	"sort"
)

type shadeState struct {
	dim      int
	lo, hi   []float64
	initPop  int
	memF     []float64
	memCR    []float64
	memIdx   int
	archive  [][]float64
	lastF    []float64
	lastCR   []float64
	succF    []float64
	succCR   []float64
	succW    []float64
	stall    int
	prevBest float64
}

func newShadeState(pop, dim int, lo, hi []float64) *shadeState {
	s := &shadeState{dim: dim, lo: lo, hi: hi, initPop: pop}
	s.memF = make([]float64, shadeMemSize)
	s.memCR = make([]float64, shadeMemSize)
	for i := range s.memF {
		s.memF[i], s.memCR[i] = 0.5, 0.5
	}
	s.lastF = make([]float64, pop)
	s.lastCR = make([]float64, pop)
	s.prevBest = math.Inf(1)
	return s
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

func (s *shadeState) trial(pop [][]float64, loss []float64, i int, rng *rand.Rand) []float64 {
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
	base := pop[i]
	pbX := pop[pb]
	var r1X, r2X []float64
	r1X = pop[r1]
	if r2 < len(pop) {
		r2X = pop[r2]
	} else {
		r2X = s.archive[r2-len(pop)]
	}
	forced := rng.Intn(s.dim)
	trial := make([]float64, s.dim)
	copy(trial, base)
	for j := 0; j < s.dim; j++ {
		if j == forced || rng.Float64() < cr {
			trial[j] = base[j] + f*(pbX[j]-base[j]) + f*(r1X[j]-r2X[j])
		}
	}
	return trial
}

func (s *shadeState) record(i int, delta float64, accept bool) {
	if accept && delta < 0 {
		s.succF = append(s.succF, s.lastF[i])
		s.succCR = append(s.succCR, s.lastCR[i])
		s.succW = append(s.succW, -delta)
	}
}

func (s *shadeState) pushArchive(x []float64) {
	s.archive = append(s.archive, append([]float64(nil), x...))
}

func (s *shadeState) trimArchive(rng *rand.Rand) {
	for len(s.archive) > s.initPop {
		j := rng.Intn(len(s.archive))
		s.archive = append(s.archive[:j], s.archive[j+1:]...)
	}
}

func (s *shadeState) diversity(pop [][]float64) float64 {
	if len(pop) < 2 {
		return 0
	}
	span := 0.0
	for i := range s.lo {
		span += (s.hi[i] - s.lo[i]) * (s.hi[i] - s.lo[i])
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

func (s *shadeState) restartAround(pop [][]float64, loss []float64, best int, rng *rand.Rand) {
	for i := range pop {
		if i == best {
			continue
		}
		for j := range pop[i] {
			pop[i][j] = math.Max(s.lo[j], math.Min(s.hi[j], pop[best][j]+rng.NormFloat64()*0.1*(s.hi[j]-s.lo[j])))
		}
		loss[i] = math.Inf(1)
	}
	s.archive = nil
	s.stall = 0
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

func (s *shadeState) afterGeneration(rng *rand.Rand, gen, maxGens int, pop [][]float64, loss []float64, best int) ([][]float64, []float64, int) {
	if len(s.succF) > 0 {
		s.memF[s.memIdx] = clampF(lehmerMean(s.succF, s.succW))
		s.memCR[s.memIdx] = clampCR(weightedMean(s.succCR, s.succW))
		s.memIdx = (s.memIdx + 1) % shadeMemSize
		s.succF, s.succCR, s.succW = nil, nil, nil
	}
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
	n := lpsrNext(gen, maxGens, s.initPop, shadeNMin)
	if n < len(pop) {
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
		np := make([][]float64, len(keep))
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
	return pop, loss, best
}
