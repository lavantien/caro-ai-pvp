package main

import (
	"math"
	"testing"
)

func TestClampFRange(t *testing.T) {
	if clampF(0) <= 0 || clampF(0) > 1 {
		t.Fatal("clampF(0) out of range")
	}
	if clampF(2) != 1 || clampF(0.5) != 0.5 {
		t.Fatal("clampF bounds wrong")
	}
	if clampCR(-1) != 0 || clampCR(2) != 1 {
		t.Fatal("clampCR bounds wrong")
	}
}

func TestLehmerMean(t *testing.T) {
	v := []float64{1, 2}
	w := []float64{1, 1}
	got := lehmerMean(v, w)
	if math.Abs(got-5.0/3.0) > 1e-12 {
		t.Fatalf("lehmer %v", got)
	}
	if weightedMean(v, w) != 1.5 {
		t.Fatal("weighted mean")
	}
}

func TestLpsrSchedule(t *testing.T) {
	if lpsrNext(0, 100, 48, 4) != 48 {
		t.Fatal("first generation must keep init pop")
	}
	if lpsrNext(100, 100, 48, 4) != 4 {
		t.Fatal("last generation must shrink to floor")
	}
	if n := lpsrNext(50, 100, 48, 4); n <= 4 || n >= 48 {
		t.Fatalf("midpoint %d", n)
	}
}

func TestDiversityCollapsed(t *testing.T) {
	s := newShadeState(4, 2, []float64{0, 0}, []float64{1, 1})
	pts := [][]float64{{0.5, 0.5}, {0.5, 0.5}, {0.5, 0.5}, {0.5, 0.5}}
	if s.diversity(pts) != 0 {
		t.Fatal("identical points must give zero diversity")
	}
	pts[0] = []float64{0, 0}
	if s.diversity(pts) <= 0 {
		t.Fatal("spread points must give positive diversity")
	}
}

func TestShadeMemoryUpdate(t *testing.T) {
	s := newShadeState(4, 2, []float64{0, 0}, []float64{1, 1})
	s.succF = []float64{0.3, 0.7}
	s.succCR = []float64{0.2, 0.4}
	s.succW = []float64{1, 3}
	pop := [][]float64{{0.2, 0.2}, {0.4, 0.4}, {0.6, 0.6}, {0.8, 0.8}}
	loss := []float64{0.1, 0.2, 0.3, 0.4}
	before := s.memF[0]
	_, _, _ = s.afterGeneration(newRng(3), 1, 100, pop, loss, 0)
	if s.memF[0] == before {
		t.Fatal("memory not updated")
	}
	if len(s.succF) != 0 {
		t.Fatal("success log not cleared")
	}
}

func TestShadeRestartOnCollapse(t *testing.T) {
	s := newShadeState(3, 2, []float64{0, 0}, []float64{1, 1})
	pop := [][]float64{{0.5, 0.5}, {0.5, 0.5}, {0.5, 0.5}}
	loss := []float64{0.1, 0.2, 0.3}
	s.prevBest = loss[0]
	for i := 0; i < shadeStallGens; i++ {
		s.stall++
	}
	moved := 0
	for j := range pop[0] {
		pop[1][j] = pop[0][j]
	}
	_, _, _ = s.afterGeneration(newRng(5), 50, 100, pop, loss, 0)
	for j := range pop[1] {
		if pop[1][j] != pop[0][j] {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("restart did not perturb population")
	}
	if s.stall != 0 {
		t.Fatal("stall counter not reset")
	}
}

func TestShadeArchiveTrim(t *testing.T) {
	s := newShadeState(2, 2, []float64{0, 0}, []float64{1, 1})
	s.pushArchive([]float64{0.1, 0.1})
	s.pushArchive([]float64{0.2, 0.2})
	s.pushArchive([]float64{0.3, 0.3})
	s.trimArchive(newRng(4))
	if len(s.archive) > 2 {
		t.Fatalf("archive %d", len(s.archive))
	}
}

func TestShadeTrialWithinOperators(t *testing.T) {
	s := newShadeState(6, 3, []float64{-5, -5, -5}, []float64{5, 5, 5})
	pop := make([][]float64, 6)
	loss := make([]float64, 6)
	rng := newRng(8)
	for i := range pop {
		pop[i] = uniformIn(rng, []float64{-5, -5, -5}, []float64{5, 5, 5})
		loss[i] = float64(i)
	}
	tr := s.trial(pop, loss, 0, rng)
	if len(tr) != 3 {
		t.Fatalf("trial dim %d", len(tr))
	}
	f := s.lastF[0]
	if f <= 0 || f > 1 {
		t.Fatalf("lastF %v", f)
	}
}
