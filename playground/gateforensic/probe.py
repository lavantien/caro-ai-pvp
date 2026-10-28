import sys

import analyze


def main():
    if len(sys.argv) != 2:
        print("usage: probe.py <sql-file>")
        sys.exit(2)
    con = analyze.connect()
    sql = open(sys.argv[1], encoding="utf-8").read().format(**analyze.placeholders())
    table, nrows = analyze.render_table(con, sql)
    print(table)
    print(f"rows={nrows}")


if __name__ == "__main__":
    main()
