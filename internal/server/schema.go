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

// schemaV2 indexes the games player columns: MatchHistory and UserStats
// filter on red_user/blue_user, which full-scanned games before this.
const schemaV2 = `
CREATE INDEX IF NOT EXISTS idx_games_red_user ON games (red_user);
CREATE INDEX IF NOT EXISTS idx_games_blue_user ON games (blue_user);
`

// schemaV3 is the M7 tournament layout of Scenario 2. Participants are
// normalized rows keyed (run_id, slot), not a roster json blob, so pairings
// and leaderboards join plain integers and no encoding can drift from the
// series rows. Standings carry no ratings table: tournament_games is the
// single source of truth, the rating law replays over it in Go, and
// tournament_standings is only the frozen snapshot written at run close.
// A series' score line is the structured red_first_wins/blue_first_wins
// pair (wins of the participant hosting game 1 with red, and of the other);
// winner_slot stays NULL for a majorityless drawn series, mirroring series.
const schemaV3 = `
CREATE TABLE IF NOT EXISTS tournament_runs (
	id INTEGER PRIMARY KEY,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	tc_idx INTEGER NOT NULL,
	bo_len INTEGER NOT NULL,
	start_rating INTEGER NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('ongoing', 'finished')),
	finished_at INTEGER
);

CREATE TABLE IF NOT EXISTS tournament_participants (
	run_id INTEGER NOT NULL REFERENCES tournament_runs (id),
	slot INTEGER NOT NULL,
	name TEXT NOT NULL,
	tier TEXT NOT NULL,
	PRIMARY KEY (run_id, slot)
);

CREATE TABLE IF NOT EXISTS tournament_series (
	id INTEGER PRIMARY KEY,
	run_id INTEGER NOT NULL REFERENCES tournament_runs (id),
	pairing_slot INTEGER NOT NULL,
	red_first_slot INTEGER NOT NULL,
	blue_first_slot INTEGER NOT NULL,
	winner_slot INTEGER,
	red_first_wins INTEGER NOT NULL DEFAULT 0,
	blue_first_wins INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	finished_at INTEGER,
	UNIQUE (run_id, pairing_slot),
	FOREIGN KEY (run_id, red_first_slot) REFERENCES tournament_participants (run_id, slot),
	FOREIGN KEY (run_id, blue_first_slot) REFERENCES tournament_participants (run_id, slot)
);

CREATE TABLE IF NOT EXISTS tournament_games (
	id INTEGER PRIMARY KEY,
	run_id INTEGER NOT NULL REFERENCES tournament_runs (id),
	series_id INTEGER NOT NULL REFERENCES tournament_series (id),
	idx_in_series INTEGER NOT NULL,
	red_slot INTEGER NOT NULL,
	blue_slot INTEGER NOT NULL,
	outcome TEXT NOT NULL CHECK (outcome IN ('red', 'blue', 'draw')),
	won_by TEXT,
	full_turns INTEGER NOT NULL,
	moves BLOB NOT NULL,
	played_at INTEGER NOT NULL DEFAULT (unixepoch()),
	FOREIGN KEY (run_id, red_slot) REFERENCES tournament_participants (run_id, slot),
	FOREIGN KEY (run_id, blue_slot) REFERENCES tournament_participants (run_id, slot)
);

CREATE TABLE IF NOT EXISTS tournament_standings (
	run_id INTEGER NOT NULL REFERENCES tournament_runs (id),
	slot INTEGER NOT NULL,
	rating INTEGER NOT NULL,
	wins INTEGER NOT NULL,
	losses INTEGER NOT NULL,
	draws INTEGER NOT NULL,
	series_won INTEGER NOT NULL,
	games_played INTEGER NOT NULL,
	PRIMARY KEY (run_id, slot)
);

CREATE INDEX IF NOT EXISTS idx_tournament_games_series ON tournament_games (series_id);
CREATE INDEX IF NOT EXISTS idx_tournament_games_run ON tournament_games (run_id);
CREATE INDEX IF NOT EXISTS idx_tournament_series_run ON tournament_series (run_id);
`

// migrations holds one SQL script per schema version: index i upgrades
// version i to version i+1. Its length must equal config.SQLiteSchemaVersion
// so the constants hub stays authoritative; migrate enforces that at
// startup. New versions only ever append, never edit a landed script.
var migrations = []string{schemaV1, schemaV2, schemaV3}
