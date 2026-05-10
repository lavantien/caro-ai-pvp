package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type GameRecord struct {
	ID              string
	GameMode        string
	TimeControl     string
	RedType         string
	BlueType        string
	RedDifficulty   *int
	BlueDifficulty  *int
	Winner          string
	MoveCount       int
	CreatedAt       time.Time
	CompletedAt     *time.Time
}

type MoveRecord struct {
	GameID          string
	MoveNumber      int
	Player          string
	PosX            int
	PosY            int
	IsBot           bool
	Difficulty      *int
	ThinkTimeMs     *int64
	RemainingTimeMs *int64
	SearchDepth     *int
	NodesSearched   *int64
	NPS             *float64
	TTHitRate       *float64
	SearchScore     *int
	ThreadsUsed     *int
	AllocatedTimeMs *int64
	MoveType        *string
	MasterPct       *float64
	SlaveDepth      *int
	SlaveNodes      *int64
	PonderDepth     *int
	PonderNodes     *int64
}

type MatchStore struct {
	db *sql.DB
}

const matchSchema = `
CREATE TABLE IF NOT EXISTS games (
    id TEXT PRIMARY KEY,
    game_mode TEXT NOT NULL,
    time_control TEXT NOT NULL,
    red_type TEXT NOT NULL,
    blue_type TEXT NOT NULL,
    red_difficulty INTEGER,
    blue_difficulty INTEGER,
    winner TEXT NOT NULL DEFAULT 'none',
    move_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS moves (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id TEXT NOT NULL REFERENCES games(id),
    move_number INTEGER NOT NULL,
    player TEXT NOT NULL,
    pos_x INTEGER NOT NULL,
    pos_y INTEGER NOT NULL,
    is_bot INTEGER NOT NULL DEFAULT 0,
    difficulty INTEGER,
    think_time_ms INTEGER,
    remaining_time_ms INTEGER,
    search_depth INTEGER,
    nodes_searched INTEGER,
    nps REAL,
    tt_hit_rate REAL,
    search_score INTEGER,
    threads_used INTEGER,
    allocated_time_ms INTEGER,
    move_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_moves_game_id ON moves(game_id);
CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at);
`

func NewMatchStore(dbPath string) (*MatchStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(matchSchema); err != nil {
		db.Close()
		return nil, err
	}

	s := &MatchStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *MatchStore) migrate() error {
	newCols := map[string]string{
		"master_pct":   "REAL",
		"slave_depth":  "INTEGER",
		"slave_nodes":  "INTEGER",
		"ponder_depth": "INTEGER",
		"ponder_nodes": "INTEGER",
	}

	rows, err := s.db.Query("PRAGMA table_info(moves)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for col, typ := range newCols {
		if !existing[col] {
			_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE moves ADD COLUMN %s %s", col, typ))
			if err != nil {
				return fmt.Errorf("migrate add column %s: %w", col, err)
			}
		}
	}
	return nil
}

