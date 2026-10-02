package server

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func seedUser(t *testing.T, s *Store, name string) User {
	t.Helper()
	u, created, err := s.CreateUserIfAbsent(name, []byte("salt-"+name), []byte("hash-"+name))
	if err != nil {
		t.Fatalf("seed user %s: %v", name, err)
	}
	if !created {
		t.Fatalf("seed user %s: want created", name)
	}
	return u
}

func seedSeries(t *testing.T, s *Store, red, blue User) SeriesRow {
	t.Helper()
	sr, err := s.CreateSeries(context.Background(), 1, config.SeriesBO3, red.ID, blue.ID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	return sr
}

func TestUserCreateIfAbsentAndFetch(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	u, created, err := s.CreateUserIfAbsent("alice", []byte("salt-a"), []byte("hash-a"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Fatal("first create: want created true")
	}
	if u.ID == 0 || u.CreatedAt == 0 {
		t.Fatalf("create = id %d, created_at %d, want both nonzero", u.ID, u.CreatedAt)
	}
	if u.Username != "alice" || u.Argon2Time != config.Argon2Time || u.Argon2MemoryKiB != config.Argon2MemoryKiB || u.Argon2Parallelism != config.Argon2Parallelism {
		t.Errorf("create = %+v, want config argon2 params", u)
	}
	if string(u.Salt) != "salt-a" || string(u.Hash) != "hash-a" {
		t.Errorf("create roundtrip = salt %q hash %q", u.Salt, u.Hash)
	}

	got, err := s.UserByUsername("alice")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("fetch id = %d, want %d", got.ID, u.ID)
	}

	if _, err := s.UserByUsername("nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("fetch missing = %v, want ErrNotFound", err)
	}
}

func TestUserCreateIfAbsentCollisionKeepsFirstRow(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	first, created, err := s.CreateUserIfAbsent("alice", []byte("salt-a"), []byte("hash-a"))
	if err != nil || !created {
		t.Fatalf("first create = %v, created %t", err, created)
	}
	second, created, err := s.CreateUserIfAbsent("alice", []byte("salt-b"), []byte("hash-b"))
	if err != nil {
		t.Fatalf("colliding create: %v", err)
	}
	if created {
		t.Error("colliding create: want created false")
	}
	if second.ID != first.ID || string(second.Salt) != "salt-a" || string(second.Hash) != "hash-a" {
		t.Errorf("colliding create = id %d salt %q hash %q, want the first row kept", second.ID, second.Salt, second.Hash)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	u := seedUser(t, s, "alice")
	now := time.Now().Unix()
	const ttl = 3600
	sess := Session{Token: []byte("token-1"), UserID: u.ID, ExpiresAt: now + ttl}
	if err := s.InsertSession(sess); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.SessionByToken(sess.Token, now)
	if err != nil {
		t.Fatalf("fetch valid: %v", err)
	}
	if got.UserID != u.ID || got.ExpiresAt != sess.ExpiresAt {
		t.Errorf("fetch = %+v, want %+v", got, sess)
	}
	// Expiry is exclusive: at the deadline the session is already gone.
	if _, err := s.SessionByToken(sess.Token, sess.ExpiresAt); !errors.Is(err, ErrNotFound) {
		t.Errorf("fetch at expiry = %v, want ErrNotFound", err)
	}
	if _, err := s.SessionByToken([]byte("bogus"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("fetch bogus = %v, want ErrNotFound", err)
	}

	if err := s.DeleteSession(sess.Token); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.SessionByToken(sess.Token, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("fetch deleted = %v, want ErrNotFound", err)
	}
	if err := s.DeleteSession([]byte("bogus")); err != nil {
		t.Errorf("delete missing = %v, want nil", err)
	}
}

func TestSeriesLifecycle(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	red, blue := seedUser(t, s, "alice"), seedUser(t, s, "bob")
	sr, err := s.CreateSeries(context.Background(), 2, config.SeriesBO5, red.ID, blue.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sr.ID == 0 || sr.CreatedAt == 0 || sr.State != SeriesStateOngoing || sr.Winner != nil || sr.FinishedAt != nil {
		t.Fatalf("create = %+v, want fresh ongoing series", sr)
	}
	if sr.TCIdx != 2 || sr.BOLen != config.SeriesBO5 || sr.RedUser != red.ID || sr.BlueUser != blue.ID {
		t.Errorf("create = %+v, want the seeded pairing", sr)
	}

	got, err := s.SeriesByID(sr.ID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != sr {
		t.Errorf("fetch = %+v, want %+v", got, sr)
	}

	finished := time.Now().Unix()
	if err := s.UpdateSeries(sr.ID, SeriesStateFinished, &red.ID, &finished); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = s.SeriesByID(sr.ID)
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if got.State != SeriesStateFinished || got.Winner == nil || *got.Winner != red.ID || got.FinishedAt == nil || *got.FinishedAt != finished {
		t.Errorf("after update = %+v, want finished with winner %d at %d", got, red.ID, finished)
	}

	if err := s.UpdateSeries(99999, SeriesStateFinished, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing = %v, want ErrNotFound", err)
	}
}

func TestAppendGameEnforcesForeignKeys(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	red, blue := seedUser(t, s, "alice"), seedUser(t, s, "bob")
	_, err := s.AppendGame(Game{
		SeriesID: 999999, IdxInSeries: 0, RedUser: red.ID, BlueUser: blue.ID,
		Outcome: OutcomeRed, Moves: []byte{1, 2}, FullTurns: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("append with bogus series_id = %v, want FOREIGN KEY constraint failure", err)
	}
}

func TestInsertsEnforceForeignKeys(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	red := seedUser(t, s, "alice")
	if err := s.InsertSession(Session{Token: []byte("t"), UserID: 999999, ExpiresAt: 1}); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("session with bogus user = %v, want FOREIGN KEY constraint failure", err)
	}
	if _, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, red.ID, 999999); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("series with bogus blue user = %v, want FOREIGN KEY constraint failure", err)
	}
	if _, err := s.AppendRatingEvent(RatingEvent{GameID: 999999, UserID: red.ID}); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("rating event with bogus game = %v, want FOREIGN KEY constraint failure", err)
	}
}

func TestRatingHistoryByUser(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	alice, bob := seedUser(t, s, "alice"), seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)
	game, err := s.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: []byte{1}, FullTurns: 3, WonBy: ptr("open 4"),
	})
	if err != nil {
		t.Fatalf("append game: %v", err)
	}

	want := []RatingEvent{
		{GameID: game.ID, UserID: alice.ID, Delta: 30, RatingAfter: 30},
		{GameID: game.ID, UserID: alice.ID, Delta: -30, RatingAfter: 0},
		{GameID: game.ID, UserID: alice.ID, Delta: -12, RatingAfter: -12},
	}
	for i, e := range want {
		got, err := s.AppendRatingEvent(e)
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if got.ID == 0 || got.CreatedAt == 0 {
			t.Errorf("append %d = id %d created_at %d, want both nonzero", i, got.ID, got.CreatedAt)
		}
	}
	// Ratings may go negative per the spec, and other users' events never leak in.
	if _, err := s.AppendRatingEvent(RatingEvent{GameID: game.ID, UserID: bob.ID, Delta: -30, RatingAfter: -30}); err != nil {
		t.Fatalf("append bob: %v", err)
	}

	hist, err := s.RatingHistoryByUser(alice.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("history length = %d, want 3", len(hist))
	}
	for i, e := range hist {
		if e.GameID != want[i].GameID || e.Delta != want[i].Delta || e.RatingAfter != want[i].RatingAfter {
			t.Errorf("history[%d] = %+v, want %+v (ascending id order)", i, e, want[i])
		}
	}
}

