package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestSidecarLockRefusesTheSecondHolder(t *testing.T) {
	corpus := filepath.Join(t.TempDir(), "c.txt")
	lk, err := acquireSidecarLock(corpus, "corpus")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := acquireSidecarLock(corpus, "corpus"); err == nil {
		t.Fatalf("second acquire over a live lock succeeded")
	} else if !strings.Contains(err.Error(), "concurrent corpus run refused") {
		t.Fatalf("second acquire error = %v, want the concurrent-run refusal", err)
	}
	if _, err := acquireSidecarLock(corpus, "overlay"); err != nil {
		t.Fatalf("a different phase shares the lock: %v", err)
	}
	lk.Release()
	again, err := acquireSidecarLock(corpus, "corpus")
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	again.Release()
	if _, err := os.Stat(lockPath(corpus, "corpus")); !os.IsNotExist(err) {
		t.Fatalf("release left the lock file behind: %v", err)
	}
}

func TestSidecarLockTakesOverAStaleHolder(t *testing.T) {
	corpus := filepath.Join(t.TempDir(), "c.txt")
	dead := lockPath(corpus, "corpus")
	if err := os.WriteFile(dead, []byte(strconv.Itoa(1234)), 0o644); err != nil {
		t.Fatalf("seed dead holder: %v", err)
	}
	stale := time.Now().Add(-time.Duration(2*config.BookLockStaleMs) * time.Millisecond)
	if err := os.Chtimes(dead, stale, stale); err != nil {
		t.Fatalf("age the lock: %v", err)
	}
	takeover, err := acquireSidecarLock(corpus, "corpus")
	if err != nil {
		t.Fatalf("stale takeover refused: %v", err)
	}
	takeover.Release()
	fresh, err := acquireSidecarLock(corpus, "corpus")
	if err != nil {
		t.Fatalf("acquire after takeover release: %v", err)
	}
	fresh.Release()
}
