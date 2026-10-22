package main

import (
	"math"
	"testing"
)

func TestEIZeroSigma(t *testing.T) {
	if got := expectedImprovement(-1, 0, 0, 0.01); math.Abs(got-0.99) > 1e-12 {
		t.Fatalf("EI %v want 0.99", got)
	}
	if got := expectedImprovement(1, 0, 0, 0.01); got != 0 {
		t.Fatalf("EI %v want 0", got)
	}
}

func TestEIMonotoneInMu(t *testing.T) {
	prev := math.Inf(1)
	for mu := -3.0; mu <= 3.0; mu += 0.25 {
		ei := expectedImprovement(mu, 0.5, 0, 0.01)
		if ei > prev+1e-12 {
			t.Fatalf("EI rose as mu increased at mu %v", mu)
		}
		prev = ei
	}
}

func TestEIMonotoneInSigma(t *testing.T) {
	prev := -1.0
	for sg := 0.05; sg <= 5.0; sg += 0.25 {
		ei := expectedImprovement(0.2, sg, 0, 0.01)
		if ei < prev-1e-12 {
			t.Fatalf("EI fell as sigma increased at sigma %v", sg)
		}
		prev = ei
	}
}

func TestEIExploresAboveBest(t *testing.T) {
	if ei := expectedImprovement(0.5, 0.5, 0, 0.01); ei <= 0 {
		t.Fatalf("EI must stay positive above best with positive sigma, got %v", ei)
	}
}
