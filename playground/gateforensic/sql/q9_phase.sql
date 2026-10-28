WITH s AS (
    SELECT
        arm,
        geometry,
        opp,
        tc_idx,
        game,
        move_no,
        CASE WHEN is_master THEN score_cp ELSE -score_cp END AS master_cp,
        CASE WHEN is_master THEN mate_dist ELSE -mate_dist END AS master_mate
    FROM moves
    WHERE result != 'U'
),
phased AS (
    SELECT
        s.*,
        CASE
            WHEN s.move_no <= {early} THEN 'early'
            WHEN s.move_no <= {mid} THEN 'mid'
            ELSE 'late'
        END AS phase,
        s.master_cp > 0 AS ahead,
        s.master_cp = 0 AS level,
        s.master_cp < 0 AS behind,
        s.master_mate > 0 AS mating,
        s.master_mate < 0 AS getting_mated
    FROM s
)
SELECT
    arm,
    geometry,
    opp,
    tc_idx,
    phase,
    count(*) AS moves,
    round(count(*) FILTER (ahead)::DOUBLE / count(*), 3) AS ahead_share,
    round(count(*) FILTER (behind)::DOUBLE / count(*), 3) AS behind_share,
    round(count(*) FILTER (mating)::DOUBLE / count(*), 3) AS mating_share,
    round(count(*) FILTER (getting_mated)::DOUBLE / count(*), 3) AS getting_mated_share,
    round(avg(least(greatest(master_cp, -30000), 30000)), 0) AS clamped_cp_mean
FROM phased
GROUP BY arm, geometry, opp, tc_idx, phase
ORDER BY geometry, opp, tc_idx, arm, phase