func seedFinishedSeries(t *testing.T, s *Store, tcIdx int, red, blue User, outcomes []string, winner *int64) SeriesRow {
	t.Helper()
	sr := seedSeries(t, s, red, blue)
	for i, outcome := range outcomes {
		var wonBy *string
		if outcome != OutcomeDraw {
			wonBy = ptr("open 4")
		}
		if _, err := s.AppendGame(Game{
			SeriesID: sr.ID, IdxInSeries: i, RedUser: red.ID, BlueUser: blue.ID,
			Outcome: outcome, Moves: []byte{byte(i)}, FullTurns: i + 1, WonBy: wonBy,
		}); err != nil {
			t.Fatalf("seed game %d: %v", i, err)
		}
	}
	fin := time.Now().Unix()
	if err := s.UpdateSeries(sr.ID, SeriesStateFinished, winner, &fin); err != nil {
		t.Fatalf("finish series: %v", err)
	}
	return sr
}

func TestUserStats(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	alice, bob, carol := seedUser(t, s, "alice"), seedUser(t, s, "bob"), seedUser(t, s, "carol")
	// Alice red: wins g0, loses g1, draws g2, takes the series.
	seedFinishedSeries(t, s, 0, alice, bob, []string{OutcomeRed, OutcomeBlue, OutcomeDraw}, &alice.ID)
	// Alice blue against carol: one loss, carol takes the series.
	seedFinishedSeries(t, s, 1, carol, alice, []string{OutcomeRed}, &carol.ID)

	sa, err := s.UserStats(alice.ID)
	if err != nil {
		t.Fatalf("alice stats: %v", err)
	}
	if sa != (UserStats{Wins: 1, Losses: 2, Draws: 1, SeriesWon: 1}) {
		t.Errorf("alice stats = %+v, want 1-2-1 with 1 series won", sa)
	}
	sb, err := s.UserStats(bob.ID)
	if err != nil {
		t.Fatalf("bob stats: %v", err)
	}
	if sb != (UserStats{Wins: 1, Losses: 1, Draws: 1, SeriesWon: 0}) {
		t.Errorf("bob stats = %+v, want 1-1-1 with 0 series won", sb)
	}
	sc, err := s.UserStats(carol.ID)
	if err != nil {
		t.Fatalf("carol stats: %v", err)
	}
	if sc != (UserStats{Wins: 1, Losses: 0, Draws: 0, SeriesWon: 1}) {
		t.Errorf("carol stats = %+v, want 1-0-0 with 1 series won", sc)
	}
}

