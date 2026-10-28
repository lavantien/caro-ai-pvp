import re

import config

ARM_NAME_BODY = r"[\w.-]+"
TS = r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}"
QTY = r"\d+(?:\.\d+)?[km]?"
SCORE = r"[+-]?(?:\d+|M\d+)"
SIDE = r"master|easy|medium|hard"

ARM_START_RE = re.compile(rf"^({TS}) arm ({ARM_NAME_BODY}) start: (.+)$")
ARM_EXIT_RE = re.compile(rf"^({TS}) arm ({ARM_NAME_BODY}) exit (\d+)$")
ARM_ADOPTED_RE = re.compile(rf"^({TS}) arm ({ARM_NAME_BODY}) adopted$")
DESCRIPTOR_RE = re.compile(
    rf"^passgate master vs ({SIDE}) tc (\d+)\+(\d+) games (\d+)(?: blocks (\d+))? geometry (hex|square)$"
)
GAME_START_RE = re.compile(rf"^game (\d+): ({SIDE}) \(red\) vs ({SIDE}) \(blue\)$")
GAME_RESULT_RE = re.compile(
    rf"^game (\d+): ({SIDE}) \(red\) vs ({SIDE}), winner (red|blue|draw) over (\d+) moves$"
)
MLINE_RE = re.compile(
    rf"^g(\d+) M(\d+), (Red|Blue), ([A-Z]+\d+), "
    rf"d=(\d+), n=({QTY}), nps=({QTY}), ebf=(\d+(?:\.\d+)?), "
    rf"tt=(\d+)%, hf=(\d+)%, fh1=(\d+)%, s=({SCORE}), thr=(\d+), "
    rf"t=(\d+(?:\.\d+)?), alloc=(\d+(?:\.\d+)?)(?:, \[({"|".join(config.TAGS)})\])?, "
    rf"pv=(.*)$"
)
PONDER_RE = re.compile(rf"^g(\d+) ponder ({SIDE}) m(\d+) ({"|".join(config.PONDER_VERDICTS)})$")
EXTEND_BLOCK_RE = re.compile(
    r"^extend: block (\d+)/(\d+) n=(\d+) wd=(\d+(?:\.\d+)?) lb=(\d+(?:\.\d+)?) ub=(\d+(?:\.\d+)?)$"
)
EXTEND_NOTE_RE = re.compile(r"^extend: (.+)$")
CELL_TOKEN_RE = re.compile(r"^[A-Z]+\d+$")
VERDICT_RE = re.compile(
    r"^(hex-passgates): (DONE|FAILED)"
    r"(?: opp=(\w+) tc=(\d+) wd=(\d+(?:\.\d+)?) threshold=(\d+(?:\.\d+)?)"
    r"(?: n=(\d+) lb=(\d+(?:\.\d+)?) ub=(\d+(?:\.\d+)?) cap=(\w+))?)?$"
)

KIND_ARM_START = "arm-start"
KIND_ARM_EXIT = "arm-exit"
KIND_ARM_ADOPTED = "arm-adopted"
KIND_DESCRIPTOR = "descriptor"
KIND_GAME_START = "game-start"
KIND_GAME_RESULT = "game-result"
KIND_MLINE = "m-line"
KIND_PONDER = "ponder"
KIND_EXTEND_BLOCK = "extend-block"
KIND_EXTEND_NOTE = "extend-note"
KIND_VERDICT = "verdict"
KIND_BLANK = "blank"
KIND_BAD = "bad"
KIND_TRUNCATED = "truncated-tail"


def parse_compact(tok):
    body, scale = tok, 1
    if tok.endswith("k"):
        body, scale = tok[:-1], 1_000
    elif tok.endswith("m"):
        body, scale = tok[:-1], 1_000_000
    return int(round(float(body) * scale))


def parse_score(tok):
    sign = -1 if tok.startswith("-") else 1
    body = tok.lstrip("+-")
    if body.startswith("M"):
        dist = int(body[1:])
        return sign * (config.MATE_BASE - dist * config.MATE_STEP), sign * dist
    return sign * int(body), None


