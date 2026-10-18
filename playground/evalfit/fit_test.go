package main

import (
	"math"
	"math/rand"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func sigmoid(x float64) float64 {
	if x >= 0 {
		return 1 / (1 + math.Exp(-x))
	}
	e := math.Exp(x)
	return e / (1 + e)
}

func genSynthetic(rng *rand.Rand, patterns, draws int, w [5]float64, s float64) []sample {
	loud := [5]int{8, 4, 4, 2, 1}
	quiet := [5]int{3, 1, 1, 0, 0}
	var out []sample
	for range patterns {
		ranges := &loud
		if rng.Float64() < 0.3 {
			ranges = &quiet
		}
		var f [5]int32
		for c := range f {
			f[c] = int32(rng.Intn(ranges[c]+1)) - int32(rng.Intn(ranges[c]+1))
		}
		dot := 0.0
		for c := range f {
			dot += float64(f[c]) * w[c]
		}
		p := sigmoid((dot + float64(config.EvalTempo)) / s)
		for range draws {
			y := 0.0
			if rng.Float64() < p {
				y = 1.0
			}
			out = append(out, sample{Series: "synth", F: f, Y: y})
		}
	}
	return out
}

func assertStrictlyAscending(t *testing.T, w [5]float64) {
	t.Helper()
	for i := 1; i < len(w); i++ {
		if !(w[i-1] < w[i]) {
			t.Fatalf("weights not strictly ascending: %v", w)
		}
	}
}

func TestEvalLossMatchesHandComputation(t *testing.T) {
	samples := []sample{
		{Series: "s", F: [5]int32{1, 0, 0, 0, 0}, Y: 1.0},
		{Series: "s", F: [5]int32{0, 0, 0, 0, 0}, Y: 0.5},
	}
	w := [5]float64{300, 800, 1500, 5000, 20000}
	s := 4000.0
	p1 := sigmoid((300 + float64(config.EvalTempo)) / s)
	p2 := sigmoid(float64(config.EvalTempo) / s)
	want := -0.5 * (math.Log(p1) + 0.5*math.Log(p2) + 0.5*math.Log(1-p2))
	got := evalLoss(samples, w, s)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("loss = %.12f, want %.12f", got, want)
	}
}

func TestFitRecoversKnownWeights(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	want := [5]float64{300, 800, 1500, 5000, 20000}
	samples := genSynthetic(rng, 500, 2000, want, 4000)
	res := fitGD(aggregateFit(samples), defaultGDOptions(), rng, nil)
	assertStrictlyAscending(t, res.W)
	for i, wv := range want {
		rel := math.Abs(res.W[i]-wv) / wv
		if rel > 0.15 {
			t.Fatalf("weight %d = %.1f, want %.1f within 15 percent (rel %.3f)", i+1, res.W[i], wv, rel)
		}
	}
	if rel := math.Abs(res.S-4000) / 4000; rel > 0.15 {
		t.Fatalf("scale = %.1f, want 4000 within 15 percent (rel %.3f)", res.S, rel)
	}
}

func TestFitMonotoneOnAnyInput(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	want := [5]float64{4000, 100, 300, 8000, 200}
	samples := genSynthetic(rng, 200, 40, want, 3000)
	opts := defaultGDOptions()
	opts.Restarts = 3
	opts.MaxSteps = 20000
	res := fitGD(aggregateFit(samples), opts, rng, nil)
	assertStrictlyAscending(t, res.W)
}

func TestFitScaleOnly(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	w := [5]float64{100, 500, 1200, 4000, 15000}
	samples := genSynthetic(rng, 240, 60, w, 3000)
	res := fitScaleGD(aggregateFit(samples), w, defaultGDOptions(), rng)
	if rel := math.Abs(res.S-3000) / 3000; rel > 0.15 {
		t.Fatalf("scale = %.1f, want 3000 within 15 percent (rel %.3f)", res.S, rel)
	}
}

func TestSplitSeriesKeys(t *testing.T) {
	keys := make([]string, 100)
	for i := range keys {
		keys[i] = "s" + string(rune('a'+i/26)) + string(rune('a'+i%26))
	}
	train := splitSeriesKeys(keys, 7, 0.9)
	if len(train) != 90 {
		t.Fatalf("train size = %d, want 90", len(train))
	}
	again := splitSeriesKeys(keys, 7, 0.9)
	if len(again) != len(train) {
		t.Fatal("split must be deterministic for a fixed seed")
	}
	for k := range train {
		if !again[k] {
			t.Fatalf("key %q drifted between identical seeds", k)
		}
	}
	other := splitSeriesKeys(keys, 8, 0.9)
	differs := false
	for k := range train {
		if !other[k] {
			differs = true
		}
	}
	if !differs {
		t.Fatal("different seeds must produce different splits")
	}
}
