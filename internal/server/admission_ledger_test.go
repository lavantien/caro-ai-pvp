package server

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func newTestAdmission(wait time.Duration) *Admission {
	a := newAdmission()
	a.waitMax = wait
	return a
}

func admissionHealthy(t *testing.T, a *Admission) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.coresFree < 0 || a.coresFree > config.MachineCores {
		t.Fatalf("coresFree = %d, want inside [0, %d]", a.coresFree, config.MachineCores)
	}
	if a.ttFree < 0 || a.ttFree > config.MachineTTBudgetBytes {
		t.Fatalf("ttFree = %d, want inside [0, %d]", a.ttFree, config.MachineTTBudgetBytes)
	}
}

func TestAdmissionAcquireWaitsForRelease(t *testing.T) {
	a := newTestAdmission(2 * time.Second)
	if err := a.AcquireCores(config.MachineCores); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(released)
		a.ReleaseCores(config.MachineCores)
	}()
	if err := a.AcquireCores(config.MachineCores); err != nil {
		t.Fatalf("second acquire after the release: %v", err)
	}
	<-released
	admissionHealthy(t, a)
}

func TestAdmissionAcquireTimesOutLoudly(t *testing.T) {
	a := newTestAdmission(30 * time.Millisecond)
	if err := a.AcquireCores(config.MachineCores); err != nil {
		t.Fatalf("hold the budget: %v", err)
	}
	start := time.Now()
	err := a.AcquireCores(1)
	if err == nil {
		t.Fatal("acquire under a full budget = nil, want the timeout refusal")
	}
	if !errors.Is(err, ErrAdmissionTimeout) {
		t.Fatalf("timeout error = %v, want ErrAdmissionTimeout", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("refusal after %v, want the bounded wait, not an instant fail", elapsed)
	}
	if got := a.LiveCores(); got != config.MachineCores {
		t.Fatalf("live cores = %d after the refused acquire, want the held %d with no partial grant", got, config.MachineCores)
	}
}

func TestAdmissionNeverGrantsPartially(t *testing.T) {
	a := newTestAdmission(20 * time.Millisecond)
	if err := a.AcquireCores(config.MachineCores - 3); err != nil {
		t.Fatalf("partial fill: %v", err)
	}
	if err := a.AcquireCores(5); !errors.Is(err, ErrAdmissionTimeout) {
		t.Fatalf("oversized acquire = %v, want the timeout refusal", err)
	}
	if got := a.LiveCores(); got != config.MachineCores-3 {
		t.Fatalf("live cores = %d after the refused oversized acquire, want the held %d", got, config.MachineCores-3)
	}
}

func TestAdmissionOversizeRequestCannotEverFit(t *testing.T) {
	a := newTestAdmission(10 * time.Millisecond)
	if err := a.AcquireCores(config.MachineCores + 1); !errors.Is(err, ErrAdmissionTimeout) {
		t.Fatalf("acquire over the machine total = %v, want the timeout refusal", err)
	}
	if got := a.LiveCores(); got != 0 {
		t.Fatalf("live cores = %d after the impossible acquire, want 0", got)
	}
}

func TestAdmissionZeroCoreAcquireIsFree(t *testing.T) {
	a := newTestAdmission(time.Nanosecond)
	if err := a.AcquireCores(0); err != nil {
		t.Fatalf("zero acquire: %v", err)
	}
	if err := a.AcquireCores(-1); err != nil {
		t.Fatalf("negative acquire: %v", err)
	}
	if got := a.LiveCores(); got != 0 {
		t.Fatalf("live cores = %d, want 0", got)
	}
}

func TestAdmissionTryAcquireNeverWaits(t *testing.T) {
	a := newTestAdmission(time.Hour)
	if !a.TryAcquireCores(4) {
		t.Fatal("try acquire on a fresh budget = false, want true")
	}
	if !a.TryAcquireCores(config.MachineCores - 4) {
		t.Fatal("try acquire to the exact fit = false, want true")
	}
	if a.TryAcquireCores(1) {
		t.Fatal("try acquire over the full budget = true, want the instant refusal")
	}
	a.ReleaseCores(config.MachineCores)
	if got := a.LiveCores(); got != 0 {
		t.Fatalf("live cores = %d after the release, want 0", got)
	}
}

func TestAdmissionTTRefuseAndRelease(t *testing.T) {
	a := newTestAdmission(time.Millisecond)
	if err := a.AcquireTT(config.MachineTTBudgetBytes); err != nil {
		t.Fatalf("exact-fit TT acquire: %v", err)
	}
	if err := a.AcquireTT(1); !errors.Is(err, ErrMachineBusy) {
		t.Fatalf("TT acquire over the budget = %v, want ErrMachineBusy", err)
	}
	if got := a.TTFree(); got != 0 {
		t.Fatalf("ttFree = %d after the refusal, want 0 with no partial grant", got)
	}
	a.ReleaseTT(config.MachineTTBudgetBytes)
	if got := a.TTFree(); got != config.MachineTTBudgetBytes {
		t.Fatalf("ttFree = %d after the release, want %d", got, config.MachineTTBudgetBytes)
	}
	if err := a.AcquireTT(0); err != nil {
		t.Fatalf("zero-byte TT acquire: %v", err)
	}
	admissionHealthy(t, a)
}

func TestAdmissionBroadcastWakesEveryWaiter(t *testing.T) {
	a := newTestAdmission(2 * time.Second)
	if err := a.AcquireCores(config.MachineCores - 1); err != nil {
		t.Fatalf("partial hold: %v", err)
	}
	const waiters = 4
	var wg sync.WaitGroup
	wg.Add(waiters)
	for range waiters {
		go func() {
			defer wg.Done()
			if err := a.AcquireCores(1); err != nil {
				t.Errorf("waiter: %v", err)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	a.ReleaseCores(config.MachineCores - 1)
	wg.Wait()
	if got := a.LiveCores(); got != waiters {
		t.Fatalf("live cores = %d after the woken waiters, want %d", got, waiters)
	}
}

func TestAdmissionChurnInvariants(t *testing.T) {
	a := newTestAdmission(2 * time.Millisecond)
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(w int) {
			defer wg.Done()
			for i := range 100 {
				n := 1 + (w+i)%4
				if a.TryAcquireCores(n) {
					a.ReleaseCores(n)
				}
				if err := a.AcquireCores(1); err == nil {
					a.ReleaseCores(1)
				}
				bytes := int64(1+(w+i)%3) << 20
				if a.AcquireTT(bytes) == nil {
					a.ReleaseTT(bytes)
				}
			}
		}(w)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	for {
		select {
		case <-done:
			if got := a.LiveCores(); got != 0 {
				t.Fatalf("live cores = %d after the balanced churn, want 0", got)
			}
			if got := a.TTFree(); got != config.MachineTTBudgetBytes {
				t.Fatalf("ttFree = %d after the balanced churn, want the budget", got)
			}
			return
		default:
			admissionHealthy(t, a)
		}
	}
}

func FuzzAdmissionSequences(f *testing.F) {
	f.Add("1001R1A2T80")
	f.Add("AAAA1111R4TffffffR4")
	f.Add("a1b2c3d4e5")
	f.Fuzz(func(t *testing.T, script string) {
		a := newTestAdmission(time.Millisecond)
		var heldCores int
		var heldTT int64
		for _, c := range script {
			switch c {
			case 'a', 'b', 'c', 'd':
				n := int(c-'a') + 2
				if a.TryAcquireCores(n) {
					heldCores += n
				}
			case 'A', 'B', 'C':
				n := int(c-'A') + 2
				if a.AcquireCores(n) == nil {
					heldCores += n
				}
			case '1', '2', '3', '4':
				n := int(c - '0')
				if heldCores >= n {
					a.ReleaseCores(n)
					heldCores -= n
				}
			case 'T':
				if a.AcquireTT(64<<20) == nil {
					heldTT += 64 << 20
				}
			case 't':
				if heldTT >= 64<<20 {
					a.ReleaseTT(64 << 20)
					heldTT -= 64 << 20
				}
			}
			admissionHealthy(t, a)
		}
		a.ReleaseCores(heldCores)
		a.ReleaseTT(heldTT)
		admissionHealthy(t, a)
	})
}
