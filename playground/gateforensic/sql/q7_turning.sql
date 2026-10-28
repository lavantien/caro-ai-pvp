WITH s AS (
    SELECT
        arm,
        game,
        move_no,
        tag,
        CASE WHEN is_master THEN score_cp ELSE -score_cp END AS master_cp
    FROM moves
    WHERE result = 'L'
),
flips AS (
    SELECT arm, game, max(CASE WHEN master_cp >= 0 THEN move_no END) AS flip_move
    FROM s
    GROUP BY arm, game
),
folded AS (
    SELECT
        f.arm,
        f.game,
        f.flip_move,
        max(s.master_cp) FILTER (s.move_no <= f.flip_move) AS max_pro_before,
        max(s.master_cp) FILTER (s.move_no = f.flip_move) AS cp_at_flip,
        min(s.move_no) FILTER (s.move_no > f.flip_move) AS after_move
    FROM flips f
    JOIN s USING (arm, game)
    GROUP BY f.arm, f.game, f.flip_move
)
SELECT
    folded.arm,
    folded.game,
    folded.flip_move,
    CASE
        WHEN folded.flip_move IS NULL THEN 'never-ahead'
        WHEN folded.flip_move <= {early} THEN 'early'
        WHEN folded.flip_move <= {mid} THEN 'mid'
        ELSE 'late'
    END AS phase,
    folded.max_pro_before,
    folded.cp_at_flip,
    CASE
        WHEN folded.max_pro_before IS NULL THEN ''
        WHEN abs(folded.max_pro_before) >= {mate_half} THEN 'mate'
        ELSE 'cp'
    END AS max_pro_kind,
    folded.after_move,
    any_value(s.master_cp) FILTER (s.move_no = folded.after_move) AS cp_after,
    any_value(s.tag) FILTER (s.move_no = folded.after_move) AS tag_after
FROM folded
JOIN s USING (arm, game)
GROUP BY
    folded.arm, folded.game, folded.flip_move, folded.max_pro_before,
    folded.cp_at_flip, folded.after_move
ORDER BY folded.arm, folded.game
