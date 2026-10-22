package main

import (
	"math"
	"math/rand"
)

const rastriginPerDimMax = 46.2144

type objective interface {
	Name() string
	Dim() int
	Lo() []float64
	Hi() []float64
	Evals() int
	True(x []float64) float64
	Eval(x []float64) float64
}

type rastrigin struct {
	d         int
	evalCount int
}

func (r *rastrigin) Name() string { return "rastrigin" }
func (r *rastrigin) Dim() int     { return r.d }

func (r *rastrigin) bounds() ([]float64, []float64) {
	lo := make([]float64, r.d)
	hi := make([]float64, r.d)
	for i := range lo {
		lo[i], hi[i] = -5.12, 5.12
	}
	return lo, hi
}

func (r *rastrigin) Lo() []float64 { lo, _ := r.bounds(); return lo }
func (r *rastrigin) Hi() []float64 { _, hi := r.bounds(); return hi }

func (r *rastrigin) raw(x []float64) float64 {
	s := 10 * float64(r.d)
	for _, v := range x {
		s += v*v - 10*math.Cos(2*math.Pi*v)
	}
	return s
}

func (r *rastrigin) True(x []float64) float64 { return r.raw(x) }
func (r *rastrigin) Evals() int               { return r.evalCount }
func (r *rastrigin) Eval(x []float64) float64 {
	r.evalCount++
	return r.raw(x)
}

type rosenbrock struct {
	d         int
	projector func([]float64) []float64
	evalCount int
}

func (r *rosenbrock) Name() string { return "rosenbrock" }
func (r *rosenbrock) Dim() int     { return r.d }

func (r *rosenbrock) bounds() ([]float64, []float64) {
	lo := make([]float64, r.d)
	hi := make([]float64, r.d)
	for i := range lo {
		lo[i], hi[i] = -5, 10
	}
	return lo, hi
}

func (r *rosenbrock) Lo() []float64 { lo, _ := r.bounds(); return lo }
func (r *rosenbrock) Hi() []float64 { _, hi := r.bounds(); return hi }

func (r *rosenbrock) raw(x []float64) float64 {
	s := 0.0
	for i := 0; i+1 < len(x); i++ {
		t := x[i+1] - x[i]*x[i]
		s += 100*t*t + (1-x[i])*(1-x[i])
	}
	return s
}

func (r *rosenbrock) True(x []float64) float64 {
	if r.projector == nil {
		return r.raw(x)
	}
	return r.raw(r.projector(x))
}

func (r *rosenbrock) Evals() int { return r.evalCount }

func (r *rosenbrock) Eval(x []float64) float64 {
	r.evalCount++
	return r.True(x)
}

func newRosenbrockDec(d int) *rosenbrock {
	r := &rosenbrock{d: d}
	lo, hi := r.bounds()
	r.projector = func(x []float64) []float64 { return exactDecreasing(x, lo, hi) }
	return r
}

type gameObj struct {
	margin float64
	d      int
	ref    []float64
	games  int
	rng    *rand.Rand
}

func newGameObj(d int, margin float64, seed int64) *gameObj {
	r := &rastrigin{d: d}
	lo, hi := r.bounds()
	rng := newRng(seed)
	ref := make([]float64, d)
	for i := range ref {
		ref[i] = lo[i] + rng.Float64()*(hi[i]-lo[i])
	}
	return &gameObj{margin: margin, d: d, ref: ref, rng: rng}
}

func (g *gameObj) Name() string { return "game" }
func (g *gameObj) Dim() int     { return g.d }

func (g *gameObj) bounds() ([]float64, []float64) {
	r := &rastrigin{d: g.d}
	return r.bounds()
}

func (g *gameObj) Lo() []float64 { lo, _ := g.bounds(); return lo }
func (g *gameObj) Hi() []float64 { _, hi := g.bounds(); return hi }

func (g *gameObj) skill(x []float64) float64 {
	r := &rastrigin{d: g.d}
	return 1 - r.raw(x)/(rastriginPerDimMax*float64(g.d))
}

func (g *gameObj) True(x []float64) float64 { return 1 - g.skill(x) }
func (g *gameObj) Evals() int               { return g.games }

func clampP(p float64) float64 {
	if p < clampPMin {
		return clampPMin
	}
	if p > clampPMax {
		return clampPMax
	}
	return p
}

func (g *gameObj) Play(a, b []float64, u float64) float64 {
	p := clampP(0.5 + (g.skill(a)-g.skill(b))*g.margin)
	g.games++
	if u < p {
		return 1
	}
	return 0
}

func (g *gameObj) evalDraws(x []float64, draws []float64) float64 {
	s := 0.0
	for _, u := range draws {
		s += g.Play(x, g.ref, u)
	}
	return 1 - s/float64(len(draws))
}

func (g *gameObj) Eval(x []float64) float64 {
	d := make([]float64, gamesPerCandidate)
	for i := range d {
		d[i] = g.rng.Float64()
	}
	return g.evalDraws(x, d)
}

func (g *gameObj) Reference() []float64 { return g.ref }
