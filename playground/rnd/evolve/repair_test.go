package main

import (
	"math"
	"testing"
)

func TestPavaNonincreasing(t *testing.T) {
	x := []float64{0, 5, 1, 4, 2, 3}
	pavaNonincreasing(x)
	for i := 0; i+1 < len(x); i++ {
		if x[i] < x[i+1] {
			t.Fatalf("still increasing %v", x)
		}
	}
	mean := (0.0 + 5 + 1 + 4 + 2 + 3) / 6
	if math.Abs(x[0]-mean) > 1e-12 {
		t.Fatalf("fully increasing chain must pool to the mean, got %v want %v", x[0], mean)
	}
}

func TestRepairBoxHoldsBox(t *testing.T) {
	x := []float64{-9, 6, 0.5}
	lo := []float64{-2, -2, -2}
	hi := []float64{2, 2, 2}
	repairBox(x, lo, hi)
	for i := range x {
		if x[i] < lo[i] || x[i] > hi[i] {
			t.Fatalf("box violated %v", x)
		}
	}
	for i := 0; i+1 < len(x); i++ {
		if x[i] < x[i+1]-1e-9 {
			t.Fatalf("mono violated %v", x)
		}
	}
}

func TestRepairBoxIdempotent(t *testing.T) {
	x := []float64{1, 1, 1, 1, 1, 1}
	lo := []float64{-2, -2, -2, -2, -2, -2}
	hi := []float64{2, 2, 2, 2, 2, 2}
	want := append([]float64{}, x...)
	repairBox(x, lo, hi)
	for i := range x {
		if x[i] != want[i] {
			t.Fatalf("feasible point moved %v", x)
		}
	}
}

func TestRepairProjectionConverges(t *testing.T) {
	for trial := 0; trial < 200; trial++ {
		rng := newRng(int64(trial))
		lo := []float64{-5, -5, -5, -5, -5, -5}
		hi := []float64{10, 10, 10, 10, 10, 10}
		x := uniformIn(rng, lo, hi)
		repairBox(x, lo, hi)
		for i := 0; i+1 < len(x); i++ {
			if x[i] < x[i+1]-1e-9 {
				t.Fatalf("trial %d still increasing %v", trial, x)
			}
		}
	}
}

func TestBestDiagonal(t *testing.T) {
	c, v := bestDiagonal(6)
	if c <= -5 || c >= 10 {
		t.Fatalf("diagonal argmin %v", c)
	}
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("diagonal value %v", v)
	}
	lo := &rosenbrock{d: 6}
	if v < lo.raw([]float64{c, c, c, c, c, c})-1e-6 {
		t.Fatal("value inconsistent with raw evaluation")
	}
}
