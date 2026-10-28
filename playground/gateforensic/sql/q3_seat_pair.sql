SELECT
    arm,
    geometry,
    opp,
    tc_idx,
    master_seat,
    count(*) FILTER (result = 'W') AS master_wins,
    count(*) FILTER (result = 'D') AS draws,
    count(*) FILTER (result = 'L') AS master_losses,
    count(*) FILTER (result = 'U') AS unfinished
FROM games
GROUP BY arm, geometry, opp, tc_idx, master_seat
ORDER BY geometry, opp, tc_idx, master_seat, arm
