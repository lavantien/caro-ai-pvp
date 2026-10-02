package tourney

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func newTestStore(t *testing.T) (*Store, *server.Store) {
	t.Helper()
	srv, err := server.Open(filepath.Join(t.TempDir(), "caro.sqlite"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return NewStore(srv), srv
}

// rosterSix builds the Implication 2.4 roster: two instances of each tier.
func rosterSix() []Participant {
	var out []Participant
	for _, tier := range config.Tiers {
		for i := 1; i <= 2; i++ {
			out = append(out, Participant{Slot: len(out), Name: fmt.Sprintf("%s-%d", tier.Name, i), Tier: tier.Name})
		}
	}
	return out
}

// mustSchedule reads a run's series rows in pairing order for assertions.
func mustSchedule(t *testing.T, srv *server.Store, runID int64) []Series {
	t.Helper()
	var out []Series
	err := srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(context.Background(), `
		SELECT id, pairing_slot, red_first_slot, blue_first_slot, winner_slot, red_first_wins, blue_first_wins, finished_at
		FROM tournament_series WHERE run_id = ? ORDER BY pairing_slot`, runID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s Series
			var winner, finished sql.NullInt64
			if err := rows.Scan(&s.ID, &s.PairingSlot, &s.RedFirstSlot, &s.BlueFirstSlot,
				&winner, &s.RedFirstWins, &s.BlueFirstWins, &finished); err != nil {
				return err
			}
			if winner.Valid {
				w := int(winner.Int64)
				s.WinnerSlot = &w
			}
			if finished.Valid {
				f := finished.Int64
				s.FinishedAt = &f
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read schedule of run %d: %v", runID, err)
	}
	return out
}

func countRows(t *testing.T, srv *server.Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := srv.WithinTx(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRowContext(context.Background(), query, args...).Scan(&n)
	}); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func TestCreateRunPersistsSchedule(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, rosterSix())
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.Status != RunStateOngoing || run.TCIdx != 1 || run.BOLen != config.SeriesBO3 ||
		run.StartRating != config.TournamentStartRating || run.ID == 0 || run.CreatedAt == 0 || run.FinishedAt != nil {
		t.Errorf("run = %+v", run)
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_participants WHERE run_id = ?`, run.ID); n != 6 {
		t.Errorf("participants = %d, want 6", n)
	}
	schedule := mustSchedule(t, srv, run.ID)
	if len(schedule) != 30 {
		t.Fatalf("series = %d, want n*(n-1) = 30", len(schedule))
	}
	if schedule[0].PairingSlot != 0 || schedule[0].RedFirstSlot != 0 || schedule[0].BlueFirstSlot != 1 {
		t.Errorf("first series = %+v, want pairing 0 with slots 0-1", schedule[0])
	}
	if schedule[29].PairingSlot != 29 || schedule[29].RedFirstSlot != 1 || schedule[29].BlueFirstSlot != 0 {
		t.Errorf("last series = %+v, want pairing 29 with slots 1-0", schedule[29])
	}
	for _, s := range schedule {
		if s.WinnerSlot != nil || s.RedFirstWins != 0 || s.BlueFirstWins != 0 || s.FinishedAt != nil {
			t.Errorf("fresh series %+v carries results", s)
		}
	}
}

func TestCreateRunValidation(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	cases := []struct {
		name   string
		tc, bo int
		roster []Participant
	}{
		{"tc above range", len(config.TimeControls), config.SeriesBO3, roster(2)},
		{"tc negative", -1, config.SeriesBO3, roster(2)},
		{"bo not configured", 0, 4, roster(2)},
		{"slot drift", 0, config.SeriesBO3, []Participant{
			{Slot: 1, Name: "a", Tier: "easy"}, {Slot: 1, Name: "b", Tier: "easy"},
		}},
		{"empty name", 0, config.SeriesBO3, []Participant{{Slot: 0, Name: "", Tier: "easy"}}},
		{"empty tier", 0, config.SeriesBO3, []Participant{{Slot: 0, Name: "a", Tier: ""}}},
	}
	for _, c := range cases {
		if _, err := ts.CreateRun(ctx, c.tc, c.bo, config.TournamentStartRating, c.roster); err == nil {
			t.Errorf("%s: create run succeeded, want rejection", c.name)
		}
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_runs`); n != 0 {
		t.Errorf("runs after rejections = %d, want 0 (whole unit rolled back)", n)
	}
}

// mustAppend wraps AppendGame for the happy path.
func mustAppend(t *testing.T, ts *Store, g Game) Game {
	t.Helper()
	out, err := ts.AppendGame(context.Background(), g)
	if err != nil {
		t.Fatalf("append game series %d idx %d: %v", g.SeriesID, g.IdxInSeries, err)
	}
	return out
}

func gameOf(runID, seriesID int64, idx, red, blue int, outcome server.Outcome) Game {
	return Game{RunID: runID, SeriesID: seriesID, IdxInSeries: idx,
		RedSlot: red, BlueSlot: blue, Outcome: outcome, FullTurns: idx + 1,
		Moves: []rules.Move{rules.Move(7), rules.Move(120)}}
}

func TestAppendGameSettlesSeriesFromGames(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	schedule := mustSchedule(t, srv, run.ID)
	first, second := schedule[0], schedule[1]
	if first.RedFirstSlot != 0 || second.RedFirstSlot != 1 {
		t.Fatalf("schedule seats = %+v %+v", first, second)
	}

	g := mustAppend(t, ts, gameOf(run.ID, first.ID, 0, 0, 1, server.RedWins))
	if g.ID == 0 || g.PlayedAt == 0 {
		t.Errorf("appended game = %+v, want id and played_at stamped", g)
	}
	s := mustSchedule(t, srv, run.ID)[0]
	if s.RedFirstWins != 1 || s.BlueFirstWins != 0 || s.WinnerSlot != nil || s.FinishedAt != nil {
		t.Errorf("series after game 0 = %+v, want 1-0 still ongoing", s)
	}

	// The moves blob must decode through the shared server codec.
	var blob []byte
	if err := srv.WithinTx(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT moves FROM tournament_games WHERE id = ?`, g.ID).Scan(&blob)
	}); err != nil {
		t.Fatalf("read moves blob: %v", err)
	}
	moves, err := server.DecodeMoves(blob)
	if err != nil {
		t.Fatalf("decode moves: %v", err)
	}
	if len(moves) != 2 || moves[0] != rules.Move(7) || moves[1] != rules.Move(120) {
		t.Errorf("moves round-trip = %v", moves)
	}

	// 2-0 closes the bo3 on the majority; a billed third game keeps the
	// winner and the cap rejects a fourth.
	mustAppend(t, ts, gameOf(run.ID, first.ID, 1, 0, 1, server.RedWins))
	s = mustSchedule(t, srv, run.ID)[0]
	if s.RedFirstWins != 2 || s.WinnerSlot == nil || *s.WinnerSlot != 0 || s.FinishedAt == nil {
		t.Errorf("series after 2-0 = %+v, want slot 0 finished on majority", s)
	}
	mustAppend(t, ts, gameOf(run.ID, first.ID, 2, 0, 1, server.BlueWins))
	s = mustSchedule(t, srv, run.ID)[0]
	if s.RedFirstWins != 2 || s.BlueFirstWins != 1 || *s.WinnerSlot != 0 {
		t.Errorf("series after billed game = %+v, want 2-1 still slot 0", s)
	}
	if _, err := ts.AppendGame(ctx, gameOf(run.ID, first.ID, 3, 0, 1, server.Draw)); err == nil {
		t.Errorf("game past the bo_len cap accepted, want rejection")
	}

	for i := 0; i < 3; i++ {
		outcomes := []server.Outcome{server.BlueWins, server.RedWins, server.RedWins}
		mustAppend(t, ts, gameOf(run.ID, second.ID, i, 1, 0, outcomes[i]))
	}
	s = mustSchedule(t, srv, run.ID)[1]
	if s.RedFirstWins != 2 || s.BlueFirstWins != 1 || *s.WinnerSlot != 1 || s.FinishedAt == nil {
		t.Errorf("second series = %+v, want slot 1 finishing 2-1", s)
	}
}

func TestAppendGameRejects(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	other, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create other run: %v", err)
	}
	first := mustSchedule(t, srv, run.ID)[0]
	foreign := mustSchedule(t, srv, other.ID)[0]

	cases := []struct {
		name string
		g    Game
		want string
	}{
		{"unknown run", gameOf(999, first.ID, 0, 0, 1, server.RedWins), "not found"},
		{"series of another run", gameOf(run.ID, foreign.ID, 0, 0, 1, server.RedWins), "not found"},
		{"slot outside roster", gameOf(run.ID, first.ID, 0, 5, 1, server.RedWins), "not a participant"},
		{"same red and blue", gameOf(run.ID, first.ID, 0, 0, 0, server.RedWins), "both 0"},
		{"idx out of order", gameOf(run.ID, first.ID, 1, 0, 1, server.RedWins), "append in order"},
		{"outcome outside enum", gameOf(run.ID, first.ID, 0, 0, 1, server.Outcome(7)), "not a server outcome"},
	}
	for _, c := range cases {
		_, err := ts.AppendGame(ctx, c.g)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", c.name, err, c.want)
		}
	}
	if n := countRows(t, srv, `SELECT COUNT(*) FROM tournament_games`); n != 0 {
		t.Errorf("games after rejections = %d, want 0 (units rolled back)", n)
	}
	if s := mustSchedule(t, srv, run.ID)[0]; s.RedFirstWins != 0 || s.BlueFirstWins != 0 {
		t.Errorf("series after rejections = %+v, want untouched aggregates", s)
	}
}

// TestLeaderboardSeededRun pins the hand-computed fold of a partly played
// 3-bot run: the law's chained ratings, the W-L-D tallies, the series-won
// credit, and the rating-desc ordering.
func TestLeaderboardSeededRun(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, roster(3))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	schedule := mustSchedule(t, srv, run.ID)
	v01, v02 := schedule[0], schedule[1]
	if v01.RedFirstSlot != 0 || v01.BlueFirstSlot != 1 || v02.RedFirstSlot != 0 || v02.BlueFirstSlot != 2 {
		t.Fatalf("schedule seats = %+v %+v", v01, v02)
	}

	// Three games: 0 beats 1 (d=30, 1030/970), then 1 as red beats 0
	// (upset, d=31, 1001/999), then 0 vs 2 drawn (nothing moves).
	mustAppend(t, ts, gameOf(run.ID, v01.ID, 0, 0, 1, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, v01.ID, 1, 1, 0, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, v02.ID, 0, 0, 2, server.Draw))

	board := mustBoard(t, ts, run.ID)
	want := []Standings{
		{Slot: 1, Rating: 1001, Wins: 1, Losses: 1, GamesPlayed: 2},
		{Slot: 2, Rating: 1000, Draws: 1, GamesPlayed: 1},
		{Slot: 0, Rating: 999, Wins: 1, Losses: 1, Draws: 1, GamesPlayed: 3},
	}
	assertBoard(t, board, want)

	// The fourth game closes the 0-1 series 2-1: 0 as red beats 1 (d=30,
	// 1029/971) and takes the series-won credit.
	mustAppend(t, ts, gameOf(run.ID, v01.ID, 2, 0, 1, server.RedWins))
	board = mustBoard(t, ts, run.ID)
	want = []Standings{
		{Slot: 0, Rating: 1029, Wins: 2, Losses: 1, Draws: 1, SeriesWon: 1, GamesPlayed: 4},
		{Slot: 2, Rating: 1000, Draws: 1, GamesPlayed: 1},
		{Slot: 1, Rating: 971, Wins: 1, Losses: 2, GamesPlayed: 3},
	}
	assertBoard(t, board, want)
}

// TestRunCompletionSnapshot plays a full 2-bot run, closes it, and pins the
// frozen standings: the snapshot equals the live leaderboard, ratings stay
// zero-sum at 2x the start, and the close is a one-way door.
func TestRunCompletionSnapshot(t *testing.T) {
	ts, srv := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, roster(2))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	schedule := mustSchedule(t, srv, run.ID)

	if err := ts.FinishRun(ctx, run.ID, 123); err == nil {
		t.Fatalf("finish with unfinished series succeeded, want rejection")
	}
	// 2-0 then a billed loss for series 0; 1-1 then a decider for series 1.
	mustAppend(t, ts, gameOf(run.ID, schedule[0].ID, 0, 0, 1, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, schedule[0].ID, 1, 0, 1, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, schedule[0].ID, 2, 0, 1, server.BlueWins))
	mustAppend(t, ts, gameOf(run.ID, schedule[1].ID, 0, 1, 0, server.BlueWins))
	mustAppend(t, ts, gameOf(run.ID, schedule[1].ID, 1, 1, 0, server.RedWins))
	mustAppend(t, ts, gameOf(run.ID, schedule[1].ID, 2, 1, 0, server.RedWins))

	// Expected ratings chain the law directly, game by game.
	ratings := map[int]int{0: config.TournamentStartRating, 1: config.TournamentStartRating}
	for _, s := range []struct {
		red, blue int
		outcome   server.Outcome
	}{
		{0, 1, server.RedWins}, {0, 1, server.RedWins}, {0, 1, server.BlueWins},
		{1, 0, server.BlueWins}, {1, 0, server.RedWins}, {1, 0, server.RedWins},
	} {
		_, _, afterRed, afterBlue := server.RatingDeltas(ratings[s.red], ratings[s.blue], s.outcome)
		ratings[s.red], ratings[s.blue] = afterRed, afterBlue
	}
	if ratings[0]+ratings[1] != 2*config.TournamentStartRating {
		t.Fatalf("chained law lost zero-sum: %+v", ratings)
	}

	board := mustBoard(t, ts, run.ID)
	want := []Standings{
		{Slot: 1, Rating: ratings[1], Wins: 3, Losses: 3, SeriesWon: 1, GamesPlayed: 6},
		{Slot: 0, Rating: ratings[0], Wins: 3, Losses: 3, SeriesWon: 1, GamesPlayed: 6},
	}
	assertBoard(t, board, want)
	if board[0].Rating <= board[1].Rating {
		t.Fatalf("order = %+v, want rating desc to separate the pair", board)
	}

	if err := ts.FinishRun(ctx, run.ID, 456); err != nil {
		t.Fatalf("finish run: %v", err)
	}
	if err := ts.FinishRun(ctx, run.ID, 789); err == nil {
		t.Errorf("second finish accepted, want rejection")
	}
	if _, err := ts.AppendGame(ctx, gameOf(run.ID, schedule[0].ID, 3, 0, 1, server.Draw)); err == nil {
		t.Errorf("game after close accepted, want rejection")
	}

	snapshot := make(map[int]Standings)
	err = srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
		SELECT slot, rating, wins, losses, draws, series_won, games_played
		FROM tournament_standings WHERE run_id = ?`, run.ID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var st Standings
			if err := rows.Scan(&st.Slot, &st.Rating, &st.Wins, &st.Losses, &st.Draws, &st.SeriesWon, &st.GamesPlayed); err != nil {
				return err
			}
			snapshot[st.Slot] = st
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("snapshot rows = %d, want 2", len(snapshot))
	}
	for _, st := range board {
		if snapshot[st.Slot] != st {
			t.Errorf("snapshot[%d] = %+v, want the leaderboard's %+v", st.Slot, snapshot[st.Slot], st)
		}
	}
	var status string
	var finishedAt sql.NullInt64
	if err := srv.WithinTx(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT status, finished_at FROM tournament_runs WHERE id = ?`, run.ID).Scan(&status, &finishedAt)
	}); err != nil {
		t.Fatalf("read run: %v", err)
	}
	if status != RunStateFinished || !finishedAt.Valid || finishedAt.Int64 != 456 {
		t.Errorf("run after close = %s at %v, want finished at 456", status, finishedAt)
	}
}

func TestLeaderboardUnknownRun(t *testing.T) {
	ts, _ := newTestStore(t)
	if _, err := ts.Leaderboard(context.Background(), 999); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("leaderboard of unknown run = %v, want ErrNotFound", err)
	}
}

func mustBoard(t *testing.T, ts *Store, runID int64) []Standings {
	t.Helper()
	board, err := ts.Leaderboard(context.Background(), runID)
	if err != nil {
		t.Fatalf("leaderboard of run %d: %v", runID, err)
	}
	return board
}

func assertBoard(t *testing.T, got, want []Standings) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("board = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("board[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
