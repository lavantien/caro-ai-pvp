package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestDEMatchesGDOnSynthetic(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	truth := [5]float64{300, 900, 1600, 5200, 21000}
	samples := genSynthetic(rng, 140, 60, truth, 3600)
	d := aggregateFit(samples)
	opts := defaultGDOptions()
	opts.Restarts = 3
	gd := fitGD(d, opts, rng, nil)
	de := fitDE(d, defaultDEOptions(), rng)
	if gd.Loss == 0 || math.IsInf(gd.Loss, 0) {
		t.Fatalf("gd loss not usable: %v", gd.Loss)
	}
	rel := math.Abs(de.Loss-gd.Loss) / math.Abs(gd.Loss)
	if rel > 0.02 {
		t.Fatalf("de loss %.9f vs gd loss %.9f differ by %.3f percent, want within 2 percent", de.Loss, gd.Loss, rel*100)
	}
	assertStrictlyAscending(t, de.W)
}

func TestDESeedDeterminism(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	truth := [5]float64{200, 700, 1400, 4800, 18000}
	samples := genSynthetic(rng, 60, 20, truth, 3000)
	d := aggregateFit(samples)
	a := fitDE(d, defaultDEOptions(), rand.New(rand.NewSource(9)))
	b := fitDE(d, defaultDEOptions(), rand.New(rand.NewSource(9)))
	if a.Loss != b.Loss || a.S != b.S || a.W != b.W {
		t.Fatal("DE must be deterministic for a fixed seed")
	}
}
