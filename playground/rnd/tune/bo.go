package main

import (
	"math"
	"math/rand/v2"
	"slices"
)

type dim struct {
	name    string
	lo, hi  float64
	integer bool
}

type observation struct {
	x []float64
	f float64
}

type optimizer struct {
	dims  []dim
	rng   *rand.Rand
	obs   []observation
	bestF float64
	bestX []float64
	gp    *gp
	ymu   float64
	ysd   float64
}

func newSeededRng(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(tuneSeedHi, seed))
}

func newOptimizer(dims []dim, seed uint64) *optimizer {
	return &optimizer{dims: dims, rng: newSeededRng(seed), bestF: math.Inf(1)}
}

func clampScalar(d dim, v float64) float64 {
	if d.integer {
		v = math.Round(v)
	}
	v = math.Max(d.lo, math.Min(d.hi, v))
	if d.integer {
		v = math.Round(v)
	}
	return v
}

func clampPoint(dims []dim, x []float64) []float64 {
	out := append([]float64(nil), x...)
	for i, d := range dims {
		out[i] = clampScalar(d, out[i])
	}
	return out
}

func randomPoint(dims []dim, rng *rand.Rand) []float64 {
	x := make([]float64, len(dims))
	for i, d := range dims {
		x[i] = d.lo + (d.hi-d.lo)*rng.Float64()
	}
	return clampPoint(dims, x)
}

func (o *optimizer) suggest() []float64 {
	if len(o.obs) < nInit {
		return randomPoint(o.dims, o.rng)
	}
	o.fitGP()
	bestNorm := (o.bestF - o.ymu) / o.ysd
	bestEI := -1.0
	var pick []float64
	consider := func(raw []float64) {
		c := clampPoint(o.dims, raw)
		mu, sg := o.gp.predict(c)
		ei := expectedImprovement(mu, sg, bestNorm, acqXi)
		if ei > bestEI {
			bestEI = ei
			pick = c
		}
	}
	for i := 0; i < candidatePool; i++ {
		consider(randomPoint(o.dims, o.rng))
	}
	for i := 0; i < perturbPool; i++ {
		c := append([]float64(nil), o.bestX...)
		for j, d := range o.dims {
			c[j] += (2*o.rng.Float64() - 1) * perturbScale * (d.hi - d.lo)
		}
		consider(c)
	}
	if pick == nil {
		return randomPoint(o.dims, o.rng)
	}
	return pick
}

func (o *optimizer) fitGP() {
	xs := make([][]float64, len(o.obs))
	ys := make([]float64, len(o.obs))
	for i, ob := range o.obs {
		xs[i] = ob.x
		ys[i] = ob.f
	}
	o.ymu = 0
	for _, y := range ys {
		o.ymu += y
	}
	o.ymu /= float64(len(ys))
	ss := 0.0
	for _, y := range ys {
		d := y - o.ymu
		ss += d * d
	}
	o.ysd = math.Sqrt(ss / float64(len(ys)))
	if o.ysd < 1e-12 {
		o.ysd = 1
	}
	norm := make([]float64, len(ys))
	for i, y := range ys {
		norm[i] = (y - o.ymu) / o.ysd
	}
	lengths := make([]float64, len(o.dims))
	for i, d := range o.dims {
		lengths[i] = priorMeanFrac * (d.hi - d.lo)
		if lengths[i] <= 0 {
			lengths[i] = 1
		}
	}
	o.gp = newGP(len(o.dims), lengths, gpSigmaF, mapSigmaN)
	o.gp.fitMap(xs, norm, o.dims)
}

func (o *optimizer) observe(x []float64, f float64) {
	xc := clampPoint(o.dims, x)
	o.obs = append(o.obs, observation{x: xc, f: f})
	if f < o.bestF {
		o.bestF = f
		o.bestX = slices.Clone(xc)
	}
}

type runResult struct {
	bestX          []float64
	bestF          float64
	trialsToTarget int
}

func runBayes(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult {
	o := newOptimizer(dims, seed)
	for i := 1; i <= budget; i++ {
		x := o.suggest()
		f := obj(x)
		o.observe(x, f)
		if f <= target {
			return runResult{bestX: o.bestX, bestF: o.bestF, trialsToTarget: i}
		}
	}
	return runResult{bestX: o.bestX, bestF: o.bestF, trialsToTarget: 0}
}
