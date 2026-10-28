import csv
import duckdb

con = duckdb.connect()
moves = "read_parquet('playground/gateforensic/out/moves.parquet')"

rows = con.execute(f"""
WITH ordered AS (
  SELECT *, lag(mate_dist) OVER (PARTITION BY arm, game ORDER BY move_no) AS prev_mate,
         lag(is_master) OVER (PARTITION BY arm, game ORDER BY move_no) AS prev_master,
         lag(score_cp) OVER (PARTITION BY arm, game ORDER BY move_no) AS prev_score
  FROM {moves}
), flips AS (
  SELECT * FROM ordered
  WHERE prev_master AND NOT is_master
    AND prev_mate > 0 AND mate_dist > 0
)
SELECT f.arm, f.geometry, f.game, f.result, f.master_seat, f.game_moves,
       f.move_no AS killer_move_no, f.cell AS killer_cell, f.mate_dist AS killer_mate,
       f.tag AS killer_tag, f.color AS killer_color,
       f.prev_mate AS master_mate, f.prev_score AS master_score,
       o.move_no AS master_move_no, o.cell AS master_cell,
       o.depth AS master_depth, o.nodes AS master_nodes,
       o.tt_pct AS master_tt, o.hf_pct AS master_hf, o.tag AS master_tag
FROM flips f
JOIN {moves} o ON o.arm = f.arm AND o.game = f.game AND o.move_no = f.move_no - 1
ORDER BY f.arm, f.game, f.move_no
""").fetchall()

loss_flips = [r for r in rows if r[3] == 'L']
print(f'one ply mate inversions: {len(rows)} total, {len(loss_flips)} in master losses')

with open('playground/antiblind/out/flips.csv', 'w', newline='') as f:
    w = csv.writer(f)
    w.writerow(['arm', 'geometry', 'game', 'result', 'master_seat', 'game_moves',
                'killer_move_no', 'killer_cell', 'killer_mate', 'killer_tag', 'killer_color',
                'master_mate', 'master_score', 'master_move_no', 'master_cell',
                'master_depth', 'master_nodes', 'master_tt', 'master_hf', 'master_tag'])
    for r in rows:
        w.writerow(r)

games = con.execute(f"""
SELECT DISTINCT arm, game FROM {moves}
WHERE (arm, game) IN (
  SELECT arm, game FROM {moves}
  WHERE is_master AND mate_dist > 0
)
ORDER BY arm, game
""").fetchall()
print(f'games with at least one master mate-for claim: {len(games)}')

with open('playground/antiblind/out/games_with_claims.csv', 'w', newline='') as f:
    w = csv.writer(f)
    w.writerow(['arm', 'game'])
    for r in games:
        w.writerow(r)

seq = con.execute(f"""
SELECT m.arm, m.game, m.geometry, m.move_no, m.color, m.cell
FROM {moves} m
JOIN (SELECT DISTINCT arm, game FROM read_csv_auto('playground/antiblind/out/games_with_claims.csv')) g
  ON g.arm = m.arm AND g.game = m.game
ORDER BY m.arm, m.game, m.move_no
""").fetchall()
with open('playground/antiblind/out/claim_games_moves.csv', 'w', newline='') as f:
    w = csv.writer(f)
    w.writerow(['arm', 'game', 'geometry', 'move_no', 'color', 'cell'])
    for r in seq:
        w.writerow(r)
print(f'move rows for reconstruction: {len(seq)}')
