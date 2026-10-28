import csv
from collections import defaultdict

rows = list(csv.DictReader(open('playground/antiblind/out/replay_config.csv')))
print(len(rows), 'rows')

per_cfg = defaultdict(list)
for r in rows:
    per_cfg[(int(r['tt_mib']), int(r['threads']))].append(r)

print('cfg tt thr | blind_share avg_depth avg_nodem avg_tthit avg_hashfull | blind_share_gt_proven')
for k in sorted(per_cfg):
    g = per_cfg[k]
    blind = sum(1 for r in g if r['blind'] == 'True') / len(g)
    dep = sum(int(r['depth']) for r in g) / len(g)
    nod = sum(float(r['nodes_m']) for r in g) / len(g)
    tth = sum(int(r['tt_hit']) for r in g) / len(g)
    hf = sum(int(r['hash_full']) for r in g) / len(g)
    gt = [r for r in g if r['gt_kind'] != 'none']
    blind_gt = (sum(1 for r in gt if r['blind'] == 'True') / len(gt)) if gt else None
    bs = f'{blind_gt:.2f}' if blind_gt is not None else 'na'
    print(f'{k[0]:4d} {k[1]:3d} | {blind:.2f} {dep:5.2f} {nod:6.2f} {tth:6.0f} {hf:6.0f} | {bs} n_gt {len(gt)}')

by_depth = defaultdict(lambda: [0, 0])
for r in rows:
    d = int(r['depth'])
    by_depth[d][1] += 1
    if r['blind'] == 'True':
        by_depth[d][0] += 1
print('depth -> blind/n')
for d in sorted(by_depth):
    b, n = by_depth[d]
    print(f'{d:3d} {b:3d} {n:3d} {b / n:.2f}')

gt_rows = [r for r in rows if r['gt_kind'] != 'none']
mate_order = [(r['arm'], r['game'], r['gt_kind'], r['gt_plies'], r['blind'], int(r['tt_mib']), int(r['threads'])) for r in gt_rows]
seen = set()
print('proven kills: arm game kind plies, blind by config')
for m in mate_order:
    key = (m[0], m[1], m[5], m[6])
    if key in seen:
        continue
    seen.add(key)
