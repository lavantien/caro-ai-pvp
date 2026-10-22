package main

import (
	"math"
	"testing"
)

func TestACOReproducibleAndBounded(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0]*x[0] + x[1]*x[1]
	}
	dims := quad2Dims()
	r1 := runACO(dims, obj, 9, 60, -1)
	calls = 0
	r2 := runACO(dims, obj, 9, 60, -1)
	if calls != 60 {
		t.Fatalf("aco calls %d want 60", calls)
	}
	if r1.bestF != r2.bestF || r1.trialsToTarget != r2.trialsToTarget {
		t.Fatalf("aco not reproducible: %+v vs %+v", r1, r2)
	}
}

func TestACOConvergesOnQuadratic(t *testing.T) {
	obj := func(x []float64) float64 {
		d0 := x[0] - 1
		d1 := x[1] + 1
		return d0*d0 + d1*d1
	}
	r := runACO([]dim{{"x", -4, 4, false}, {"y", -4, 4, false}}, obj, 4, 120, -1)
	if r.bestF > 0.5 {
		t.Fatalf("aco bestF %v want at or below 0.5", r.bestF)
	}
}

func TestACOIntegerDimsIntegral(t *testing.T) {
	dims := []dim{{"a", -2, 6, true}, {"b", 0.5, 3, false}}
	s := newAcoState(dims)
	rng := newSeededRng(5)
	for i := 0; i < 30; i++ {
		x := s.construct(rng)
		if x[0] != math.Trunc(x[0]) {
			t.Fatalf("integer dim got %v", x[0])
		}
	}
}

func TestACOPheromoneFocuses(t *testing.T) {
	dims := quad2Dims()
	s := newAcoState(dims)
	for i := 0; i < 50; i++ {
		s.evaporate()
		s.deposit([]float64{0, 0}, acoGlobalBoost*acoDeposit)
	}
	if s.pher[0][s.nearestLevel(0, 0)] <= 1.0 {
		t.Fatalf("deposited pheromone must exceed its initial value")
	}
}