func TestMatchHistory(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	alice, bob, carol := seedUser(t, s, "alice"), seedUser(t, s, "bob"), seedUser(t, s, "carol")
	seedFinishedSeries(t, s, 0, alice, bob, []string{OutcomeRed, OutcomeBlue, OutcomeRed}, &alice.ID)
	// A second series whose first game is a draw: draws carry no won_by tag.
	drawSeries := seedSeries(t, s, bob, alice)
	if _, err := s.AppendGame(Game{
		SeriesID: drawSeries.ID, IdxInSeries: 0, RedUser: bob.ID, BlueUser: alice.ID,
		Outcome: OutcomeDraw, Moves: []byte{9, 9}, FullTurns: 128,
	}); err != nil {
		t.Fatalf("append draw: %v", err)
	}

	rows, err := s.MatchHistory(alice.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	// Newest game first, and each line carries the score line as of that
	// game plus the full move blob for the caller-side preview.
	wantWins := [][2]int{{0, 0}, {2, 1}, {1, 1}, {1, 0}}
	wantMoves := []byte{2}
	for i, r := range rows {
		if r.PlayedAt == 0 || r.FullTurns == 0 || len(r.Moves) == 0 {
			t.Errorf("row %d = played_at %d turns %d moves %v, want all set", i, r.PlayedAt, r.FullTurns, r.Moves)
		}
		if i == 0 {
			// The drawn game: alice played blue, score line 0-0, no won_by.
			if r.Red != "bob" || r.Blue != "alice" || r.WonBy != nil {
				t.Errorf("draw row = %s vs %s won_by %v, want bob vs alice with nil won_by", r.Red, r.Blue, r.WonBy)
			}
			continue
		}
		if r.Red != "alice" || r.Blue != "bob" {
			t.Errorf("row %d = %s vs %s, want alice vs bob", i, r.Red, r.Blue)
		}
		if r.RedWins != wantWins[i][0] || r.BlueWins != wantWins[i][1] {
			t.Errorf("row %d score = %d-%d, want %d-%d", i, r.RedWins, r.BlueWins, wantWins[i][0], wantWins[i][1])
		}
		if r.WonBy == nil || *r.WonBy != "open 4" {
			t.Errorf("row %d won_by = %v, want open 4", i, r.WonBy)
		}
	}
	if rows[1].FullTurns != 3 || string(rows[1].Moves) != string(wantMoves) {
		t.Errorf("newest decisive row = turns %d moves %v, want 3 and %v", rows[1].FullTurns, rows[1].Moves, wantMoves)
	}

	if rows, err = s.MatchHistory(carol.ID); err != nil {
		t.Fatalf("carol history: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("carol rows = %d, want 0", len(rows))
	}
}

// seedGame appends one game row with explicit per-game seating, so tests can
// reproduce the red rotation the loser-takes-red law produces in play.
func seedGame(t *testing.T, s *Store, sr SeriesRow, idx int, redUser, blueUser int64, outcome string) {
	t.Helper()
	if _, err := s.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: idx, RedUser: redUser, BlueUser: blueUser,
		Outcome: outcome, Moves: []byte{byte(idx)}, FullTurns: idx + 1,
	}); err != nil {
		t.Fatalf("seed game %d: %v", idx, err)
	}
}

