package main

import (
	"math"
	"testing"
)

func TestBraninCanonicalMinima(t *testing.T) {
	minima := [][2]float64{
		{-math.Pi, 12.275},
		{math.Pi, 2.275},
		{9.42478, 2.475},
	}
	for _, m := range minima {
		f := branin([]float64{m[0], m[1]})
		if math.Abs(f-0.397887) > 1e-3 {
			t.Fatalf("branin(%v) = %v, want canonical minimum 0.397887", m, f)
		}
	}
}
