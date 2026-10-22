package main

import (
	"testing"
)

func TestRunDeterministic(t *testing.T) {
	o1 := &rastrigin{d: 6}
	o2 := &rastrigin{d: 6}
	c := smoothBaseCfg("base")
	r1 := run(c, o1, 42, rastriginTarget)
	r2 := run(c, o2, 42, rastriginTarget)
	if r1.TotalEvals != r2.TotalEvals || r1.BestTrue != r2.BestTrue {
		t.Fatalf("seed divergence: %v vs %v", r1.BestTrue, r2.BestTrue)
	}
	for i := range r1.Best {
		if r1.Best[i] != r2.Best[i] {
			t.Fatal("best point diverged")
		}
	}
}

func TestBaseDEFindsBasinRastrigin(t *testing.T) {
	bests := []float64{}
	for _, sd := range []int64{42, 43, 44, 45, 46} {
		o := &rastrigin{d: 6}
		r := run(smoothBaseCfg("base"), o, sd, rastriginTarget)
		if r.EvalsToTarget > r.TotalEvals {
			t.Fatal("target evals exceed total")
		}
		bests = append(bests, r.BestTrue)
	}
	t.Logf("base bests %v", bests)
	if medianFloat(bests) > 15 {
		t.Fatalf("median base best %v", medianFloat(bests))
	}
}

func TestShadeCracksRastrigin(t *testing.T) {
	bests := []float64{}
	c := smoothBaseCfg("shade")
	c.Shade = true
	for _, sd := range []int64{42, 43, 44, 45, 46} {
		o := &rastrigin{d: 6}
		r := run(c, o, sd, rastriginTarget)
		bests = append(bests, r.BestTrue)
	}
	t.Logf("shade bests %v", bests)
	if medianFloat(bests) > 1e-3 {
		t.Fatalf("median shade best %v", medianFloat(bests))
	}
}

func TestPickDistinct(t *testing.T) {
	rng := newRng(1)
	for trial := 0; trial < 100; trial++ {
		p := pickDistinct(rng, 10, 3, 3)
		seen := map[int]bool{3: true}
		for _, v := range p {
			if seen[v] {
				t.Fatalf("collision %v", p)
			}
			seen[v] = true
		}
	}
}

func TestDETrialShape(t *testing.T) {
	rng := newRng(2)
	pop := [][]float64{{0, 0, 0, 0, 0, 0}, {1, 1, 1, 1, 1, 1}, {0.5, 0.5, 0.5, 0.5, 0.5, 0.5}, {2, 2, 2, 2, 2, 2}, {1.5, 1.5, 1.5, 1.5, 1.5, 1.5}}
	diff := 0
	tr := deTrial(rng, pop, 0, deF, deCR)
	for j := range tr {
		if tr[j] != pop[0][j] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("trial identical to parent")
	}
}

func TestClipTo(t *testing.T) {
	x := []float64{-9, 5, 0}
	lo := []float64{-1, 0, 0}
	hi := []float64{1, 3, 1}
	clipTo(x, lo, hi)
	for i := range x {
		if x[i] < lo[i] || x[i] > hi[i] {
			t.Fatalf("clip failed %v", x)
		}
	}
}

func TestScalarGenerationNeverWorsensBest(t *testing.T) {
	o := &rastrigin{d: 6}
	e := &engine{cfg: smoothBaseCfg("base"), obj: o, rng: newRng(9)}
	e.lo, e.hi = o.Lo(), o.Hi()
	pop := make([][]float64, 8)
	loss := make([]float64, 8)
	for i := range pop {
		pop[i] = uniformIn(e.rng, e.lo, e.hi)
		loss[i] = o.Eval(pop[i])
	}
	best := argmin(loss)
	before := loss[best]
	for gen := 0; gen < 50; gen++ {
		pop, loss, best = e.scalarGeneration(pop, loss, best, nil, nil)
		if loss[best] > before+1e-12 {
			t.Fatalf("best loss rose %v to %v", before, loss[best])
		}
	}
}
