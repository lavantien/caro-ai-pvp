import json
import pathlib
import sys

GATEFORENSIC_DIR = pathlib.Path(__file__).resolve().parents[1] / "gateforensic"
sys.path.insert(0, str(GATEFORENSIC_DIR))

import parse as gf

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
ARENA_OUT = REPO_ROOT / "playground" / "arena" / "out"
OUT_DIR = pathlib.Path(__file__).resolve().parent / "out"

LEGS = [
    ("square-hard-tc1", 4),
    ("square-hard-tc2", 4),
    ("square-medium-tc1", 4),
    ("square-medium-tc2", 4),
    ("square-easy-tc1", 2),
    ("square-easy-tc2", 2),
]
MOVES = (4, 6, 8)


def load_games(name):
    lines = (ARENA_OUT / name).read_text(encoding="utf-8", errors="replace").splitlines()
    games = {}
    order = []
    for line in lines:
        start = gf.parse_game_start(line)
        if start:
            games[start["game"]] = {
                "game": start["game"],
                "red": start["red"],
                "blue": start["blue"],
                "moves": [],
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
        res = gf.parse_game_result(line)
        if res:
            games[res["game"]]["result"] = res["winner"]
    return [games[n] for n in order if games[n]["result"]]


def eligible(m):
    return m["move_no"] in MOVES and m["color"] == "Blue" and m["tag"] in ("", "PONDER") and m["score_cp"] >= 0


def first_match(pool, target, used, fresh_game_only):
    for g in pool:
        if (g["game"], target) in used:
            continue
        if fresh_game_only and any(k[0] == g["game"] for k in used):
            continue
        for m in g["moves"]:
            if m["move_no"] == target and eligible(m):
                return (g, m)
    return None


def pick(pool, quota):
    chosen = []
    used = set()
    while len(chosen) < quota:
        rot = len(chosen) % len(MOVES)
        rotation = MOVES[rot:] + MOVES[:rot]
        found = None
        for fresh in (True, False):
            if found:
                break
            for target in rotation:
                found = first_match(pool, target, used, fresh)
                if found:
                    break
        if not found:
            break
        g, m = found
        used.add((g["game"], m["move_no"]))
        chosen.append((g, m))
    return chosen


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    positions = []
    for leg, quota in LEGS:
        games = [g for g in load_games("arena-rerun-" + leg + ".log") if g["blue"] == "master"]
        pool = [g for g in games if g["result"] == "blue"]
        filled = 0
        for g, m in pick(pool, quota):
            positions.append({
                "leg": leg,
                "game": g["game"],
                "move": m["move_no"],
                "pool": "W",
                "cell": m["cell"],
                "score": m["score_cp"],
                "depth": m["depth"],
                "tag": m["tag"],
                "moves": [x["cell"] for x in g["moves"] if x["move_no"] < m["move_no"]],
            })
            filled += 1
        print(f"{leg}: {filled}/{quota} W positions from {len(pool)} master-blue wins")
    text = json.dumps({"positions": positions}, indent=2) + "\n"
    (OUT_DIR / "wpool-positions.json").write_text(text, encoding="utf-8")
    print(f"total {len(positions)} positions written to out/wpool-positions.json")
    by_move = {}
    for p in positions:
        by_move[p["move"]] = by_move.get(p["move"], 0) + 1
    print(f"by move: {sorted(by_move.items())}")


if __name__ == "__main__":
    main()
