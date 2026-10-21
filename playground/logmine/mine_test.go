package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func fixtureDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logmine-fixture.db")
	st, err := server.Open(path)
	if err != nil {
		t.Fatalf("open fixture store: %v", err)
	}
	defer func() {
		if st != nil {
			_ = st.Close()
		}
	}()
	if _, _, err := st.CreateUserIfAbsent("alice", []byte{1}, []byte{1}); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	ctx := context.Background()
	var botHard, alice int64
	err = st.WithinTx(ctx, func(tx *sql.Tx) error {
		for _, row := range []struct {
			id   *int64
			name string
		}{{&botHard, "AI hard"}, {&alice, "alice"}} {
			if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, row.name).Scan(row.id); err != nil {
				return fmt.Errorf("user %s: %w", row.name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO series (id, tc_idx, bo_len, red_user, blue_user, state, winner) VALUES (1, 1, 1, ?, ?, 'finished', ?)`, botHard, alice, botHard); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO games (id, series_id, idx_in_series, red_user, blue_user, outcome, moves, full_turns, won_by) VALUES (1, 1, 1, ?, ?, 'red', X'0102', 2, 'five')`, botHard, alice); err != nil {
			return err
		}
		for _, row := range []struct {
			no   int
			line string
		}{
			{1, "M1, Red, H8, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=2, t=0.03, alloc=3.00, [VCT], pv=H8 G8"},
			{2, "M2, Blue, H9, d=8, n=40k, nps=1.0m, ebf=1.3, tt=20%, hf=55%, fh1=85%, s=-10, thr=2, t=1.00, alloc=3.00, pv=H9 G9"},
		} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO game_stats (game_id, move_no, line) VALUES (1, ?, ?)`, row.no, row.line); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tournament_runs (id, tc_idx, bo_len, start_rating, status) VALUES (1, 0, 1, 1000, 'finished')`); err != nil {
			return err
		}
		for _, row := range []string{
			`INSERT INTO tournament_participants (run_id, slot, name, tier) VALUES (1, 1, 'e1', 'easy')`,
			`INSERT INTO tournament_participants (run_id, slot, name, tier) VALUES (1, 2, 'h1', 'hard')`,
			`INSERT INTO tournament_series (id, run_id, pairing_slot, red_first_slot, blue_first_slot, winner_slot, red_first_wins, blue_first_wins) VALUES (1, 1, 0, 1, 2, 1, 3, 1)`,
			`INSERT INTO tournament_games (id, run_id, series_id, idx_in_series, red_slot, blue_slot, outcome, full_turns, moves) VALUES (1, 1, 1, 0, 1, 2, 'red', 30, X'0102')`,
			`INSERT INTO tournament_games (id, run_id, series_id, idx_in_series, red_slot, blue_slot, outcome, full_turns, moves) VALUES (2, 1, 1, 1, 2, 1, 'blue', 28, X'0103')`,
			`INSERT INTO tournament_games (id, run_id, series_id, idx_in_series, red_slot, blue_slot, outcome, full_turns, moves) VALUES (3, 1, 1, 2, 1, 2, 'draw', 30, X'0104')`,
		} {
			if _, err := tx.ExecContext(ctx, row); err != nil {
				return fmt.Errorf("row %q: %w", row, err)
			}
		}
		rows := []struct {
			game int64
			no   int
			mvr  string
			d    int
			n    uint64
			nps  uint64
			ebf  int
			tt   int
			hf   int
			fh1  int
			sc   int
			thr  int
			el   int64
			al   int64
			tag  string
		}{
			{1, 1, "red", 10, 100_000, 1_000_000, 2000, 380, 450, 930, 150, 1, 1_500_000_000, 2_000_000_000, ""},
			{1, 2, "blue", 16, 5_000_000, 2_500_000, 2100, 440, 780, 910, -25, 4, 1_900_000_000, 12_000_000_000, ""},
			{2, 1, "red", 16, 6_000_000, 2_500_000, 2100, 460, 790, 920, 25, 4, 14_000_000_000, 12_000_000_000, "VCT"},
		}
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO tournament_game_stats
				(tournament_game_id, move_no, mover, move, depth, nodes, nps, ebf_milli, tt_permille, hf_permille, fh1_permille, score, threads, elapsed_ns, alloc_ns, tag, pv)
				VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'H8')`,
				r.game, r.no, r.mvr, r.d, r.n, r.nps, r.ebf, r.tt, r.hf, r.fh1, r.sc, r.thr, r.el, r.al, r.tag); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	st = nil
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestLoadTournamentMoves(t *testing.T) {
	rows, err := loadTournamentMoves(fixtureDB(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	wantTiers := []string{"easy", "hard", "hard"}
	wantDepths := []int{10, 16, 16}
	wantTags := []string{"", "", "VCT"}
	for i, r := range rows {
		if r.tier != wantTiers[i] || r.depth != wantDepths[i] || r.tag != wantTags[i] || r.tc != "1+0" {
			t.Errorf("row %d = tier %q tc %s depth %d tag %q, want %q 1+0 %d %q", i, r.tier, r.tc, r.depth, r.tag, wantTiers[i], wantDepths[i], wantTags[i])
		}
	}
}

func TestLoadTournamentMovesOnEmptyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	st, err := server.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = st.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	rows, err := loadTournamentMoves(db)
	if err != nil || len(rows) != 0 {
		t.Errorf("empty db = %d rows err %v, want 0 nil", len(rows), err)
	}
}

func TestTierTelemetryAggregates(t *testing.T) {
	rows, err := loadTournamentMoves(fixtureDB(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	tele := tierTelemetry(rows)
	easy, ok := tele[telemetryKey{tier: "easy", tc: "1+0"}]
	if !ok {
		t.Fatal("easy tier missing")
	}
	if easy.moves != 1 || easy.games != 1 || easy.depth.q1 != 10 || easy.depth.med != 10 || easy.depth.q3 != 10 {
		t.Errorf("easy = %+v", easy)
	}
	if easy.nodes.med != 100_000 || easy.nps.med != 1_000_000 || easy.tt.med != 380 || easy.hf.med != 450 || easy.fh1.med != 930 {
		t.Errorf("easy metrics = %+v", easy)
	}
	hard, ok := tele[telemetryKey{tier: "hard", tc: "1+0"}]
	if !ok {
		t.Fatal("hard tier missing")
	}
	if hard.moves != 2 || hard.games != 2 || hard.depth.q1 != 16 || hard.depth.med != 16 || hard.depth.q3 != 16 {
		t.Errorf("hard = %+v", hard)
	}
}

func TestGrantDepthBuckets(t *testing.T) {
	rows, err := loadTournamentMoves(fixtureDB(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	gd := grantDepth(rows)
	if len(gd) != 2 {
		t.Fatalf("buckets = %d, want 2: %+v", len(gd), gd)
	}
	easy, ok := gd[grantKey{tier: "easy", bucket: "2-5"}]
	if !ok || easy.moves != 1 || easy.depth.med != 10 {
		t.Errorf("easy 2-5s = %+v ok %t", easy, ok)
	}
	hard, ok := gd[grantKey{tier: "hard", bucket: "10-20"}]
	if !ok || hard.moves != 2 || hard.depth.med != 16 {
		t.Errorf("hard 10-20s = %+v ok %t", hard, ok)
	}
	if allocBucketLabel(400_000_000) != "<0.5" || allocBucketLabel(900_000_000) != "0.5-1" ||
		allocBucketLabel(1_500_000_000) != "1-2" || allocBucketLabel(4_000_000_000) != "2-5" ||
		allocBucketLabel(8_000_000_000) != "5-10" || allocBucketLabel(25_000_000_000) != ">=20" {
		t.Error("bucket edges mislabeled")
	}
}

func TestRedFirstAndOutcomes(t *testing.T) {
	db := fixtureDB(t)
	wins, err := loadSeriesFirstWins(db)
	if err != nil {
		t.Fatalf("series wins: %v", err)
	}
	if len(wins) != 1 {
		t.Fatalf("series rows = %d, want 1", len(wins))
	}
	w := wins[0]
	if w.tc != "1+0" || w.firstTier != "easy" || w.secondTier != "hard" || w.firstWins != 3 || w.secondWins != 1 {
		t.Errorf("series row = %+v", w)
	}
	outs, err := loadGameOutcomes(db)
	if err != nil {
		t.Fatalf("outcomes: %v", err)
	}
	want := map[outcomeKey]int{
		{tc: "1+0", redTier: "easy", blueTier: "hard", outcome: "red"}:  1,
		{tc: "1+0", redTier: "hard", blueTier: "easy", outcome: "blue"}: 1,
		{tc: "1+0", redTier: "easy", blueTier: "hard", outcome: "draw"}: 1,
	}
	if len(outs) != len(want) {
		t.Fatalf("outcome rows = %d, want %d: %+v", len(outs), len(want), outs)
	}
	for _, o := range outs {
		if want[o.key] != o.count {
			t.Errorf("outcome %+v = %d, want %d", o.key, o.count, want[o.key])
		}
	}
}

func TestLoadCasualMoves(t *testing.T) {
	rows, rejects, err := loadCasualMoves(fixtureDB(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rejects != 0 {
		t.Errorf("rejects = %d, want 0", rejects)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].tier != "hard" || rows[0].tc != "2+1" || rows[0].depth != 9 || rows[0].nodes != 45_000 || rows[0].tag != "[VCT]" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].tier != "human" || rows[1].tc != "2+1" || rows[1].depth != 8 || rows[1].nodes != 40_000 || rows[1].tag != "" {
		t.Errorf("row 1 = %+v", rows[1])
	}
}

func TestGrantUsageTails(t *testing.T) {
	rows, err := loadTournamentMoves(fixtureDB(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	u := grantUsage(rows)
	hard, ok := u[telemetryKey{tier: "hard", tc: "1+0"}]
	if !ok {
		t.Fatal("hard usage missing")
	}
	if hard.moves != 2 || hard.used.med < 0.66 || hard.used.med > 0.67 {
		t.Errorf("hard usage = %+v", hard)
	}
	if hard.under10 != 0 || hard.over100 != 1 {
		t.Errorf("hard tails = under %d over %d, want 0 1", hard.under10, hard.over100)
	}
}

func TestQuantilesOddAndEven(t *testing.T) {
	odd := quantiles([]float64{3, 1, 2})
	if odd.q1 != 1.5 || odd.med != 2 || odd.q3 != 2.5 {
		t.Errorf("odd = %+v", odd)
	}
	even := quantiles([]float64{4, 1, 2, 3})
	if even.q1 != 1.75 || even.med != 2.5 || even.q3 != 3.25 {
		t.Errorf("even = %+v", even)
	}
	one := quantiles([]float64{7})
	if one.q1 != 7 || one.med != 7 || one.q3 != 7 {
		t.Errorf("one = %+v", one)
	}
}
