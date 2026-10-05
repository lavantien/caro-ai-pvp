package tourney

// The fault surfaces of the M7 write and read paths, injected through the
// real SQLite store: dropped tables, aborted statements (RAISE triggers),
// and type-corrupted rows make every persistence unit fail where it must,
// with the failure wrapped in its context and the whole unit rolled back.
// The conductor's drive-level arms ride the same faults: the schedule
// cross-check refuses a drifted schedule, a failing snapshot aborts the
// run, and a dead context aborts before any series starts.

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// runSQL runs one statement through the store's own transaction surface:
// the schema faults these tests inject.
func runSQL(t *testing.T, srv *server.Store, query string, args ...any) {
	t.Helper()
	if err := srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), query, args...)
		return err
	}); err != nil {
		t.Fatalf("run %q: %v", query, err)
	}
}

// literalSource hands every series the same raw event slice verbatim: the
// fixture never parses move payloads, so malformed scripts reach the
// conductor's own validators. Truth stays nil: these scripts fail before
// any game-end reconciliation reads it.
type literalSource struct {
	events []server.Event
}

func (s *literalSource) StartSeries(*config.Tier, *config.Tier, string, string, int, int) (SeriesStream, error) {
	ch := make(chan server.Event, len(s.events))
	for _, ev := range s.events {
		ch <- ev
	}
	close(ch)
	return literalStream{ch: ch}, nil
}

type literalStream struct{ ch chan server.Event }

func (s literalStream) Events() <-chan server.Event { return s.ch }
func (s literalStream) Err() error                  { return nil }
func (s literalStream) TruthMoves() []rules.Move    { return nil }
func (s literalStream) Close()                      {}

