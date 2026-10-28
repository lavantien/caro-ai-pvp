import csv
import hashlib
import json
import logging
import sys
import time

import config
import parse

RAW_SUFFIX = ".raw"
VERDICT_PROBE = b"hex-passgates: "
MLINE_PROBE = b", pv="


def setup_logging():
    config.OUT_DIR.mkdir(parents=True, exist_ok=True)
    handlers = [logging.StreamHandler(sys.stderr), logging.FileHandler(config.RUN_LOG, encoding="utf-8")]
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
        handlers=handlers,
        force=True,
    )


def resolve_source(arm):
    log_path = config.ARENA_OUT / f"{arm['name']}.log"
    raw_path = config.ARENA_OUT / f"{arm['name']}.log{RAW_SUFFIX}"
    log_bytes = log_path.read_bytes() if log_path.exists() else None
    if log_bytes is not None and (MLINE_PROBE in log_bytes or not raw_path.exists()):
        provisional = arm["role"] == config.ARM_PROVISIONAL and VERDICT_PROBE not in log_bytes
        return log_path, log_bytes, provisional
    if raw_path.exists():
        return raw_path, raw_path.read_bytes(), True
    if log_bytes is not None:
        return log_path, log_bytes, arm["role"] == config.ARM_PROVISIONAL
    return None, None, arm["role"] == config.ARM_PROVISIONAL


def read_lines(data):
    text = data.decode("utf-8", errors="replace")
    lines = text.split("\n")
    truncated = False
    if lines and lines[-1] != "":
        truncated = True
    elif lines:
        lines = lines[:-1]
    return lines, truncated


def result_for_master(winner, master_seat):
    if winner == "draw":
        return config.RESULT_DRAW
    if winner == master_seat:
        return config.RESULT_WIN
    return config.RESULT_LOSS


