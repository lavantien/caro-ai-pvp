package main

import (
	"math"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
)

func smallArtifact() artifactData {
	return artifactData{
		Date:            "2026-10-19",
		Roots:           []string{"logs/a", "logs/b"},
		Tiers:           []string{"master", "grandmaster"},
		PerRootKept:     map[string]int{"logs/a": 10, "logs/b": 5},
		PerRootRejected: map[string]int{"logs/a": 1, "logs/b": 0},
		GamesKept:       15,
		GamesRejected:   1,
		PositionsKept:   900,
		DrawRate:        0.0625,
		Seed:            1,
		TrainSeries:     8,
		ValSeries:       1,
		W:               [5]float64{304.6, 512.2, 1180.4, 3900.9, 15100.3},
		WInt:            [5]int64{305, 512, 1180, 3901, 15100},
		IntDrift:        0.4,
		S:               4123.5,
		BestRestart:     3,
		TrainLossGD:     0.601234567,
		ValLossGD:       0.612345678,
		BaselineS:       3900.25,
		BaselineTrain:   0.654321,
		BaselineVal:     0.665432,
		DELoss:          0.601235,
		ValLossRecorded: 0.612345999,
		Calibration: []bucketRow{
			{Lo: -5000, Hi: -1000, Empirical: 0.21, Predicted: 0.22, N: 90},
			{Lo: -1000, Hi: 2000, Empirical: 0.58, Predicted: 0.57, N: 91},
		},
	}
}

func TestArtifactRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evalfit-test.txt")
	want := smallArtifact()
	if err := writeArtifact(path, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := parseArtifact(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Date != want.Date || got.Seed != want.Seed || got.GamesKept != want.GamesKept || got.GamesRejected != want.GamesRejected {
		t.Fatalf("scalar fields drifted: %+v vs %+v", got, want)
	}
	if strings.Join(got.Roots, ",") != strings.Join(want.Roots, ",") || strings.Join(got.Tiers, ",") != strings.Join(want.Tiers, ",") {
		t.Fatalf("roots/tiers drifted: %v %v", got.Roots, got.Tiers)
	}
	for root, n := range want.PerRootKept {
		if got.PerRootKept[root] != n {
			t.Fatalf("per-root kept for %s = %d, want %d", root, got.PerRootKept[root], n)
		}
	}
	for root, n := range want.PerRootRejected {
		if got.PerRootRejected[root] != n {
			t.Fatalf("per-root rejected for %s = %d, want %d", root, got.PerRootRejected[root], n)
		}
	}
	if got.WInt != want.WInt {
		t.Fatalf("integer weights drifted: %v vs %v", got.WInt, want.WInt)
	}
	for i := range got.W {
		if math.Abs(got.W[i]-want.W[i]) > 1e-9*math.Abs(want.W[i]) {
			t.Fatalf("weight %d drifted: %v vs %v", i+1, got.W[i], want.W[i])
		}
	}
	check := func(name string, gotV, wantV float64) {
		if math.Abs(gotV-wantV) > 1e-9*(1+math.Abs(wantV)) {
			t.Fatalf("%s drifted: %v vs %v", name, gotV, wantV)
		}
	}
	check("scale", got.S, want.S)
	check("train loss gd", got.TrainLossGD, want.TrainLossGD)
	check("val loss gd", got.ValLossGD, want.ValLossGD)
	check("baseline scale", got.BaselineS, want.BaselineS)
	check("baseline train", got.BaselineTrain, want.BaselineTrain)
	check("baseline val", got.BaselineVal, want.BaselineVal)
	check("de loss", got.DELoss, want.DELoss)
	check("recorded val loss", got.ValLossRecorded, want.ValLossRecorded)
	check("int drift", got.IntDrift, want.IntDrift)
	if len(got.Calibration) != len(want.Calibration) {
		t.Fatalf("calibration rows = %d, want %d", len(got.Calibration), len(want.Calibration))
	}
	for i, row := range want.Calibration {
		gotRow := got.Calibration[i]
		check("bucket lo", gotRow.Lo, row.Lo)
		check("bucket hi", gotRow.Hi, row.Hi)
		check("bucket empirical", gotRow.Empirical, row.Empirical)
		check("bucket predicted", gotRow.Predicted, row.Predicted)
		if gotRow.N != row.N {
			t.Fatalf("bucket %d n = %d, want %d", i, gotRow.N, row.N)
		}
	}
}

func TestArtifactVerify(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	truth := [5]float64{300, 800, 1500, 5000, 20000}
	samples := genSynthetic(rng, 80, 30, truth, 4000)
	a := smallArtifact()
	for i, wv := range a.WInt {
		a.W[i] = float64(wv)
	}
	a.IntDrift = 0
	a.S = 4000
	a.ValLossRecorded = evalLoss(samples, a.W, a.S)
	if err := verifyArtifact(a, samples); err != nil {
		t.Fatalf("verify on consistent artifact: %v", err)
	}
	farDrift := a
	farDrift.W[4] += 0.9
	farDrift.IntDrift = 0.9
	if err := verifyArtifact(farDrift, samples); err != nil {
		t.Fatalf("honestly recorded drift past 0.5 must pass: %v", err)
	}
	badDrift := a
	badDrift.IntDrift = 0.3
	if err := verifyArtifact(badDrift, samples); err == nil {
		t.Fatal("verify must fail when the recorded drift does not reproduce from the weights")
	}
	descending := a
	descending.WInt[2] = descending.WInt[1] - 400
	if err := verifyArtifact(descending, samples); err == nil {
		t.Fatal("verify must fail when integer weights are not strictly ascending")
	}
	tampered := a
	tampered.ValLossRecorded += 1e-3
	if err := verifyArtifact(tampered, samples); err == nil {
		t.Fatal("verify must fail when the recorded loss does not match the weights")
	}
	wrongWeights := a
	wrongWeights.WInt = [5]int64{200, 600, 1200, 4000, 12000}
	wrongWeights.W = [5]float64{200, 600, 1200, 4000, 12000}
	wrongWeights.IntDrift = 0
	if err := verifyArtifact(wrongWeights, samples); err == nil {
		t.Fatal("verify must fail on weights that do not reproduce the recorded loss")
	}
}