// TestConductorDisqualifiesMalformedStreams pins the conductor's own event
// validators: an unparsable move payload, a game end that is not a room
// outcome, and an event kind outside the room contract each fail the run.
func TestConductorDisqualifiesMalformedStreams(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []server.Event
		want   string
	}{
		{
			"unparsable move payload",
			[]server.Event{{Kind: server.EventKindMove, Payload: "nope"}},
			`move "nope"`,
		},
		{
			"game end outside the outcomes",
			[]server.Event{{Kind: server.EventKindGameEnd, Payload: "sideways"}},
			"not a room outcome",
		},
		{
			"event kind outside the contract",
			[]server.Event{{Kind: "noise", Payload: "x"}},
			"not part of the room contract",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, _ := newTestStore(t)
			pointLogsAt(t)
			_, err := NewConductor(&literalSource{events: tc.events}).Run(context.Background(), ts, rosterTwo(),
				mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
			if err == nil {
				t.Fatal("run succeeded, want the disqualifier")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("run error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestConductorFailsWhenSeriesLogCannotOpen pins the log-seam failure: a
// series whose log file cannot be created fails the series, and with it
// the run, before any game persists.
func TestConductorFailsWhenSeriesLogCannotOpen(t *testing.T) {
	ts, _ := newTestStore(t)
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	saved := config.TournamentLogRoot
	config.TournamentLogRoot = blocker
	t.Cleanup(func() { config.TournamentLogRoot = saved })

	_, err := NewConductor(&fakeSource{script: easySweeps}).Run(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err == nil || !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("run error = %v, want the log-dir failure", err)
	}
	if status := runStatus(t, ts, 1); status != RunStateOngoing {
		t.Errorf("run status = %q, want %q for the post-mortem", status, RunStateOngoing)
	}
}

// TestConductorDriveCrossChecksSchedule pins the drive-side schedule
// cross-check: a schedule whose length or seats disagree with the pairing
// plan of the roster it is driven with refuses the run instead of playing
// the wrong pairings.
func TestConductorDriveCrossChecksSchedule(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{script: easySweeps}

	// A roster larger than the persisted schedule: 6 pairings against 2
	// series rows.
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	c := NewConductor(src)
	run, _, err := c.startRun(ctx, ts, rosterTwo(), mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	tiers, err := tiersOfRoster(roster(3))
	if err != nil {
		t.Fatalf("resolve 3-roster: %v", err)
	}
	_, err = c.drive(ctx, ts, run, roster(3), tiers, 1, "test")
	if err == nil || !strings.Contains(err.Error(), "series rows for") {
		t.Fatalf("length-drift drive = %v, want the row-count refusal", err)
	}
	if status := runStatus(t, ts, run.ID); status != RunStateOngoing {
		t.Errorf("status after the refusal = %q, want %q", status, RunStateOngoing)
	}
	if starts, _, _ := src.snapshot(); len(starts) != 0 {
		t.Errorf("started = %d series against the drifted schedule, want 0", len(starts))
	}

	// A schedule row whose seats were corrupted in storage: same length,
	// wrong pair.
	ts2, srv2 := newTestStore(t)
	pointLogsAt(t)
	c2 := NewConductor(&fakeSource{script: easySweeps})
	run2, tiers2, err := c2.startRun(ctx, ts2, rosterTwo(), mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("start run 2: %v", err)
	}
	runSQL(t, srv2, `UPDATE tournament_series SET red_first_slot = 1, blue_first_slot = 0 WHERE pairing_slot = 0`)
	_, err = c2.drive(ctx, ts2, run2, rosterTwo(), tiers2, 1, "test")
	if err == nil || !strings.Contains(err.Error(), "disagree with pairing") {
		t.Fatalf("seat-drift drive = %v, want the seat refusal", err)
	}
	if status := runStatus(t, ts2, run2.ID); status != RunStateOngoing {
		t.Errorf("status after the seat refusal = %q, want %q", status, RunStateOngoing)
	}
}

// TestConductorDriveFailsWhenScheduleReadFails pins the drive's first store
// read: a schedule that cannot be read at all fails the run with the
// wrapped error.
func TestConductorDriveFailsWhenScheduleReadFails(t *testing.T) {
	ts, srv := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	c := NewConductor(&fakeSource{script: easySweeps})
	run, tiers, err := c.startRun(ctx, ts, rosterTwo(), mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runSQL(t, srv, `DROP TABLE tournament_series`)

	_, err = c.drive(ctx, ts, run, rosterTwo(), tiers, 1, "test")
	if err == nil || !strings.Contains(err.Error(), "read schedule") {
		t.Fatalf("drive = %v, want the schedule-read failure", err)
	}
	if status := runStatus(t, ts, run.ID); status != RunStateOngoing {
		t.Errorf("status = %q, want %q", status, RunStateOngoing)
	}
}

// TestConductorDriveAbortsOnDeadContext pins the pre-dispatch abort: with
// the dispatch loop parked on a zero-capacity semaphore (its send can never
// succeed, so the loop's only exit is the cancellation) a context that dies
// mid-drive aborts before any series starts, and the abort names the
// context error. The cancel lands a beat after the drive's schedule read, a
// local SQLite read of two rows, and the assertion on the abort wrap makes
// a cancel that raced ahead of it a loud failure.
func TestConductorDriveAbortsOnDeadContext(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	ctx, cancel := context.WithCancel(context.Background())
	src := &fakeSource{script: easySweeps}
	c := NewConductor(src)
	run, tiers, err := c.startRun(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := c.drive(ctx, ts, run, rosterTwo(), tiers, 0, "test")
		done <- err
	}()
	time.Sleep(250 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "aborted: context canceled") {
			t.Fatalf("drive = %v, want the run-abort wrap over the cancel", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("drive never returned after the cancel")
	}
	if starts, closed, _ := src.snapshot(); len(starts) != 0 || closed != 0 {
		t.Errorf("source saw %d starts and %d closes, want none", len(starts), closed)
	}
}

// TestConductorStartRunFailsWhenGateReadFails pins the run gate's read
// fault: a store that cannot answer the ongoing-run check refuses the start
// before anything persists.
func TestConductorStartRunFailsWhenGateReadFails(t *testing.T) {
	ts, srv := newTestStore(t)
	runSQL(t, srv, `DROP TABLE tournament_runs`)
	_, _, err := NewConductor(&fakeSource{script: easySweeps}).startRun(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err == nil || !strings.Contains(err.Error(), "ongoing run") {
		t.Fatalf("start run = %v, want the gate-read failure", err)
	}
}

// TestConductorDriveFailsWhenSnapshotCannotLand pins the snapshot arm: with
// every series settled, a failing standings snapshot aborts the run's
// FinishRun and the row stays ongoing for the post-mortem.
func TestConductorDriveFailsWhenSnapshotCannotLand(t *testing.T) {
	ts, srv := newTestStore(t)
	pointLogsAt(t)
	started := make(chan struct{})
	var once sync.Once
	release := make(chan struct{})
	src := &fakeSource{script: easySweeps, onStart: func(*fakeStream) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}}
	c := NewConductor(src)
	run, tiers, err := c.startRun(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := c.drive(context.Background(), ts, run, rosterTwo(), tiers, 1, "test")
		done <- err
	}()
	<-started
	runSQL(t, srv, `DROP TABLE tournament_standings`)
	close(release)

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "snapshot") {
			t.Fatalf("drive = %v, want the standings-snapshot failure", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("drive never returned after the snapshot fault")
	}
	if status := runStatus(t, ts, run.ID); status != RunStateOngoing {
		t.Errorf("status = %q, want %q after the failed close", status, RunStateOngoing)
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_games`); n != 4 {
		t.Errorf("games = %d, want both pairings' 4 before the failed close", n)
	}
}

// TestStoreCreateRunFaults pins the create unit's fault arms: each
// statement failing anywhere rolls the whole unit back, so every surviving
// tournament table stays empty.
func TestStoreCreateRunFaults(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fault     string
		want      string
		survivors []string
	}{
		{"run insert", `DROP TABLE tournament_runs`, "create run",
			[]string{"tournament_participants", "tournament_series", "tournament_games"}},
		{"participant insert", `DROP TABLE tournament_participants`, "create participant",
			[]string{"tournament_runs", "tournament_series", "tournament_games"}},
		{"series insert", `DROP TABLE tournament_series`, "create series",
			[]string{"tournament_runs", "tournament_participants", "tournament_games"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, srv := newTestStore(t)
			runSQL(t, srv, tc.fault)
			_, err := ts.CreateRun(context.Background(), 0, config.SeriesBO3, config.TournamentStartRating, rosterTwo())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("create run = %v, want the %q failure", err, tc.want)
			}
			for _, table := range tc.survivors {
				if n := countRows(t, srv, `SELECT COUNT(*) FROM `+table); n != 0 {
					t.Errorf("%s holds %d rows after the failed create, want the rolled-back 0", table, n)
				}
			}
		})
	}
}

// TestCreateRunRequiresNames: the roster validation arms the earlier
// length and slot checks shadow in the existing table.
func TestCreateRunRequiresNames(t *testing.T) {
	ts, srv := newTestStore(t)
	for name, roster := range map[string][]Participant{
		"empty name": {
			{Slot: 0, Name: "a", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "", Tier: config.TierEasy.Name},
		},
		"empty tier": {
			{Slot: 0, Name: "a", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "b", Tier: ""},
		},
	} {
		if _, err := ts.CreateRun(context.Background(), 0, config.SeriesBO3, config.TournamentStartRating, roster); err == nil {
			t.Errorf("%s: create run succeeded, want rejection", name)
		}
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_runs`); n != 0 {
		t.Errorf("runs after the rejections = %d, want 0", n)
	}
}

// TestAppendGameStatementFaults pins the append unit against statement
// faults: a blocked insert or a blocked settle rolls the whole unit back,
// and a missing runs table surfaces through the not-found wrap rather than
// the no-rows sentinel.
func TestAppendGameStatementFaults(t *testing.T) {
	ctx := context.Background()

	ts, srv := newTestStore(t)
	runSQL(t, srv, `DROP TABLE tournament_runs`)
	if _, err := ts.AppendGame(ctx, Game{RunID: 1, SeriesID: 1, RedSlot: 0, BlueSlot: 1, Outcome: server.RedWins}); err == nil {
		t.Fatal("append over a dropped runs table succeeded, want failure")
	} else if errors.Is(err, server.ErrNotFound) {
		t.Errorf("dropped-table failure = %v, want it not to pose as ErrNotFound", err)
	}

	cases := []struct {
		name        string
		fault       string
		want        string
		gamesExists bool
	}{
		{"games count", `DROP TABLE tournament_games`, "count games", false},
		{"blocked insert", `CREATE TRIGGER block_game_insert BEFORE INSERT ON tournament_games
			BEGIN SELECT RAISE(ABORT, 'blocked'); END`, "append game", true},
		{"blocked settle", `CREATE TRIGGER block_series_update BEFORE UPDATE ON tournament_series
			BEGIN SELECT RAISE(ABORT, 'blocked'); END`, "settle series", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, srv := newTestStore(t)
			run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
			if err != nil {
				t.Fatalf("create run: %v", err)
			}
			first := mustSchedule(t, srv, run.ID)[0]
			runSQL(t, srv, tc.fault)

			_, err = ts.AppendGame(ctx, gameOf(run.ID, first.ID, 0, 0, 1, server.RedWins))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("append = %v, want the %q failure", err, tc.want)
			}
			if tc.gamesExists {
				if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_games`); n != 0 {
					t.Errorf("games after the failed append = %d, want the rolled-back 0", n)
				}
			}
			if s := mustSchedule(t, srv, run.ID)[0]; s.RedFirstWins != 0 || s.BlueFirstWins != 0 || s.FinishedAt != nil {
				t.Errorf("series after the failed append = %+v, want untouched", s)
			}
		})
	}
}

// settleBothSeries appends the 2-0 majority that closes each series of a
// fresh 2-roster run.
func settleBothSeries(t *testing.T, ts *Store, runID int64) {
	t.Helper()
	for _, s := range mustSchedule(t, ts.srv, runID) {
		mustAppend(t, ts, gameOf(runID, s.ID, 0, s.RedFirstSlot, s.BlueFirstSlot, server.RedWins))
		mustAppend(t, ts, gameOf(runID, s.ID, 1, s.RedFirstSlot, s.BlueFirstSlot, server.RedWins))
	}
}

// TestFinishRunFaults pins the close unit's fault arms: the header read,
// the unfinished count, the fold, the status flip, and the snapshot insert
// each failing surfaces its own wrap.
func TestFinishRunFaults(t *testing.T) {
	ctx := context.Background()

	ts, srv := newTestStore(t)
	runSQL(t, srv, `DROP TABLE tournament_runs`)
	if err := ts.FinishRun(ctx, 1, 1); err == nil || !strings.Contains(err.Error(), "run 1") {
		t.Errorf("finish over a dropped runs table = %v, want the wrapped read failure", err)
	}

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, ts *Store, srv *server.Store, runID int64)
		want  string
	}{
		{
			"unfinished count",
			func(t *testing.T, _ *Store, srv *server.Store, _ int64) {
				runSQL(t, srv, `DROP TABLE tournament_series`)
			},
			"unfinished series",
		},
		{
			"fold inside the close",
			func(t *testing.T, _ *Store, srv *server.Store, _ int64) {
				runSQL(t, srv, `DELETE FROM tournament_series`)
				runSQL(t, srv, `DROP TABLE tournament_participants`)
			},
			"participants",
		},
		{
			"status flip",
			func(t *testing.T, ts *Store, srv *server.Store, runID int64) {
				settleBothSeries(t, ts, runID)
				runSQL(t, srv, `CREATE TRIGGER block_run_update BEFORE UPDATE ON tournament_runs
					BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
			},
			"finish run",
		},
		{
			"snapshot insert",
			func(t *testing.T, ts *Store, srv *server.Store, runID int64) {
				settleBothSeries(t, ts, runID)
				runSQL(t, srv, `DROP TABLE tournament_standings`)
			},
			"snapshot",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, srv := newTestStore(t)
			run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
			if err != nil {
				t.Fatalf("create run: %v", err)
			}
			tc.setup(t, ts, srv, run.ID)
			if err := ts.FinishRun(ctx, run.ID, 1); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("finish = %v, want the %q failure", err, tc.want)
			}
		})
	}
}

// TestCloseStalledRunFault: the stalled close over a missing runs table
// surfaces the wrapped update failure.
func TestCloseStalledRunFault(t *testing.T) {
	ts, srv := newTestStore(t)
	runSQL(t, srv, `DROP TABLE tournament_runs`)
	if err := ts.CloseStalledRun(context.Background(), 1, 1); err == nil || !strings.Contains(err.Error(), "close stalled run") {
		t.Errorf("close over a dropped runs table = %v, want the wrapped update failure", err)
	}
}

// TestScheduleFaults pins the conductor's schedule read against a missing
// table and a type-corrupted row.
func TestScheduleFaults(t *testing.T) {
	ctx := context.Background()

	ts, srv := newTestStore(t)
	runSQL(t, srv, `DROP TABLE tournament_series`)
	if _, err := ts.Schedule(ctx, 1); err == nil || !strings.Contains(err.Error(), "read schedule") {
		t.Errorf("schedule over a dropped table = %v, want the read failure", err)
	}

	ts, srv = newTestStore(t)
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runSQL(t, srv, `INSERT INTO tournament_series (run_id, pairing_slot, red_first_slot, blue_first_slot)
		VALUES (?, 'x', 0, 1)`, run.ID)
	if _, err := ts.Schedule(ctx, run.ID); err == nil {
		t.Error("schedule over a corrupted row succeeded, want the scan failure")
	}
}

// TestLeaderboardFaults pins the fold's reads: each input query failing,
// and a type-corrupted participant or winner row failing its scan.
func TestLeaderboardFaults(t *testing.T) {
	ctx := context.Background()
	dropSeriesFirst := func(t *testing.T, srv *server.Store) {
		runSQL(t, srv, `DELETE FROM tournament_series`)
		runSQL(t, srv, `DROP TABLE tournament_participants`)
	}

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, ts *Store, srv *server.Store, runID int64)
		want  string
	}{
		{
			"participants read",
			func(t *testing.T, _ *Store, srv *server.Store, _ int64) { dropSeriesFirst(t, srv) },
			"participants",
		},
		{
			"games read",
			func(t *testing.T, _ *Store, srv *server.Store, _ int64) {
				runSQL(t, srv, `DROP TABLE tournament_games`)
			},
			"games",
		},
		{
			"series winners read",
			func(t *testing.T, _ *Store, srv *server.Store, _ int64) {
				runSQL(t, srv, `DROP TABLE tournament_series`)
			},
			"series winners",
		},
		{
			"corrupted participant slot",
			func(t *testing.T, _ *Store, srv *server.Store, runID int64) {
				runSQL(t, srv, `INSERT INTO tournament_participants (run_id, slot, name, tier)
					VALUES (?, 'x', 'ghost', 'easy')`, runID)
			},
			"participant",
		},
		{
			"corrupted series winner",
			func(t *testing.T, _ *Store, srv *server.Store, runID int64) {
				runSQL(t, srv, `INSERT INTO tournament_series (run_id, pairing_slot, red_first_slot, blue_first_slot, winner_slot)
					VALUES (?, 99, 0, 1, 'x')`, runID)
			},
			"series winner",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, srv := newTestStore(t)
			run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
			if err != nil {
				t.Fatalf("create run: %v", err)
			}
			tc.setup(t, ts, srv, run.ID)
			if _, err := ts.Leaderboard(ctx, run.ID); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("leaderboard = %v, want the %q failure", err, tc.want)
			}
		})
	}
}

// TestQueriesFaults pins the page reads: each list query against a missing
// table, and type-corrupted rows failing their scans mid-list.
func TestQueriesFaults(t *testing.T) {
	ctx := context.Background()

	// A store with no rows at all: every table can drop without the
	// foreign keys objecting, and each list read fails on its own table.
	ts, srv := newTestStore(t)
	for _, drop := range []string{
		`DROP TABLE tournament_runs`,
		`DROP TABLE tournament_participants`,
		`DROP TABLE tournament_series`,
	} {
		runSQL(t, srv, drop)
	}
	if _, err := ts.Runs(ctx); err == nil || !strings.Contains(err.Error(), "list runs") {
		t.Errorf("runs over a dropped table = %v, want the list failure", err)
	}
	if _, _, err := ts.OngoingRunID(ctx); err == nil || !strings.Contains(err.Error(), "ongoing run") {
		t.Errorf("ongoing-run read over a dropped table = %v, want the read failure", err)
	}
	if _, err := ts.Roster(ctx, 1); err == nil || !strings.Contains(err.Error(), "roster") {
		t.Errorf("roster over a dropped participants table = %v, want the read failure", err)
	}
	if _, err := ts.SeriesAll(ctx, 1); err == nil || !strings.Contains(err.Error(), "series of") {
		t.Errorf("series list over a dropped table = %v, want the read failure", err)
	}

	// The scan arms: text planted into integer columns.
	ts, srv = newTestStore(t)
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	runSQL(t, srv, `INSERT INTO tournament_runs (tc_idx, bo_len, start_rating, status)
		VALUES ('bogus', 3, 1000, 'ongoing')`)
	if _, err := ts.Runs(ctx); err == nil {
		t.Error("runs over a corrupted tc_idx succeeded, want the scan failure")
	}
	runSQL(t, srv, `INSERT INTO tournament_participants (run_id, slot, name, tier)
		VALUES (?, 'x', 'ghost', 'easy')`, run.ID)
	if _, err := ts.Roster(ctx, run.ID); err == nil {
		t.Error("roster over a corrupted slot succeeded, want the scan failure")
	}
	runSQL(t, srv, `INSERT INTO tournament_series (run_id, pairing_slot, red_first_slot, blue_first_slot)
		VALUES (?, 'x', 0, 1)`, run.ID)
	if _, err := ts.SeriesAll(ctx, run.ID); err == nil {
		t.Error("series list over a corrupted pairing slot succeeded, want the scan failure")
	}
}

// TestManagerDetailFaults pins the page read's ordering: the first failing
// projection surfaces, with the projections before it already read.
func TestManagerDetailFaults(t *testing.T) {
	ctx := context.Background()

	ts, srv := newTestStore(t)
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	m := NewManager(ts, &fakeSource{script: easySweeps})

	runSQL(t, srv, `DROP TABLE tournament_games`)
	if _, err := m.Detail(ctx, run.ID); err == nil || !strings.Contains(err.Error(), "games") {
		t.Errorf("detail without the games table = %v, want the leaderboard failure", err)
	}

	runSQL(t, srv, `DROP TABLE tournament_series`)
	if _, err := m.Detail(ctx, run.ID); err == nil || !strings.Contains(err.Error(), "series of") {
		t.Errorf("detail without the series table = %v, want the series-list failure", err)
	}

	ts2, srv2 := newTestStore(t)
	run2, err := ts2.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run 2: %v", err)
	}
	runSQL(t, srv2, `DELETE FROM tournament_series`)
	runSQL(t, srv2, `DROP TABLE tournament_participants`)
	if _, err := NewManager(ts2, &fakeSource{script: easySweeps}).Detail(ctx, run2.ID); err == nil ||
		!strings.Contains(err.Error(), "roster") {
		t.Errorf("detail without the participants table = %v, want the roster failure", err)
	}
}
