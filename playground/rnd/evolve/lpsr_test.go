package main

import "testing"

func TestLpsrRemovesWorstFirst(t *testing.T) {
	s := newShadeState(8, 1, []float64{0}, []float64{10})
	pop := make([][]float64, 8)
	loss := make([]float64, 8)
	for i := range pop {
		pop[i] = []float64{float64(i)}
		loss[i] = float64(i)
	}
	best := 2
	loss[best] = -1
	np, nl, nb := s.afterGeneration(newRng(11), 90, 100, pop, loss, best)
	if len(np) != 4 {
		t.Fatalf("shrunk pop %d", len(np))
	}
	for _, l := range nl {
		if l < -1 || l > 3 {
			t.Fatalf("kept loss %v outside the four best", l)
		}
	}
	if np[nb][0] != float64(best) {
		t.Fatalf("incumbent vector lost: best index %d points at %v", nb, np[nb])
	}
}
