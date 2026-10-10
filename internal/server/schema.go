package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	SeriesStateOngoing  = "ongoing"
	SeriesStateFinished = "finished"
)

const (
	OutcomeRed  = "red"
	OutcomeBlue = "blue"
	OutcomeDraw = "draw"
)

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

const schemaV2 = `
CREATE INDEX IF NOT EXISTS idx_games_red_user ON games (red_user);
CREATE INDEX IF NOT EXISTS idx_games_blue_user ON games (blue_user);
`

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

const schemaV4 = `
CREATE TABLE IF NOT EXISTS game_stats (
	game_id INTEGER NOT NULL REFERENCES games (id),
	move_no INTEGER NOT NULL,
	line TEXT NOT NULL,
	PRIMARY KEY (game_id, move_no)
);
`

func botSeatSeedSQL() string {
	values := make([]string, 0, len(config.Tiers))
	for i := range config.Tiers {
		values = append(values, fmt.Sprintf("('%s', %d, %d, %d, X'', X'')",
			config.BotAccountName(i),
			config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism))
	}
	return "INSERT OR IGNORE INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES " +
		strings.Join(values, ", ")
}

const schemaV5 = `
ALTER TABLE games ADD COLUMN bot_name TEXT;
`

const schemaV7 = `
ALTER TABLE tournament_runs ADD COLUMN label TEXT NOT NULL DEFAULT 'ui';
`

const schemaV8 = `
ALTER TABLE tournament_runs ADD COLUMN drive_pid INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tournament_runs ADD COLUMN drive_heartbeat INTEGER NOT NULL DEFAULT 0;
UPDATE tournament_runs SET label = '` + config.TournamentLegacyLabel + `' WHERE label = 'ui' AND status = 'ongoing';
`

var (
	adminSeedSalt = adminSeedSaltOf()
	adminSeedHash = HashPassword(config.AdminPassword, adminSeedSalt)
)

func adminSeedSaltOf() []byte {
	sum := sha256.Sum256([]byte("caro admin seed " + config.AdminName + " " + config.AdminPassword))
	return sum[:config.Argon2SaltBytes]
}

func adminSeedSQL() string {
	return fmt.Sprintf(
		"INSERT OR IGNORE INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES ('%s', %d, %d, %d, X'%s', X'%s')",
		config.AdminName, config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism,
		hex.EncodeToString(adminSeedSalt), hex.EncodeToString(adminSeedHash),
	)
}

var migrations = []string{schemaV1, schemaV2, schemaV3, schemaV4 + botSeatSeedSQL(), schemaV5, adminSeedSQL(), schemaV7, schemaV8, botSeatSeedSQL(), botSeatSeedSQL()}
