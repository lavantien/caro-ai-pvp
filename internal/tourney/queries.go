package tourney

// The run-page reads of the M7 UI: additive, read-only projections over the
// schema v3 tournament tables, kept beside the write surface they mirror.
// Nothing here feeds the conductor; the pages and the manager ride these.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// scanRun reads one run header row, the shared shape of Run and Runs.
func scanRun(scan func(...any) error) (Run, error) {
	var run Run
	var finished sql.NullInt64
	if err := scan(&run.ID, &run.CreatedAt, &run.TCIdx, &run.BOLen,
		&run.StartRating, &run.Status, &finished); err != nil {
		return Run{}, err
	}
	run.FinishedAt = nullInt64Ptr(finished)
	return run, nil
}

// Run reads one run header; an unknown id maps onto server.ErrNotFound.
func (t *Store) Run(ctx context.Context, runID int64) (Run, error) {
	var run Run
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		r, err := scanRun(tx.QueryRowContext(ctx, `
		SELECT id, created_at, tc_idx, bo_len, start_rating, status, finished_at
		FROM tournament_runs WHERE id = ?`, runID).Scan)
		run = r
		return err
	})
	if err != nil {
		return Run{}, notFound(err, fmt.Sprintf("run %d", runID))
	}
	return run, nil
}

// Runs lists run headers newest first, the setup page's past-runs section.
func (t *Store) Runs(ctx context.Context) ([]Run, error) {
	var out []Run
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
		SELECT id, created_at, tc_idx, bo_len, start_rating, status, finished_at
		FROM tournament_runs ORDER BY id DESC`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			run, err := scanRun(rows.Scan)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("tourney: list runs: %w", err)
	}
	return out, nil
}

// Roster reads one run's participants in slot order; an unknown run reads
// back empty, its header read is the page's not-found gate.
func (t *Store) Roster(ctx context.Context, runID int64) ([]Participant, error) {
	var out []Participant
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
		SELECT slot, name, tier FROM tournament_participants
		WHERE run_id = ? ORDER BY slot`, runID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p Participant
			if err := rows.Scan(&p.Slot, &p.Name, &p.Tier); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("tourney: roster of run %d: %w", runID, err)
	}
	return out, nil
}

// SeriesAll reads one run's series rows whole, in pairing order, with the
// settled aggregates Schedule leaves out.
func (t *Store) SeriesAll(ctx context.Context, runID int64) ([]Series, error) {
	var out []Series
	err := t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
		SELECT id, pairing_slot, red_first_slot, blue_first_slot, winner_slot,
			red_first_wins, blue_first_wins, created_at, finished_at
		FROM tournament_series WHERE run_id = ? ORDER BY pairing_slot`, runID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s Series
			var winner, finished sql.NullInt64
			if err := rows.Scan(&s.ID, &s.PairingSlot, &s.RedFirstSlot, &s.BlueFirstSlot,
				&winner, &s.RedFirstWins, &s.BlueFirstWins, &s.CreatedAt, &finished); err != nil {
				return err
			}
			s.WinnerSlot = nullIntPtr(winner)
			s.FinishedAt = nullInt64Ptr(finished)
			out = append(out, s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("tourney: series of run %d: %w", runID, err)
	}
	return out, nil
}

// OngoingRunID reports the oldest ongoing run row, the machine-wide run
// gate's holder; held is false when no run is ongoing.
func (t *Store) OngoingRunID(ctx context.Context) (id int64, held bool, err error) {
	err = t.srv.WithinTx(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT id FROM tournament_runs WHERE status = ? ORDER BY id LIMIT 1`,
			RunStateOngoing).Scan(&id)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("tourney: read the ongoing run: %w", err)
	}
	return id, true, nil
}

// nullInt64Ptr maps a nullable column onto the run and series rows' optional
// timestamps, nullIntPtr onto the series row's optional winner slot.
func nullInt64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func nullIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}
