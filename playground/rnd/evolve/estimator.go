package main

import (
	"math"
	"math/rand"
	"sort"
)

func sigmoidStable(x float64) float64 {
	if x >= 0 {
		return 1 / (1 + math.Exp(-x))
	}
	e := math.Exp(x)
	return e / (1 + e)
}

type pairResult struct {
	wins  float64
	games float64
}

func swissPairs(n, k int, rng *rand.Rand) [][2]int {
	seen := make(map[[2]int]bool)
	for i := 0; i < n; i++ {
		need := k
		for t := 0; t < 8*k && need > 0; t++ {
			j := rng.Intn(n)
			if j == i {
				continue
			}
			key := [2]int{min(i, j), max(i, j)}
			if seen[key] {
				continue
			}
			seen[key] = true
			need--
		}
	}
	pairs := make([][2]int, 0, len(seen))
	for k := range seen {
		pairs = append(pairs, k)
	}
	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a][0] != pairs[b][0] {
			return pairs[a][0] < pairs[b][0]
		}
		return pairs[a][1] < pairs[b][1]
	})
	return pairs
}

func sprtBounds() (float64, float64) {
	lo := math.Log(sprtBeta / (1 - sprtAlpha))
	hi := math.Log((1 - sprtBeta) / sprtAlpha)
	return lo, hi
}

func drawFor(crn []float64, i int, rng *rand.Rand) float64 {
	if crn != nil {
		return crn[i%len(crn)]
	}
	return rng.Float64()
}

func playPairSprt(g *gameObj, a, b []float64, crn []float64, rng *rand.Rand) pairResult {
	lo, hi := sprtBounds()
	llr, w, n := 0.0, 0.0, 0
	for n < sprtMaxGames {
		if g.Play(a, b, drawFor(crn, n, rng)) == 1 {
			w++
			llr += math.Log(sprtP1 / 0.5)
		} else {
			llr += math.Log((1 - sprtP1) / 0.5)
		}
		n++
		if llr >= hi || llr <= lo {
			break
		}
	}
	return pairResult{wins: w, games: float64(n)}
}

func playPairFixed(g *gameObj, a, b []float64, crn []float64, rng *rand.Rand) pairResult {
	w := 0.0
	for i := 0; i < gamesPerCandidate; i++ {
		w += g.Play(a, b, drawFor(crn, i, rng))
	}
	return pairResult{wins: w, games: float64(gamesPerCandidate)}
}

func btFit(n int, res map[[2]int]pairResult) []float64 {
	s := make([]float64, n)
	wins := make([][]float64, n)
	games := make([][]float64, n)
	for i := range wins {
		wins[i] = make([]float64, n)
		games[i] = make([]float64, n)
	}
	for k, v := range res {
		a, b := k[0], k[1]
		wins[a][b] += v.wins
		games[a][b] += v.games
		wins[b][a] += v.games - v.wins
		games[b][a] += v.games
	}
	for range btIters {
		for i := 0; i < n; i++ {
			exp, act, wt := 0.0, 0.0, 0.0
			for j := 0; j < n; j++ {
				if games[i][j] == 0 {
					continue
				}
				exp += games[i][j] * sigmoidStable(s[i]-s[j])
				act += wins[i][j]
				wt += games[i][j]
			}
			if wt == 0 {
				continue
			}
			s[i] += btStep * (act - exp) / wt
		}
	}
	return s
}

func (e *engine) rankPool(pool [][]float64) []float64 {
	g := e.obj.(*gameObj)
	pairs := swissPairs(len(pool), swissOpponents, e.rng)
	var crn []float64
	if e.cfg.CRN {
		crn = e.freshDraws()
	}
	res := make(map[[2]int]pairResult, len(pairs))
	for _, p := range pairs {
		if e.cfg.Estimator == "btsprt" {
			res[p] = playPairSprt(g, pool[p[0]], pool[p[1]], crn, e.rng)
		} else {
			res[p] = playPairFixed(g, pool[p[0]], pool[p[1]], crn, e.rng)
		}
		if !e.evalBudgetLeft() {
			break
		}
	}
	return btFit(len(pool), res)
}

func (e *engine) rankGeneration(pop [][]float64, s *shadeState) ([][]float64, []float64, int) {
	loss := negStrengths(e.rankPool(pop))
	trials := make([][]float64, 0, len(pop))
	tidx := make([]int, 0, len(pop))
	for i := range pop {
		if !e.evalBudgetLeft() {
			break
		}
		trial := e.makeTrial(pop, loss, i, s)
		if e.screen != nil && e.screen.skip(trial, loss, len(trials), len(pop)) {
			continue
		}
		trials = append(trials, trial)
		tidx = append(tidx, i)
	}
	pool := append(append([][]float64{}, pop...), trials...)
	str2 := e.rankPool(pool)
	order := make([]int, len(pool))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return str2[order[a]] > str2[order[b]] })
	np := make([][]float64, len(pop))
	nl := make([]float64, len(pop))
	survived := make([]bool, len(pool))
	for k := 0; k < len(pop); k++ {
		np[k] = pool[order[k]]
		nl[k] = -str2[order[k]]
		if order[k] >= len(pop) {
			survived[order[k]] = true
		}
	}
	nb := argmin(nl)
	if s != nil {
		for k, pi := range tidx {
			if survived[len(pop)+k] {
				s.record(pi, -1, true)
			}
		}
	}
	return np, nl, nb
}

func negStrengths(s []float64) []float64 {
	out := make([]float64, len(s))
	for i, v := range s {
		out[i] = -v
	}
	return out
}

func argmin(v []float64) int {
	b := 0
	for i := range v {
		if v[i] < v[b] {
			b = i
		}
	}
	return b
}
