import logging
import sys
import time

import duckdb

import config

QUERY_TITLES = {
    "q1_arm_wd": "per arm W+D with Wald 95 percent intervals",
    "q2_seat_split": "W+D by master seat color",
    "q3_seat_pair": "per seat pair game outcomes",
    "q4_parity": "search parity master versus opponent, untagged moves only",
    "q5_length": "game length distributions",
    "q6_draws": "draw profile per geometry",
    "q7_turning": "turning points of master losses",
    "q7b_turning_summary": "turning point summary per arm",
    "q8_ponder": "ponder verdict counts",
    "q9_phase": "phase folds of the master edge",
    "q10_loss_mechanism": "loss mechanism classification",
    "q11_flip_owner": "flip ownership and immediate mate after the flip",
}

CHECK_WD_RECOMPUTE = """
SELECT v.arm, v.wd AS verdict_wd, v.threshold AS verdict_threshold,
    round(sum(CASE WHEN g.result IN ('W', 'D') THEN 1 ELSE 0 END)::DOUBLE
        / count(*), 3) AS recomputed_wd,
    count(*) AS games
FROM verdicts v JOIN games g USING (arm)
WHERE v.status = 'FAILED' AND g.result != 'U'
GROUP BY v.arm, v.wd, v.threshold
"""

CHECK_SEAT_PARITY = """
SELECT count(*) AS violations FROM games
WHERE (game % 2 = 1 AND master_seat != 'red')
   OR (game % 2 = 0 AND master_seat != 'blue')
"""

CHECK_MOVE_COUNTS = """
SELECT count(*) AS violations FROM (
    SELECT arm, game FROM moves GROUP BY arm, game
    EXCEPT
    SELECT arm, game FROM games
) UNION ALL SELECT count(*) FROM (
    SELECT g.arm, g.game, g.moves, count(m.move_no) AS have
    FROM games g LEFT JOIN moves m USING (arm, game)
    WHERE g.result != 'U'
    GROUP BY g.arm, g.game, g.moves
    HAVING have != g.moves OR have = 0
)
"""

CHECK_VERDICT_COUNT = """
SELECT arm, count(*) AS verdicts FROM verdicts GROUP BY arm HAVING count(*) > 1
"""

CHECK_EXTEND_TOTALS = """
WITH totals AS (
    SELECT arm,
        count(*) AS games,
        count(*) FILTER (result != 'U') AS finished,
        max(provisional) AS provisional
    FROM games GROUP BY arm
), checkpoints AS (
    SELECT arm, max(n) AS max_n FROM extends GROUP BY arm
)
SELECT t.arm FROM totals t JOIN checkpoints c USING (arm)
WHERE c.max_n > t.games
   OR (NOT t.provisional AND c.max_n != t.finished)
"""

CHECK_PONDER_REFS = """
WITH p AS (
    SELECT arm, kind, game, CAST(replace(split_part(detail, '|', 2), 'm', '') AS INT) AS move_no
    FROM events WHERE kind = 'ponder'
)
SELECT count(*) AS violations FROM p
LEFT JOIN moves m USING (arm, game, move_no)
WHERE m.arm IS NULL
"""

CHECK_UNFINISHED = """
SELECT DISTINCT arm FROM games WHERE result = 'U'
"""

CHECK_WIN_SIGN = """
WITH last AS (
    SELECT arm, game, result,
        arg_max(CASE WHEN is_master THEN score_cp ELSE -score_cp END, move_no) AS last_master_cp
    FROM moves WHERE result IN ('W', 'L')
    GROUP BY arm, game, result
)
SELECT count(*) AS total,
    count(*) FILTER (
        (result = 'W' AND last_master_cp > 0) OR (result = 'L' AND last_master_cp < 0)
    ) AS consistent
FROM last
"""


class Failure(Exception):
    pass


def connect():
    con = duckdb.connect()
    con.execute(f"CREATE VIEW moves AS SELECT * FROM read_csv_auto('{config.OUT_DIR / 'moves.csv'}')")
    con.execute(f"CREATE VIEW games AS SELECT * FROM read_csv_auto('{config.OUT_DIR / 'games.csv'}')")
    con.execute(f"CREATE VIEW events AS SELECT * FROM read_csv_auto('{config.OUT_DIR / 'events.csv'}')")
    con.execute(f"CREATE VIEW verdicts AS SELECT * FROM read_csv_auto('{config.OUT_DIR / 'verdicts.csv'}')")
    con.execute(f"CREATE VIEW extends AS SELECT * FROM read_csv_auto('{config.OUT_DIR / 'extends.csv'}')")
    return con


def placeholders():
    return {
        "z": config.WALD_Z,
        "early": config.PHASE_EARLY_MAX,
        "mid": config.PHASE_MID_MAX,
        "mate_half": config.MATE_BASE // 2,
    }


def load_query(name):
    text = (config.SQL_DIR / f"{name}.sql").read_text(encoding="utf-8")
    return text.format(**placeholders())


