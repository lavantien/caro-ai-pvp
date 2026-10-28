WITH agg AS (
    SELECT
        arm,
        any_value(geometry) AS geometry,
        any_value(opp) AS opp,
        any_value(tc_idx) AS tc_idx,
        count(*) AS n,
        sum(CASE WHEN result = 'W' THEN 1 ELSE 0 END) AS wins,
        sum(CASE WHEN result = 'D' THEN 1 ELSE 0 END) AS draws,
        sum(CASE WHEN result = 'L' THEN 1 ELSE 0 END) AS losses,
        sum(CASE WHEN result = 'U' THEN 1 ELSE 0 END) AS unfinished,
        sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END) AS wd_count
    FROM games
    WHERE result != 'U'
    GROUP BY arm
)
SELECT
    agg.arm,
    agg.geometry,
    agg.opp,
    agg.tc_idx,
    agg.n,
    agg.wins,
    agg.draws,
    agg.losses,
    round(agg.wd_count::DOUBLE / agg.n, 3) AS wd,
    round(greatest(agg.wd_count::DOUBLE / agg.n
        - {z} * sqrt((agg.wd_count::DOUBLE / agg.n) * (1 - agg.wd_count::DOUBLE / agg.n) / agg.n), 0), 3) AS lb,
    round(least(agg.wd_count::DOUBLE / agg.n
        + {z} * sqrt((agg.wd_count::DOUBLE / agg.n) * (1 - agg.wd_count::DOUBLE / agg.n) / agg.n), 1), 3) AS ub,
    v.status AS verdict_status,
    v.wd AS verdict_wd,
    v.threshold,
    v.n AS verdict_n,
    v.lb AS verdict_lb,
    v.ub AS verdict_ub,
    max(g.provisional) AS provisional
FROM agg
LEFT JOIN verdicts v USING (arm)
LEFT JOIN games g USING (arm)
GROUP BY agg.arm, agg.geometry, agg.opp, agg.tc_idx, agg.n, agg.wins, agg.draws,
    agg.losses, agg.wd_count, v.status, v.wd, v.threshold, v.n, v.lb, v.ub
ORDER BY agg.geometry, agg.opp, agg.tc_idx, agg.arm
