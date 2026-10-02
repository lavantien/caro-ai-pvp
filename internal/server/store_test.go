package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func dbPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "caro.sqlite")
}

func TestOpenAppliesConfiguredPragmas(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != config.SQLiteJournalWAL {
		t.Errorf("journal_mode = %q, want %q", mode, config.SQLiteJournalWAL)
	}
	var sync int
	if err := s.db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("synchronous: %v", err)
	}
	// The synchronous pragma reports on SQLite's 0..3 scale, NORMAL is 1.
	if sync != 1 {
		t.Errorf("synchronous = %d, want 1 for %q", sync, config.SQLiteSyncNormal)
	}
	var busy int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busy != config.SQLiteBusyTimeoutMs {
		t.Errorf("busy_timeout = %d, want %d", busy, config.SQLiteBusyTimeoutMs)
	}
}

func TestForeignKeysHoldOnEveryPooledConn(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	held, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin conn: %v", err)
	}
	defer held.Close()
	var heldFK int
	if err := held.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&heldFK); err != nil {
		t.Fatalf("pinned foreign_keys: %v", err)
	}
	// The pool must open a second connection here, the first stays pinned.
	var pooledFK int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&pooledFK); err != nil {
		t.Fatalf("second conn foreign_keys: %v", err)
	}
	if heldFK != 1 || pooledFK != 1 {
		t.Errorf("foreign_keys = pinned %d, second conn %d, want 1 on both", heldFK, pooledFK)
	}
}

func TestCloseIsIdempotentPerStore(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Close drains the pool, a second call must not panic or error on a nil
	// busy loop: sql.DB.Close is documented safe to call repeatedly.
	if err := s.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}
