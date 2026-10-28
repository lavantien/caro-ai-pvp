package engine

import (
	"testing"
	"time"
)

func TestSearchMateHaltWaitsForVerificationIteration(t *testing.T) {
	b := mate1Board(t)
	e := New(testTTBytes)
	_, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 4)
	if stats.RootIters < 2 {
		t.Fatalf("mate in 1 halted after %d iterations, want the verification iteration first", stats.RootIters)
	}
	if want := stats.RootIters; stats.Depth > want {
		t.Errorf("completed depth %d exceeds root iters %d", stats.Depth, want)
	}
}

func TestSMPMateHaltWaitsForVerificationIteration(t *testing.T) {
	b := mate1Board(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	_, stats := s.SearchDepth(b, NewFixedBudget(2*time.Second), 4)
	if stats.RootIters < 2 {
		t.Fatalf("smp mate in 1 halted after %d iterations, want the verification iteration first", stats.RootIters)
	}
}
