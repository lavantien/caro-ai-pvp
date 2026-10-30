package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func dbPath(t *testing.T) string {
	t.Helper()
	saved := config.CasualLogRoot
	config.CasualLogRoot = t.TempDir()
	t.Cleanup(func() { config.CasualLogRoot = saved })
	return filepath.Join(t.TempDir(), "caro.sqlite")
}

func TestOpenAppliesConfiguredPragmas(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != config.SQLiteJournalWAL {
		t.Errorf("journal_mode = %q, want %q", mode, config.SQLiteJournalWAL)
	}
	var sync int
	if err := s.db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("synchronous: %v", err)
	}
	if sync != 1 {
		t.Errorf("synchronous = %d, want 1 for %q", sync, config.SQLiteSyncNormal)
	}
	var busy int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busy != config.SQLiteBusyTimeoutMs {
		t.Errorf("busy_timeout = %d, want %d", busy, config.SQLiteBusyTimeoutMs)
	}
}

func TestOpenCreatesMissingParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "db", "caro.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open in a missing directory: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file after open: %v", err)
	}
}

func TestForeignKeysHoldOnEveryPooledConn(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	held, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin conn: %v", err)
	}
	defer func() { _ = held.Close() }()
	var heldFK int
	if err := held.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&heldFK); err != nil {
		t.Fatalf("pinned foreign_keys: %v", err)
	}
	var pooledFK int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&pooledFK); err != nil {
		t.Fatalf("second conn foreign_keys: %v", err)
	}
	if heldFK != 1 || pooledFK != 1 {
		t.Errorf("foreign_keys = pinned %d, second conn %d, want 1 on both", heldFK, pooledFK)
	}
}

