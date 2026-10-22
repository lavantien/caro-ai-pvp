package main

import "math/rand"

func newRng(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

func uniformIn(rng *rand.Rand, lo, hi []float64) []float64 {
	x := make([]float64, len(lo))
	for i := range x {
		x[i] = lo[i] + rng.Float64()*(hi[i]-lo[i])
	}
	return x
}

func clipTo(x []float64, lo, hi []float64) {
	for i := range x {
		if x[i] < lo[i] {
			x[i] = lo[i]
		}
		if x[i] > hi[i] {
			x[i] = hi[i]
		}
	}
}

func pickDistinct(rng *rand.Rand, n, skip int, k int) []int {
	used := make(map[int]bool, k+1)
	used[skip] = true
	out := make([]int, 0, k)
	for len(out) < k {
		c := rng.Intn(n)
		if used[c] {
			continue
		}
		used[c] = true
		out = append(out, c)
	}
	return out
}

func deTrial(rng *rand.Rand, pop [][]float64, i int, f, cr float64) []float64 {
	d := len(pop[i])
	rix := pickDistinct(rng, len(pop), i, 3)
	forced := rng.Intn(d)
	trial := make([]float64, d)
	copy(trial, pop[i])
	for j := 0; j < d; j++ {
		if j == forced || rng.Float64() < cr {
			trial[j] = pop[rix[0]][j] + f*(pop[rix[1]][j]-pop[rix[2]][j])
		}
	}
	return trial
}

type runResult struct {
	Cfg           string
	Obj           string
	Seed          int64
	Best          []float64
	BestTrue      float64
	EvalsToTarget int
	TotalEvals    int
	Hit           bool
}

type engine struct {
	cfg    cfg
	obj    objective
	rng    *rand.Rand
	lo     []float64
	hi     []float64
	screen *surrogate
}

func (e *engine) evalBudgetLeft() bool { return e.obj.Evals() < e.cfg.Budget }

func (e *engine) scalarEval(x []float64, draws []float64) float64 {
	if g, ok := e.obj.(*gameObj); ok {
		return g.evalDraws(x, draws)
	}
	return e.obj.Eval(x)
}

func (e *engine) freshDraws() []float64 {
	d := make([]float64, gamesPerCandidate)
	for i := range d {
		d[i] = e.rng.Float64()
	}
	return d
}

func (e *engine) makeTrial(pop [][]float64, loss []float64, i int, s *shadeState) []float64 {
	var trial []float64
	if s != nil {
		trial = s.trial(pop, loss, i, e.rng)
	} else {
		trial = deTrial(e.rng, pop, i, deF, deCR)
	}
	if e.cfg.Repair {
		repairBox(trial, e.lo, e.hi)
	} else {
		clipTo(trial, e.lo, e.hi)
	}
	return trial
}

func run(c cfg, o objective, seed int64, target float64) runResult {
	e := &engine{cfg: c, obj: o, rng: newRng(seed)}
	e.lo, e.hi = o.Lo(), o.Hi()
	if c.Surrogate {
		e.screen = newSurrogate(o.Dim(), e.lo, e.hi)
	}
	var s *shadeState
	if c.Shade {
		s = newShadeState(c.Pop, o.Dim(), e.lo, e.hi)
	}
	pop := make([][]float64, c.Pop)
	loss := make([]float64, c.Pop)
	draws := e.freshDraws()
	for i := range pop {
		pop[i] = uniformIn(e.rng, e.lo, e.hi)
		loss[i] = e.scalarEval(pop[i], draws)
	}
	best := argmin(loss)
	res := runResult{Cfg: c.Name, Obj: o.Name(), Seed: seed, Best: append([]float64(nil), pop[best]...), BestTrue: o.True(pop[best]), TotalEvals: o.Evals()}
	if o.True(pop[best]) <= target {
		res.Hit = true
		res.EvalsToTarget = o.Evals()
		return res
	}
	for gen := 1; gen <= c.Gens && e.evalBudgetLeft(); gen++ {
		crn := e.freshDraws()
		if !c.CRN {
			crn = nil
		}
		if isRankMode(c) {
			pop, loss, best = e.rankGeneration(pop, s)
		} else {
			pop, loss, best = e.scalarGeneration(pop, loss, best, s, crn)
		}
		if s != nil {
			pop, loss, best = s.afterGeneration(e.rng, gen, c.Gens, pop, loss, best)
		}
		if best >= len(pop) {
			best = argmin(loss)
		}
		res.Best = append([]float64(nil), pop[best]...)
		res.BestTrue = o.True(pop[best])
		res.TotalEvals = o.Evals()
		if res.BestTrue <= target {
			res.Hit = true
			res.EvalsToTarget = o.Evals()
			return res
		}
	}
	res.TotalEvals = o.Evals()
	return res
}

func (e *engine) scalarGeneration(pop [][]float64, loss []float64, best int, s *shadeState, crn []float64) ([][]float64, []float64, int) {
	skipped := 0
	for i := range pop {
		if !e.evalBudgetLeft() {
			break
		}
		trial := e.makeTrial(pop, loss, i, s)
		if e.screen != nil && e.screen.skip(trial, loss, skipped, len(pop)) {
			skipped++
			continue
		}
		draws := e.freshDraws()
		if crn != nil {
			draws = crn
		}
		tloss := e.scalarEval(trial, draws)
		if e.screen != nil {
			e.screen.observe(trial, tloss)
		}
		delta := tloss - loss[i]
		accept := tloss <= loss[i]
		if s != nil {
			s.record(i, delta, accept)
		}
		if accept {
			if s != nil {
				s.pushArchive(pop[i])
			}
			pop[i], loss[i] = trial, tloss
			if tloss < loss[best] {
				best = i
			}
		}
	}
	return pop, loss, best
}
