SELECT
    geometry,
    count(*) AS games,
    count(*) FILTER (result = 'D') AS draws,
    round(count(*) FILTER (result = 'D')::DOUBLE / count(*), 3) AS draw_share,
    min(moves) FILTER (result = 'D') AS draw_moves_min,
    max(moves) FILTER (result = 'D') AS draw_moves_max
FROM games
WHERE result != 'U'
GROUP BY geometry
ORDER BY geometry