func TestCloseIsIdempotentPerStore(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestOpenRejectsAnAdminRowTheSeedDidNotCreate(t *testing.T) {
	for _, column := range []string{"salt", "hash"} {
		path := dbPath(t)
		s := mustOpen(t, path)
		if _, err := s.db.Exec(
			fmt.Sprintf("UPDATE users SET %s = X'00' WHERE username = ?", column), config.AdminName,
		); err != nil {
			t.Fatalf("corrupt the admin %s: %v", column, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("close after corrupting the admin %s: %v", column, err)
		}
		if _, err := Open(path); err == nil || !strings.Contains(err.Error(), config.AdminName) {
			t.Errorf("open over a corrupted admin %s = %v, want the boot failure naming %q", column, err, config.AdminName)
		}
	}
}

func TestCloseSurfacesTheCheckpointFailure(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if err := s.Close(); err == nil {
		t.Error("close over a dead pool = nil, want the checkpoint failure surfaced")
	}
}

func TestCreateUserIfAbsentReportsNotCreatedWhenTheWriteFails(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	_, created, err := s.CreateUserIfAbsent("alice", []byte("s"), []byte("h"))
	if err == nil {
		t.Fatal("create over a dead pool = nil, want the pool failure")
	}
	if created {
		t.Error("created over a dead pool = true, want false")
	}
}

func TestBotAccountIDFailuresReturnAZeroID(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	id, err := s.BotAccountID(config.Tier{Name: "bogus"})
	if !errors.Is(err, ErrUnknownTier) || id != 0 {
		t.Errorf("unknown tier = id %d err %v, want 0 and ErrUnknownTier", id, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if id, err = s.BotAccountID(config.TierEasy); err == nil || id != 0 {
		t.Errorf("known tier over a dead pool = id %d err %v, want 0 and the pool failure", id, err)
	}
}

func TestCasualSeatNameClassifiesBotAndHumanSeats(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	ctx := context.Background()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	botIdx, ok := config.TierIndex(config.TierEasy)
	if !ok {
		t.Fatalf("resolve the tier index of %q", config.TierEasy.Name)
	}
	for _, row := range []struct {
		name    string
		userID  int64
		botName string
		want    string
		wantBot bool
	}{
		{"bot seat by account", botID, "", config.BotAccountName(botIdx), true},
		{"bot seat by display name", botID, "easy-x", "easy-x", true},
		{"human seat", alice.ID, "", "alice", false},
	} {
		got, isBot, err := s.casualSeatName(ctx, row.userID, row.botName)
		if err != nil || got != row.want || isBot != row.wantBot {
			t.Errorf("%s = (%q, %t, %v), want (%q, %t, nil)", row.name, got, isBot, err, row.want, row.wantBot)
		}
	}
	if got, isBot, err := s.casualSeatName(ctx, 424242, ""); err == nil || got != "" || isBot {
		t.Errorf("missing user = (%q, %t, %v), want an error over an empty human seat", got, isBot, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if got, isBot, err := s.casualSeatName(ctx, alice.ID, ""); err == nil || got != "" || isBot {
		t.Errorf("dead pool = (%q, %t, %v), want an error over an empty human seat", got, isBot, err)
	}
}

func TestAnyBotSeatProbesExactlyTheTwoSeriesSeats(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	if _, err := s.db.Exec(
		`INSERT INTO users (id, username, argon2_time, argon2_memory, argon2_parallelism, salt, hash)
		VALUES (0, 'zero-seat', ?, ?, ?, X'', X'')`,
		config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism,
	); err != nil {
		t.Fatalf("seed the id-zero seat: %v", err)
	}
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	ctx := context.Background()
	if bot, err := anyBotSeat(ctx, s.db, alice.ID, bob.ID); err != nil || bot {
		t.Errorf("human pairing over the id-zero seat = (%t, %v), want false and nil", bot, err)
	}
	if bot, err := anyBotSeat(ctx, s.db, alice.ID, botID); err != nil || !bot {
		t.Errorf("mixed pairing = (%t, %v), want true and nil", bot, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if bot, err := anyBotSeat(ctx, s.db, alice.ID, bob.ID); err == nil || bot {
		t.Errorf("dead pool = (%t, %v), want the pool failure over false", bot, err)
	}
}

func TestCurrentRatingFailureReturnsZero(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	alice := seedUser(t, s, "alice")
	if r, err := currentRating(s, alice.ID); err != nil || r != config.RatingStart {
		t.Errorf("fresh user rating = (%d, %v), want (%d, nil)", r, err, config.RatingStart)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if r, err := currentRating(s, alice.ID); err == nil || r != 0 {
		t.Errorf("rating over a dead pool = (%d, %v), want 0 and the pool failure", r, err)
	}
}

func TestGameRatingDeltaFailuresReturnZero(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	if d, err := s.gameRatingDelta(424242, 424242); err != nil || d != 0 {
		t.Errorf("missing event delta = (%d, %v), want (0, nil)", d, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if d, err := s.gameRatingDelta(1, 1); err == nil || d != 0 {
		t.Errorf("delta over a dead pool = (%d, %v), want 0 and the pool failure", d, err)
	}
}

func TestSeriesGameCountFailureReturnsZero(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	if n, err := s.seriesGameCount(424242); err != nil || n != 0 {
		t.Errorf("missing series count = (%d, %v), want (0, nil)", n, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if n, err := s.seriesGameCount(1); err == nil || n != 0 {
		t.Errorf("count over a dead pool = (%d, %v), want 0 and the pool failure", n, err)
	}
}

func TestGameStatTailWindowsAndFailure(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	g, err := insertGameRow(context.Background(), s.db, Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	if err := insertGameStatRows(context.Background(), s.db, g.ID, []GameStat{
		{MoveNo: 1, Line: "one"}, {MoveNo: 2, Line: "two"}, {MoveNo: 3, Line: "three"},
	}); err != nil {
		t.Fatalf("seed stats: %v", err)
	}
	if lines, more, err := s.gameStatTail(g.ID, 10); err != nil || more || len(lines) != 3 || lines[0] != "three" {
		t.Errorf("tail under the limit = (%q, %t, %v), want 3 newest-first lines and no more flag", lines, more, err)
	}
	if lines, more, err := s.gameStatTail(g.ID, 2); err != nil || !more || len(lines) != 2 || lines[0] != "three" {
		t.Errorf("tail over the limit = (%q, %t, %v), want 2 lines and the more flag", lines, more, err)
	}
	if lines, more, err := s.gameStatTail(g.ID+424242, 5); err != nil || more || lines != nil {
		t.Errorf("tail of a game without stats = (%q, %t, %v), want (nil, false, nil)", lines, more, err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatalf("close the pool: %v", err)
	}
	if lines, more, err := s.gameStatTail(g.ID, 5); err == nil || more || lines != nil {
		t.Errorf("tail over a dead pool = (%q, %t, %v), want the pool failure over (nil, false)", lines, more, err)
	}
}

func TestUserWLDsCountsBothSeatOrdersAndDecisiveOutcomes(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	carol := seedUser(t, s, "carol")
	ctx := context.Background()
	lossSeries, err := s.CreateSeries(ctx, 0, config.SeriesBO3, carol.ID, alice.ID)
	if err != nil {
		t.Fatalf("seed the loss series: %v", err)
	}
	winSeries, err := s.CreateSeries(ctx, 0, config.SeriesBO3, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("seed the win series: %v", err)
	}
	blueWinSeries, err := s.CreateSeries(ctx, 0, config.SeriesBO3, bob.ID, alice.ID)
	if err != nil {
		t.Fatalf("seed the blue win series: %v", err)
	}
	unit := Game{Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1}
	unit.SeriesID, unit.IdxInSeries, unit.RedUser, unit.BlueUser = lossSeries.ID, 0, carol.ID, alice.ID
	if _, err := insertGameRow(ctx, s.db, unit); err != nil {
		t.Fatalf("seed the loss game: %v", err)
	}
	unit.SeriesID, unit.RedUser, unit.BlueUser = winSeries.ID, alice.ID, bob.ID
	if _, err := insertGameRow(ctx, s.db, unit); err != nil {
		t.Fatalf("seed the win game: %v", err)
	}
	unit.Outcome = OutcomeBlue
	unit.SeriesID, unit.RedUser, unit.BlueUser = blueWinSeries.ID, bob.ID, alice.ID
	if _, err := insertGameRow(ctx, s.db, unit); err != nil {
		t.Fatalf("seed the blue win game: %v", err)
	}
	recs, err := s.userWLDs([]int64{alice.ID, bob.ID})
	if err != nil {
		t.Fatalf("wlds: %v", err)
	}
	rec, ok := recs[alice.ID]
	if !ok || rec.Wins != 2 || rec.Losses != 1 || rec.Draws != 0 {
		t.Errorf("alice record = (%d, %d, %d) ok %t, want (2, 1, 0)", rec.Wins, rec.Losses, rec.Draws, ok)
	}
	bobRec, ok := recs[bob.ID]
	if !ok || bobRec.Wins != 0 || bobRec.Losses != 2 || bobRec.Draws != 0 {
		t.Errorf("bob record = (%d, %d, %d) ok %t, want (0, 2, 0)", bobRec.Wins, bobRec.Losses, bobRec.Draws, ok)
	}
	if empty, err := s.userWLDs(nil); err != nil || len(empty) != 0 {
		t.Errorf("wlds of no ids = (%d entries, %v), want an empty map and nil", len(empty), err)
	}
}

func TestWithinTxCommitsAndRollsBack(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	if err := s.WithinTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE withintx_probe (x INTEGER)`)
		return err
	}); err != nil {
		t.Fatalf("commit unit: %v", err)
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM withintx_probe").Scan(&n); err != nil {
		t.Fatalf("read committed unit: %v", err)
	}
	if n != 0 {
		t.Errorf("probe rows = %d, want 0", n)
	}

	sentinel := errors.New("boom")
	if err := s.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO withintx_probe VALUES (1)"); err != nil {
			return err
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("failing unit = %v, want the sentinel", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM withintx_probe").Scan(&n); err != nil {
		t.Fatalf("reread after rollback: %v", err)
	}
	if n != 0 {
		t.Errorf("probe rows after rollback = %d, want 0", n)
	}
}

func TestAccessorsFailCleanlyAfterClose(t *testing.T) {
	s, err := Open(dbPath(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	calls := map[string]func() error{
		"create user":    func() error { _, _, err := s.CreateUserIfAbsent("x", []byte("s"), []byte("h")); return err },
		"fetch user":     func() error { _, err := s.UserByUsername("x"); return err },
		"insert session": func() error { return s.InsertSession(Session{Token: []byte("t")}) },
		"fetch session":  func() error { _, err := s.SessionByToken([]byte("t"), 0); return err },
		"delete session": func() error { return s.DeleteSession([]byte("t")) },
		"create series":  func() error { _, err := s.CreateSeries(context.Background(), 0, 3, 1, 2); return err },
		"fetch series":   func() error { _, err := s.SeriesByID(1); return err },
		"update series":  func() error { return s.UpdateSeries(1, SeriesStateFinished, nil, nil) },
		"append game":    func() error { _, err := s.AppendGame(Game{Moves: []byte{1}}); return err },
		"append rating":  func() error { _, err := s.AppendRatingEvent(RatingEvent{}); return err },
		"rating history": func() error { _, err := s.RatingHistoryByUser(1); return err },
		"user stats":     func() error { _, err := s.UserStats(1); return err },
		"match history":  func() error { _, err := s.MatchHistory(1); return err },
		"migrate":        s.migrate,
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s after close: want error, got nil", name)
		} else if errors.Is(err, ErrNotFound) {
			t.Errorf("%s after close = ErrNotFound, want the closed-pool error", name)
		}
	}
}
