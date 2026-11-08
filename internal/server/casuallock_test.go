package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockExclusiveBlocksASecondHandleUntilRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	first, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	defer func() { _ = first.Close() }()
	if err := lockExclusive(first, 0); err != nil {
		t.Fatalf("lock first: %v", err)
	}
	second, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open second: %v", err)
	}
	defer func() { _ = second.Close() }()
	start := time.Now()
	err = lockExclusive(second, 80*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("second handle locked while the first held the file, want exclusion")
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("second handle gave up after %s, want at least the 80ms wait", elapsed)
	}
	if err := unlockExclusive(first); err != nil {
		t.Fatalf("unlock first: %v", err)
	}
	if err := lockExclusive(second, 0); err != nil {
		t.Fatalf("relock after release: %v", err)
	}
	if err := unlockExclusive(second); err != nil {
		t.Fatalf("unlock second: %v", err)
	}
}

func TestLockExclusiveTakesAFreeFileImmediately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.txt")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := lockExclusive(f, 0); err != nil {
		t.Fatalf("lock a free file: %v", err)
	}
	if err := unlockExclusive(f); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := lockExclusive(f, time.Nanosecond); err != nil {
		t.Fatalf("lock a released file with a tiny wait: %v", err)
	}
	if err := unlockExclusive(f); err != nil {
		t.Fatalf("second unlock: %v", err)
	}
}
