package server

import (
	"errors"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

var ErrAdmissionTimeout = errors.New("server: machine core budget stayed full past the admission wait")

type Admission struct {
	mu        sync.Mutex
	cond      *sync.Cond
	coresFree int
	ttFree    int64
	waitMax   time.Duration
}

func newAdmission() *Admission {
	a := &Admission{
		coresFree: config.MachineCores,
		ttFree:    config.MachineTTBudgetBytes,
		waitMax:   time.Duration(config.AdmissionWaitMaxMs) * time.Millisecond,
	}
	a.cond = sync.NewCond(&a.mu)
	return a
}

func (a *Admission) AcquireCores(n int) error {
	if n <= 0 {
		return nil
	}
	return a.acquireCoresWait(n, a.waitMax)
}

func (a *Admission) TryAcquireCores(n int) bool {
	if n <= 0 {
		return true
	}
	return a.acquireCoresWait(n, 0) == nil
}

func (a *Admission) acquireCoresWait(n int, wait time.Duration) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.coresFree < n {
		if wait <= 0 {
			return ErrAdmissionTimeout
		}
		deadline := time.Now().Add(wait)
		timer := time.AfterFunc(wait, func() {
			a.mu.Lock()
			a.cond.Broadcast()
			a.mu.Unlock()
		})
		defer timer.Stop()
		for a.coresFree < n {
			if time.Now().After(deadline) {
				return ErrAdmissionTimeout
			}
			a.cond.Wait()
		}
	}
	a.coresFree -= n
	return nil
}

func (a *Admission) ReleaseCores(n int) {
	if n <= 0 {
		return
	}
	a.mu.Lock()
	a.coresFree += n
	a.cond.Broadcast()
	a.mu.Unlock()
}

func (a *Admission) AcquireTT(bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ttFree < bytes {
		return ErrMachineBusy
	}
	a.ttFree -= bytes
	return nil
}

func (a *Admission) ReleaseTT(bytes int64) {
	if bytes <= 0 {
		return
	}
	a.mu.Lock()
	a.ttFree += bytes
	a.mu.Unlock()
}

func (a *Admission) LiveCores() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return config.MachineCores - a.coresFree
}

func (a *Admission) TTFree() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ttFree
}

func (a *Admission) CanAcquireTT(bytes int64) bool {
	if bytes <= 0 {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ttFree >= bytes
}

func (rm *RoomManager) LiveCoreBookings() int {
	return rm.adm.LiveCores()
}
