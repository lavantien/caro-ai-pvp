package main

import (
	"database/sql"
	"fmt"
	"sort"
)

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type moveRow struct {
	tier        string
	tc          int
	gameID      int64
	depth       int
	nodes       uint64
	nps         uint64
	ebfMilli    int
	tt, hf, fh1 int
	score       int
	threads     int
	elapsedNS   int64
	allocNS     int64
	tag         string
}

type firstWins struct {
	tc                    int
	firstTier, secondTier string
	firstWins, secondWins int
}

type outcomeKey struct {
	tc                int
	redTier, blueTier string
	outcome           string
}

type outcomeRow struct {
	key   outcomeKey
	count int
}

type telemetryKey struct {
	tier string
	tc   int
}

type grantKey struct {
	tier   string
	bucket string
}

type spread struct {
	q1, med, q3 float64
}

type tierAgg struct {
	games int
	moves int
	depth spread
	nodes spread
	nps   spread
	tt    spread
	hf    spread
	fh1   spread
	ebf   spread
}

type grantAgg struct {
	moves int
	depth spread
}

type usageAgg struct {
	moves   int
	used    spread
	under10 int
	over100 int
}

var allocBucketEdges = []struct {
	maxSec float64
	label  string
}{
	{0.5, "<0.5"}, {1, "0.5-1"}, {2, "1-2"}, {5, "2-5"}, {10, "5-10"}, {20, "10-20"}, {0, ">=20"},
}

func allocBucketLabel(allocNS int64) string {
	sec := float64(allocNS) / 1e9
	for _, e := range allocBucketEdges {
		if e.maxSec != 0 && sec < e.maxSec {
			return e.label
		}
	}
	return ">=20"
}

func quantiles(xs []float64) spread {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	at := func(q float64) float64 {
		if len(s) == 0 {
			return 0
		}
		pos := q * float64(len(s)-1)
		lo := int(pos)
		hi := lo + 1
		if hi >= len(s) {
			return s[lo]
		}
		return s[lo] + (pos-float64(lo))*(s[hi]-s[lo])
	}
	return spread{q1: at(0.25), med: at(0.5), q3: at(0.75)}
}

