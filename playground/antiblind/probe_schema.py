import duckdb

con = duckdb.connect()
for name in ('moves', 'events', 'games'):
    path = f'playground/gateforensic/out/{name}.parquet'
    cols = con.execute(f"describe select * from read_parquet('{path}')").fetchall()
    print(name)
    for c in cols:
        print(f'  {c[0]} {c[1]}')