def render_table(con, sql):
    cur = con.execute(sql)
    cols = [d[0] for d in cur.description]
    rows = cur.fetchall()
    out = ["| " + " | ".join(cols) + " |", "|" + "|".join("---" for _ in cols) + "|"]
    for row in rows:
        cells = []
        for v in row:
            if v is None:
                cells.append("")
            elif isinstance(v, float):
                cells.append(f"{v:g}")
            else:
                cells.append(str(v))
        out.append("| " + " | ".join(cells) + " |")
    return "\n".join(out), len(rows)


def run_checks(con, provisional_arms):
    log = logging.getLogger("gateforensic.checks")
    lines, failures = [], []
    wd_rows = con.execute(CHECK_WD_RECOMPUTE).fetchall()
    for arm, verdict_wd, verdict_threshold, recomputed, games in wd_rows:
        ok = abs(verdict_wd - recomputed) <= 0.0005
        lines.append(f"{'PASS' if ok else 'FAIL'} wd-recompute {arm} verdict={verdict_wd} recomputed={recomputed} n={games}")
        if not ok:
            failures.append(f"wd-recompute {arm}")
        cfg_threshold = config.THRESHOLDS.get(arm.split("-")[2])
        if cfg_threshold is not None and abs(cfg_threshold - verdict_threshold) > 1e-9:
            lines.append(f"FAIL threshold-mismatch {arm} verdict={verdict_threshold} config={cfg_threshold}")
            failures.append(f"threshold-mismatch {arm}")
        lines.append(f"PASS threshold {arm} verdict={verdict_threshold}")
    simple = [
        ("seat-parity", CHECK_SEAT_PARITY),
        ("move-counts", CHECK_MOVE_COUNTS),
        ("verdict-count", CHECK_VERDICT_COUNT),
        ("extend-totals", CHECK_EXTEND_TOTALS),
        ("ponder-refs", CHECK_PONDER_REFS),
    ]
    for name, sql in simple:
        rows = con.execute(sql).fetchall()
        violated = any(r[0] != 0 for r in rows)
        lines.append(f"{'FAIL' if violated else 'PASS'} {name} violations={rows}")
        if violated:
            failures.append(name)
    for (arm,) in con.execute(CHECK_UNFINISHED).fetchall():
        ok = arm in provisional_arms
        lines.append(f"{'PASS' if ok else 'FAIL'} unfinished-game {arm}")
        if not ok:
            failures.append(f"unfinished-game {arm}")
    total, consistent = con.execute(CHECK_WIN_SIGN).fetchone()
    share = consistent / total if total else 1.0
    lines.append(f"{'PASS' if share >= 0.95 else 'WARN'} win-sign total={total} consistent={consistent} share={share:.3f}")
    log.info("checks failures=%d lines=%d", len(failures), len(lines))
    return lines, failures, share


def run():
    log = logging.getLogger("gateforensic.analyze")
    log.info("analyze entry")
    started = time.monotonic()
    con = connect()
    ledger = con.execute(
        f"SELECT arm, provisional FROM read_csv_auto('{config.OUT_DIR / 'ledger.csv'}')"
    ).fetchall()
    provisional_arms = {arm for arm, p in ledger if p}
    sections, integrity, failures = [], [], []
    for name in QUERY_TITLES:
        try:
            table, nrows = render_table(con, load_query(name))
        except Exception:
            log.exception("analyze failure query=%s", name)
            raise
        sections.append(f"## {QUERY_TITLES[name]}\n\n{table}\n")
        log.info("analyze query=%s rows=%d", name, nrows)
    check_lines, check_failures, win_sign_share = run_checks(con, provisional_arms)
    integrity.extend(check_lines)
    failures.extend(check_failures)
    header = [
        "# gateforensic report",
        "",
        f"generated from playground/gateforensic/out datasets, wald z={config.WALD_Z},",
        f"phases early<={config.PHASE_EARLY_MAX} mid<={config.PHASE_MID_MAX},",
        f"win-sign consistency {win_sign_share:.3f}, provisional arms {sorted(provisional_arms)}",
        "",
    ]
    config.REPORT_MD.write_text("\n".join(header + sections), encoding="utf-8")
    config.INTEGRITY_TXT.write_text("\n".join(integrity) + "\n", encoding="utf-8")
    for table in ("moves", "events"):
        con.execute(
            f"COPY (SELECT * FROM {table}) TO '{config.OUT_DIR / f'{table}.parquet'}' (FORMAT PARQUET)"
        )
    log.info("analyze parquet exports done")
    log.info("analyze exit sections=%d failures=%d elapsed=%.2fs",
             len(sections), len(failures), time.monotonic() - started)
    if failures:
        raise Failure(f"integrity failures: {failures}")


def main():
    config.OUT_DIR.mkdir(parents=True, exist_ok=True)
    handlers = [logging.StreamHandler(sys.stderr), logging.FileHandler(config.RUN_LOG, encoding="utf-8")]
    logging.basicConfig(
        level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s",
        handlers=handlers, force=True,
    )
    try:
        run()
    except Failure as exc:
        logging.getLogger("gateforensic.analyze").error("analyze failure %s", exc)
        print("\n".join(config.INTEGRITY_TXT.read_text(encoding="utf-8").splitlines()))
        sys.exit(1)


if __name__ == "__main__":
    main()
