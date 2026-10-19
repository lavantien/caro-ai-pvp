package tourney

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

const (
	RunStateOngoing  = "ongoing"
	RunStateFinished = "finished"
)

var outcomeTag = map[server.Outcome]string{
	server.Draw:     server.OutcomeDraw,
	server.RedWins:  server.OutcomeRed,
	server.BlueWins: server.OutcomeBlue,
}

type Run struct {
	ID          int64
	CreatedAt   int64
	TCIdx       int
	BOLen       int
	StartRating int
	Status      string
	FinishedAt  *int64
	Label       string
}

type Series struct {
	ID            int64
	RunID         int64
	PairingSlot   int
	RedFirstSlot  int
	BlueFirstSlot int
	WinnerSlot    *int
	RedFirstWins  int
	BlueFirstWins int
	CreatedAt     int64
	FinishedAt    *int64
}

type Game struct {
	ID          int64
	RunID       int64
	SeriesID    int64
	IdxInSeries int
	RedSlot     int
	BlueSlot    int
	Outcome     server.Outcome
	WonBy       *string
	FullTurns   int
	Moves       []rules.Move
	Stats       []server.MoveStat
	PlayedAt    int64
}

type Store struct {
	srv      *server.Store
	drivePid int
	fakePid  int
}

func NewStore(s *server.Store) *Store {
	return &Store{srv: s}
}

func (t *Store) pid() int {
	if t.fakePid != 0 {
		return t.fakePid
	}
	return os.Getpid()
}

func notFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("tourney: %s: %w", what, server.ErrNotFound)
	}
	return fmt.Errorf("tourney: %s: %w", what, err)
}

var ErrNotSeriesPair = errors.New("tourney: game seats are not the series' pair")

