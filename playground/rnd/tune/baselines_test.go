package main

import (
	"math"
	"testing"
)

func TestGridDeterministicAndBounded(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0]*x[0] + x[1]*x[1]
	}
	dims := []dim{{"x", -4, 4, false}, {"y", -4, 4, false}}
	r1 := runGrid(dims, obj, 30, -1)
	calls = 0
	r2 := runGrid(dims, obj, 30, -1)
	if calls != 30 {
		t.Fatalf("grid calls %d want 30", calls)
	}
	if r1.bestF != r2.bestF {
		t.Fatalf("grid not deterministic: %v vs %v", r1.bestF, r2.bestF)
	}
}

func TestGridFullBudgetCoversWholeGrid(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0]*x[0] + x[1]*x[1]
	}
	dims := []dim{{"x", -4, 4, false}, {"y", -4, 4, false}}
	r := runGrid(dims, obj, 500, -1)
	if calls != gridLevels*gridLevels {
		t.Fatalf("grid calls %d want %d", calls, gridLevels*gridLevels)
	}
	if r.bestF > 1e-9 {
		t.Fatalf("full-budget grid best %v want 0", r.bestF)
	}
}

func TestRandomReproducible(t *testing.T) {
	obj := func(x []float64) float64 { return x[0] * x[0] }
	r1 := runRandom(quad2Dims(), obj, 13, 25, -1)
	r2 := runRandom(quad2Dims(), obj, 13, 25, -1)
	if r1.bestF != r2.bestF || r1.trialsToTarget != r2.trialsToTarget {
		t.Fatalf("random not reproducible: %+v vs %+v", r1, r2)
	}
}

func TestTargetDetectionIndex(t *testing.T) {
	dims := []dim{{"x", 0, 10, false}}
	obj := func(x []float64) float64 { return math.Abs(x[0] - 4.5) }
	r := runGrid(dims, obj, 30, 0.5)
	if r.trialsToTarget != 6 {
		t.Fatalf("trials to target %d want 6", r.trialsToTarget)
	}
}
