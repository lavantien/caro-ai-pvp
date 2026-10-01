package engine

import (
	"testing"
	"time"
)

func TestFixedBudgetWindow(t *testing.T) {
	dl := NewFixedBudget(50 * time.Millisecond)
	if dl.Exceeded() {
		t.Fatal("fresh 50ms budget already exceeded")
	}
	time.Sleep(60 * time.Millisecond)
	if !dl.Exceeded() {
		t.Fatal("budget not exceeded after its window")
	}
}

func TestFixedBudgetNegativeIsExceeded(t *testing.T) {
	if !NewFixedBudget(-time.Second).Exceeded() {
		t.Fatal("negative budget must be exceeded immediately")
	}
}

func TestFixedBudgetStop(t *testing.T) {
	dl := NewFixedBudget(time.Hour)
	if dl.Exceeded() {
		t.Fatal("hour budget exceeded at once")
	}
	dl.Stop()
	if !dl.Exceeded() {
		t.Fatal("Stop must force the deadline exceeded")
	}
}

func TestFixedBudgetBudget(t *testing.T) {
	if got := NewFixedBudget(1500 * time.Millisecond).Budget(); got != 1500*time.Millisecond {
		t.Errorf("Budget() = %v, want 1.5s", got)
	}
}