func sqlArg[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func (t *Store) CreateRun(ctx context.Context, tcIdx, boLen, startRating int, roster []Participant, label string) (Run, error) {
	if len(roster) < 2 {
		return Run{}, fmt.Errorf("tourney: roster holds %d participants, a pairing needs at least 2", len(roster))
	}
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return Run{}, fmt.Errorf("tourney: tc_idx %d outside the %d configured time controls", tcIdx, len(config.TimeControls))
	}
	if !slices.Contains(config.SeriesLengths[:], boLen) {
		return Run{}, fmt.Errorf("tourney: bo_len %d is not a configured series length", boLen)
	}
	if label == "" {
		return Run{}, errors.New("tourney: run label is empty, the log folder needs it")
	}
	if label == config.TournamentLegacyLabel {
		return Run{}, fmt.Errorf("tourney: run label %q is reserved for pre-v7 runs whose folder was never persisted", label)
	}
	for i, p := range roster {
		if p.Slot != i {
			return Run{}, fmt.Errorf("tourney: roster slot %d at position %d: slots must equal positions", p.Slot, i)
		}
		if p.Name == "" || p.Tier == "" {
			return Run{}, fmt.Errorf("tourney: roster slot %d: name and tier must be set", i)
		}
	}
	var run Run
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO tournament_runs (tc_idx, bo_len, start_rating, status, label)
			VALUES (?, ?, ?, ?, ?) RETURNING id, created_at`,
			tcIdx, boLen, startRating, RunStateOngoing, label,
		).Scan(&run.ID, &run.CreatedAt); err != nil {
			return fmt.Errorf("tourney: create run: %w", err)
		}
		for _, p := range roster {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tournament_participants (run_id, slot, name, tier) VALUES (?, ?, ?, ?)`,
				run.ID, p.Slot, p.Name, p.Tier); err != nil {
				return fmt.Errorf("tourney: create participant %d: %w", p.Slot, err)
			}
		}
		for slot, pair := range Pairings(roster) {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tournament_series (run_id, pairing_slot, red_first_slot, blue_first_slot)
				VALUES (?, ?, ?, ?)`,
				run.ID, slot, pair.RedFirst.Slot, pair.BlueFirst.Slot); err != nil {
				return fmt.Errorf("tourney: create series %d: %w", slot, err)
			}
		}
		return nil
	})
	if err != nil {
		return Run{}, err
	}
	run.TCIdx, run.BOLen, run.StartRating, run.Status, run.Label = tcIdx, boLen, startRating, RunStateOngoing, label
	return run, nil
}

func (t *Store) AppendGame(ctx context.Context, g Game) (Game, error) {
	tag, ok := outcomeTag[g.Outcome]
	if !ok {
		return Game{}, fmt.Errorf("tourney: game outcome %d is not a server outcome", int(g.Outcome))
	}
	if g.RedSlot == g.BlueSlot {
		return Game{}, fmt.Errorf("tourney: red and blue slot are both %d", g.RedSlot)
	}
	blob := server.EncodeMoves(nil, g.Moves)
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		var status string
		var drivePid int64
		if err := tx.QueryRowContext(ctx,
			`SELECT status, drive_pid FROM tournament_runs WHERE id = ?`, g.RunID,
		).Scan(&status, &drivePid); err != nil {
			return notFound(err, fmt.Sprintf("run %d", g.RunID))
		}
		if status != RunStateOngoing {
			return fmt.Errorf("tourney: run %d is %s: no further games", g.RunID, status)
		}
		if drivePid != int64(t.drivePid) {
			return fmt.Errorf("tourney: run %d is driven by pid %d, this drive is pid %d: the lease was taken over",
				g.RunID, drivePid, t.drivePid)
		}
		var boLen, redFirst, blueFirst int
		if err := tx.QueryRowContext(ctx,
			`SELECT r.bo_len, s.red_first_slot, s.blue_first_slot
			FROM tournament_series s JOIN tournament_runs r ON r.id = s.run_id
			WHERE s.id = ? AND s.run_id = ?`, g.SeriesID, g.RunID,
		).Scan(&boLen, &redFirst, &blueFirst); err != nil {
			return notFound(err, fmt.Sprintf("series %d of run %d", g.SeriesID, g.RunID))
		}
		for _, slot := range [2]int{g.RedSlot, g.BlueSlot} {
			if err := tx.QueryRowContext(ctx,
				`SELECT 1 FROM tournament_participants WHERE run_id = ? AND slot = ?`, g.RunID, slot,
			).Scan(new(int)); err != nil {
				return fmt.Errorf("tourney: slot %d is not a participant of run %d: %w", slot, g.RunID, err)
			}
		}
		if (g.RedSlot != redFirst || g.BlueSlot != blueFirst) &&
			(g.RedSlot != blueFirst || g.BlueSlot != redFirst) {
			return fmt.Errorf("tourney: series %d pairs %d with %d, game seats (%d, %d): %w",
				g.SeriesID, redFirst, blueFirst, g.RedSlot, g.BlueSlot, ErrNotSeriesPair)
		}
		var played int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tournament_games WHERE series_id = ?`, g.SeriesID,
		).Scan(&played); err != nil {
			return fmt.Errorf("tourney: count games of series %d: %w", g.SeriesID, err)
		}
		if g.IdxInSeries != played {
			return fmt.Errorf("tourney: game idx %d but series %d holds %d games: append in order", g.IdxInSeries, g.SeriesID, played)
		}
		if played >= boLen {
			return fmt.Errorf("tourney: series %d already played its %d games", g.SeriesID, boLen)
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO tournament_games (run_id, series_id, idx_in_series, red_slot, blue_slot, outcome, won_by, full_turns, moves)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, played_at`,
			g.RunID, g.SeriesID, g.IdxInSeries, g.RedSlot, g.BlueSlot, tag, sqlArg(g.WonBy), g.FullTurns, blob,
		).Scan(&g.ID, &g.PlayedAt); err != nil {
			return fmt.Errorf("tourney: append game: %w", err)
		}
		for i, st := range g.Stats {
			mover := "red"
			if st.Side == rules.Blue {
				mover = "blue"
			}
			var pv strings.Builder
			for j := range st.Stats.PVLen {
				if j > 0 {
					pv.WriteByte(' ')
				}
				name, nerr := rules.CellName(rules.Cell(st.Stats.PV[j]))
				if nerr != nil {
					return fmt.Errorf("tourney: m-line %d pv cell %d of game %d: %w", i+1, st.Stats.PV[j], g.ID, nerr)
				}
				pv.WriteString(name)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tournament_game_stats (tournament_game_id, move_no, mover, move, depth, nodes, nps, ebf_milli, tt_permille, hf_permille, fh1_permille, score, threads, elapsed_ns, alloc_ns, tag, pv)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				g.ID, i+1, mover, int(st.Move), st.Stats.Depth, st.Stats.Nodes, st.Stats.Nps, st.Stats.EBFMilli,
				st.Stats.TTHitPermille, st.Stats.HashFullPermille, st.Stats.FirstMoveFailHighPermille,
				st.Stats.Score, st.Stats.Threads, st.Stats.ElapsedNs, st.Stats.AllocNs, st.TagName(), pv.String(),
			); err != nil {
				return fmt.Errorf("tourney: append m-line %d of game %d: %w", i+1, g.ID, err)
			}
		}
		redWins, blueWins, total, err := seriesWins(ctx, tx, g.SeriesID, redFirst, blueFirst)
		if err != nil {
			return err
		}
		winner, finished := settleSeries(boLen, redFirst, blueFirst, redWins, blueWins, total)
		var finishedAt any
		if finished {
			finishedAt = g.PlayedAt
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tournament_series SET red_first_wins = ?, blue_first_wins = ?, winner_slot = ?, finished_at = ?
			WHERE id = ?`,
			redWins, blueWins, sqlArg(winner), finishedAt, g.SeriesID,
		); err != nil {
			return fmt.Errorf("tourney: settle series %d: %w", g.SeriesID, err)
		}
		return nil
	})
	if err != nil {
		return Game{}, err
	}
	return g, nil
}

