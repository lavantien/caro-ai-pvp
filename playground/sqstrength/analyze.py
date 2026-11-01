import pathlib
import statistics
import sys

GATEFORENSIC_DIR = pathlib.Path(__file__).resolve().parents[1] / "gateforensic"
sys.path.insert(0, str(GATEFORENSIC_DIR))

import parse as gf

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT_DIR = pathlib.Path(__file__).resolve().parent / "out"
ARENA_OUT = REPO_ROOT / "playground" / "arena" / "out"

ARMS = [
    ("square-hard-tc1", "square", "hard", "arena-rerun-square-hard-tc1.log"),
    ("square-hard-tc2", "square", "hard", "arena-rerun-square-hard-tc2.log"),
    ("square-medium-tc1", "square", "medium", "arena-rerun-square-medium-tc1.log"),
    ("square-medium-tc2", "square", "medium", "arena-rerun-square-medium-tc2.log"),
    ("square-easy-tc1", "square", "easy", "arena-rerun-square-easy-tc1.log"),
    ("square-easy-tc2", "square", "easy", "arena-rerun-square-easy-tc2.log"),
    ("hex-hard-tc1", "hex", "hard", "arena-rerun-hex-hard-tc1.log"),
    ("hex-hard-tc2", "hex", "hard", "arena-rerun-hex-hard-tc2.log"),
]

MATE = 1 << 30
MATE_ZONE = 1_000_000_000
RECOVER = -300
DROP_FROM = 1000
DROP_TO = -3000
EARLY = 20
MID = 40
OPENING_PAIRS = 4


def verdict_of(lines, geometry):
    for line in lines:
        if line.startswith(f"{geometry}-passgates: "):
            return line
    return ""


def load_arm(label, geometry, opp, name):
    path = ARENA_OUT / name
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    games = {}
    order = []
    red = blue = ""
    for line in lines:
        start = gf.parse_game_start(line)
        if start:
            red, blue = start["red"], start["blue"]
            games[start["game"]] = {
                "game": start["game"],
                "red": red,
                "blue": blue,
                "master_color": "Red" if red == "master" else "Blue",
                "moves": [],
                "ponder": [],
                "result": "",
            }
            order.append(start["game"])
            continue
        mv = gf.parse_mline(line)
        if mv:
            g = games.get(mv["game"])
            if g:
                g["moves"].append(mv)
            continue
        pd = gf.parse_ponder(line)
        if pd:
            g = games.get(pd["game"])
            if g:
                g["ponder"].append(pd)
            continue
        res = gf.parse_game_result(line)
        if res:
            g = games.get(res["game"])
            if g:
                g["result"] = res["winner"]
                g["length"] = res["moves"]
    out = [games[n] for n in order if games[n]["result"]]
    return {
        "label": label,
        "geometry": geometry,
        "opp": opp,
        "verdict": verdict_of(lines, geometry),
        "games": out,
    }


def is_mate_for(s):
    return s >= MATE_ZONE


def is_mate_against(s):
    return s <= -MATE_ZONE


def master_moves(g):
    color = g["master_color"]
    return [m for m in g["moves"] if m["color"] == color]


def opp_moves(g):
    color = g["master_color"]
    return [m for m in g["moves"] if m["color"] != color]


def master_result(g):
    if g["result"] == "draw":
        return "D"
    if (g["result"] == "red") == (g["master_color"] == "Red"):
        return "W"
    return "L"


def flip_index(ms):
    for i, m in enumerate(ms):
        if m["score_cp"] < RECOVER:
            return i
    return None


def turn_index(ms):
    last = None
    for i, m in enumerate(ms):
        if m["score_cp"] < RECOVER:
            last = i
    if last is None:
        return None
    for i in range(last, len(ms)):
        if ms[i]["score_cp"] >= RECOVER:
            return None
    return last


