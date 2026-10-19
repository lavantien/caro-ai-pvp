package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func TestParseEvalWeightsAcceptsFittedSet(t *testing.T) {
	w, err := parseEvalWeights("137,162,483,484,1561")
	if err != nil {
		t.Fatalf("parseEvalWeights of the fitted set: %v", err)
	}
	want := [config.PatternClassCount]int{0, 137, 162, 483, 484, 1561}
	if w != want {
		t.Errorf("parseEvalWeights = %v, want %v with class 0 pinned to zero", w, want)
	}
}

func TestParseEvalWeightsRejectsMalformed(t *testing.T) {
	for _, s := range []string{
		"",
		"137,162,483,484",
		"137,162,483,484,1561,9",
		"137,162,483,484,1561,",
		",137,162,483,484,1561",
		"137,,483,484,1561",
		"137,162,x,484,1561",
		"137,162,-5,484,1561",
		"137, 162,483,484,1561",
		"137.162.483.484.1561",
	} {
		if w, err := parseEvalWeights(s); err == nil {
			t.Errorf("parseEvalWeights(%q) = %v, want error", s, w)
		}
	}
}

func TestEvalWeightsLabel(t *testing.T) {
	w, err := parseEvalWeights("137,162,483,484,1561")
	if err != nil {
		t.Fatalf("parseEvalWeights: %v", err)
	}
	if got := evalWeightsLabel("grandmaster", &w); got != "grandmaster-w137.162.483.484.1561" {
		t.Errorf("evalWeightsLabel = %q, want grandmaster-w137.162.483.484.1561", got)
	}
}

func TestRegisterEvalWeightsSeat(t *testing.T) {
	w, err := parseEvalWeights("137,162,483,484,1561")
	if err != nil {
		t.Fatalf("parseEvalWeights: %v", err)
	}
	label, err := registerEvalWeightsSeat("master", w)
	if err != nil {
		t.Fatalf("registerEvalWeightsSeat: %v", err)
	}
	defer delete(seats, label)
	if label != "master-w137.162.483.484.1561" {
		t.Fatalf("label = %q, want master-w137.162.483.484.1561", label)
	}
	over, ok := seats[label]
	if !ok {
		t.Fatalf("seat %q missing from the roster", label)
	}
	if over.EvalWeights == nil {
		t.Fatal("weights seat carries nil eval weights")
	}
	if *over.EvalWeights != w || over.EvalWeights[0] != 0 {
		t.Errorf("weights seat eval weights = %v, want %v with class 0 pinned to zero", *over.EvalWeights, w)
	}
	base := seats["master"]
	if base.EvalWeights != nil {
		t.Error("master base seat gained eval weights, want nil global default")
	}
	if over.Name != label || over.Cores != base.Cores || over.TTBytes != base.TTBytes ||
		over.VCF != base.VCF || over.VCT != base.VCT || over.Book != base.Book ||
		over.BookMaxPly != base.BookMaxPly || over.PonderThreads != base.PonderThreads {
		t.Errorf("weights seat perturbs beyond EvalWeights: %+v over %+v", over, base)
	}
	if got := seatLabels(label); len(got) != 1 || got[0] != label {
		t.Errorf("seatLabels of the weights seat = %v, want [%s]", got, label)
	}
	if again, err := registerEvalWeightsSeat("master", w); err != nil || again != label {
		t.Errorf("re-register = %q, %v, want idempotent %q", again, err, label)
	}
}

func TestRegisterEvalWeightsSeatRejectsUnknownBase(t *testing.T) {
	w, err := parseEvalWeights("1,2,3,4,5")
	if err != nil {
		t.Fatalf("parseEvalWeights: %v", err)
	}
	if label, err := registerEvalWeightsSeat("nope", w); err == nil {
		t.Errorf("registerEvalWeightsSeat from unknown base = %q, want error", label)
	}
}

func TestEvalWeightsSeatConstructsSearchersOnBothCoreBranches(t *testing.T) {
	for _, base := range []string{"easy", "medium"} {
		w, err := parseEvalWeights("137,162,483,484,1561")
		if err != nil {
			t.Fatalf("parseEvalWeights: %v", err)
		}
		label, err := registerEvalWeightsSeat(base, w)
		if err != nil {
			t.Fatalf("registerEvalWeightsSeat on %s: %v", base, err)
		}
		s := server.NewBotSearcher(seats[label])
		if s == nil {
			delete(seats, label)
			t.Fatalf("NewBotSearcher on %s returned nil", label)
		}
		s.Close()
		delete(seats, label)
	}
}
