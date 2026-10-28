SELECT
    arm,
    geometry,
    opp,
    tc_idx,
    master_seat,
    count(*) AS n,
    sum(CASE WHEN result = 'W' THEN 1 ELSE 0 END) AS wins,
    sum(CASE WHEN result = 'D' THEN 1 ELSE 0 END) AS draws,
    sum(CASE WHEN result = 'L' THEN 1 ELSE 0 END) AS losses,
    round(sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*), 3) AS wd,
    round(sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*)
        - {z} * sqrt((sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*))
        * (1 - sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*)) / count(*)), 3) AS lb,
    round(sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*)
        + {z} * sqrt((sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*))
        * (1 - sum(CASE WHEN result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE / count(*)) / count(*)), 3) AS ub
FROM games
WHERE result != 'U'
GROUP BY arm, geometry, opp, tc_idx, master_seat
ORDER BY geometry, opp, tc_idx, master_seat, arm