def classify_loss(g):
    ms = master_moves(g)
    os_ = opp_moves(g)
    flip = flip_index(ms)
    turn = turn_index(ms)
    mate_before_flip = False
    one_ply = False
    if flip is not None:
        for j in range(max(0, flip - 3), flip):
            if is_mate_for(ms[j]["score_cp"]):
                mate_before_flip = True
    for k in range(len(ms) - 1):
        if is_mate_for(ms[k]["score_cp"]) and is_mate_against(ms[k + 1]["score_cp"]):
            one_ply = True
    self_mate_first = flip is not None and is_mate_against(ms[flip]["score_cp"])
    kill_tag = os_[-1]["tag"] if os_ else ""
    eval_crash = False
    for k in range(len(ms) - 1):
        a, b = ms[k]["score_cp"], ms[k + 1]["score_cp"]
        if a >= DROP_FROM and b <= DROP_TO and not is_mate_for(a) and not is_mate_against(b):
            eval_crash = True
    ponder_at_flip = False
    if turn is not None:
        for m in ms[turn:]:
            if m["tag"] == "PONDER":
                ponder_at_flip = True
                break
    if one_ply:
        kind = "one-ply mate reversal"
    elif self_mate_first:
        kind = "first sight was mate-on-self"
    elif mate_before_flip:
        kind = "was-mating then collapsed"
    elif eval_crash:
        kind = "eval crash no mate"
    else:
        kind = "slow bleed"
    return {
        "kind": kind,
        "flip_move": ms[flip]["move_no"] if flip is not None else 0,
        "turn_move": ms[turn]["move_no"] if turn is not None else 0,
        "mate_before_flip": mate_before_flip,
        "one_ply": one_ply,
        "kill_tag": kill_tag.replace(", [", "").replace("]", ""),
        "ponder_at_flip": ponder_at_flip,
        "length": g["length"],
    }


def seat_stats(games):
    rows = {}
    for seat in ("Red", "Blue"):
        sel = [g for g in games if g["master_color"] == seat]
        w = sum(1 for g in sel if master_result(g) == "W")
        l = sum(1 for g in sel if master_result(g) == "L")
        d = sum(1 for g in sel if master_result(g) == "D")
        rows[seat] = {"n": len(sel), "w": w, "l": l, "d": d}
    return rows


def tagged_moves(ms, tag):
    return sum(1 for m in ms if m["tag"] == tag)


def search_moves(ms):
    return [m for m in ms if m["tag"] not in ("BOOK", "PONDER")]


def ponder_tally(g):
    arms = adopts = 0
    for p in g["ponder"]:
        if p["side"] == "master" and p["verdict"] in ("adopt", "reject"):
            arms += 1
            if p["verdict"] == "adopt":
                adopts += 1
    return arms, adopts


def phase_shares(g):
    zones = {"early": [0, 0], "mid": [0, 0], "late": [0, 0]}
    mating = {"early": 0, "mid": 0, "late": 0}
    mated = {"early": 0, "mid": 0, "late": 0}
    for m in master_moves(g):
        no = m["move_no"]
        zone = "early" if no <= EARLY else ("mid" if no <= MID else "late")
        zones[zone][0] += 1
        if m["score_cp"] > 500:
            zones[zone][1] += 1
        if is_mate_for(m["score_cp"]):
            mating[zone] += 1
        if is_mate_against(m["score_cp"]):
            mated[zone] += 1
    return zones, mating, mated


def proof_claims(g, who):
    if who == "master":
        ms = master_moves(g)
    else:
        ms = opp_moves(g)
    claims = []
    for i, m in enumerate(ms):
        if m["tag"] in ("VCF", "VCT") and is_mate_for(m["score_cp"]):
            claims.append((i, m))
    return claims


def proof_stats(g, who):
    if who == "master":
        ms = master_moves(g)
    else:
        ms = opp_moves(g)
    claims = proof_claims(g, who)
    collapsed = 0
    for i, m in claims:
        for j in range(i + 1, min(i + 3, len(ms))):
            if ms[j]["score_cp"] <= -10000 or is_mate_against(ms[j]["score_cp"]):
                collapsed += 1
                break
    unconverted = 0
    if who == "master" and master_result(g) == "L" and claims:
        unconverted = len(claims)
    return len(claims), collapsed, unconverted


def line_table(games, holder, seat):
    table = {}
    for g in games:
        holds = g["master_color"] == seat
        if holds != (holder == "master"):
            continue
        if holder == "master":
            ms = master_moves(g)
            res = master_result(g)
        else:
            ms = opp_moves(g)
            if g["result"] == "draw":
                res = "D"
            elif (g["result"] == "red") == (seat == "Red"):
                res = "W"
            else:
                res = "L"
        if not ms:
            continue
        head = " ".join(m["cell"] for m in ms[:OPENING_PAIRS])
        e = table.setdefault(head, [0, 0, 0])
        e[0 if res == "W" else (1 if res == "D" else 2)] += 1
    return table


