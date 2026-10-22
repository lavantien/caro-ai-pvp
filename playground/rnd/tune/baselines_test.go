package main

import (
	"math"
	"testing"
)

func TestGridReproduciblePerSeed(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0]*x[0] + x[1]*x[1]
	}
	r1 := runGrid(quad2Dims(), obj, 7, 30, -1)
	calls = 0
	r2 := runGrid(quad2Dims(), obj, 7, 30, -1)
	if calls != 30 {
		t.Fatalf("grid calls %d want 30", calls)
	}
	if r1.bestF != r2.bestF || r1.trialsToTarget != r2.trialsToTarget {
		t.Fatalf("grid not reproducible per seed: %+v vs %+v", r1, r2)
	}
}

func TestGridOffsetVariesWithSeed(t *testing.T) {
	dims := quad2Dims()
	a := gridLattice(dims, 5, 1)
	b := gridLattice(dims, 5, 2)
	if len(a) != 25 || len(b) != 25 {
		t.Fatalf("lattice sizes %d and %d want 25", len(a), len(b))
	}
	if a[0][0] == b[0][0] {
		t.Fatalf("seeds 1 and 2 produced the same first point %v", a[0])
	}
}

func TestGridLatticeInBoundsAndIntegral(t *testing.T) {
	dims := []dim{{"x", -4, 4, false}, {"k", 0, 2, true}}
	pts := gridLattice(dims, 6, 9)
	for _, p := range pts {
		if p[0] < -4 || p[0] > 4 {
			t.Fatalf("point %v outside bounds", p)
		}
		if p[1] != math.Floor(p[1]) {
			t.Fatalf("integer dim value %v not integral", p[1])
		}
	}
}

func TestGridLatticeUniformSpacing(t *testing.T) {
	dims := quad2Dims()
	pts := gridLattice(dims, 13, 4)
	spacing := 10.0 / 13.0
	for k := 1; k < 13; k++ {
		x := pts[k*13][0]
		prev := pts[(k-1)*13][0]
		if math.Abs((x-prev)-spacing) > 1e-12 {
			t.Fatalf("axis step %v want %v", x-prev, spacing)
		}
	}
}

func TestFittedLevels(t *testing.T) {
	cases := []struct {
		dims   []dim
		budget int
		want   int
	}{
		{quad2Dims(), 120, 10},
		{[]dim{{"x", 0, 1, false}, {"y", 0, 1, false}, {"z", 0, 1, false}}, 120, 4},
		{[]dim{{"x", 0, 1, false}, {"y", 0, 1, false}, {"k", 0, 2, true}}, 120, 6},
		{[]dim{{"x", 0, 1, false}}, 120, 13},
	}
	for _, c := range cases {
		got := fittedLevels(c.dims, c.budget)
		if got != c.want {
			t.Fatalf("fittedLevels(%d dims, budget %d) = %d want %d", len(c.dims), c.budget, got, c.want)
		}
	}
}

func TestGridFullBudgetCoversWholeLattice(t *testing.T) {
	calls := 0
	obj := func(x []float64) float64 {
		calls++
		return x[0]*x[0] + x[1]*x[1]
	}
	r := runGrid(quad2Dims(), obj, 3, 500, -1)
	if calls != gridLevels*gridLevels {
		t.Fatalf("grid calls %d want %d", calls, gridLevels*gridLevels)
	}
	if r.bestF > 0.5 {
		t.Fatalf("full-budget grid best %v want near 0", r.bestF)
	}
}

func TestGridTrialsMatchLatticeOrder(t *testing.T) {
	dims := []dim{{"x", 0, 10, false}}
	target := 0.5
	obj := func(x []float64) float64 { return math.Abs(x[0] - 4.5) }
	want := 0
	for i, p := range gridLattice(dims, 13, 5) {
		if obj(p) <= target {
			want = i + 1
			break
		}
	}
	if want == 0 {
		t.Fatal("lattice never hits target, test needs a reachable target")
	}
	r := runGrid(dims, obj, 5, 30, target)
	if r.trialsToTarget != want {
		t.Fatalf("trials to target %d want %d", r.trialsToTarget, want)
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