func (s *MatchStore) CreateGame(g GameRecord) error {
	var redDiff, blueDiff any
	if g.RedDifficulty != nil {
		redDiff = *g.RedDifficulty
	}
	if g.BlueDifficulty != nil {
		blueDiff = *g.BlueDifficulty
	}
	_, err := s.db.Exec(
		`INSERT INTO games (id, game_mode, time_control, red_type, blue_type, red_difficulty, blue_difficulty)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.GameMode, g.TimeControl, g.RedType, g.BlueType, redDiff, blueDiff,
	)
	return err
}

func (s *MatchStore) RecordMove(m MoveRecord) error {
	var diff any
	if m.Difficulty != nil {
		diff = *m.Difficulty
	}
	_, err := s.db.Exec(
		`INSERT INTO moves (game_id, move_number, player, pos_x, pos_y, is_bot, difficulty,
		    think_time_ms, remaining_time_ms, search_depth, nodes_searched, nps, tt_hit_rate,
		    search_score, threads_used, allocated_time_ms, move_type,
		    master_pct, slave_depth, slave_nodes, ponder_depth, ponder_nodes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.GameID, m.MoveNumber, m.Player, m.PosX, m.PosY, m.IsBot, diff,
		m.ThinkTimeMs, m.RemainingTimeMs, m.SearchDepth, m.NodesSearched, m.NPS, m.TTHitRate,
		m.SearchScore, m.ThreadsUsed, m.AllocatedTimeMs, m.MoveType,
		m.MasterPct, m.SlaveDepth, m.SlaveNodes, m.PonderDepth, m.PonderNodes,
	)
	return err
}

func (s *MatchStore) CompleteGame(gameID, winner string, moveCount int) error {
	_, err := s.db.Exec(
		`UPDATE games SET winner = ?, move_count = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`,
		winner, moveCount, gameID,
	)
	return err
}

func (s *MatchStore) GetGame(gameID string) (*GameRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, game_mode, time_control, red_type, blue_type, red_difficulty, blue_difficulty,
		        winner, move_count, created_at, completed_at
		 FROM games WHERE id = ?`, gameID,
	)
	var g GameRecord
	var redDiff, blueDiff sql.NullInt64
	var completedAt sql.NullString
	var createdAt string
	err := row.Scan(&g.ID, &g.GameMode, &g.TimeControl, &g.RedType, &g.BlueType,
		&redDiff, &blueDiff, &g.Winner, &g.MoveCount, &createdAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if redDiff.Valid {
		v := int(redDiff.Int64)
		g.RedDifficulty = &v
	}
	if blueDiff.Valid {
		v := int(blueDiff.Int64)
		g.BlueDifficulty = &v
	}
	g.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	if completedAt.Valid {
		t, _ := time.Parse("2006-01-02T15:04:05Z", completedAt.String)
		g.CompletedAt = &t
	}
	return &g, nil
}

func (s *MatchStore) GetMoves(gameID string) ([]MoveRecord, error) {
	rows, err := s.db.Query(
		`SELECT move_number, player, pos_x, pos_y, is_bot, difficulty,
		        think_time_ms, remaining_time_ms, search_depth, nodes_searched,
		        nps, tt_hit_rate, search_score, threads_used, allocated_time_ms, move_type,
		        master_pct, slave_depth, slave_nodes, ponder_depth, ponder_nodes
		 FROM moves WHERE game_id = ? ORDER BY move_number`, gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []MoveRecord
	for rows.Next() {
		var m MoveRecord
		var isBot int
		var diff, searchDepth, threadsUsed sql.NullInt64
		var thinkTime, remainingTime, nodesSearched, allocTime sql.NullInt64
		var nps, ttHitRate sql.NullFloat64
		var searchScore sql.NullInt64
		var moveType sql.NullString
		var masterPct sql.NullFloat64
		var slaveDepth, ponderDepth sql.NullInt64
		var slaveNodes, ponderNodes sql.NullInt64

		err := rows.Scan(&m.MoveNumber, &m.Player, &m.PosX, &m.PosY, &isBot, &diff,
			&thinkTime, &remainingTime, &searchDepth, &nodesSearched,
			&nps, &ttHitRate, &searchScore, &threadsUsed, &allocTime, &moveType,
			&masterPct, &slaveDepth, &slaveNodes, &ponderDepth, &ponderNodes)
		if err != nil {
			return nil, err
		}

		m.GameID = gameID
		m.IsBot = isBot != 0
		if diff.Valid {
			v := int(diff.Int64)
			m.Difficulty = &v
		}
		if thinkTime.Valid {
			m.ThinkTimeMs = &thinkTime.Int64
		}
		if remainingTime.Valid {
			m.RemainingTimeMs = &remainingTime.Int64
		}
		if searchDepth.Valid {
			v := int(searchDepth.Int64)
			m.SearchDepth = &v
		}
		if nodesSearched.Valid {
			m.NodesSearched = &nodesSearched.Int64
		}
		if nps.Valid {
			m.NPS = &nps.Float64
		}
		if ttHitRate.Valid {
			m.TTHitRate = &ttHitRate.Float64
		}
		if searchScore.Valid {
			v := int(searchScore.Int64)
			m.SearchScore = &v
		}
		if threadsUsed.Valid {
			v := int(threadsUsed.Int64)
			m.ThreadsUsed = &v
		}
		if allocTime.Valid {
			m.AllocatedTimeMs = &allocTime.Int64
		}
		if moveType.Valid {
			m.MoveType = &moveType.String
		}
		if masterPct.Valid {
			m.MasterPct = &masterPct.Float64
		}
		if slaveDepth.Valid {
			v := int(slaveDepth.Int64)
			m.SlaveDepth = &v
		}
		if slaveNodes.Valid {
			m.SlaveNodes = &slaveNodes.Int64
		}
		if ponderDepth.Valid {
			v := int(ponderDepth.Int64)
			m.PonderDepth = &v
		}
		if ponderNodes.Valid {
			m.PonderNodes = &ponderNodes.Int64
		}

		moves = append(moves, m)
	}
	return moves, rows.Err()
}

func (s *MatchStore) Close() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}
