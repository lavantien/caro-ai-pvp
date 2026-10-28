import duckdb

con = duckdb.connect()
moves = "read_parquet('playground/gateforensic/out/moves.parquet')"
rows = con.execute(f"""
SELECT arm, geometry, game, move_no, cell
FROM {moves} WHERE geometry = 'hex'
""").fetchall()

def in_region(cell):
    q = ord(cell[0]) - 65
    r = int(cell[1:]) - 1
    return abs(q - 11) <= 11 and abs(r - 11) <= 11 and abs(q + r - 22) <= 11

bad = [r for r in rows if not in_region(r[4])]
print('hex moves out of region:', len(bad), 'of', len(rows))
from collections import Counter
print(Counter((r[0], r[4]) for r in bad).most_common(12))
sq = con.execute(f"""
SELECT arm, game, move_no, cell FROM {moves} WHERE geometry = 'square'
""").fetchall()
badsq = [r for r in sq if ord(r[3][0]) > 80 or int(r[3][1:]) > 16]
print('square moves out of range:', len(badsq), 'of', len(sq))
print(badsq[:10])
