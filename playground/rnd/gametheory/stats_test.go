package main

import (
	"math"
	"testing"
)

func TestWilsonKnownValues(t *testing.T) {
	lo, hi := wilson(0, 10)
	if math.Abs(lo) > 1e-9 || math.Abs(hi-0.2776) > 5e-4 {
		t.Fatalf("wilson(0,10) = [%f, %f], want [0, 0.2776]", lo, hi)
	}
	lo, hi = wilson(19, 19)
	if math.Abs(hi-1) > 1e-9 || math.Abs(lo-0.8318) > 5e-4 {
		t.Fatalf("wilson(19,19) = [%f, %f], want [0.8318, 1]", lo, hi)
	}
}

func TestWilsonSymmetry(t *testing.T) {
	loA, hiA := wilson(7, 20)
	loB, hiB := wilson(13, 20)
	if math.Abs(hiA+loB-1) > 1e-9 || math.Abs(loA+hiB-1) > 1e-9 {
		t.Fatalf("symmetry broken: [%f,%f] vs [%f,%f]", loA, hiA, loB, hiB)
	}
}

func TestWilsonDegenerate(t *testing.T) {
	lo, hi := wilson(0, 0)
	if lo != 0 || hi != 1 {
		t.Fatalf("wilson(0,0) = [%f, %f], want [0, 1]", lo, hi)
	}
}

func TestQuantile(t *testing.T) {
	sorted := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if quantile(sorted, 0) != 10 || quantile(sorted, 0.5) != 60 || quantile(sorted, 1) != 100 {
		t.Fatalf("quantiles = %d %d %d", quantile(sorted, 0), quantile(sorted, 0.5), quantile(sorted, 1))
	}
	if quantile(nil, 0.5) != 0 {
		t.Fatalf("empty quantile not zero")
	}
}
