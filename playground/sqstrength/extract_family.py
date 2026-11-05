import json
import pathlib
import sys

GATEFORENSIC_DIR = pathlib.Path(__file__).resolve().parents[1] / "gateforensic"
sys.path.insert(0, str(GATEFORENSIC_DIR))

import parse as gf

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
ARENA_OUT = REPO_ROOT / "playground" / "arena" / "out"
OUT_DIR = pathlib.Path(__file__).resolve().parent / "out"

MOVES = (4, 6, 8, 10, 12, 14, 16)

REGATE2_LOSS = [
    ("square-easy-tc2", 1, "regate2-loss-master"),
    ("square-easy-tc2", 9, "regate2-loss-master"),
    ("square-easy-tc2", 26, "regate2-loss-easy"),
    ("square-easy-tc2", 31, "regate2-loss-master"),
    ("square-easy-tc2", 43, "regate2-loss-master"),
    ("square-easy-tc2", 57, "regate2-loss-master"),
    ("square-medium-tc2", 1, "regate2-loss-master"),
    ("square-medium-tc2", 2, "regate2-loss-medium"),
    ("square-medium-tc2", 3, "regate2-loss-master"),
    ("square-medium-tc2", 9, "regate2-loss-master"),
    ("square-medium-tc2", 27, "regate2-loss-master"),
    ("square-medium-tc2", 47, "regate2-loss-master"),
]

REGATE2_WIN = [
    ("square-easy-tc2", 5, "regate2-win-red"),
    ("square-easy-tc2", 17, "regate2-win-red"),
    ("square-easy-tc2", 21, "regate2-win-red"),
    ("square-easy-tc2", 25, "regate2-win-red"),
    ("square-easy-tc2", 33, "regate2-win-red"),
    ("square-easy-tc2", 2, "regate2-win-blue"),
    ("square-easy-tc2", 4, "regate2-win-blue"),
    ("square-easy-tc2", 8, "regate2-win-blue"),
    ("square-medium-tc2", 5, "regate2-win-red"),
    ("square-medium-tc2", 11, "regate2-win-red"),
    ("square-medium-tc2", 13, "regate2-win-red"),
    ("square-medium-tc2", 19, "regate2-win-red"),
    ("square-medium-tc2", 21, "regate2-win-red"),
    ("square-medium-tc2", 6, "regate2-win-blue"),
    ("square-medium-tc2", 8, "regate2-win-blue"),
    ("square-medium-tc2", 16, "regate2-win-blue"),
    ("square-medium-tc2", 18, "regate2-win-blue"),
]

FEEDBOARD = ("square-easy-tc1", 2, "feedboard", "g2-m10")


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
    return games


def positions_for(log_name, picks):
    games = load_games(log_name)
    positions = []
    for leg, game, pool in picks:
        g = games[game]
        for m in g["moves"]:
            if m["move_no"] in MOVES and m["color"] == "Blue":
                positions.append({
                    "leg": leg,
                    "game": game,
                    "move": m["move_no"],
                    "pool": pool,
                    "cell": m["cell"],
                    "score": m["score_cp"],
                    "depth": m["depth"],
                    "tag": m["tag"],
                    "moves": [x["cell"] for x in g["moves"] if x["move_no"] < m["move_no"]],
                })
    return positions


def regate2_positions(picks):
    positions = []
    for leg in ("square-easy-tc2", "square-medium-tc2"):
        leg_picks = [p for p in picks if p[0] == leg]
        positions.extend(positions_for("arena-regate2-" + leg + ".log", leg_picks))
    return positions


def feedboard_position():
    leg, game, pool, label = FEEDBOARD
    positions = positions_for("arena-rerun-" + leg + ".log", [(leg, game, pool)])
    out = []
    for p in positions:
        if p["move"] != 10:
            continue
        p["leg"] = label
        out.append(p)
    return out


def write_positions(name, positions):
    text = json.dumps({"positions": positions}, indent=2) + "\n"
    (OUT_DIR / name).write_text(text, encoding="utf-8")
    print(f"{name}: {len(positions)} positions")


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    write_positions("family-loss-positions.json", regate2_positions(REGATE2_LOSS))
    write_positions("family-win-positions.json", regate2_positions(REGATE2_WIN))
    write_positions("family-m10-positions.json", feedboard_position())


if __name__ == "__main__":
    main()