func tableExists(q querier, name string) bool {
	rows, err := q.Query(`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?`, name)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

func loadTournamentMoves(q querier) ([]moveRow, error) {
	if !tableExists(q, "tournament_game_stats") {
		return nil, nil
	}
	rows, err := q.Query(`
		SELECT p.tier, r.tc_idx, g.id, s.depth, s.nodes, s.nps, s.ebf_milli,
			s.tt_permille, s.hf_permille, s.fh1_permille, s.score, s.threads,
			s.elapsed_ns, s.alloc_ns, s.tag
		FROM tournament_game_stats s
		JOIN tournament_games g ON g.id = s.tournament_game_id
		JOIN tournament_runs r ON r.id = g.run_id
		JOIN tournament_participants p ON p.run_id = g.run_id
			AND p.slot = CASE WHEN s.mover = 'red' THEN g.red_slot ELSE g.blue_slot END
		ORDER BY s.tournament_game_id, s.move_no`)
	if err != nil {
		return nil, fmt.Errorf("logmine: tournament moves: %w", err)
	}
	defer rows.Close()
	var out []moveRow
	for rows.Next() {
		var r moveRow
		if err := rows.Scan(&r.tier, &r.tc, &r.gameID, &r.depth, &r.nodes, &r.nps, &r.ebfMilli,
			&r.tt, &r.hf, &r.fh1, &r.score, &r.threads, &r.elapsedNS, &r.allocNS, &r.tag); err != nil {
			return nil, fmt.Errorf("logmine: tournament move scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadSeriesFirstWins(q querier) ([]firstWins, error) {
	if !tableExists(q, "tournament_series") {
		return nil, nil
	}
	rows, err := q.Query(`
		SELECT r.tc_idx, pf.tier, ps.tier, t.red_first_wins, t.blue_first_wins
		FROM tournament_series t
		JOIN tournament_runs r ON r.id = t.run_id
		JOIN tournament_participants pf ON pf.run_id = t.run_id AND pf.slot = t.red_first_slot
		JOIN tournament_participants ps ON ps.run_id = t.run_id AND ps.slot = t.blue_first_slot
		ORDER BY r.tc_idx, pf.tier, ps.tier`)
	if err != nil {
		return nil, fmt.Errorf("logmine: series first wins: %w", err)
	}
	defer rows.Close()
	var out []firstWins
	for rows.Next() {
		var f firstWins
		if err := rows.Scan(&f.tc, &f.firstTier, &f.secondTier, &f.firstWins, &f.secondWins); err != nil {
			return nil, fmt.Errorf("logmine: series first wins scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func loadGameOutcomes(q querier) ([]outcomeRow, error) {
	if !tableExists(q, "tournament_games") {
		return nil, nil
	}
	rows, err := q.Query(`
		SELECT r.tc_idx, pr.tier, pb.tier, g.outcome, COUNT(*)
		FROM tournament_games g
		JOIN tournament_runs r ON r.id = g.run_id
		JOIN tournament_participants pr ON pr.run_id = g.run_id AND pr.slot = g.red_slot
		JOIN tournament_participants pb ON pb.run_id = g.run_id AND pb.slot = g.blue_slot
		GROUP BY r.tc_idx, pr.tier, pb.tier, g.outcome
		ORDER BY r.tc_idx, pr.tier, pb.tier, g.outcome`)
	if err != nil {
		return nil, fmt.Errorf("logmine: game outcomes: %w", err)
	}
	defer rows.Close()
	var out []outcomeRow
	for rows.Next() {
		var o outcomeRow
		if err := rows.Scan(&o.key.tc, &o.key.redTier, &o.key.blueTier, &o.key.outcome, &o.count); err != nil {
			return nil, fmt.Errorf("logmine: game outcome scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func seatTier(username string) string {
	if len(username) > 3 && username[:3] == "AI " {
		return username[3:]
	}
	return "human"
}

func loadCasualMoves(q querier) ([]moveRow, int, error) {
	if !tableExists(q, "game_stats") {
		return nil, 0, nil
	}
	rows, err := q.Query(`
		SELECT ur.username, ub.username, s.tc_idx, g.id, st.line
		FROM game_stats st
		JOIN games g ON g.id = st.game_id
		JOIN series s ON s.id = g.series_id
		JOIN users ur ON ur.id = g.red_user
		JOIN users ub ON ub.id = g.blue_user
		ORDER BY st.game_id, st.move_no`)
	if err != nil {
		return nil, 0, fmt.Errorf("logmine: casual moves: %w", err)
	}
	defer rows.Close()
	var out []moveRow
	rejects := 0
	for rows.Next() {
		var redName, blueName, line string
		var tc int
		var gameID int64
		if err := rows.Scan(&redName, &blueName, &tc, &gameID, &line); err != nil {
			return nil, 0, fmt.Errorf("logmine: casual scan: %w", err)
		}
		ml, err := parseMLine(line)
		if err != nil {
			rejects++
			continue
		}
		r := moveRow{
			tc: tc, gameID: gameID, depth: ml.depth, nodes: ml.nodes, nps: ml.nps,
			ebfMilli: int(ml.ebf * 1000), tt: ml.tt * 10, hf: ml.hf * 10, fh1: ml.fh1 * 10,
			threads: ml.thr, tag: ml.tag,
		}
		if ml.side == "Red" {
			r.tier = seatTier(redName)
		} else {
			r.tier = seatTier(blueName)
		}
		r.elapsedNS = int64(ml.t * 1e9)
		r.allocNS = int64(ml.alloc * 1e9)
		out = append(out, r)
	}
	return out, rejects, rows.Err()
}

func tierTelemetry(rows []moveRow) map[telemetryKey]tierAgg {
	byKey := map[telemetryKey][]moveRow{}
	for _, r := range rows {
		byKey[telemetryKey{tier: r.tier, tc: r.tc}] = append(byKey[telemetryKey{tier: r.tier, tc: r.tc}], r)
	}
	out := map[telemetryKey]tierAgg{}
	for k, rs := range byKey {
		games := map[int64]bool{}
		depths, nodes, nps, tts, hfs, fh1s, ebfs := []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}
		for _, r := range rs {
			games[r.gameID] = true
			depths = append(depths, float64(r.depth))
			nodes = append(nodes, float64(r.nodes))
			nps = append(nps, float64(r.nps))
			tts = append(tts, float64(r.tt))
			hfs = append(hfs, float64(r.hf))
			fh1s = append(fh1s, float64(r.fh1))
			ebfs = append(ebfs, float64(r.ebfMilli))
		}
		out[k] = tierAgg{
			games: len(games), moves: len(rs),
			depth: quantiles(depths), nodes: quantiles(nodes), nps: quantiles(nps),
			tt: quantiles(tts), hf: quantiles(hfs), fh1: quantiles(fh1s), ebf: quantiles(ebfs),
		}
	}
	return out
}

func grantDepth(rows []moveRow) map[grantKey]grantAgg {
	byKey := map[grantKey][]float64{}
	for _, r := range rows {
		k := grantKey{tier: r.tier, bucket: allocBucketLabel(r.allocNS)}
		byKey[k] = append(byKey[k], float64(r.depth))
	}
	out := map[grantKey]grantAgg{}
	for k, ds := range byKey {
		out[k] = grantAgg{moves: len(ds), depth: quantiles(ds)}
	}
	return out
}

func grantUsage(rows []moveRow) map[telemetryKey]usageAgg {
	byKey := map[telemetryKey][]float64{}
	under := map[telemetryKey]int{}
	over := map[telemetryKey]int{}
	for _, r := range rows {
		if r.allocNS <= 0 {
			continue
		}
		k := telemetryKey{tier: r.tier, tc: r.tc}
		frac := float64(r.elapsedNS) / float64(r.allocNS)
		byKey[k] = append(byKey[k], frac)
		if frac < 0.10 {
			under[k]++
		}
		if frac > 1.0 {
			over[k]++
		}
	}
	out := map[telemetryKey]usageAgg{}
	for k, fs := range byKey {
		out[k] = usageAgg{moves: len(fs), used: quantiles(fs), under10: under[k], over100: over[k]}
	}
	return out
}