class ArmIngest:
    def __init__(self, arm):
        self.arm = arm
        self.provisional = False
        self.moves = []
        self.games = []
        self.events = []
        self.verdicts = []
        self.extends = []
        self.bad = []
        self.counts = {}
        self.lines_total = 0
        self.truncated = 0
        self.state = {"game": 0, "block": 1, "master_seat": "", "red": "", "blue": ""}
        self.buffer = []

    def event(self, source, provisional, lineno, kind, game, detail):
        self.events.append([self.arm["name"], source, provisional, lineno, kind, game, detail])

    def ingest(self, source, data, provisional):
        self.provisional = provisional
        lines, truncated_tail = read_lines(data)
        base = {
            "arm": self.arm["name"],
            "geometry": self.arm["geometry"],
            "opp": self.arm["opp"],
            "tc_idx": self.arm["tc"],
            "source": source.name,
            "provisional": provisional,
            "attempt": self.arm["role"] == config.ARM_ATTEMPT,
        }
        for i, raw in enumerate(lines):
            lineno = i + 1
            line = raw.removesuffix("\r")
            self.lines_total += 1
            kind = parse.classify(line)
            if kind is None:
                self.bad.append([self.arm["name"], source.name, lineno, "unmatched", line])
                self.counts["bad"] = self.counts.get("bad", 0) + 1
                continue
            self.counts[kind] = self.counts.get(kind, 0) + 1
            if kind == parse.KIND_BLANK:
                self.event(source.name, provisional, lineno, kind, "", "")
            elif kind == parse.KIND_MLINE:
                self.take_mline(base, line, lineno)
            elif kind == parse.KIND_GAME_START:
                self.take_game_start(source, provisional, base, line, lineno)
            elif kind == parse.KIND_GAME_RESULT:
                self.take_game_result(source, provisional, base, line, lineno)
            elif kind == parse.KIND_PONDER:
                self.take_ponder(source, provisional, line, lineno)
            elif kind == parse.KIND_EXTEND_BLOCK:
                self.take_extend(source, provisional, line, lineno)
            elif kind == parse.KIND_VERDICT:
                self.take_verdict(source, provisional, base, line, lineno)
            else:
                detail = {
                    parse.KIND_ARM_START: lambda: parse.parse_arm_start(line)["argv"],
                    parse.KIND_ARM_EXIT: lambda: f"exit={parse.parse_arm_exit(line)['exit']}",
                    parse.KIND_ARM_ADOPTED: lambda: "adopted",
                    parse.KIND_DESCRIPTOR: lambda: line,
                    parse.KIND_EXTEND_NOTE: lambda: line.removeprefix("extend: "),
                }[kind]()
                self.event(source.name, provisional, lineno, kind, "", detail)
        if truncated_tail:
            self.truncated = 1
            self.event(source.name, provisional, len(lines), parse.KIND_TRUNCATED, self.state["game"], lines[-1])
        self.close_open_game(base)
        return self

    def take_mline(self, base, line, lineno):
        m = parse.parse_mline(line)
        if m is None:
            self.bad.append([self.arm["name"], base["source"], lineno, "m-line-malformed", line])
            self.counts["bad"] = self.counts.get("bad", 0) + 1
            self.counts[parse.KIND_MLINE] = self.counts.get(parse.KIND_MLINE, 0) - 1
            return
        if m["game"] != self.state["game"]:
            self.bad.append([self.arm["name"], base["source"], lineno, "orphan-m-line", line])
            self.counts["bad"] = self.counts.get("bad", 0) + 1
            self.counts[parse.KIND_MLINE] = self.counts.get(parse.KIND_MLINE, 0) - 1
            return
        row = {**base, **{k: m[k] for k in (
            "game", "move_no", "color", "cell", "depth", "nodes", "nps", "ebf",
            "tt_pct", "hf_pct", "fh1_pct", "score_raw", "score_cp", "mate_dist",
            "thr", "t", "alloc", "tag", "pv_len",
        )}}
        row["block"] = self.state["block"]
        row["master_seat"] = self.state["master_seat"]
        row["is_master"] = (m["color"] == "Red") == (self.state["master_seat"] == "red")
        self.buffer.append(row)

    def take_game_start(self, source, provisional, base, line, lineno):
        g = parse.parse_game_start(line)
        self.close_open_game(base)
        self.state = {
            "game": g["game"],
            "block": self.state["block"],
            "master_seat": "red" if g["red"] == config.MASTER else "blue",
            "red": g["red"],
            "blue": g["blue"],
        }
        self.event(source.name, provisional, lineno, parse.KIND_GAME_START, g["game"], f"{g['red']}|{g['blue']}")

    def take_game_result(self, source, provisional, base, line, lineno):
        r = parse.parse_game_result(line)
        seat = "red" if r["red"] == config.MASTER else "blue"
        result = result_for_master(r["winner"], seat)
        row = {
            **base,
            "game": r["game"],
            "block": self.state["block"],
            "master_seat": seat,
            "result": result,
            "winner": r["winner"],
            "moves": r["moves"],
        }
        self.games.append(row)
        for buffered in self.buffer:
            buffered["result"] = result
            buffered["game_moves"] = r["moves"]
        self.moves.extend(self.buffer)
        self.buffer = []
        self.state["game"] = 0
        self.event(
            source.name, provisional, lineno, parse.KIND_GAME_RESULT, r["game"],
            f"{r['winner']}|{r['moves']}|{result}",
        )

    def close_open_game(self, base):
        if not self.buffer:
            return
        game = self.buffer[0]["game"]
        row = {
            **base,
            "game": game,
            "block": self.state["block"],
            "master_seat": self.state["master_seat"],
            "result": config.RESULT_UNFINISHED,
            "winner": "",
            "moves": len(self.buffer),
        }
        self.games.append(row)
        for buffered in self.buffer:
            buffered["result"] = config.RESULT_UNFINISHED
            buffered["game_moves"] = len(self.buffer)
        self.moves.extend(self.buffer)
        self.buffer = []

    def take_ponder(self, source, provisional, line, lineno):
        p = parse.parse_ponder(line)
        self.event(
            source.name, provisional, lineno, parse.KIND_PONDER, p["game"],
            f"{p['side']}|m{p['move_no']}|{p['verdict']}",
        )

    def take_extend(self, source, provisional, line, lineno):
        e = parse.parse_extend_block(line)
        self.extends.append([
            self.arm["name"], source.name, provisional, lineno,
            e["block"], e["max_blocks"], e["n"], e["wd"], e["lb"], e["ub"],
        ])
        self.state["block"] = e["block"] + 1
        self.event(
            source.name, provisional, lineno, parse.KIND_EXTEND_BLOCK, "",
            f"block={e['block']}/{e['max_blocks']} n={e['n']} wd={e['wd']:.3f}",
        )

    def take_verdict(self, source, provisional, base, line, lineno):
        v = parse.parse_verdict(line)
        self.verdicts.append([
            base["arm"], base["geometry"], base["opp"], base["tc_idx"],
            base["source"], provisional, base["attempt"], v["status"],
            v["opp_field"], v["tc_field"] if v["tc_field"] is not None else "",
            v["wd"] if v["wd"] is not None else "",
            v["threshold"] if v["threshold"] is not None else "",
            v["n"] if v["n"] is not None else "",
            v["lb"] if v["lb"] is not None else "",
            v["ub"] if v["ub"] is not None else "",
            v["cap"], lineno,
        ])
        self.event(source.name, provisional, lineno, parse.KIND_VERDICT, "", line)