func (t *Store) ResetSeries(ctx context.Context, seriesID int64) error {
	return t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		var runID sql.NullInt64
		var finished sql.NullInt64
		var status string
		var drivePid int64
		if err := tx.QueryRowContext(ctx,
			`SELECT s.run_id, s.finished_at, r.status, r.drive_pid
			FROM tournament_series s JOIN tournament_runs r ON r.id = s.run_id
			WHERE s.id = ?`, seriesID,
		).Scan(&runID, &finished, &status, &drivePid); err != nil {
			return notFound(err, fmt.Sprintf("series %d", seriesID))
		}
		if finished.Valid {
			return fmt.Errorf("tourney: series %d is settled, its record is the run's evidence", seriesID)
		}
		if status != RunStateOngoing {
			return fmt.Errorf("tourney: run %d is %s: no further games", runID.Int64, status)
		}
		if drivePid != int64(t.drivePid) {
			return fmt.Errorf("tourney: run %d is driven by pid %d, this drive is pid %d: the lease was taken over",
				runID.Int64, drivePid, t.drivePid)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM tournament_game_stats WHERE tournament_game_id IN
			(SELECT id FROM tournament_games WHERE series_id = ?)`, seriesID); err != nil {
			return fmt.Errorf("tourney: scrub m-lines of series %d: %w", seriesID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM tournament_games WHERE series_id = ?`, seriesID); err != nil {
			return fmt.Errorf("tourney: scrub games of series %d: %w", seriesID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tournament_series SET red_first_wins = 0, blue_first_wins = 0, winner_slot = NULL, finished_at = NULL
			WHERE id = ?`, seriesID); err != nil {
			return fmt.Errorf("tourney: reset series %d: %w", seriesID, err)
		}
		return nil
	})
}

func seriesWins(ctx context.Context, tx *sql.Tx, seriesID int64, redFirstSlot, blueFirstSlot int) (red, blue, total int, err error) {
	err = tx.QueryRowContext(ctx, `
	SELECT
		COALESCE(SUM(CASE WHEN (outcome = ? AND red_slot = ?) OR (outcome = ? AND blue_slot = ?) THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN (outcome = ? AND red_slot = ?) OR (outcome = ? AND blue_slot = ?) THEN 1 ELSE 0 END), 0),
		COUNT(*)
	FROM tournament_games WHERE series_id = ?`,
		server.OutcomeRed, redFirstSlot, server.OutcomeBlue, redFirstSlot,
		server.OutcomeRed, blueFirstSlot, server.OutcomeBlue, blueFirstSlot, seriesID,
	).Scan(&red, &blue, &total)
	if err != nil {
		err = fmt.Errorf("tourney: recompute series %d wins: %w", seriesID, err)
	}
	return
}

func (t *Store) FinishRun(ctx context.Context, runID int64, finishedAt int64) error {
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		var status string
		var drivePid int64
		if err := tx.QueryRowContext(ctx,
			`SELECT status, drive_pid FROM tournament_runs WHERE id = ?`, runID,
		).Scan(&status, &drivePid); err != nil {
			return notFound(err, fmt.Sprintf("run %d", runID))
		}
		if status != RunStateOngoing {
			return fmt.Errorf("tourney: run %d is already %s", runID, status)
		}
		if drivePid != int64(t.drivePid) {
			return fmt.Errorf("tourney: run %d is driven by pid %d, this drive is pid %d: the lease was taken over",
				runID, drivePid, t.drivePid)
		}
		var unfinished int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tournament_series WHERE run_id = ? AND finished_at IS NULL`, runID,
		).Scan(&unfinished); err != nil {
			return fmt.Errorf("tourney: count unfinished series of run %d: %w", runID, err)
		}
		if unfinished != 0 {
			return fmt.Errorf("tourney: run %d still has %d unfinished series", runID, unfinished)
		}
		board, err := leaderboardIn(ctx, tx, runID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tournament_runs SET status = ?, finished_at = ?, drive_pid = 0, drive_heartbeat = 0 WHERE id = ?`,
			RunStateFinished, finishedAt, runID); err != nil {
			return fmt.Errorf("tourney: finish run %d: %w", runID, err)
		}
		for _, st := range board {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tournament_standings (run_id, slot, rating, wins, losses, draws, series_won, games_played)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				runID, st.Slot, st.Rating, st.Wins, st.Losses, st.Draws, st.SeriesWon, st.GamesPlayed); err != nil {
				return fmt.Errorf("tourney: snapshot slot %d: %w", st.Slot, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	t.drivePid = 0
	return nil
}

func (t *Store) CloseStalledRun(ctx context.Context, runID int64, finishedAt int64) error {
	return t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		var pid, beat int64
		if err := tx.QueryRowContext(ctx,
			`SELECT drive_pid, drive_heartbeat FROM tournament_runs WHERE id = ?`, runID,
		).Scan(&pid, &beat); err != nil {
			return notFound(err, fmt.Sprintf("run %d", runID))
		}
		if pid != 0 && beat > finishedAt-config.TournamentDriveStaleSec {
			return fmt.Errorf("tourney: run %d is driven by pid %d (last beat %ds ago): a live drive owns its finish",
				runID, pid, finishedAt-beat)
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE tournament_runs SET status = ?, finished_at = ?, drive_pid = 0, drive_heartbeat = 0
			WHERE id = ? AND status = ?`,
			RunStateFinished, finishedAt, runID, RunStateOngoing)
		if err != nil {
			return fmt.Errorf("tourney: close stalled run %d: %w", runID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("tourney: close stalled run %d: %w", runID, err)
		}
		if n == 0 {
			return fmt.Errorf("tourney: run %d is no longer ongoing", runID)
		}
		return nil
	})
}

