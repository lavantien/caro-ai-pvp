package config

import "testing"

func TestVariantEvalSwitches(t *testing.T) {
	square := Variants[0].Name
	other := "hex-side-12"
	if square == other {
		t.Fatal("variant table collapsed to one entry")
	}
	if got := EvalCleanWindowWeight(square); got != EvalCleanWindowWeightSquare {
		t.Fatalf("EvalCleanWindowWeight(%s) = %d", square, got)
	}
	if got := EvalCleanWindowWeight(other); got != EvalCleanWindowWeightHex {
		t.Fatalf("EvalCleanWindowWeight(%s) = %d", other, got)
	}
	if got := EvalBoxedCutThree(square); got != EvalBoxedCutThreeSquare {
		t.Fatalf("EvalBoxedCutThree(%s) = %d", square, got)
	}
	if got := EvalBoxedCutThree(other); got != EvalBoxedCutThreeHex {
		t.Fatalf("EvalBoxedCutThree(%s) = %d", other, got)
	}
}

func TestBookArtifactPathForSquareOnly(t *testing.T) {
	path, ok := BookArtifactPathFor("square")
	if !ok || path != BookArtifactPath {
		t.Fatalf("BookArtifactPathFor(square) = %q, %v", path, ok)
	}
	for _, variant := range Variants[1:] {
		if path, ok := BookArtifactPathFor(variant.Name); ok {
			t.Fatalf("BookArtifactPathFor(%s) = %q, %v, want none", variant.Name, path, ok)
		}
	}
}
