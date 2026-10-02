package server

import (
	"context"
	"database/sql"
	"errors"
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
	defer func() { _ = s.Close() }()

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
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	held, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin conn: %v", err)
	}
	defer func() { _ = held.Close() }()
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

// TestWithinTxCommitsAndRollsBack pins the cross-package persistence
// surface: a returning fn commits and its writes are visible, an erroring fn
// leaves nothing behind.
func TestWithinTxCommitsAndRollsBack(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	if err := s.WithinTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE withintx_probe (x INTEGER)`)
		return err
	}); err != nil {
		t.Fatalf("commit unit: %v", err)
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM withintx_probe").Scan(&n); err != nil {
		t.Fatalf("read committed unit: %v", err)
	}
	if n != 0 {
		t.Errorf("probe rows = %d, want 0", n)
	}

	sentinel := errors.New("boom")
	if err := s.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO withintx_probe VALUES (1)"); err != nil {
			return err
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("failing unit = %v, want the sentinel", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM withintx_probe").Scan(&n); err != nil {
		t.Fatalf("reread after rollback: %v", err)
	}
	if n != 0 {
		t.Errorf("probe rows after rollback = %d, want 0", n)
	}
}

func TestAccessorsFailCleanlyAfterClose(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	calls := map[string]func() error{
		"create user":    func() error { _, _, err := s.CreateUserIfAbsent("x", []byte("s"), []byte("h")); return err },
		"fetch user":     func() error { _, err := s.UserByUsername("x"); return err },
		"insert session": func() error { return s.InsertSession(Session{Token: []byte("t")}) },
		"fetch session":  func() error { _, err := s.SessionByToken([]byte("t"), 0); return err },
		"delete session": func() error { return s.DeleteSession([]byte("t")) },
		"create series":  func() error { _, err := s.CreateSeries(context.Background(), 0, 3, 1, 2); return err },
		"fetch series":   func() error { _, err := s.SeriesByID(1); return err },
		"update series":  func() error { return s.UpdateSeries(1, SeriesStateFinished, nil, nil) },
		"append game":    func() error { _, err := s.AppendGame(Game{Moves: []byte{1}}); return err },
		"append rating":  func() error { _, err := s.AppendRatingEvent(RatingEvent{}); return err },
		"rating history": func() error { _, err := s.RatingHistoryByUser(1); return err },
		"user stats":     func() error { _, err := s.UserStats(1); return err },
		"match history":  func() error { _, err := s.MatchHistory(1); return err },
		"migrate":        s.migrate,
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s after close: want error, got nil", name)
		} else if errors.Is(err, ErrNotFound) {
			t.Errorf("%s after close = ErrNotFound, want the closed-pool error", name)
		}
	}
}
