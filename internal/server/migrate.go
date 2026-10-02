package server

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// schemaVersionTable is the migration ledger. It is bootstrapped outside
// the numbered scripts so a fresh database can record its first version.
const schemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL DEFAULT (unixepoch())
)`

// migrate brings the database up to the newest script at startup. Scripts
// apply serially, each inside its own transaction with the target version
// recorded only on commit: a fresh database applies all, an up-to-date one
// applies none, and a database from the future fails loudly. There are no
// down migrations, ever.
func (s *Store) migrate() error {
	if len(migrations) != config.SQLiteSchemaVersion {
		return fmt.Errorf("server: %d migrations defined but config.SQLiteSchemaVersion = %d", len(migrations), config.SQLiteSchemaVersion)
	}
	if _, err := s.db.Exec(schemaVersionTable); err != nil {
		return fmt.Errorf("server: create schema_version: %w", err)
	}
	var current int
	if err := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&current); err != nil {
		return fmt.Errorf("server: read schema version: %w", err)
	}
	if current > len(migrations) {
		return fmt.Errorf("server: database schema version %d is newer than the supported version %d, forward-only", current, len(migrations))
	}
	for v := current; v < len(migrations); v++ {
		if err := s.applyMigration(v+1, migrations[v]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyMigration(version int, script string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("server: begin migration %d: %w", version, err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(script); err != nil {
		return fmt.Errorf("server: apply migration %d: %w", version, err)
	}
	if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("server: record migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server: commit migration %d: %w", version, err)
	}
	return nil
}
