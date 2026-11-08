//go:build windows

package server

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

const casualLockRetry = 2 * time.Millisecond

func lockExclusive(f *os.File, wait time.Duration) error {
	if wait <= 0 {
		return lockFileRange(f, 0)
	}
	deadline := time.Now().Add(wait)
	for {
		err := lockFileRange(f, windows.LOCKFILE_FAIL_IMMEDIATELY)
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return err
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("server: casual lock still held after %s: %w", wait, err)
		}
		time.Sleep(casualLockRetry)
	}
}

func unlockExclusive(f *os.File) error {
	var overlapped windows.Overlapped
	err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), &overlapped)
	if err != nil {
		return fmt.Errorf("UnlockFileEx: %w", err)
	}
	return nil
}

func lockFileRange(f *os.File, extraFlags uint32) error {
	flags := windows.LOCKFILE_EXCLUSIVE_LOCK | extraFlags
	var overlapped windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, ^uint32(0), ^uint32(0), &overlapped)
	if err != nil {
		return fmt.Errorf("LockFileEx: %w", err)
	}
	return nil
}
