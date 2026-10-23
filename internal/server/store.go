package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

var ErrNotFound = errors.New("server: record not found")

func (s *Store) WithinTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("server: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server: commit tx: %w", err)
	}
	return nil
}

type sqlRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db       *sql.DB
	casual   *casualLogs
	closeOne sync.Once
	closeErr error
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("server: create db dir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("server: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(runtime.NumCPU())
	db.SetMaxIdleConns(runtime.NumCPU())
	s := &Store{db: db, casual: newCasualLogs(config.CasualLogRoot, config.CasualSeriesLogFormat)}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.verifyBotSeats(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.verifyAdminSeed(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) verifyAdminSeed() error {
	var salt, hash []byte
	err := s.db.QueryRow(
		`SELECT salt, hash FROM users WHERE username = ?`, config.AdminName,
	).Scan(&salt, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("server: seeded admin account %q missing after migration", config.AdminName)
	}
	if err != nil {
		return fmt.Errorf("server: read seeded admin: %w", err)
	}
	if !bytes.Equal(salt, adminSeedSalt) || !bytes.Equal(hash, adminSeedHash) {
		return fmt.Errorf(
			"server: admin name %q is held by a row the seed did not create, the seeded credential will not log in",
			config.AdminName)
	}
	return nil
}

func (s *Store) verifyBotSeats() error {
	for i := range config.Tiers {
		if _, err := s.BotAccountID(config.Tiers[i]); err != nil {
			return fmt.Errorf("server: reserved bot seat %q: %w", config.BotAccountName(i), err)
		}
	}
	return nil
}

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

func dsn(path string) string {
	q := make(url.Values, 5)
	q.Set("_journal_mode", config.SQLiteJournalWAL)
	q.Set("_synchronous", config.SQLiteSyncNormal)
	q.Set("_busy_timeout", strconv.Itoa(config.SQLiteBusyTimeoutMs))
	q.Set("_foreign_keys", "on")
	q.Set("_txlock", config.SQLiteTxLockImmediate)
	return filepath.ToSlash(path) + "?" + q.Encode()
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

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

func (s *Store) UserByID(id int64) (User, error) {
	var u User
	err := notFound(s.db.QueryRow(
		`SELECT id, username, argon2_time, argon2_memory, argon2_parallelism, salt, hash, created_at
		FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Argon2Time, &u.Argon2MemoryKiB, &u.Argon2Parallelism, &u.Salt, &u.Hash, &u.CreatedAt))
	if err != nil {
		return User{}, fmt.Errorf("server: fetch user %d: %w", id, err)
	}
	return u, nil
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

func (s *Store) DeleteSession(token []byte) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("server: delete session: %w", err)
	}
	return nil
}

const botAccountMarker = "length(salt) = 0 AND length(hash) = 0"

func (s *Store) BotAccountID(tier config.Tier) (int64, error) {
	idx, ok := config.TierIndex(tier)
	if !ok {
		return 0, fmt.Errorf("server: bot account of tier %q: %w", tier.Name, ErrUnknownTier)
	}
	var id int64
	err := notFound(s.db.QueryRow(
		`SELECT id FROM users WHERE username = ? AND `+botAccountMarker,
		config.BotAccountName(idx),
	).Scan(&id))
	if err != nil {
		return 0, fmt.Errorf("server: bot account of tier %q: %w", tier.Name, err)
	}
	return id, nil
}

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

func (s *Store) CreateSeries(ctx context.Context, tcIdx, boLen int, redUser, blueUser int64) (SeriesRow, error) {
	var sr SeriesRow
	err := s.db.QueryRowContext(ctx,
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

func (s *Store) UpdateSeries(id int64, state string, winner, finishedAt *int64) error {
	return updateSeriesRow(context.Background(), s.db, id, state, winner, finishedAt)
}

func updateSeriesRow(ctx context.Context, run sqlRunner, id int64, state string, winner, finishedAt *int64) error {
	res, err := run.ExecContext(ctx,
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
	BotName     string
	StatLines   []GameStat
	PlayedAt    int64
}

type GameStat struct {
	MoveNo int
	Line   string
}

func (s *Store) AppendGame(g Game) (Game, error) {
	return insertGameRow(context.Background(), s.db, g)
}

func insertGameRow(ctx context.Context, run sqlRunner, g Game) (Game, error) {
	var botName any
	if g.BotName != "" {
		botName = g.BotName
	}
	err := run.QueryRowContext(ctx,
		`INSERT INTO games (series_id, idx_in_series, red_user, blue_user, outcome, moves, full_turns, won_by, bot_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, played_at`,
		g.SeriesID, g.IdxInSeries, g.RedUser, g.BlueUser, g.Outcome, g.Moves, g.FullTurns, sqlArg(g.WonBy), botName,
	).Scan(&g.ID, &g.PlayedAt)
	if err != nil {
		return Game{}, fmt.Errorf("server: append game: %w", err)
	}
	return g, nil
}

type RatingEvent struct {
	ID          int64
	GameID      int64
	UserID      int64
	Delta       int
	RatingAfter int
	CreatedAt   int64
}

func (s *Store) AppendRatingEvent(e RatingEvent) (RatingEvent, error) {
	return insertRatingEventRow(context.Background(), s.db, e)
}

func insertRatingEventRow(ctx context.Context, run sqlRunner, e RatingEvent) (RatingEvent, error) {
	err := run.QueryRowContext(ctx,
		`INSERT INTO rating_events (game_id, user_id, delta, rating_after)
		VALUES (?, ?, ?, ?) RETURNING id, created_at`,
		e.GameID, e.UserID, e.Delta, e.RatingAfter,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return RatingEvent{}, fmt.Errorf("server: append rating event: %w", err)
	}
	return e, nil
}

type Completion struct {
	Games  []Game
	Finish *SeriesFinish
}

type SeriesFinish struct {
	SeriesID   int64
	Winner     *int64
	FinishedAt int64
}

func (s *Store) ApplyCompletion(ctx context.Context, c Completion) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("server: begin completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, g := range c.Games {
		row, err := insertGameRow(ctx, tx, g)
		if err != nil {
			return err
		}
		if err := insertGameStatRows(ctx, tx, row.ID, g.StatLines); err != nil {
			return err
		}
		botSeat, err := anyBotSeat(ctx, tx, g.RedUser, g.BlueUser)
		if err != nil {
			return err
		}
		if !botSeat {
			if err := applyRatingPair(ctx, tx, row.ID, g.RedUser, g.BlueUser, g.Outcome); err != nil {
				return err
			}
		}
	}
	if c.Finish != nil {
		if err := updateSeriesRow(ctx, tx, c.Finish.SeriesID, SeriesStateFinished, c.Finish.Winner, &c.Finish.FinishedAt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server: commit completion: %w", err)
	}
	s.logCasualCompletion(ctx, c)
	return nil
}

func (s *Store) logCasualCompletion(ctx context.Context, c Completion) {
	if s.casual == nil {
		return
	}
	bg := context.Background()
	for _, g := range c.Games {
		if len(g.StatLines) == 0 {
			continue
		}
		h, err := s.casualHeaderFor(bg, g)
		if err != nil {
			log.Printf("server: casual log header for series %d game %d: %v", g.SeriesID, g.IdxInSeries+1, err)
			continue
		}
		if err := s.casual.WriteGame(h, g.StatLines); err != nil {
			log.Printf("server: casual log for series %d game %d: %v", g.SeriesID, h.GameNo, err)
		}
	}
}

func (s *Store) casualHeaderFor(ctx context.Context, g Game) (casualGameHeader, error) {
	sr, err := s.SeriesByID(g.SeriesID)
	if err != nil {
		return casualGameHeader{}, err
	}
	redName, redBot, err := s.casualSeatName(ctx, g.RedUser, g.BotName)
	if err != nil {
		return casualGameHeader{}, err
	}
	blueName, blueBot, err := s.casualSeatName(ctx, g.BlueUser, g.BotName)
	if err != nil {
		return casualGameHeader{}, err
	}
	seriesRed, _, err := s.casualSeatName(ctx, sr.RedUser, g.BotName)
	if err != nil {
		return casualGameHeader{}, err
	}
	seriesBlue, _, err := s.casualSeatName(ctx, sr.BlueUser, g.BotName)
	if err != nil {
		return casualGameHeader{}, err
	}
	mode := lobbyModeCasual
	if redBot && blueBot {
		mode = lobbyModeBotVsBot
	} else if redBot || blueBot {
		mode = lobbyModeVsBot
	}
	_, roomID, _ := strings.Cut(g.BotName, "-")
	h := casualGameHeader{
		GameNo: g.IdxInSeries + 1, RoomID: roomID, Mode: mode,
		TCIdx: sr.TCIdx, BOLen: sr.BOLen, Outcome: g.Outcome,
	}
	h.SeriesID, h.Red, h.Blue = g.SeriesID, seriesRed, seriesBlue
	h.PairingRed, h.PairingBlue = redName, blueName
	return h, nil
}

func (s *Store) casualSeatName(ctx context.Context, userID int64, botName string) (string, bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM users WHERE id = ? AND `+botAccountMarker, userID,
	).Scan(&one)
	if err == nil {
		if botName != "" {
			return botName, true, nil
		}
		u, err := s.UserByID(userID)
		if err != nil {
			return "", false, err
		}
		return u.Username, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("server: bot seat probe %d: %w", userID, err)
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return "", false, err
	}
	return u.Username, false, nil
}

func insertGameStatRows(ctx context.Context, run sqlRunner, gameID int64, stats []GameStat) error {
	for _, s := range stats {
		if _, err := run.ExecContext(ctx,
			`INSERT INTO game_stats (game_id, move_no, line) VALUES (?, ?, ?)`,
			gameID, s.MoveNo, s.Line,
		); err != nil {
			return fmt.Errorf("server: append game stat: %w", err)
		}
	}
	return nil
}

func anyBotSeat(ctx context.Context, run sqlRunner, redUser, blueUser int64) (bool, error) {
	for _, id := range [2]int64{redUser, blueUser} {
		var one int
		err := run.QueryRowContext(ctx,
			`SELECT 1 FROM users WHERE id = ? AND `+botAccountMarker, id,
		).Scan(&one)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("server: bot seat probe %d: %w", id, err)
		}
	}
	return false, nil
}

func applyRatingPair(ctx context.Context, run sqlRunner, gameID, redUser, blueUser int64, outcome string) error {
	var dRed, dBlue, afterRed, afterBlue int
	switch outcome {
	case OutcomeRed, OutcomeBlue:
		rRed, err := currentRatingOn(ctx, run, redUser)
		if err != nil {
			return err
		}
		rBlue, err := currentRatingOn(ctx, run, blueUser)
		if err != nil {
			return err
		}
		if outcome == OutcomeRed {
			dRed, dBlue, afterRed, afterBlue = RatingDeltas(rRed, rBlue, RedWins)
		} else {
			dRed, dBlue, afterRed, afterBlue = RatingDeltas(rRed, rBlue, BlueWins)
		}
	case OutcomeDraw:
		return nil
	default:
		return fmt.Errorf("server: rating pair for outcome %q: %w", outcome, ErrInvalidOutcome)
	}
	for _, e := range [2]RatingEvent{
		{GameID: gameID, UserID: redUser, Delta: dRed, RatingAfter: afterRed},
		{GameID: gameID, UserID: blueUser, Delta: dBlue, RatingAfter: afterBlue},
	} {
		if _, err := insertRatingEventRow(ctx, run, e); err != nil {
			return err
		}
	}
	return nil
}

func currentRating(st *Store, userID int64) (int, error) {
	return currentRatingOn(context.Background(), st.db, userID)
}

func currentRatingOn(ctx context.Context, run sqlRunner, userID int64) (int, error) {
	var r int
	err := run.QueryRowContext(ctx,
		`SELECT rating_after FROM rating_events WHERE user_id = ? ORDER BY id DESC LIMIT 1`, userID,
	).Scan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return config.RatingStart, nil
	}
	if err != nil {
		return 0, fmt.Errorf("server: current rating %d: %w", userID, err)
	}
	return r, nil
}

func (s *Store) RatingHistoryByUser(userID int64) ([]RatingEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, game_id, user_id, delta, rating_after, created_at
		FROM rating_events WHERE user_id = ? ORDER BY id`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("server: rating history %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
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

func (s *Store) MatchHistory(userID int64) ([]MatchHistoryRow, error) {
	rows, err := s.db.Query(`
SELECT g.played_at,
	CASE WHEN length(ru.salt) = 0 AND length(ru.hash) = 0
	     THEN COALESCE(g.bot_name, ru.username) ELSE ru.username END,
	CASE WHEN length(bu.salt) = 0 AND length(bu.hash) = 0
	     THEN COALESCE(g.bot_name, bu.username) ELSE bu.username END,
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.red_user) OR (w.outcome = ? AND w.blue_user = g.red_user))),
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.blue_user) OR (w.outcome = ? AND w.blue_user = g.blue_user))),
	g.full_turns, g.moves, g.won_by
FROM games g
JOIN users ru ON ru.id = g.red_user
JOIN users bu ON bu.id = g.blue_user
WHERE g.red_user = ? OR g.blue_user = ?
ORDER BY g.played_at DESC, g.id DESC`,
		OutcomeRed, OutcomeBlue, OutcomeRed, OutcomeBlue, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("server: match history %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
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
