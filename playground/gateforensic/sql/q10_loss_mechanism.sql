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
        min(s.move_no) FILTER (s.move_no > f.flip_move) AS after_move
    FROM flips f
    JOIN s USING (arm, game)
    GROUP BY f.arm, f.game, f.flip_move
),
after AS (
    SELECT folded.*, any_value(s.tag) FILTER (s.move_no = folded.after_move) AS tag_after
    FROM folded JOIN s USING (arm, game)
    GROUP BY folded.arm, folded.game, folded.flip_move, folded.max_pro_before, folded.after_move
)
SELECT
    arm,
    count(*) AS losses,
    count(*) FILTER (max_pro_before >= {mate_half}) AS was_mating,
    count(*) FILTER (tag_after = 'VCF') AS vcf_after,
    count(*) FILTER (tag_after = 'VCT') AS vct_after,
    count(*) FILTER (tag_after = 'PONDER') AS ponder_after,
    count(*) FILTER (tag_after IS NULL AND after_move IS NOT NULL) AS plain_after
FROM after
GROUP BY arm
ORDER BY arm
