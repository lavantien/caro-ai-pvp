WITH s AS (
    SELECT arm, game, move_no, is_master, tag,
        CASE WHEN is_master THEN score_cp ELSE -score_cp END AS master_cp
    FROM moves WHERE result = 'L'
),
flips AS (
    SELECT arm, game, max(CASE WHEN master_cp >= 0 THEN move_no END) AS flip_move
    FROM s GROUP BY arm, game
),
folded AS (
    SELECT f.arm, f.game, f.flip_move,
        max(s.master_cp) FILTER (s.move_no <= f.flip_move) AS max_pro_before,
        min(s.move_no) FILTER (s.move_no > f.flip_move) AS after_move
    FROM flips f JOIN s USING (arm, game)
    GROUP BY f.arm, f.game, f.flip_move
)
SELECT
    f.arm,
    count(*) AS losses,
    count(*) FILTER (f.max_pro_before >= {mate_half}) AS was_mating,
    count(*) FILTER (a.master_cp <= -{mate_half}) AS mated_immediately_after,
    count(*) FILTER (a.is_master) AS after_by_master,
    count(*) FILTER (NOT a.is_master) AS after_by_opponent,
    count(*) FILTER (NOT a.is_master AND a.master_cp <= -{mate_half}) AS opp_reply_mated
FROM folded f
JOIN s a ON a.arm = f.arm AND a.game = f.game AND a.move_no = f.after_move
GROUP BY f.arm
ORDER BY f.arm