// TestMatchHistoryScoreCountsParticipantsNotColors pins the score line
// semantics: RedWins and BlueWins are the row's red and blue PLAYERS' wins in
// the series so far. Counting color outcomes instead mixes the two players,
// because the loser-takes-red law rotates the red seat between games.
func TestMatchHistoryScoreCountsParticipantsNotColors(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	// Distinct users per scenario: history spans every series of a user.
	alice, bob := seedUser(t, s, "alice"), seedUser(t, s, "bob")
	carol, dave := seedUser(t, s, "carol"), seedUser(t, s, "dave")
	erin, frank := seedUser(t, s, "erin"), seedUser(t, s, "frank")

	// Bo3, each player wins one game as red: game 1 alice red, game 2 bob
	// red. The game-2 row must read 1-1 (bob 1, alice 1); a color count
	// reads 2-0.
	swapped := seedSeries(t, s, alice, bob)
	seedGame(t, s, swapped, 0, alice.ID, bob.ID, OutcomeRed)
	seedGame(t, s, swapped, 1, bob.ID, alice.ID, OutcomeRed)
	rows, err := s.MatchHistory(alice.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("swapped-seating rows = %d, want 2", len(rows))
	}
	g2, g1 := rows[0], rows[1]
	if g1.Red != "alice" || g1.RedWins != 1 || g1.BlueWins != 0 {
		t.Errorf("game 1 row = %s %d-%d, want alice red at 1-0", g1.Red, g1.RedWins, g1.BlueWins)
	}
	if g2.Red != "bob" || g2.Blue != "alice" || g2.RedWins != 1 || g2.BlueWins != 1 {
		t.Errorf("game 2 row = %s vs %s %d-%d, want bob vs alice at 1-1 by participant", g2.Red, g2.Blue, g2.RedWins, g2.BlueWins)
	}

	// A three-game line whose final row reverses the leader under a color
	// count: dave wins games 1 and 2, carol game 3, so dave leads 2-1. The
	// game-3 row seats carol on red; a color count reads 2-1 for carol.
	reversed := seedSeries(t, s, carol, dave)
	seedGame(t, s, reversed, 0, carol.ID, dave.ID, OutcomeBlue)
	seedGame(t, s, reversed, 1, dave.ID, carol.ID, OutcomeRed)
	seedGame(t, s, reversed, 2, carol.ID, dave.ID, OutcomeRed)
	rows, err = s.MatchHistory(dave.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("reversal rows = %d, want 3", len(rows))
	}
	final := rows[0]
	if final.Red != "carol" || final.Blue != "dave" || final.RedWins != 1 || final.BlueWins != 2 {
		t.Errorf("final row = %s vs %s %d-%d, want carol vs dave at 1-2: dave leads", final.Red, final.Blue, final.RedWins, final.BlueWins)
	}

	// Fixed seating, the case a color count already gets right: one red in
	// every game, so the participant line and the color line coincide.
	fixed := seedSeries(t, s, erin, frank)
	seedGame(t, s, fixed, 0, erin.ID, frank.ID, OutcomeRed)
	seedGame(t, s, fixed, 1, erin.ID, frank.ID, OutcomeBlue)
	seedGame(t, s, fixed, 2, erin.ID, frank.ID, OutcomeRed)
	rows, err = s.MatchHistory(erin.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	wantFixed := [][2]int{{2, 1}, {1, 1}, {1, 0}}
	for i, r := range rows {
		if r.RedWins != wantFixed[i][0] || r.BlueWins != wantFixed[i][1] {
			t.Errorf("fixed row %d score = %d-%d, want %d-%d", i, r.RedWins, r.BlueWins, wantFixed[i][0], wantFixed[i][1])
		}
	}
}

// TestStoreMutationsHonorCanceledContext pins the wedged-write ceiling of
// the write queue: the store mutations that run under the queue's apply
// context (CreateSeries on join, ApplyCompletion on every game end) thread
// that context into the driver, so a context already done fails the write
// instead of hanging past the deadline the queue believed it enforced.
func TestStoreMutationsHonorCanceledContext(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	alice, bob := seedUser(t, s, "alice"), seedUser(t, s, "bob")
	sr := seedSeries(t, s, alice, bob)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.CreateSeries(ctx, 0, config.SeriesBO3, alice.ID, bob.ID); !errors.Is(err, context.Canceled) {
		t.Errorf("create series under a canceled ctx = %v, want context.Canceled", err)
	}
	err := s.ApplyCompletion(ctx, Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
			Outcome: OutcomeRed, Moves: []byte{1},
		}},
		Finish: &SeriesFinish{SeriesID: sr.ID, Winner: &alice.ID, FinishedAt: time.Now().Unix()},
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("apply completion under a canceled ctx = %v, want context.Canceled", err)
	}

	var games, ratings int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&games); err != nil {
		t.Fatalf("count games: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM rating_events`).Scan(&ratings); err != nil {
		t.Fatalf("count ratings: %v", err)
	}
	if games != 0 || ratings != 0 {
		t.Errorf("canceled mutations left %d games and %d ratings, want none", games, ratings)
	}
	if got, err := s.SeriesByID(sr.ID); err != nil || got.State != SeriesStateOngoing {
		t.Errorf("series after canceled mutations = %+v err %v, want untouched ongoing", got, err)
	}
}

func TestConcurrentReadersWhileWriterHolds(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()

	alice := seedUser(t, s, "alice")
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatalf("begin writer tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		"bob", config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("uncommitted insert: %v", err)
	}

	readers := runtime.NumCPU()
	if readers > 4 {
		readers = 4
	}
	done := make(chan error)
	for range readers {
		go func() {
			u, err := s.UserByUsername("alice")
			if err != nil {
				done <- err
				return
			}
			if u.ID != alice.ID {
				done <- errors.New("concurrent read lost the committed row")
				return
			}
			done <- nil
		}()
	}
	deadline := time.After(time.Duration(config.SQLiteBusyTimeoutMs) * time.Millisecond)
	for range readers {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("reader under open writer tx: %v", err)
			}
		case <-deadline:
			t.Fatal("readers stalled behind the writer transaction, WAL snapshot isolation broken")
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := s.UserByUsername("bob"); err != nil {
		t.Fatalf("read after commit: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
