import duckdb

con = duckdb.connect()
moves = "read_parquet('playground/gateforensic/out/moves.parquet')"
print(con.execute(f"""
SELECT is_master, count(*) n,
       count(*) FILTER (WHERE mate_dist > 0) mpos,
       count(*) FILTER (WHERE mate_dist < 0) mneg,
       count(*) FILTER (WHERE mate_dist = 0) mzero,
       min(mate_dist) mn, max(mate_dist) mx,
       min(score_cp) sn, max(score_cp) sx
FROM {moves} GROUP BY is_master
""").fetchall())
print(con.execute(f"""
SELECT arm, result, count(*) FROM {moves}
WHERE mate_dist > 0 AND is_master
GROUP BY arm, result ORDER BY arm, result
""").fetchall())
