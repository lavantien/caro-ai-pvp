import csv
import duckdb

con = duckdb.connect()
moves = "read_parquet('playground/gateforensic/out/moves.parquet')"
flips = list(csv.DictReader(open('playground/antiblind/out/flips.csv')))
flip_keys = {(r['arm'], int(r['game']), int(r['master_move_no'])) for r in flips}
loss_keys = {(r['arm'], int(r['game'])) for r in flips if r['result'] == 'L'}

rows = con.execute(f"""
SELECT arm, game, move_no, color, tt_pct, hf_pct, fh1_pct, depth, nodes, t, tag,
       mate_dist, result, master_seat
FROM {moves} WHERE is_master AND mate_dist > 0
ORDER BY arm, game, move_no
""").fetchall()

def fold(items):
    if not items:
        return None
    n = len(items)
    tt = sum(r[4] for r in items) / n
    hf = sum(r[5] for r in items) / n
    fh1 = sum(r[6] for r in items) / n
    dep = sum(r[7] for r in items) / n
    nod = sum(r[8] for r in items) / n
    pond = sum(1 for r in items if 'PONDER' in (r[10] or '')) / n
    return n, round(tt, 1), round(hf, 1), round(fh1, 1), round(dep, 2), round(nod / 1e6, 2), round(pond, 3)

groups = {}
for r in rows:
    arm = r[0]
    key = (arm, r[1])
    is_flip = (arm, r[1], r[2]) in flip_keys
    in_loss_game = key in loss_keys
    g1 = 'flip' if is_flip else 'noflip'
    g2 = 'lossgame' if in_loss_game else 'wingame'
    groups.setdefault((arm, g1, g2), []).append(r)

with open('playground/antiblind/out/q1_flips_vs_nonflips.csv', 'w', newline='') as f:
    w = csv.writer(f)
    w.writerow(['arm', 'class', 'gameclass', 'n', 'tt_pct', 'hf_pct', 'fh1_pct',
                'depth', 'nodes_m', 'ponder_share'])
    for k in sorted(groups):
        fo = fold(groups[k])
        if fo:
            w.writerow([k[0], k[1], k[2]] + list(fo))

agg = {}
for r in rows:
    is_flip = (r[0], r[1], r[2]) in flip_keys
    agg.setdefault(is_flip, []).append(r)
for k in (True, False):
    print('flip' if k else 'noflip', fold(agg[k]))
seat = {}
for r in rows:
    if r[0].startswith('square') and r[0] != 'square-passgates-easy-tc1':
        seat.setdefault((r[3] == r[13], (r[0], r[1], r[2]) in flip_keys), []).append(r)
for k in sorted(seat, key=str):
    print('seatred', k, fold(seat[k]))
