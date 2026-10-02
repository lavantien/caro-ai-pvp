package server

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Migration-ledger failure paths: a corrupted version table refuses startup,
// a dead pool fails the transaction begin loudly, and a ledger conflict rolls
// the whole script back.

// TestMigrateRefusesCorruptedVersionLedger seeds a schema_version table
// without the version column (the shape a foreign writer or a truncated file
// leaves): the bootstrapping create passes because the table exists, and the
// version read fails loudly instead of applying scripts against an unknown
// base.
func TestMigrateRefusesCorruptedVersionLedger(t *testing.T) {
	path := dbPath(t)
	raw, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_version (junk INTEGER)`); err != nil {
		t.Fatalf("seed corrupted ledger: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw close: %v", err)
	}

	_, err = Open(path)
	if err == nil || !strings.Contains(err.Error(), "read schema version") {
		t.Fatalf("open over a corrupted ledger = %v, want the read-schema-version failure", err)
	}
}

func TestApplyMigrationBeginFailureOnClosedPool(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	err := s.applyMigration(1, "CREATE TABLE probe(x)")
	if err == nil || !strings.Contains(err.Error(), "begin migration 1") {
		t.Fatalf("apply migration on a closed pool = %v, want the begin failure", err)
	}
}

// TestApplyMigrationLedgerConflictRollsBackScript applies a version already
// recorded: the script's table creation succeeds inside the transaction, the
// ledger insert violates the primary key, and the rollback must take the
// table with it.
func TestApplyMigrationLedgerConflictRollsBackScript(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	err := s.applyMigration(1, "CREATE TABLE ledger_probe(x)")
	if err == nil || !strings.Contains(err.Error(), "record migration 1") {
		t.Fatalf("re-apply version 1 = %v, want the ledger conflict", err)
	}
	if n := schemaCount(t, s, "table", "'ledger_probe'"); n != 0 {
		t.Errorf("ledger_probe tables = %d, want 0 (the script rolled back with the ledger insert)", n)
	}
	if count, max := schemaVersionRows(t, s); count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("ledger after the conflict = %d rows max %d, want untouched %d and %d",
			count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
}
