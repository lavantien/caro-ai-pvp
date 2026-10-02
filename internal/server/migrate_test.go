package server

import (
	"os"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func mustOpen(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	return s
}

func schemaCount(t *testing.T, s *Store, kind string, names string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name IN ("+names+")",
		kind,
	).Scan(&n); err != nil {
		t.Fatalf("count %s %s: %v", kind, names, err)
	}
	return n
}

func schemaVersionRows(t *testing.T, s *Store) (count, max int) {
	t.Helper()
	if err := s.db.QueryRow("SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_version").Scan(&count, &max); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	return count, max
}

func TestMigrateFreshAppliesAll(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer s.Close()

	count, max := schemaVersionRows(t, s)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("schema_version rows = %d, max = %d, want %d and %d", count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	tables := schemaCount(t, s, "table", "'schema_version','users','sessions','series','games','rating_events'")
	if tables != 6 {
		t.Errorf("known tables found = %d, want 6", tables)
	}
	indexes := schemaCount(t, s, "index", "'idx_games_series','idx_rating_events_user','idx_series_red','idx_series_blue'")
	if indexes != 4 {
		t.Errorf("known indexes found = %d, want 4", indexes)
	}
}

func TestMigrateIdempotentAcrossReopen(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	if _, err := s.db.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		"alice", config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	again := mustOpen(t, path)
	defer again.Close()
	// A second migration pass that re-applied v1 would violate the
	// schema_version primary key, so survival plus an unchanged row count
	// proves the up-to-date database applied zero scripts.
	count, max := schemaVersionRows(t, again)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("after reopen: schema_version rows = %d, max = %d, want %d and %d", count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	var n int
	if err := again.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "alice").Scan(&n); err != nil {
		t.Fatalf("reread user: %v", err)
	}
	if n != 1 {
		t.Errorf("seeded user count = %d, want 1", n)
	}
}

func TestMigrateRejectsDatabaseFromTheFuture(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	future := config.SQLiteSchemaVersion + 1
	if _, err := s.db.Exec("INSERT INTO schema_version (version) VALUES (?)", future); err != nil {
		t.Fatalf("forge future version: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("open a future-version database: want loud error, got nil")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error = %q, want it to flag the newer schema version", err)
	}
}

func TestCloseCheckpointsWAL(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	if _, err := s.db.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		"probe", config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close with checkpoint: %v", err)
	}

	if fi, err := os.Stat(path + "-wal"); err == nil && fi.Size() > 0 {
		t.Fatalf("wal sidecar still holds %d bytes after checkpointing close", fi.Size())
	}

	again := mustOpen(t, path)
	defer again.Close()
	var n int
	if err := again.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "probe").Scan(&n); err != nil {
		t.Fatalf("reread user: %v", err)
	}
	if n != 1 {
		t.Errorf("user count after checkpoint cycle = %d, want 1", n)
	}
}