def adopt_quality(games):
    stats = {}
    for seat in ("master", "hard", "medium", "easy"):
        deltas = []
        adopts = arms = 0
        for g in games:
            color = "Red" if (g["master_color"] == "Red") == (seat == "master") else "Blue"
            ms = [m for m in g["moves"] if m["color"] == color]
            hit = set()
            for p in g["ponder"]:
                if p["side"] != seat:
                    continue
                if p["verdict"] in ("adopt", "reject"):
                    arms += 1
                if p["verdict"] == "adopt":
                    hit.add(p["move_no"])
                    adopts += 1
            for i in range(1, len(ms)):
                if ms[i]["move_no"] in hit and i > 0:
                    a = ms[i - 1]["score_cp"]
                    b = ms[i]["score_cp"]
                    if abs(a) < MATE_ZONE and abs(b) < MATE_ZONE:
                        deltas.append(b - a)
        if arms:
            stats[seat] = (adopts, arms, deltas)
    return stats


def fmt(x):
    return f"{x:.3f}"


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    out = []
    for label, geometry, opp, name in ARMS:
        arm = load_arm(label, geometry, opp, name)
        games = arm["games"]
        if not games:
            out.append(f"## {label}: no completed games")
            continue
        out.append(f"## {label}")
        out.append(f"verdict: {arm['verdict'] or 'none yet'}")
        seats = seat_stats(games)
        for seat, r in seats.items():
            wd = (r["w"] + r["d"]) / r["n"] if r["n"] else 0
            out.append(
                f"seat {seat}: n={r['n']} W{r['w']} D{r['d']} L{r['l']} wd={fmt(wd)}"
            )
        losses = [g for g in games if master_result(g) == "L"]
        kinds = {}
        for g in losses:
            c = classify_loss(g)
            kinds[c["kind"]] = kinds.get(c["kind"], 0) + 1
        out.append(f"loss taxonomy over {len(losses)} losses: " + ", ".join(f"{k} x{v}" for k, v in sorted(kinds.items())))
        by_seat = {"Red": [], "Blue": []}
        for g in losses:
            c = classify_loss(g)
            c["seat"] = g["master_color"]
            by_seat[g["master_color"]].append(c)
        for seat, cs in by_seat.items():
            if not cs:
                continue
            flips = sorted(c["flip_move"] for c in cs if c["flip_move"])
            one_ply = sum(1 for c in cs if c["one_ply"])
            self_mate = sum(1 for c in cs if c["kind"] == "first sight was mate-on-self")
            kill_tags = {}
            for c in cs:
                if c["kill_tag"]:
                    kill_tags[c["kill_tag"]] = kill_tags.get(c["kill_tag"], 0) + 1
            med = statistics.median(flips) if flips else 0
            early = sum(1 for f in flips if f <= EARLY)
            pond = sum(1 for c in cs if c["ponder_at_flip"])
            out.append(
                f"losses as {seat}: n={len(cs)} one-ply={one_ply} first-self-mate={self_mate} "
                f"flip median={med} early-flips={early} ponder-at-flip={pond} kill-tags={kill_tags}"
            )
        opens = {"Red": {}, "Blue": {}}
        early_scores = {"Red": [], "Blue": []}
        for g in games:
            ms = master_moves(g)
            if not ms:
                continue
            seat = g["master_color"]
            head = " ".join(m["cell"] for m in ms[:OPENING_PAIRS])
            opens[seat][head] = opens[seat].get(head, 0) + 1
            early_scores[seat].extend(m["score_cp"] for m in ms[:3] if m["tag"] not in ("BOOK",))
        for seat in ("Red", "Blue"):
            top = sorted(opens[seat].items(), key=lambda kv: -kv[1])[:6]
            out.append(f"openings as {seat}: " + ", ".join(f"{k} x{v}" for k, v in top))
        for seat in ("Red", "Blue"):
            sel = [g for g in games if g["master_color"] == seat]
            for res in ("W", "L"):
                vals = []
                for g in sel:
                    if master_result(g) != res:
                        continue
                    ms = master_moves(g)
                    vals.extend(m["score_cp"] for m in ms[:3])
                if vals:
                    out.append(
                        f"master first-3-move scores as {seat} result {res}: mean={int(statistics.mean(vals))} min={min(vals)}"
                    )
        for seat in ("Red", "Blue"):
            ms_all, os_all = [], []
            for g in games:
                if g["master_color"] != seat:
                    continue
                ms_all.extend(search_moves(master_moves(g)))
                os_all.extend(search_moves(opp_moves(g)))
            if ms_all:
                out.append(
                    f"search parity as {seat}: master d={fmt(statistics.mean(m['depth'] for m in ms_all))} "
                    f"n={int(statistics.mean(m['nodes'] for m in ms_all))} "
                    f"opp d={fmt(statistics.mean(m['depth'] for m in os_all))} "
                    f"n={int(statistics.mean(m['nodes'] for m in os_all))}"
                )
        book = sum(tagged_moves(master_moves(g), "BOOK") for g in games)
        book_blue = sum(
            1 for g in games for m in master_moves(g) if m["tag"] == "BOOK" and g["master_color"] == "Blue"
        )
        arms_p = adopts_p = 0
        for g in games:
            a, ad = ponder_tally(g)
            arms_p += a
            adopts_p += ad
        out.append(f"master book moves total={book} of which as blue={book_blue}")
        out.append(
            f"master ponder adopt {adopts_p}/{arms_p}" if arms_p else "master ponder adopt 0/0"
        )
        for seat in ("Red", "Blue"):
            zones = {"early": [0, 0], "mid": [0, 0], "late": [0, 0]}
            mating = {"early": 0, "mid": 0, "late": 0}
            mated = {"early": 0, "mid": 0, "late": 0}
            for g in games:
                if g["master_color"] != seat:
                    continue
                z, ma, md = phase_shares(g)
                for k in zones:
                    zones[k][0] += z[k][0]
                    zones[k][1] += z[k][1]
                    mating[k] += ma[k]
                    mated[k] += md[k]
            parts = []
            for k in ("early", "mid", "late"):
                n, ahead = zones[k]
                share = fmt(ahead / n) if n else "n/a"
                parts.append(f"{k}: ahead {share} mating {mating[k]} mated {mated[k]} over {n}")
            out.append(f"master phase shares as {seat}: " + "; ".join(parts))
        overrun = starve = 0
        for g in games:
            for m in search_moves(master_moves(g)):
                if m["alloc"] > 0:
                    if m["t"] > 1.25 * m["alloc"]:
                        overrun += 1
                    if m["t"] < 0.5 * m["alloc"]:
                        starve += 1
        out.append(f"master move timing: overruns={overrun} early-stops={starve}")
        for who in ("master", "opp"):
            claims = collapsed = unconverted = 0
            for g in games:
                c, co, u = proof_stats(g, who)
                claims += c
                collapsed += co
                unconverted += u
            out.append(
                f"{who} solver mate claims={claims} collapsed-within-2={collapsed}"
                + (f" unconverted-in-losses={unconverted}" if who == "master" else "")
            )
        if opp == "hard":
            for holder in ("master", "opp"):
                for seat in ("Blue", "Red"):
                    table = line_table(games, holder, seat)
                    rows = sorted(table.items(), key=lambda kv: -sum(kv[1]))[:8]
                    body = ", ".join(f"{k} {v[0]}-{v[1]}-{v[2]}" for k, v in rows)
                    out.append(f"{holder} as {seat} line W-D-L: {body}")
        tag_counts = {}
        for g in games:
            for m in g["moves"]:
                if m["tag"]:
                    tag_counts[m["tag"]] = tag_counts.get(m["tag"], 0) + 1
        out.append(f"move tags both engines: {tag_counts}")
        for seat, (adopts, arms, deltas) in adopt_quality(games).items():
            if deltas:
                neg = sum(1 for d in deltas if d <= -500)
                out.append(
                    f"ponder {seat}: {adopts}/{arms} adopt, adopted-move own-score delta mean={int(statistics.mean(deltas))} drops<=-500 {neg}/{len(deltas)}"
                )
            else:
                out.append(f"ponder {seat}: {adopts}/{arms} adopt")
        out.append("")
    text = "\n".join(out) + "\n"
    target = OUT_DIR / "taxonomy.md"
    target.write_text(text, encoding="utf-8")
    print(text)


if __name__ == "__main__":
    main()
