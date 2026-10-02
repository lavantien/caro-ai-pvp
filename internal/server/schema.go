package server

// Rooms are deliberately NOT persisted: live occupancy, ready flags, and
// connections are in-memory runtime state that dies with the process. The
// database records only what happened (users, sessions, series, games,
// rating events), never what is currently happening.

// Series lifecycle states, mirrored by the series.state CHECK constraint.
const (
	SeriesStateOngoing  = "ongoing"
	SeriesStateFinished = "finished"
)

// Game outcomes by stone color, mirrored by the games.outcome CHECK
// constraint. How a win happened is the separate games.won_by tag.
const (
	OutcomeRed  = "red"
	OutcomeBlue = "blue"
	OutcomeDraw = "draw"
)

// schemaV1 is the initial layout. Every timestamp is INTEGER unix seconds
// with a unixepoch() default, so expiry filters and ordering stay plain
// integer comparisons and DuckDB reads them without parsing.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	argon2_time INTEGER NOT NULL,
	argon2_memory INTEGER NOT NULL,
	argon2_parallelism INTEGER NOT NULL,
	salt BLOB NOT NULL,
	hash BLOB NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS sessions (
	token BLOB PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users (id),
	expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS series (
	id INTEGER PRIMARY KEY,
	tc_idx INTEGER NOT NULL,
	bo_len INTEGER NOT NULL,
	red_user INTEGER NOT NULL REFERENCES users (id),
	blue_user INTEGER NOT NULL REFERENCES users (id),
	state TEXT NOT NULL CHECK (state IN ('ongoing', 'finished')),
	winner INTEGER REFERENCES users (id),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	finished_at INTEGER
);

CREATE TABLE IF NOT EXISTS games (
	id INTEGER PRIMARY KEY,
	series_id INTEGER NOT NULL REFERENCES series (id),
	idx_in_series INTEGER NOT NULL,
	red_user INTEGER NOT NULL REFERENCES users (id),
	blue_user INTEGER NOT NULL REFERENCES users (id),
	outcome TEXT NOT NULL CHECK (outcome IN ('red', 'blue', 'draw')),
	moves BLOB NOT NULL,
	full_turns INTEGER NOT NULL,
	won_by TEXT,
	played_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS rating_events (
	id INTEGER PRIMARY KEY,
	game_id INTEGER NOT NULL REFERENCES games (id),
	user_id INTEGER NOT NULL REFERENCES users (id),
	delta INTEGER NOT NULL,
	rating_after INTEGER NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_games_series ON games (series_id);
CREATE INDEX IF NOT EXISTS idx_rating_events_user ON rating_events (user_id);
CREATE INDEX IF NOT EXISTS idx_series_red ON series (red_user);
CREATE INDEX IF NOT EXISTS idx_series_blue ON series (blue_user);
`

// migrations holds one SQL script per schema version: index i upgrades
// version i to version i+1. Its length must equal config.SQLiteSchemaVersion
// so the constants hub stays authoritative; migrate enforces that at
// startup. New versions only ever append, never edit a landed script.
var migrations = []string{schemaV1}
