//go:build !windows

package server

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const casualLockRetry = 2 * time.Millisecond

func lockExclusive(f *os.File, wait time.Duration) error {
	if wait <= 0 {
		return flockHold(f, syscall.LOCK_EX)
	}
	deadline := time.Now().Add(wait)
	for {
		err := flockHold(f, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("flock: %w", err)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("server: casual lock still held after %s: %w", wait, err)
		}
		time.Sleep(casualLockRetry)
	}
}

func unlockExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("flock unlock: %w", err)
	}
	return nil
}

func flockHold(f *os.File, how int) error {
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			if err != nil {
				return fmt.Errorf("flock: %w", err)
			}
			return nil
		}
	}
}
