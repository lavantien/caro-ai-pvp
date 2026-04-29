package persistence

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type GameLogEvent struct {
	ID        int64
	GameID    string
	EventType string
	Data      string
	Timestamp string
}

type GameLogService struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS game_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    data TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE VIRTUAL TABLE IF NOT EXISTS game_logs_fts USING fts5(
    game_id, event_type, data
);
CREATE INDEX IF NOT EXISTS idx_game_logs_game_id ON game_logs(game_id);
`

func NewGameLogService(dbPath string) (*GameLogService, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &GameLogService{db: db}, nil
}

func (s *GameLogService) LogEvent(gameID, eventType, data string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	result, err := tx.Exec(
		"INSERT INTO game_logs (game_id, event_type, data) VALUES (?, ?, ?)",
		gameID, eventType, data,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	id, _ := result.LastInsertId()
	_, err = tx.Exec(
		"INSERT INTO game_logs_fts (rowid, game_id, event_type, data) VALUES (?, ?, ?, ?)",
		id, gameID, eventType, data,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *GameLogService) QueryByGameID(gameID string) ([]GameLogEvent, error) {
	rows, err := s.db.Query(
		"SELECT id, game_id, event_type, data, timestamp FROM game_logs WHERE game_id = ? ORDER BY id",
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []GameLogEvent
	for rows.Next() {
		var e GameLogEvent
		if err := rows.Scan(&e.ID, &e.GameID, &e.EventType, &e.Data, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *GameLogService) Search(query string) ([]GameLogEvent, error) {
	rows, err := s.db.Query(
		`SELECT g.id, g.game_id, g.event_type, g.data, g.timestamp
		 FROM game_logs g
		 WHERE g.id IN (SELECT rowid FROM game_logs_fts WHERE game_logs_fts MATCH ?)
		 ORDER BY g.id`,
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []GameLogEvent
	for rows.Next() {
		var e GameLogEvent
		if err := rows.Scan(&e.ID, &e.GameID, &e.EventType, &e.Data, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *GameLogService) Close() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}
