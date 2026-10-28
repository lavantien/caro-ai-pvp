SELECT
    arm,
    split_part(detail, '|', 1) AS ponder_side,
    split_part(detail, '|', 3) AS verdict,
    count(*) AS n
FROM events
WHERE kind = 'ponder'
GROUP BY arm, ponder_side, verdict
ORDER BY arm, ponder_side, verdict
