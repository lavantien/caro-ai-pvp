package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// Failure paths of the store's completion unit and its statement helpers:
// every arm asserts the typed error and the all-or-nothing state left behind,
// not just that something failed.

// completionFootprint counts what a completion unit left on disk.
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

// TestApplyCompletionRejectsNullMovesAtomically feeds the unit a game whose
// moves blob is NULL (a shape this server never writes, the NOT NULL column
// refuses it): the insert fails, nothing of the unit lands.
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

// TestApplyCompletionFinishOfMissingSeriesRollsBackTheUnit pins the finish
// statement's slice of the atomicity law: a game that would persist fine is
// still rolled back when the series finish targets a series that does not
// exist.
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
			Outcome: OutcomeRed, Moves: encodeMoves(nil, movesOf(t, []string{"D4", "P16"})),
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

// dyingReadsRunner reads the first query off the live database and every later
// one off a pool that already closed, the exact mid-unit death a broken
// connection produces between two statements.
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

// TestApplyRatingPairReadFailureAbortsBeforeWrites covers both read legs of
// the pricing: a dead pool fails the red read, and a pool dying between the
// two reads fails the blue read; either way no rating event lands, so the
// zero-sum pair can never tear.
func TestApplyRatingPairReadFailureAbortsBeforeWrites(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")

	// Red read dead: the whole pool is gone up front.
	dead := mustOpen(t, dbPath(t))
	if err := dead.Close(); err != nil {
		t.Fatalf("close dead pool: %v", err)
	}
	err := applyRatingPair(context.Background(), dead.db, 1, alice.ID, bob.ID, OutcomeRed)
	if err == nil || !strings.Contains(err.Error(), "current rating") {
		t.Fatalf("pricing over a dead pool = %v, want the current-rating read failure", err)
	}

	// Blue read dead: the first read answers, the second dies mid-unit.
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

// brokenCountResult is a driver result that cannot report its row count.
type brokenCountResult struct{}

func (brokenCountResult) LastInsertId() (int64, error) { return 0, nil }
func (brokenCountResult) RowsAffected() (int64, error) {
	return 0, errors.New("driver lost the count")
}

// countlessRunner executes fine but reports a broken RowsAffected, the seam
// updateSeriesRow owns for driver-level accounting failures.
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

// TestRatingHistoryScanFailureOnPoisonedRow injects the one row shape this
// server never writes but a foreign writer could: text in an INTEGER column.
// The listing fails loudly instead of returning a mangled history.
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

// TestMatchHistoryScanFailureOnPoisonedRow does the same for the history
// tab's own scan: a game row with an unparsable timestamp fails the listing
// instead of surfacing garbage.
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
