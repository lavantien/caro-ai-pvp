package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func completionFootprint(t *testing.T, s *Store, seriesID int64) (games, ratings int) {
	t.Helper()
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM games WHERE series_id = ?`, seriesID).Scan(&games); err != nil {
		t.Fatalf("count games: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM rating_events`).Scan(&ratings); err != nil {
		t.Fatalf("count rating events: %v", err)
	}
	return games, ratings
}

func TestApplyCompletionRejectsNullMovesAtomically(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)

	err := s.ApplyCompletion(context.Background(), Completion{Games: []Game{{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: nil, FullTurns: 0,
	}}})
	if err == nil || !strings.Contains(err.Error(), "NOT NULL") {
		t.Fatalf("apply a NULL-moves game = %v, want the NOT NULL rejection", err)
	}
	if games, ratings := completionFootprint(t, s, sr.ID); games != 0 || ratings != 0 {
		t.Errorf("disk after the rejected unit = %d games %d ratings, want zero of both", games, ratings)
	}
}

func TestApplyCompletionFinishOfMissingSeriesRollsBackTheUnit(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)
	wonBy := WonByFour

	missing := sr.ID + 424242
	err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
			Outcome: OutcomeRed, Moves: EncodeMoves(nil, movesOf(t, []string{"D4", "P16"})),
			FullTurns: 1, WonBy: &wonBy,
		}},
		Finish: &SeriesFinish{SeriesID: missing, FinishedAt: 1_700_000_000},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("finish of a missing series = %v, want ErrNotFound", err)
	}
	if games, ratings := completionFootprint(t, s, sr.ID); games != 0 || ratings != 0 {
		t.Errorf("disk after the rolled-back finish = %d games %d ratings, want zero of both", games, ratings)
	}
	if again, err := s.SeriesByID(sr.ID); err != nil || again.State != SeriesStateOngoing {
		t.Errorf("real series row after the rolled-back finish = %+v err %v, want untouched ongoing", again, err)
	}
}

func TestApplyRatingPairRejectsUnknownOutcome(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")

	err := applyRatingPair(context.Background(), s.db, 1, alice.ID, bob.ID, "stalemate")
	if !errors.Is(err, ErrInvalidOutcome) {
		t.Fatalf("rating pair for an unknown outcome = %v, want ErrInvalidOutcome", err)
	}
	if hist, herr := s.RatingHistoryByUser(alice.ID); herr != nil || len(hist) != 0 {
		t.Errorf("events after the rejected outcome = %d err %v, want none", len(hist), herr)
	}
}

type dyingReadsRunner struct {
	live, dead *sql.DB
	reads      int
}

func (d *dyingReadsRunner) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return d.live.ExecContext(ctx, q, args...)
}

func (d *dyingReadsRunner) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	d.reads++
	if d.reads == 1 {
		return d.live.QueryRowContext(ctx, q, args...)
	}
	return d.dead.QueryRowContext(ctx, q, args...)
}

func TestApplyRatingPairReadFailureAbortsBeforeWrites(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")

	dead := mustOpen(t, dbPath(t))
	if err := dead.Close(); err != nil {
		t.Fatalf("close dead pool: %v", err)
	}
	err := applyRatingPair(context.Background(), dead.db, 1, alice.ID, bob.ID, OutcomeRed)
	if err == nil || !strings.Contains(err.Error(), "current rating") {
		t.Fatalf("pricing over a dead pool = %v, want the current-rating read failure", err)
	}

	dying := &dyingReadsRunner{live: s.db, dead: dead.db}
	err = applyRatingPair(context.Background(), dying, 2, alice.ID, bob.ID, OutcomeRed)
	if err == nil || !strings.Contains(err.Error(), "current rating") {
		t.Fatalf("pricing over a dying pool = %v, want the blue read failure", err)
	}
	if dying.reads != 2 {
		t.Errorf("reads attempted = %d, want exactly the two pricing reads", dying.reads)
	}
	for _, u := range []User{alice, bob} {
		if hist, herr := s.RatingHistoryByUser(u.ID); herr != nil || len(hist) != 0 {
			t.Errorf("%s events after aborted pricing = %d err %v, want none", u.Username, len(hist), herr)
		}
	}
}

type brokenCountResult struct{}

func (brokenCountResult) LastInsertId() (int64, error) { return 0, nil }
func (brokenCountResult) RowsAffected() (int64, error) {
	return 0, errors.New("driver lost the count")
}

type countlessRunner struct{}

func (countlessRunner) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return brokenCountResult{}, nil
}
func (countlessRunner) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

func TestUpdateSeriesRowSurfacesRowsAffectedFailure(t *testing.T) {
	err := updateSeriesRow(context.Background(), countlessRunner{}, 7, SeriesStateOngoing, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "update series 7") || !strings.Contains(err.Error(), "lost the count") {
		t.Fatalf("update over a broken result = %v, want the wrapped accounting failure", err)
	}
}

func TestRatingHistoryScanFailureOnPoisonedRow(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)
	g, err := s.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	if _, err := s.AppendRatingEvent(RatingEvent{GameID: g.ID, UserID: alice.ID, Delta: 10, RatingAfter: 10}); err != nil {
		t.Fatalf("seed event: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE rating_events SET delta = 'oops' WHERE id = ?`, g.ID); err != nil {
		t.Fatalf("poison delta: %v", err)
	}

	_, err = s.RatingHistoryByUser(alice.ID)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("rating history over a poisoned row = %v, want the scan failure", err)
	}
	if !strings.Contains(err.Error(), "rating history") {
		t.Errorf("error = %v, want the rating-history wrap", err)
	}
}

func TestMatchHistoryScanFailureOnPoisonedRow(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)
	g, err := s.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeDraw, Moves: []byte{1, 0}, FullTurns: 1,
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE games SET played_at = 'oops' WHERE id = ?`, g.ID); err != nil {
		t.Fatalf("poison played_at: %v", err)
	}

	_, err = s.MatchHistory(alice.ID)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("match history over a poisoned row = %v, want the scan failure", err)
	}
	if !strings.Contains(err.Error(), "match history") {
		t.Errorf("error = %v, want the match-history wrap", err)
	}
}

func TestCloseKeepsCheckpointErrorWhenPoolCloseSucceeds(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	seedUser(t, s, "alice")

	ctx := context.Background()
	held, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin writer conn: %v", err)
	}
	defer func() { _ = held.Close() }()
	tx, err := held.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin held write: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET username = username`); err != nil {
		t.Fatalf("hold the write lock: %v", err)
	}

	err = s.Close()
	if err != nil {
		t.Fatalf("close under a held write transaction = %v, want nil: modernc reports a busy checkpoint through the result row and never errors the exec", err)
	}
}

func TestCreateReleasesCoresWhenSeriesWriteFails(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	if _, err := s.store.db.Exec(`DROP TABLE series`); err != nil {
		t.Fatalf("drop series: %v", err)
	}

	vsBot := config.TierGrandmaster
	for range 3 {
		_, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &vsBot)
		if err == nil {
			t.Fatal("create over the broken series write succeeded, want the store failure")
		}
		if errors.Is(err, ErrMachineBusy) {
			t.Fatalf("create = %v, want the store failure again, not the wedged core budget", err)
		}
		if got := s.rm.LiveCoreBookings(); got != 0 {
			t.Fatalf("core bookings after the failed create = %d, want 0", got)
		}
	}
}