func (t *Store) ClaimDrive(ctx context.Context, runID, now int64) error {
	pid := int64(t.pid())
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		var status string
		var held, beat int64
		if err := tx.QueryRowContext(ctx,
			`SELECT status, drive_pid, drive_heartbeat FROM tournament_runs WHERE id = ?`, runID,
		).Scan(&status, &held, &beat); err != nil {
			return notFound(err, fmt.Sprintf("run %d", runID))
		}
		if status != RunStateOngoing {
			return fmt.Errorf("tourney: run %d is already %s", runID, status)
		}
		if held != 0 && beat > now-config.TournamentDriveStaleSec {
			return fmt.Errorf("tourney: run %d is driven by pid %d (last beat %ds ago): wait out the stale window or close the run",
				runID, held, now-beat)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tournament_runs SET drive_pid = ?, drive_heartbeat = ? WHERE id = ?`,
			pid, now, runID); err != nil {
			return fmt.Errorf("tourney: claim run %d: %w", runID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	t.drivePid = int(pid)
	return nil
}

func (t *Store) BeatDrive(ctx context.Context, runID, now int64) (alive bool, err error) {
	err = t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE tournament_runs SET drive_heartbeat = ? WHERE id = ? AND drive_pid = ? AND status = ?`,
			now, runID, t.drivePid, RunStateOngoing)
		if err != nil {
			return fmt.Errorf("tourney: beat run %d: %w", runID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("tourney: beat run %d: %w", runID, err)
		}
		alive = n == 1
		return nil
	})
	return alive, err
}

