// Package server hosts the caro PvP server: embedded SQLite storage in WAL
// mode behind a single-writer mutation queue, argon2id auth, in-memory
// rooms, and the stats hub. The storage foundation lands first.
package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// ErrNotFound is the typed sentinel for every single-row accessor whose
// query matched nothing, including sessions filtered out by expiry.
var ErrNotFound = errors.New("server: record not found")

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

// notFound maps the driver's empty-result error onto the package sentinel.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// sqlArg binds an optional column: nil pointer to SQL NULL, value otherwise.
func sqlArg[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func nullString(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

// User is one account row. The argon2 columns snapshot the parameters the
// password was hashed with, so future parameter bumps can rehash on login.
type User struct {
	ID                int64
	Username          string
	Argon2Time        int
	Argon2MemoryKiB   int
	Argon2Parallelism int
	Salt              []byte
	Hash              []byte
	CreatedAt         int64
}

const insertUserSQL = `
INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (username) DO NOTHING`

// CreateUserIfAbsent inserts the account stamped with the config argon2
// parameters, or leaves the existing row untouched. The returned flag says
// which happened; on collision the caller verifies against the stored hash.
func (s *Store) CreateUserIfAbsent(username string, salt, hash []byte) (User, bool, error) {
	res, err := s.db.Exec(insertUserSQL, username, config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, salt, hash)
	if err != nil {
		return User{}, false, fmt.Errorf("server: create user %q: %w", username, err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return User{}, false, fmt.Errorf("server: create user %q: %w", username, err)
	}
	u, err := s.UserByUsername(username)
	if err != nil {
		return User{}, false, err
	}
	return u, inserted == 1, nil
}

func (s *Store) UserByUsername(username string) (User, error) {
	var u User
	err := notFound(s.db.QueryRow(
		`SELECT id, username, argon2_time, argon2_memory, argon2_parallelism, salt, hash, created_at
		FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Argon2Time, &u.Argon2MemoryKiB, &u.Argon2Parallelism, &u.Salt, &u.Hash, &u.CreatedAt))
	if err != nil {
		return User{}, fmt.Errorf("server: fetch user %q: %w", username, err)
	}
	return u, nil
}

// Session is one opaque-token login row. ExpiresAt is exclusive: a fetch at
// that instant is already expired.
type Session struct {
	Token     []byte
	UserID    int64
	ExpiresAt int64
}

func (s *Store) InsertSession(sess Session) error {
	if _, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		sess.Token, sess.UserID, sess.ExpiresAt,
	); err != nil {
		return fmt.Errorf("server: insert session: %w", err)
	}
	return nil
}

// SessionByToken returns the session only while unexpired at now, mapping
// both missing and expired tokens onto ErrNotFound.
func (s *Store) SessionByToken(token []byte, now int64) (Session, error) {
	var sess Session
	err := notFound(s.db.QueryRow(
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > ?`,
		token, now,
	).Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt))
	if err != nil {
		return Session{}, fmt.Errorf("server: fetch session: %w", err)
	}
	return sess, nil
}

// DeleteSession drops one token. Deleting a missing token is not an error:
// logout races a natural expiry.
func (s *Store) DeleteSession(token []byte) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("server: delete session: %w", err)
	}
	return nil
}

// SeriesRow is one best-of pairing's persisted record. Ongoing series carry
// NULL winner and finished_at; a draw finish keeps the winner NULL. The
// in-memory lifecycle lives in the Series state machine type.
type SeriesRow struct {
	ID         int64
	TCIdx      int
	BOLen      int
	RedUser    int64
	BlueUser   int64
	State      string
	Winner     *int64
	CreatedAt  int64
	FinishedAt *int64
}

func (s *Store) CreateSeries(tcIdx, boLen int, redUser, blueUser int64) (SeriesRow, error) {
	var sr SeriesRow
	err := s.db.QueryRow(
		`INSERT INTO series (tc_idx, bo_len, red_user, blue_user, state)
		VALUES (?, ?, ?, ?, ?) RETURNING id, created_at`,
		tcIdx, boLen, redUser, blueUser, SeriesStateOngoing,
	).Scan(&sr.ID, &sr.CreatedAt)
	if err != nil {
		return SeriesRow{}, fmt.Errorf("server: create series: %w", err)
	}
	sr.TCIdx, sr.BOLen, sr.RedUser, sr.BlueUser, sr.State = tcIdx, boLen, redUser, blueUser, SeriesStateOngoing
	return sr, nil
}

func (s *Store) SeriesByID(id int64) (SeriesRow, error) {
	var sr SeriesRow
	var winner, finished sql.NullInt64
	err := notFound(s.db.QueryRow(
		`SELECT id, tc_idx, bo_len, red_user, blue_user, state, winner, created_at, finished_at
		FROM series WHERE id = ?`, id,
	).Scan(&sr.ID, &sr.TCIdx, &sr.BOLen, &sr.RedUser, &sr.BlueUser, &sr.State, &winner, &sr.CreatedAt, &finished))
	if err != nil {
		return SeriesRow{}, fmt.Errorf("server: fetch series %d: %w", id, err)
	}
	sr.Winner, sr.FinishedAt = nullInt64(winner), nullInt64(finished)
	return sr, nil
}

