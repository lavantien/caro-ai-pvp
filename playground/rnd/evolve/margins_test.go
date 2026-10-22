package main

import (
	"math"
	"testing"
)

func TestSolvedMarginsHitLabeledProbabilities(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    float64
		p    float64
	}{
		{"weak", marginWeak, 0.55},
		{"mid", marginMid, 0.70},
		{"strong", marginStrong, 0.90},
	} {
		solved := solveMargin(tc.p)
		if math.Abs(solved-tc.m) > 1e-6 {
			t.Fatalf("%s: target p %v solves to margin %v, pinned constant is %v", tc.name, tc.p, solved, tc.m)
		}
		if got := measuredP(tc.m); math.Abs(got-tc.p) > 1e-6 {
			t.Fatalf("%s: pinned margin %v measures p %v, want %v", tc.name, tc.m, got, tc.p)
		}
	}
}
