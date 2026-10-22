package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestApplyEvalWeightFlagsRegistersDerivedSeat(t *testing.T) {
	want := [config.PatternClassCount]int{0, 97, 122, 512, 513, 2077}
	sel, err := applyEvalWeightFlags("", "97,122,512,513,2077", "master")
	if err != nil {
		t.Fatalf("applyEvalWeightFlags: %v", err)
	}
	label := "master-w97.122.512.513.2077"
	if sel != "master,"+label {
		t.Fatalf("seats sel %q, want %q", sel, "master,"+label)
	}
	got, ok := seats[label]
	if !ok {
		t.Fatalf("derived seat %s missing from the registry", label)
	}
	if got.EvalWeights == nil || *got.EvalWeights != want {
		t.Fatalf("derived seat weights %v, want %v", got.EvalWeights, want)
	}
	if !got.Book {
		t.Fatalf("derived seat lost the book flag")
	}
}

func TestApplyEvalWeightFlagsKeepsExplicitSeatsAndRejectsHalves(t *testing.T) {
	sel, err := applyEvalWeightFlags("master,master", "97,122,512,513,2077", "master")
	if err != nil {
		t.Fatalf("applyEvalWeightFlags: %v", err)
	}
	if sel != "master,master" {
		t.Fatalf("explicit seats sel overwritten: %q", sel)
	}
	if _, err := applyEvalWeightFlags("", "97,122", "master"); err == nil {
		t.Fatalf("half-set pair accepted")
	}
	if _, err := applyEvalWeightFlags("", "", "master"); err == nil {
		t.Fatalf("base without weights accepted")
	}
	if _, err := applyEvalWeightFlags("", "", ""); err != nil {
		t.Fatalf("empty pair rejected: %v", err)
	}
}

func TestFloorConfigAcceptsDerivedEvalSeat(t *testing.T) {
	if _, err := applyEvalWeightFlags("", "97,122,512,513,2077", "master"); err != nil {
		t.Fatalf("applyEvalWeightFlags: %v", err)
	}
	cfg := floorConfig{name: "evalfloor-fitted-21", seat: "master-w97.122.512.513.2077", tcIdx: 1, gamesPerDir: 20, dirs: 5, blockDirs: 5, maxBlocks: 3, z: 1.96}
	if err := validateFloorConfig(cfg, len(config.TimeControls)); err != nil {
		t.Fatalf("floor config over the derived seat: %v", err)
	}
}
