// Package server hosts the caro PvP server: embedded SQLite storage in WAL
// mode behind a single-writer mutation queue, argon2id auth, in-memory
// rooms, and the stats hub. The storage foundation lands first.
package server

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Store owns the embedded SQLite database. Every pragma rides the DSN so
// each pooled connection applies it, and the pool sizes to the host so the
// one serialized writer (the mutation queue built above this package) never
// starves concurrent readers.
type Store struct {
	db       *sql.DB
	closeOne sync.Once
	closeErr error
}

// Open opens the database at path, applies the config pragmas on every
// pooled connection, sizes the pool to the host, and runs the startup
// self-migration. On failure the pool is closed so Open leaks nothing.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("server: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(runtime.NumCPU())
	db.SetMaxIdleConns(runtime.NumCPU())
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close checkpoints the WAL back into the main file and truncates the -wal
// sidecar, so later analytics readers may copy the database file at rest.
// The pool always closes even when the checkpoint fails, and the close is
// idempotent: repeated calls replay the first result instead of erroring.
func (s *Store) Close() error {
	s.closeOne.Do(func() {
		_, ckErr := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		err := s.db.Close()
		if err == nil {
			err = ckErr
		}
		s.closeErr = err
	})
	return s.closeErr
}

// dsn builds the modernc.org/sqlite DSN. The mattn-style shorthand pragmas
// below are applied on every new connection, so pooled readers all see WAL
// journalling, NORMAL sync, the busy timeout, and enforced foreign keys.
func dsn(path string) string {
	q := make(url.Values, 4)
	q.Set("_journal_mode", config.SQLiteJournalWAL)
	q.Set("_synchronous", config.SQLiteSyncNormal)
	q.Set("_busy_timeout", strconv.Itoa(config.SQLiteBusyTimeoutMs))
	q.Set("_foreign_keys", "on")
	return filepath.ToSlash(path) + "?" + q.Encode()
}
