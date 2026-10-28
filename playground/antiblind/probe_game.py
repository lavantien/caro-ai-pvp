import duckdb

con = duckdb.connect()
moves = "read_parquet('playground/gateforensic/out/moves.parquet')"
rows = con.execute(f"""
SELECT move_no, color, cell, is_master, score_cp, mate_dist, tag, depth
FROM {moves}
WHERE arm = 'hex-passgates-medium-tc1' AND game = 3
ORDER BY move_no
""").fetchall()
for r in rows:
    if 'O2' in r[2] or 'O1' in r[2]:
        print(r)
