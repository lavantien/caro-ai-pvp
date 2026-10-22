package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestShadeMatchesGDOnSynthetic(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	truth := [5]float64{300, 900, 1600, 5200, 21000}
	samples := genSynthetic(rng, 140, 60, truth, 3600)
	d := aggregateFit(samples)
	opts := defaultGDOptions()
	opts.Restarts = 3
	gd := fitGD(d, opts, rng, nil)
	sh := fitShade(d, defaultDEOptions(), rng)
	if gd.Loss == 0 || math.IsInf(gd.Loss, 0) {
		t.Fatalf("gd loss not usable: %v", gd.Loss)
	}
	rel := math.Abs(sh.Loss-gd.Loss) / math.Abs(gd.Loss)
	if rel > 0.02 {
		t.Fatalf("shade loss %.9f vs gd loss %.9f differ by %.3f percent, want within 2 percent", sh.Loss, gd.Loss, rel*100)
	}
	assertStrictlyAscending(t, sh.W)
}

func TestShadeSeedDeterminism(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	truth := [5]float64{200, 700, 1400, 4800, 18000}
	samples := genSynthetic(rng, 60, 20, truth, 3000)
	d := aggregateFit(samples)
	a := fitShade(d, defaultDEOptions(), rand.New(rand.NewSource(9)))
	b := fitShade(d, defaultDEOptions(), rand.New(rand.NewSource(9)))
	if a.Loss != b.Loss || a.S != b.S || a.W != b.W {
		t.Fatal("shade must be deterministic for a fixed seed")
	}
}

func TestAcceptShadeOnlyOnImprovement(t *testing.T) {
	pop := [][6]float64{{0, 0, 0, 0, 0, 8}, {0, 0, 0, 0, 0, 7}}
	loss := []float64{1.0, 0.5}
	best := 1
	if !acceptShade(pop, loss, &best, 0, [6]float64{1, 1, 1, 1, 1, 8}, 0.7) {
		t.Fatal("a better trial must be accepted")
	}
	if loss[0] != 0.7 || best != 1 {
		t.Fatalf("loss[0] = %v best = %d, want 0.7 with best still 1 under the 0.5 global best", loss[0], best)
	}
	if !acceptShade(pop, loss, &best, 0, [6]float64{2, 2, 2, 2, 2, 8}, 0.4) {
		t.Fatal("a global-best trial must be accepted")
	}
	if best != 0 {
		t.Fatalf("best = %d, want 0 after a new global best", best)
	}
	if acceptShade(pop, loss, &best, 0, [6]float64{3, 3, 3, 3, 3, 8}, 0.8) {
		t.Fatal("a worse trial must be rejected")
	}
	if loss[0] != 0.4 {
		t.Fatalf("loss[0] = %v after rejection, want 0.4 unchanged", loss[0])
	}
	if !acceptShade(pop, loss, &best, 1, [6]float64{4, 4, 4, 4, 4, 7}, 0.5) {
		t.Fatal("an equal trial must be accepted")
	}
}

func TestToleranceCheckTripsOnDrift(t *testing.T) {
	if !challengerViolatesTolerance(0.68, 0.66) {
		t.Fatal("a challenger 2.9 percent under gd must trip the tolerance")
	}
	if challengerViolatesTolerance(0.68, 0.667) {
		t.Fatal("a challenger 1.9 percent under gd must pass")
	}
	if challengerViolatesTolerance(0.68, 0.70) {
		t.Fatal("a worse challenger must pass")
	}
}

func TestShadeReduceKeepsBest(t *testing.T) {
	pop := [][6]float64{{0}, {1}, {2}, {3}}
	for i := range pop {
		pop[i] = [6]float64{float64(i), 0, 0, 0, 0, 8}
	}
	loss := []float64{0.9, 0.1, 0.8, 0.7}
	best := 1
	np, nl, nb := shadeReduce(pop, loss, best, 2)
	if len(np) != 2 || len(nl) != 2 {
		t.Fatalf("reduced pop = %d, want 2", len(np))
	}
	if nl[nb] != 0.1 {
		t.Fatalf("best loss = %v at index %d, want 0.1 retained", nl[nb], nb)
	}
	for _, l := range nl {
		if l != 0.1 && l != 0.7 {
			t.Fatalf("reduced losses = %v, want the two smallest {0.1, 0.7}", nl)
		}
	}
}

func TestShadeMemoriesAdaptOnSuccess(t *testing.T) {
	s := newShadeState(4, defaultDEOptions())
	for i := range s.memF {
		if s.memF[i] != 0.5 || s.memCR[i] != 0.5 {
			t.Fatalf("seeded memory [%d] = %v %v, want 0.5 0.5", i, s.memF[i], s.memCR[i])
		}
	}
	s.lastF = []float64{0.6, 0.8, 0, 0}
	s.lastCR = []float64{0.2, 0.9, 0, 0}
	s.record(0, -0.5, true)
	s.record(1, -1.5, true)
	s.record(0, -0.1, false)
	s.afterSuccesses()
	lm := lehmerMean([]float64{0.6, 0.8}, []float64{0.5, 1.5})
	if math.Abs(s.memF[0]-lm) > 1e-12 {
		t.Fatalf("memF[0] = %v, want Lehmer mean %v", s.memF[0], lm)
	}
	wm := weightedMean([]float64{0.2, 0.9}, []float64{0.5, 1.5})
	if math.Abs(s.memCR[0]-wm) > 1e-12 {
		t.Fatalf("memCR[0] = %v, want weighted mean %v", s.memCR[0], wm)
	}
	if s.memIdx != 1 {
		t.Fatalf("memIdx = %d, want 1 after one update", s.memIdx)
	}
	if len(s.succF) != 0 {
		t.Fatalf("success buffers = %d, want drained", len(s.succF))
	}
}
