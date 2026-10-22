package main

import (
	"math"
	"testing"
)

func quad2Dims() []dim {
	return []dim{{"x", -5, 5, false}, {"y", -5, 5, false}}
}

func TestBudgetLoopReproducible(t *testing.T) {
	obj := func(x []float64) float64 {
		d0 := x[0] - 1
		d1 := x[1] + 2
		return d0*d0 + d1*d1
	}
	a := runBayes(quad2Dims(), obj, 7, 50, -1)
	b := runBayes(quad2Dims(), obj, 7, 50, -1)
	if a.bestF != b.bestF || a.trialsToTarget != b.trialsToTarget {
		t.Fatalf("runs diverged: %+v vs %+v", a, b)
	}
	for i := range a.bestX {
		if a.bestX[i] != b.bestX[i] {
			t.Fatalf("best points diverged at %d: %v vs %v", i, a.bestX[i], b.bestX[i])
		}
	}
}

func TestBudgetLoopRespectsBudget(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0] * x[0]
	}
	runBayes(quad2Dims(), obj, 3, 17, -1)
	if calls != 17 {
		t.Fatalf("objective calls %d want 17", calls)
	}
}

func TestFindsQuadraticMinimum(t *testing.T) {
	obj := func(x []float64) float64 {
		d0 := x[0] - 1
		d1 := x[1] + 2
		return d0*d0 + d1*d1
	}
	r := runBayes(quad2Dims(), obj, 11, 40, -1)
	if r.bestF > 1e-2 {
		t.Fatalf("bestF %v want below 1e-2", r.bestF)
	}
}

func TestIntegerDimsStayIntegral(t *testing.T) {
	dims := []dim{{"a", -2, 6, true}, {"b", 0.5, 3, false}}
	obj := func(x []float64) float64 {
		d0 := x[0] - 2
		d1 := x[1] - 1.5
		return d0*d0 + d1*d1
	}
	o := newOptimizer(dims, 5)
	for i := 0; i < 30; i++ {
		x := o.suggest()
		if x[0] != math.Trunc(x[0]) {
			t.Fatalf("integer dim got %v", x[0])
		}
		o.observe(x, obj(x))
	}
}