def parse_mline(line):
    m = MLINE_RE.match(line)
    if not m:
        return None
    pv_tokens = [] if m.group(17) == "" else m.group(17).split(" ")
    if any(not CELL_TOKEN_RE.match(tok) for tok in pv_tokens):
        return None
    score_cp, mate_dist = parse_score(m.group(12))
    return {
        "game": int(m.group(1)),
        "move_no": int(m.group(2)),
        "color": m.group(3),
        "cell": m.group(4),
        "depth": int(m.group(5)),
        "nodes": parse_compact(m.group(6)),
        "nps": parse_compact(m.group(7)),
        "ebf": float(m.group(8)),
        "tt_pct": int(m.group(9)),
        "hf_pct": int(m.group(10)),
        "fh1_pct": int(m.group(11)),
        "score_raw": m.group(12),
        "score_cp": score_cp,
        "mate_dist": mate_dist,
        "thr": int(m.group(13)),
        "t": float(m.group(14)),
        "alloc": float(m.group(15)),
        "tag": m.group(16) or "",
        "pv": m.group(17),
        "pv_len": len(pv_tokens),
    }


def parse_game_start(line):
    m = GAME_START_RE.match(line)
    if not m:
        return None
    return {"game": int(m.group(1)), "red": m.group(2), "blue": m.group(3)}


def parse_game_result(line):
    m = GAME_RESULT_RE.match(line)
    if not m:
        return None
    return {
        "game": int(m.group(1)),
        "red": m.group(2),
        "blue": m.group(3),
        "winner": m.group(4),
        "moves": int(m.group(5)),
    }


def parse_ponder(line):
    m = PONDER_RE.match(line)
    if not m:
        return None
    return {"game": int(m.group(1)), "side": m.group(2), "move_no": int(m.group(3)), "verdict": m.group(4)}


def parse_extend_block(line):
    m = EXTEND_BLOCK_RE.match(line)
    if not m:
        return None
    return {
        "block": int(m.group(1)),
        "max_blocks": int(m.group(2)),
        "n": int(m.group(3)),
        "wd": float(m.group(4)),
        "lb": float(m.group(5)),
        "ub": float(m.group(6)),
    }


def parse_verdict(line):
    m = VERDICT_RE.match(line)
    if not m:
        return None
    return {
        "status": m.group(2),
        "opp_field": m.group(3) or "",
        "tc_field": int(m.group(4)) if m.group(4) else None,
        "wd": float(m.group(5)) if m.group(5) else None,
        "threshold": float(m.group(6)) if m.group(6) else None,
        "n": int(m.group(7)) if m.group(7) else None,
        "lb": float(m.group(8)) if m.group(8) else None,
        "ub": float(m.group(9)) if m.group(9) else None,
        "cap": m.group(10) or "",
    }


def parse_descriptor(line):
    m = DESCRIPTOR_RE.match(line)
    if not m:
        return None
    return {
        "opp": m.group(1),
        "initial_min": int(m.group(2)),
        "inc_sec": int(m.group(3)),
        "games": int(m.group(4)),
        "blocks": int(m.group(5)) if m.group(5) else None,
        "geometry": m.group(6),
    }


def parse_arm_start(line):
    m = ARM_START_RE.match(line)
    if not m:
        return None
    return {"ts": m.group(1), "arm": m.group(2), "argv": m.group(3)}


def parse_arm_exit(line):
    m = ARM_EXIT_RE.match(line)
    if not m:
        return None
    return {"ts": m.group(1), "arm": m.group(2), "exit": int(m.group(3))}


def parse_arm_adopted(line):
    m = ARM_ADOPTED_RE.match(line)
    if not m:
        return None
    return {"ts": m.group(1), "arm": m.group(2)}


def classify(line):
    if line == "":
        return KIND_BLANK
    if ARM_START_RE.match(line):
        return KIND_ARM_START
    if ARM_EXIT_RE.match(line):
        return KIND_ARM_EXIT
    if ARM_ADOPTED_RE.match(line):
        return KIND_ARM_ADOPTED
    if line.startswith("passgate master vs "):
        return KIND_DESCRIPTOR
    if GAME_RESULT_RE.match(line):
        return KIND_GAME_RESULT
    if GAME_START_RE.match(line):
        return KIND_GAME_START
    if line.startswith("extend: "):
        if EXTEND_BLOCK_RE.match(line):
            return KIND_EXTEND_BLOCK
        if EXTEND_NOTE_RE.match(line):
            return KIND_EXTEND_NOTE
    if line.startswith("hex-passgates: "):
        return KIND_VERDICT
    if re.match(r"^g\d+ ponder ", line):
        return KIND_PONDER
    if re.match(r"^g\d+ M\d+, ", line):
        return KIND_MLINE
    return None
