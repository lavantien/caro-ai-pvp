SELECT
    arm,
    geometry,
    opp,
    tc_idx,
    count(*) AS n,
    round(avg(moves), 1) AS moves_mean,
    median(moves) AS moves_median,
    min(moves) AS moves_min,
    max(moves) AS moves_max,
    round(avg(moves) FILTER (result = 'D'), 1) AS draw_mean,
    round(avg(moves) FILTER (result != 'D'), 1) AS decided_mean,
    min(moves) FILTER (result != 'D') AS decided_min,
    max(moves) FILTER (result != 'D') AS decided_max
FROM games
WHERE result != 'U'
GROUP BY arm, geometry, opp, tc_idx
ORDER BY geometry, opp, tc_idx, arm
