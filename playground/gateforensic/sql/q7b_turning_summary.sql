WITH s AS (
    SELECT
        arm,
        game,
        move_no,
        CASE WHEN is_master THEN score_cp ELSE -score_cp END AS master_cp
    FROM moves
    WHERE result = 'L'
),
flips AS (
    SELECT arm, game, max(CASE WHEN master_cp >= 0 THEN move_no END) AS flip_move
    FROM s
    GROUP BY arm, game
)
SELECT
    arm,
    count(*) AS losses,
    count(*) FILTER (flip_move IS NULL) AS never_ahead,
    median(flip_move) AS flip_median,
    round(avg(flip_move), 1) AS flip_mean,
    count(*) FILTER (flip_move <= {early}) AS flipped_early,
    count(*) FILTER (flip_move > {early} AND flip_move <= {mid}) AS flipped_mid,
    count(*) FILTER (flip_move > {mid}) AS flipped_late
FROM flips
GROUP BY arm
ORDER BY arm
