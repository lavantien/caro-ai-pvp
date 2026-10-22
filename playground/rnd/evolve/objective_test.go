package main

import (
	"math"
	"testing"
)

func TestRastriginZero(t *testing.T) {
	r := &rastrigin{d: 6}
	x := make([]float64, 6)
	if v := r.True(x); v > 1e-9 {
		t.Fatalf("origin value %v", v)
	}
	if v := r.True(x); v < 0 {
		t.Fatalf("negative value %v", v)
	}
}

func TestRosenbrockOptimum(t *testing.T) {
	r := &rosenbrock{d: 6}
	x := []float64{1, 1, 1, 1, 1, 1}
	if v := r.True(x); math.Abs(v) > 1e-9 {
		t.Fatalf("valley value %v", v)
	}
}

func TestRosenbrockDecProjects(t *testing.T) {
	r := newRosenbrockDec(6)
	x := []float64{-4, -3, -2, -1, 0, 1}
	v := r.True(x)
	if v <= 0 {
		t.Fatalf("unsorted diagonal scored %v", v)
	}
	eq := []float64{2, 2, 2, 2, 2, 2}
	if r.True(eq) != r.raw(eq) {
		t.Fatal("feasible diagonal altered")
	}
}

func TestGameObjCountsGames(t *testing.T) {
	g := newGameObj(6, marginMid, 7)
	if g.Evals() != 0 {
		t.Fatalf("fresh counter %d", g.Evals())
	}
	lo, hi := g.Lo(), g.Hi()
	a := make([]float64, 6)
	for i := range a {
		a[i] = lo[i] + 0.5*(hi[i]-lo[i])
	}
	b := make([]float64, 6)
	copy(b, a)
	g.Play(a, b, 0.5)
	if g.Evals() != 1 {
		t.Fatalf("counter %d", g.Evals())
	}
	sa, sb := g.skill(a), g.skill(b)
	if sa != sb {
		t.Fatal("identical points differ in skill")
	}
	draws := make([]float64, gamesPerCandidate)
	for i := range draws {
		draws[i] = 0.5
	}
	before := g.Evals()
	g.evalDraws(a, draws)
	if g.Evals()-before != gamesPerCandidate {
		t.Fatalf("draw budget %d", g.Evals()-before)
	}
}

func TestGameObjWinProbMonotone(t *testing.T) {
	g := newGameObj(6, marginStrong, 11)
	lo, hi := g.Lo(), g.Hi()
	weak := make([]float64, 6)
	for i := range weak {
		weak[i] = hi[i]
	}
	strong := make([]float64, 6)
	for i := range strong {
		strong[i] = lo[i] + 0.5*(hi[i]-lo[i])
	}
	strong[0] = 0
	wins := 0
	for i := 0; i < 2000; i++ {
		wins += int(g.Play(strong, weak, float64(i)/2000))
	}
	if wins < 1700 {
		t.Fatalf("strong beat weak only %d of 2000", wins)
	}
}

func TestGameObjSkillBounds(t *testing.T) {
	g := newGameObj(6, marginMid, 3)
	_, hi := g.Lo(), g.Hi()
	x := make([]float64, 6)
	if v := g.True(x); v > 0.001 || v < 0 {
		t.Fatalf("origin true loss %v", v)
	}
	y := make([]float64, 6)
	for i := range y {
		y[i] = hi[i]
	}
	if g.True(y) > 1.001 || g.True(y) < 0 {
		t.Fatalf("corner true loss %v", g.True(y))
	}
}
