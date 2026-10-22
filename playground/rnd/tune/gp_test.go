package main

import (
	"math"
	"math/rand/v2"
	"testing"
)

func cleanSinData(n int) ([][]float64, []float64) {
	rng := rand.New(rand.NewPCG(1, 2))
	xs := make([][]float64, 0, n)
	ys := make([]float64, 0, n)
	seen := map[float64]bool{}
	for len(xs) < n {
		x := 6.283185307179586 * rng.Float64()
		if seen[x] {
			continue
		}
		seen[x] = true
		xs = append(xs, []float64{x})
		ys = append(ys, math.Sin(x))
	}
	return xs, ys
}

func noisySinData(n int, sd float64, seed uint64) ([][]float64, []float64) {
	rng := rand.New(rand.NewPCG(3, seed))
	xs := make([][]float64, 0, n)
	ys := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		x := 6.283185307179586 * float64(i) / float64(n-1)
		xs = append(xs, []float64{x})
		ys = append(ys, math.Sin(x)+sd*rng.NormFloat64())
	}
	return xs, ys
}

func TestGPInterpolatesTrainingPoints(t *testing.T) {
	xs, ys := cleanSinData(8)
	g := newGP(1, []float64{1}, gpSigmaF, gpSigmaN)
	g.fit(xs, ys)
	for i, x := range xs {
		mu, sg := g.predict(x)
		if math.Abs(mu-ys[i]) > 1e-3 {
			t.Fatalf("train point %d: mu %v want %v", i, mu, ys[i])
		}
		if sg > 1e-3 {
			t.Fatalf("train point %d: sigma %v want near 0", i, sg)
		}
	}
}

func TestGPVarianceGrowsAwayFromData(t *testing.T) {
	xs, ys := cleanSinData(8)
	g := newGP(1, []float64{1}, gpSigmaF, gpSigmaN)
	g.fit(xs, ys)
	_, near := g.predict(xs[0])
	_, sg := g.predict([]float64{xs[0][0] + 50})
	if sg <= near {
		t.Fatalf("far sigma %v must exceed near sigma %v", sg, near)
	}
}

func TestGPLogMarginalPrefersSaneScaleOnNoisyData(t *testing.T) {
	xs, ys := noisySinData(10, 0.05, 7)
	mk := func(l float64) float64 {
		g := newGP(1, []float64{l}, gpSigmaF, 0.05)
		g.fit(xs, ys)
		return g.logMarginal()
	}
	if mk(1.0) <= mk(0.01) || mk(1.0) <= mk(50) {
		t.Fatalf("LML must prefer the sane scale: sane %v tiny %v huge %v", mk(1.0), mk(0.01), mk(50))
	}
}

func TestFitMapStaysInSaneBand(t *testing.T) {
	xs, ys := cleanSinData(8)
	g := newGP(1, []float64{2}, gpSigmaF, mapSigmaN)
	g.fitMap(xs, ys, []dim{{"x", 0, 6.283185307179586, false}})
	l := g.lengths[0]
	if math.IsNaN(l) || l <= 0 {
		t.Fatalf("mapped length %v invalid", l)
	}
	span := 6.283185307179586
	if l < lengthFloorFrac*span || l > lengthCeilFrac*span {
		t.Fatalf("mapped length %v outside band [%v, %v]", l, lengthFloorFrac*span, lengthCeilFrac*span)
	}
}