func (t *Store) ReleaseDrive(ctx context.Context, runID int64) error {
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE tournament_runs SET drive_pid = 0, drive_heartbeat = 0 WHERE id = ? AND drive_pid = ?`,
			runID, t.drivePid); err != nil {
			return fmt.Errorf("tourney: release run %d: %w", runID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	t.drivePid = 0
	return nil
}

func (t *Store) Schedule(ctx context.Context, runID int64) ([]Series, error) {
	var out []Series
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
		SELECT id, pairing_slot, red_first_slot, blue_first_slot, winner_slot,
			red_first_wins, blue_first_wins, finished_at
		FROM tournament_series WHERE run_id = ? ORDER BY pairing_slot`, runID)
		if err != nil {
			return fmt.Errorf("tourney: read schedule of run %d: %w", runID, err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s Series
			var winner, finished sql.NullInt64
			if err := rows.Scan(&s.ID, &s.PairingSlot, &s.RedFirstSlot, &s.BlueFirstSlot,
				&winner, &s.RedFirstWins, &s.BlueFirstWins, &finished); err != nil {
				return fmt.Errorf("tourney: scan schedule row of run %d: %w", runID, err)
			}
			s.WinnerSlot = nullIntPtr(winner)
			s.FinishedAt = nullInt64Ptr(finished)
			out = append(out, s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (t *Store) Leaderboard(ctx context.Context, runID int64) ([]Standings, error) {
	var board []Standings
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		b, err := leaderboardIn(ctx, tx, runID)
		board = b
		return err
	})
	return board, err
}

func leaderboardIn(ctx context.Context, tx *sql.Tx, runID int64) ([]Standings, error) {
	var start int
	if err := tx.QueryRowContext(ctx,
		`SELECT start_rating FROM tournament_runs WHERE id = ?`, runID,
	).Scan(&start); err != nil {
		return nil, notFound(err, fmt.Sprintf("run %d", runID))
	}
	slots, err := slotsOf(ctx, tx, runID)
	if err != nil {
		return nil, err
	}
	games, err := gamesOf(ctx, tx, runID)
	if err != nil {
		return nil, err
	}
	winners, err := seriesWinnersOf(ctx, tx, runID)
	if err != nil {
		return nil, err
	}
	return leaderboard(foldStandings(start, slots, games, winners)), nil
}

func slotsOf(ctx context.Context, tx *sql.Tx, runID int64) ([]int, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT slot FROM tournament_participants WHERE run_id = ? ORDER BY slot`, runID)
	if err != nil {
		return nil, fmt.Errorf("tourney: read participants of run %d: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()
	var slots []int
	for rows.Next() {
		var s int
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("tourney: scan participant of run %d: %w", runID, err)
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

func gamesOf(ctx context.Context, tx *sql.Tx, runID int64) ([]foldedGame, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT red_slot, blue_slot, outcome FROM tournament_games WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("tourney: read games of run %d: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()
	var games []foldedGame
	for rows.Next() {
		var g foldedGame
		var tag string
		if err := rows.Scan(&g.RedSlot, &g.BlueSlot, &tag); err != nil {
			return nil, fmt.Errorf("tourney: scan game of run %d: %w", runID, err)
		}
		outcome, ok := outcomeOf(tag)
		if !ok {
			return nil, fmt.Errorf("tourney: game of run %d carries outcome %q", runID, tag)
		}
		g.Outcome = outcome
		games = append(games, g)
	}
	return games, rows.Err()
}

func outcomeOf(tag string) (server.Outcome, bool) {
	for outcome, t := range outcomeTag {
		if t == tag {
			return outcome, true
		}
	}
	return 0, false
}

func seriesWinnersOf(ctx context.Context, tx *sql.Tx, runID int64) ([]int, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT winner_slot FROM tournament_series WHERE run_id = ? AND winner_slot IS NOT NULL`, runID)
	if err != nil {
		return nil, fmt.Errorf("tourney: read series winners of run %d: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()
	var winners []int
	for rows.Next() {
		var w int
		if err := rows.Scan(&w); err != nil {
			return nil, fmt.Errorf("tourney: scan series winner of run %d: %w", runID, err)
		}
		winners = append(winners, w)
	}
	return winners, rows.Err()
}
