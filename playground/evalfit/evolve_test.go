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
	bnd := 0.68 * 0.98
	if challengerViolatesTolerance(0.68, bnd) {
		t.Fatal("a challenger exactly at the 2 percent boundary must not trip")
	}
	if !challengerViolatesTolerance(0.68, math.Nextafter(bnd, 0)) {
		t.Fatal("a challenger one ulp past the boundary must trip")
	}
}

func TestClampFReflectsAndBounds(t *testing.T) {
	for _, v := range []float64{-1e300, -1e15, -0.75, 0.0, 0, 0.05, 0.5, 1, 1.5, 1e300} {
		got := clampF(v)
		if !(got > 0 && got <= 1) {
			t.Fatalf("clampF(%v) = %v, want inside (0, 1]", v, got)
		}
	}
	if got := clampF(-0.75); got != 0.75 {
		t.Fatalf("clampF(-0.75) = %v, want reflection to 0.75", got)
	}
	if got := clampF(-1e300); got != 1 {
		t.Fatalf("clampF(-1e300) = %v, want 1", got)
	}
	if got := clampF(0); got != 0.05 {
		t.Fatalf("clampF(0) = %v, want 0.05", got)
	}
}

func TestTrimArchiveCapsAtInitPop(t *testing.T) {
	s := newShadeState(6, defaultDEOptions())
	for i := range 10 {
		s.pushArchive([6]float64{float64(i)})
	}
	s.trimArchive(rand.New(rand.NewSource(1)))
	if len(s.archive) > 6 {
		t.Fatalf("archive = %d entries, want at most the 6 initial population", len(s.archive))
	}
}

func TestRestartAroundKeepsIncumbentAndResets(t *testing.T) {
	opts := defaultDEOptions()
	s := newShadeState(8, opts)
	pop := make([][6]float64, 8)
	loss := make([]float64, 8)
	for i := range pop {
		pop[i] = [6]float64{1, 2, 3, 4, 5, 8}
		loss[i] = 1.0
	}
	incumbent := [6]float64{7, 6, 5, 4, 3, 9}
	pop[3] = incumbent
	loss[3] = 0.25
	s.pushArchive([6]float64{0})
	s.stall = 30
	s.restartAround(pop, loss, 3, rand.New(rand.NewSource(2)))
	if pop[3] != incumbent || loss[3] != 0.25 {
		t.Fatalf("incumbent mutated: %v loss %v", pop[3], loss[3])
	}
	for i := range pop {
		if i == 3 {
			continue
		}
		if loss[i] != math.Inf(1) {
			t.Fatalf("loss[%d] = %v, want reset to infinity", i, loss[i])
		}
		for j := range pop[i] {
			if pop[i][j] < opts.Lo[j] || pop[i][j] > opts.Hi[j] {
				t.Fatalf("restart genome out of bounds at %d,%d: %v", i, j, pop[i][j])
			}
		}
	}
	if s.archive != nil {
		t.Fatal("restart must clear the archive")
	}
	if s.stall != 0 {
		t.Fatalf("stall = %d, want reset to 0", s.stall)
	}
}

func TestAfterGenerationStallAndDiversityTriggersRestart(t *testing.T) {
	opts := defaultDEOptions()
	opts.Pop = 8
	s := newShadeState(8, opts)
	pop := make([][6]float64, 8)
	loss := make([]float64, 8)
	for i := range pop {
		pop[i] = [6]float64{1, 2, 3, 4, 5, 8}
		loss[i] = 1.0
	}
	s.prevBest = 1.0
	s.stall = 24
	np, nl, _ := s.afterGeneration(rand.New(rand.NewSource(3)), 200, pop, loss, 0)
	if len(np) != 8 {
		t.Fatalf("pop = %d after the stall generation, want the restart path without the LPSR reduction to 6", len(np))
	}
	if nl[1] != math.Inf(1) {
		t.Fatalf("loss[1] = %v, want the restart reset to infinity", nl[1])
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
	if math.Abs(s.memF[0]-0.76) > 1e-12 {
		t.Fatalf("memF[0] = %v, want Lehmer mean 0.76 over v {0.6, 0.8} w {0.5, 1.5}", s.memF[0])
	}
	if math.Abs(s.memCR[0]-0.725) > 1e-12 {
		t.Fatalf("memCR[0] = %v, want weighted mean 0.725 over v {0.2, 0.9} w {0.5, 1.5}", s.memCR[0])
	}
	if s.memIdx != 1 {
		t.Fatalf("memIdx = %d, want 1 after one update", s.memIdx)
	}
	if len(s.succF) != 0 {
		t.Fatalf("success buffers = %d, want drained", len(s.succF))
	}
}