def ledger_row(arm, source, result, data):
    counts = result.counts
    ingested = sum(
        counts.get(k, 0)
        for k in (parse.KIND_MLINE, parse.KIND_GAME_START, parse.KIND_GAME_RESULT,
                  parse.KIND_PONDER, parse.KIND_EXTEND_BLOCK, parse.KIND_VERDICT,
                  parse.KIND_ARM_START, parse.KIND_ARM_EXIT, parse.KIND_ARM_ADOPTED,
                  parse.KIND_DESCRIPTOR, parse.KIND_EXTEND_NOTE, parse.KIND_BLANK)
    )
    accounted = ingested + counts.get("bad", 0) + result.truncated
    if accounted != result.lines_total:
        raise AssertionError(
            f"ledger mismatch arm={arm['name']} accounted={accounted} total={result.lines_total}"
        )
    return [
        arm["name"], source.name if source else "", arm["role"],
        result.provisional,
        result.lines_total, counts.get(parse.KIND_MLINE, 0),
        len(result.games), len(result.events), len(result.extends),
        len(result.verdicts), counts.get(parse.KIND_BLANK, 0),
        counts.get("bad", 0), result.truncated,
        hashlib.sha256(data).hexdigest() if data else "",
    ]


def write_csv(path, header, rows):
    with open(path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(header)
        w.writerows(rows)


def fmt(v):
    if v is None:
        return ""
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, float):
        return f"{v:g}"
    return v


def run(allow_provisional=True):
    log = logging.getLogger("gateforensic.ingest")
    log.info("ingest entry allow_provisional=%s", allow_provisional)
    started = time.monotonic()
    config.OUT_DIR.mkdir(parents=True, exist_ok=True)
    moves, games, events, verdicts, extends, ledger, badlines = [], [], [], [], [], [], []
    state = {"generated_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "arms": {}}
    for arm in config.ARMS:
        if arm["role"] == config.ARM_PROVISIONAL and not allow_provisional:
            log.info("ingest skip %s provisional leg excluded by flag", arm["name"])
            continue
        source, data, provisional = resolve_source(arm)
        if source is None:
            log.error("ingest failure arm=%s reason=no-source-file", arm["name"])
            ledger.append([arm["name"], "", arm["role"], False, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, ""])
            continue
        try:
            result = ArmIngest(arm).ingest(source, data, provisional)
        except Exception:
            log.exception("ingest failure arm=%s source=%s", arm["name"], source)
            raise
        moves.extend(result.moves)
        games.extend(result.games)
        events.extend(result.events)
        verdicts.extend(result.verdicts)
        extends.extend(result.extends)
        badlines.extend(result.bad)
        ledger.append(ledger_row(arm, source, result, data))
        state["arms"][arm["name"]] = {
            "source": source.name,
            "provisional": provisional,
            "sha256": ledger[-1][-1],
            "lines": result.lines_total,
            "bad": result.counts.get("bad", 0),
            "games": len(result.games),
        }
        log.info(
            "ingest arm=%s source=%s lines=%d moves=%d games=%d extends=%d bad=%d truncated=%d",
            arm["name"], source.name, result.lines_total, len(result.moves), len(result.games),
            len(result.extends), result.counts.get("bad", 0), result.truncated,
        )
    write_csv(config.OUT_DIR / "moves.csv", config.MOVE_COLUMNS,
              [[fmt(r.get(c, "")) for c in config.MOVE_COLUMNS] for r in moves])
    write_csv(config.OUT_DIR / "games.csv", config.GAME_COLUMNS,
              [[fmt(r.get(c, "")) for c in config.GAME_COLUMNS] for r in games])
    write_csv(config.OUT_DIR / "events.csv", config.EVENT_COLUMNS, events)
    write_csv(config.OUT_DIR / "verdicts.csv", config.VERDICT_COLUMNS, verdicts)
    write_csv(config.OUT_DIR / "extends.csv", config.EXTEND_COLUMNS, extends)
    write_csv(config.OUT_DIR / "ledger.csv", config.LEDGER_COLUMNS,
              [[fmt(v) for v in row] for row in ledger])
    write_csv(config.OUT_DIR / "bad-lines.csv", config.BADLINE_COLUMNS, badlines)
    (config.OUT_DIR / "ingest-state.json").write_text(
        json.dumps(state, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    log.info(
        "ingest exit arms=%d moves=%d games=%d bad=%d elapsed=%.2fs",
        len(ledger), len(moves), len(games), len(badlines), time.monotonic() - started,
    )


def main():
    setup_logging()
    allow = "--no-provisional" not in sys.argv
    run(allow_provisional=allow)


if __name__ == "__main__":
    main()
