package main

import (
	"math"
	"testing"
)

func TestSurrogateNoSkipBeforeWarmup(t *testing.T) {
	s := newSurrogate(2, []float64{0, 0}, []float64{1, 1})
	for i := 0; i < screenWarmup-1; i++ {
		s.observe([]float64{0.5, 0.5}, 10)
	}
	if s.skip([]float64{0.5, 0.5}, nil, 0, 8) {
		t.Fatal("skipped under warmup")
	}
}

func TestSurrogateSkipsClearLoss(t *testing.T) {
	s := newSurrogate(1, []float64{0}, []float64{1})
	for i := 0; i < screenWarmup; i++ {
		x := float64(i%20) / 20
		s.observe([]float64{x}, 10*x)
	}
	if !s.skip([]float64{0.95}, nil, 0, 8) {
		t.Fatal("clearly bad point not skipped")
	}
	if s.skip([]float64{0.0}, nil, 0, 8) {
		t.Fatal("clearly good point skipped")
	}
}

func TestSurrogateSkipCap(t *testing.T) {
	s := newSurrogate(1, []float64{0}, []float64{1})
	for i := 0; i < screenWarmup; i++ {
		s.observe([]float64{0.5}, 1)
	}
	if s.skip([]float64{0.99}, nil, 7, 8) {
		t.Fatal("skip cap ignored")
	}
}

func TestSolveRidgeLinear(t *testing.T) {
	xs := [][]float64{{1, 0, 0}, {1, 1, 0}, {1, 0, 1}, {1, 1, 1}, {1, 0.5, 0.25}}
	ys := make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = 2 + 3*x[1] - 1*x[2]
	}
	theta := solveRidge(xs, ys, 1e-9)
	for i, x := range xs {
		pred := theta[0] + theta[1]*x[1] + theta[2]*x[2]
		if math.Abs(pred-ys[i]) > 1e-6 {
			t.Fatalf("ridge residual %v", pred-ys[i])
		}
	}
}

func TestQuantile(t *testing.T) {
	q := quantile([]float64{1, 2, 3, 4, 5}, 0.5)
	if q != 3 {
		t.Fatalf("median %v", q)
	}
	if quantile(nil, 0.5) != 0 {
		t.Fatal("empty quantile")
	}
}

func TestSurrogateHistoryCap(t *testing.T) {
	s := newSurrogate(1, []float64{0}, []float64{1})
	for i := 0; i < screenMaxHistory()+50; i++ {
		s.observe([]float64{0.5}, float64(i%10))
	}
	if len(s.ys) > screenMaxHistory() {
		t.Fatalf("history %d", len(s.ys))
	}
}
