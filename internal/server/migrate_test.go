package server

import (
	"database/sql"
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
	defer func() { _ = s.Close() }()

	count, max := schemaVersionRows(t, s)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("schema_version rows = %d, max = %d, want %d and %d", count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	tables := schemaCount(t, s, "table",
		"'schema_version','users','sessions','series','games','game_stats','rating_events','tournament_runs','tournament_participants','tournament_series','tournament_games','tournament_standings'")
	if tables != 12 {
		t.Errorf("known tables found = %d, want 12", tables)
	}
	// The reserved bot seat accounts of human-vs-bot matches ride the v4
	// seed: one users row per tier under the "AI <tier>" name, salt and hash
	// empty so no password can ever verify against them, and a fresh
	// database's real ids still grow from 1 past the seats.
	for i := range config.Tiers {
		var salt, hash []byte
		if err := s.db.QueryRow(
			`SELECT salt, hash FROM users WHERE username = ?`, config.BotAccountName(i),
		).Scan(&salt, &hash); err != nil {
			t.Fatalf("bot account row of tier %d: %v", i, err)
		}
		if len(salt) != 0 || len(hash) != 0 {
			t.Errorf("bot account %q = salt %d hash %d bytes, want both empty",
				config.BotAccountName(i), len(salt), len(hash))
		}
	}
	indexes := schemaCount(t, s, "index",
		"'idx_games_series','idx_rating_events_user','idx_series_red','idx_series_blue','idx_games_red_user','idx_games_blue_user','idx_tournament_games_series','idx_tournament_games_run','idx_tournament_series_run'")
	if indexes != 9 {
		t.Errorf("known indexes found = %d, want 9", indexes)
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
	defer func() { _ = again.Close() }()
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

func TestMigrateGuardRejectsScriptCountMismatch(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	saved := migrations
	migrations = saved[:0]
	defer func() { migrations = saved }()
	err := s.migrate()
	if err == nil || !strings.Contains(err.Error(), "config.SQLiteSchemaVersion") {
		t.Fatalf("migrate with truncated scripts = %v, want the hub-count guard error", err)
	}
}

func TestFailedMigrationRollsBackAndStays(t *testing.T) {
	path := dbPath(t)
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	defer func() { _ = db.Close() }()
	s := &Store{db: db}

	// Swap in a same-count broken script set so the hub guard passes and
	// the failure lands in the script application itself.
	saved := migrations
	defer func() { migrations = saved }()
	migrations = []string{
		"CREATE TABLE boom (;\nCREATE TABLE never (x)",
		"CREATE TABLE never2 (y)",
		"CREATE TABLE never3 (z)",
		"CREATE TABLE never4 (w)",
		"CREATE TABLE never5 (v)",
	}
	err = s.migrate()
	if err == nil || !strings.Contains(err.Error(), "apply migration 1") {
		t.Fatalf("migrate with broken script = %v, want apply migration 1 failure", err)
	}
	_, max := schemaVersionRows(t, s)
	if max != 0 {
		t.Errorf("version after failed migration = %d, want 0 (no partial recording)", max)
	}
	if n := schemaCount(t, s, "table", "'boom','never'"); n != 0 {
		t.Errorf("partial tables = %d, want 0 (transaction rolled back)", n)
	}

	// With the real scripts restored, the same database migrates cleanly.
	migrations = saved
	if err := s.migrate(); err != nil {
		t.Fatalf("migrate after rollback: %v", err)
	}
	if _, max = schemaVersionRows(t, s); max != config.SQLiteSchemaVersion {
		t.Errorf("version after clean retry = %d, want %d", max, config.SQLiteSchemaVersion)
	}
}

// explainDetail returns one query's EXPLAIN QUERY PLAN detail lines joined,
// so index usage asserts read the planner's own words.
func explainDetail(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()
	rows, err := s.db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain %q: %v", query, err)
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("explain scan %q: %v", query, err)
		}
		b.WriteString(detail + "; ")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows %q: %v", query, err)
	}
	return b.String()
}

// TestSchemaV2IndexesGamesPlayerColumns pins the player-column indexes of
// the games table: without them every MatchHistory and UserStats lookup
// full-scans games.
func TestSchemaV2IndexesGamesPlayerColumns(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	if n := schemaCount(t, s, "index", "'idx_games_red_user','idx_games_blue_user'"); n != 2 {
		t.Errorf("games player indexes found = %d, want 2 (idx_games_red_user, idx_games_blue_user)", n)
	}
	for _, tc := range []struct{ idx, query string }{
		{"idx_games_red_user", `SELECT COUNT(*) FROM games WHERE red_user = ?`},
		{"idx_games_blue_user", `SELECT COUNT(*) FROM games WHERE blue_user = ?`},
	} {
		plan := explainDetail(t, s, tc.query, int64(1))
		// The planner may pick the index as covering or not; either way the
		// scan of games must be gone.
		if !strings.Contains(plan, tc.idx) || strings.Contains(plan, "SCAN games") {
			t.Errorf("plan for %q = %q, want it to ride %s", tc.query, plan, tc.idx)
		}
	}
}

// TestMigrateV1DatabaseUpgradesToV2 pins the forward-only upgrade: a
// database holding only version 1 gains the v2 indexes on reopen, without
// re-running v1 or touching data. With later versions landed the reopen
// continues to the current one, so the assertion also proves the tournament
// tables and the v4 additions ride along.
func TestMigrateV1DatabaseUpgradesToV2(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	if _, err := s.db.Exec(`DELETE FROM schema_version WHERE version >= 2`); err != nil {
		t.Fatalf("roll ledger back to v1: %v", err)
	}
	// Reverse the v5 column too: the surgery rewinds the ledger only, and
	// the ALTER of migration 5 cannot re-add an existing column.
	if _, err := s.db.Exec(`ALTER TABLE games DROP COLUMN bot_name`); err != nil {
		t.Fatalf("drop the v5 column: %v", err)
	}
	for _, idx := range []string{"idx_games_red_user", "idx_games_blue_user"} {
		if _, err := s.db.Exec(`DROP INDEX ` + idx); err != nil {
			t.Fatalf("drop %s: %v", idx, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	again := mustOpen(t, path)
	defer func() { _ = again.Close() }()
	count, max := schemaVersionRows(t, again)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("after upgrade: schema_version rows = %d max = %d, want %d and %d",
			count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	if n := schemaCount(t, again, "index", "'idx_games_series','idx_games_red_user','idx_games_blue_user'"); n != 3 {
		t.Errorf("indexes after upgrade = %d, want 3 (v1 pair kept, v2 pair added)", n)
	}
	if n := schemaCount(t, again, "table",
		"'tournament_runs','tournament_participants','tournament_series','tournament_games','tournament_standings','game_stats'"); n != 6 {
		t.Errorf("later tables after upgrade = %d, want 6 (v3 and v4 applied after v2)", n)
	}
}

// TestMigrateV2DatabaseUpgradesToV3 pins the forward-only upgrade: a
// database holding only version 2 gains the tournament tables on reopen,
// without re-running v1/v2 or touching the v2-era data.
func TestMigrateV2DatabaseUpgradesToV3(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	if _, err := s.db.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		"veteran", config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_version WHERE version >= 3`); err != nil {
		t.Fatalf("roll ledger back to v2: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE games DROP COLUMN bot_name`); err != nil {
		t.Fatalf("drop the v5 column: %v", err)
	}
	for _, table := range []string{"tournament_standings", "tournament_games", "tournament_series", "tournament_participants", "tournament_runs"} {
		if _, err := s.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	again := mustOpen(t, path)
	defer func() { _ = again.Close() }()
	count, max := schemaVersionRows(t, again)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("after upgrade: schema_version rows = %d max = %d, want %d and %d",
			count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	if n := schemaCount(t, again, "table",
		"'tournament_runs','tournament_participants','tournament_series','tournament_games','tournament_standings','game_stats'"); n != 6 {
		t.Errorf("later tables after upgrade = %d, want 6 (v3 pair kept, v4 added)", n)
	}
	var users int
	if err := again.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "veteran").Scan(&users); err != nil {
		t.Fatalf("reread user: %v", err)
	}
	if users != 1 {
		t.Errorf("v2-era user count after upgrade = %d, want 1 (no data loss)", users)
	}
}

// TestMigrateV3DatabaseUpgradesToV4 pins the forward-only upgrade: a database
// holding only version 3 gains the game_stats table and the reserved bot seat
// rows on reopen, without re-running v1..v3 or touching the v3-era data.
func TestMigrateV3DatabaseUpgradesToV4(t *testing.T) {
	path := dbPath(t)
	s := mustOpen(t, path)
	if _, err := s.db.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		"veteran", config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_version WHERE version >= 4`); err != nil {
		t.Fatalf("roll ledger back to v3: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE games DROP COLUMN bot_name`); err != nil {
		t.Fatalf("drop the v5 column: %v", err)
	}
	if _, err := s.db.Exec(`DROP TABLE game_stats`); err != nil {
		t.Fatalf("drop game_stats: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM users WHERE length(salt) = 0`); err != nil {
		t.Fatalf("drop the seeded bot seats: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	again := mustOpen(t, path)
	defer func() { _ = again.Close() }()
	count, max := schemaVersionRows(t, again)
	if count != config.SQLiteSchemaVersion || max != config.SQLiteSchemaVersion {
		t.Errorf("after upgrade: schema_version rows = %d max = %d, want %d and %d",
			count, max, config.SQLiteSchemaVersion, config.SQLiteSchemaVersion)
	}
	if n := schemaCount(t, again, "table", "'game_stats'"); n != 1 {
		t.Errorf("game_stats after upgrade = %d, want 1", n)
	}
	for i := range config.Tiers {
		var salt, hash []byte
		if err := again.db.QueryRow(
			`SELECT salt, hash FROM users WHERE username = ?`, config.BotAccountName(i),
		).Scan(&salt, &hash); err != nil {
			t.Fatalf("bot seat row of tier %d after upgrade: %v", i, err)
		}
		if len(salt) != 0 || len(hash) != 0 {
			t.Errorf("bot seat %q after upgrade = salt %d hash %d bytes, want both empty",
				config.BotAccountName(i), len(salt), len(hash))
		}
	}
	var users int
	if err := again.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "veteran").Scan(&users); err != nil {
		t.Fatalf("reread user: %v", err)
	}
	if users != 1 {
		t.Errorf("v3-era user count after upgrade = %d, want 1 (no data loss)", users)
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
	defer func() { _ = again.Close() }()
	var n int
	if err := again.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "probe").Scan(&n); err != nil {
		t.Fatalf("reread user: %v", err)
	}
	if n != 1 {
		t.Errorf("user count after checkpoint cycle = %d, want 1", n)
	}
}