// UpdateSeries rewrites the mutable columns: state plus the optional winner
// and finish time, NULL while undecided. A missing id maps to ErrNotFound.
func (s *Store) UpdateSeries(id int64, state string, winner, finishedAt *int64) error {
	res, err := s.db.Exec(
		`UPDATE series SET state = ?, winner = ?, finished_at = ? WHERE id = ?`,
		state, sqlArg(winner), sqlArg(finishedAt), id,
	)
	if err != nil {
		return fmt.Errorf("server: update series %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("server: update series %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("server: update series %d: %w", id, ErrNotFound)
	}
	return nil
}

// Game is one finished game inside a series. Moves is the caller-encoded
// move blob, WonBy the analytics tag for how the win happened (NULL on
// draws), FullTurns the count of full turns played.
type Game struct {
	ID          int64
	SeriesID    int64
	IdxInSeries int
	RedUser     int64
	BlueUser    int64
	Outcome     string
	Moves       []byte
	FullTurns   int
	WonBy       *string
	PlayedAt    int64
}

// AppendGame persists one game and returns it with the assigned id and
// played_at stamped by the database.
func (s *Store) AppendGame(g Game) (Game, error) {
	err := s.db.QueryRow(
		`INSERT INTO games (series_id, idx_in_series, red_user, blue_user, outcome, moves, full_turns, won_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, played_at`,
		g.SeriesID, g.IdxInSeries, g.RedUser, g.BlueUser, g.Outcome, g.Moves, g.FullTurns, sqlArg(g.WonBy),
	).Scan(&g.ID, &g.PlayedAt)
	if err != nil {
		return Game{}, fmt.Errorf("server: append game: %w", err)
	}
	return g, nil
}

// RatingEvent records one rating step. Ratings start at 0 and may go
// negative, so RatingAfter is a plain signed integer.
type RatingEvent struct {
	ID          int64
	GameID      int64
	UserID      int64
	Delta       int
	RatingAfter int
	CreatedAt   int64
}

func (s *Store) AppendRatingEvent(e RatingEvent) (RatingEvent, error) {
	err := s.db.QueryRow(
		`INSERT INTO rating_events (game_id, user_id, delta, rating_after)
		VALUES (?, ?, ?, ?) RETURNING id, created_at`,
		e.GameID, e.UserID, e.Delta, e.RatingAfter,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return RatingEvent{}, fmt.Errorf("server: append rating event: %w", err)
	}
	return e, nil
}

// RatingHistoryByUser returns one user's events in append order.
func (s *Store) RatingHistoryByUser(userID int64) ([]RatingEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, game_id, user_id, delta, rating_after, created_at
		FROM rating_events WHERE user_id = ? ORDER BY id`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("server: rating history %d: %w", userID, err)
	}
	defer rows.Close()
	var out []RatingEvent
	for rows.Next() {
		var e RatingEvent
		if err := rows.Scan(&e.ID, &e.GameID, &e.UserID, &e.Delta, &e.RatingAfter, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("server: rating history %d: %w", userID, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UserStats is the profile field: per-game W-L-D totals plus the level,
// the count of best-of series won.
type UserStats struct {
	Wins      int
	Losses    int
	Draws     int
	SeriesWon int
}

func (s *Store) UserStats(userID int64) (UserStats, error) {
	var st UserStats
	err := s.db.QueryRow(`
SELECT
	COALESCE(SUM(CASE WHEN (g.red_user = ? AND g.outcome = ?) OR (g.blue_user = ? AND g.outcome = ?) THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN (g.red_user = ? AND g.outcome = ?) OR (g.blue_user = ? AND g.outcome = ?) THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN g.outcome = ? AND ? IN (g.red_user, g.blue_user) THEN 1 ELSE 0 END), 0),
	(SELECT COUNT(*) FROM series WHERE state = ? AND winner = ?)
FROM games g`,
		userID, OutcomeRed, userID, OutcomeBlue,
		userID, OutcomeBlue, userID, OutcomeRed,
		OutcomeDraw, userID,
		SeriesStateFinished, userID,
	).Scan(&st.Wins, &st.Losses, &st.Draws, &st.SeriesWon)
	if err != nil {
		return UserStats{}, fmt.Errorf("server: user stats %d: %w", userID, err)
	}
	return st, nil
}

// MatchHistoryRow is one line of the match history tab. Moves carries the
// full move blob; trimming to the config.HistoryPreviewTurns preview is the
// caller's rendering concern.
type MatchHistoryRow struct {
	PlayedAt  int64
	Red       string
	Blue      string
	RedWins   int
	BlueWins  int
	FullTurns int
	Moves     []byte
	WonBy     *string
}

// MatchHistory lists a user's games newest first, each with the series
// score line as of that game.
func (s *Store) MatchHistory(userID int64) ([]MatchHistoryRow, error) {
	rows, err := s.db.Query(`
SELECT g.played_at, ru.username, bu.username,
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series AND w.outcome = ?),
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series AND w.outcome = ?),
	g.full_turns, g.moves, g.won_by
FROM games g
JOIN users ru ON ru.id = g.red_user
JOIN users bu ON bu.id = g.blue_user
WHERE g.red_user = ? OR g.blue_user = ?
ORDER BY g.played_at DESC, g.id DESC`,
		OutcomeRed, OutcomeBlue, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("server: match history %d: %w", userID, err)
	}
	defer rows.Close()
	var out []MatchHistoryRow
	for rows.Next() {
		var r MatchHistoryRow
		var wonBy sql.NullString
		if err := rows.Scan(&r.PlayedAt, &r.Red, &r.Blue, &r.RedWins, &r.BlueWins, &r.FullTurns, &r.Moves, &wonBy); err != nil {
			return nil, fmt.Errorf("server: match history %d: %w", userID, err)
		}
		r.WonBy = nullString(wonBy)
		out = append(out, r)
	}
	return out, rows.Err()
}
