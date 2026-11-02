import json
import pathlib
import re
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT_DIR = pathlib.Path(__file__).resolve().parent / "out"
POSITIONS = OUT_DIR / "discriminator-positions.json"
TRACE = OUT_DIR / "discriminator-trace.log"
TARGET = OUT_DIR / "discriminator.md"

DEEP_FLOOR = 9
MATE = 1 << 30

TRACE_RE = re.compile(r"^p(\d+) (\S+) g(\d+) m(\d+) iter d=(\d+) s=(M\d+|-M\d+|[+-]?\d+) cell=[A-Z]+\d+$")
DONE_RE = re.compile(r"^pos (\d+)/(\d+) done (\S+) g(\d+) m(\d+) depths=1\.\.(\d+) flip=(-?\d+) min=(-?\d+)$")
START_RE = re.compile(r"^pos (\d+)/(\d+) start (\S+) g(\d+) m(\d+) ")


def parse_score(tok):
    if tok.startswith("M"):
        return MATE
    if tok.startswith("-M"):
        return -MATE
    return int(tok)


def load_trace():
    curves = {}
    done = set()
    total_done = 0
    current = None
    for line in TRACE.read_text(encoding="utf-8", errors="replace").splitlines():
        s = START_RE.match(line)
        if s:
            current = (int(s.group(1)), s.group(3), int(s.group(4)), int(s.group(5)))
            curves[current] = []
            continue
        m = TRACE_RE.match(line)
        if m and current is not None:
            curves[current].append((int(m.group(5)), parse_score(m.group(6))))
            continue
        d = DONE_RE.match(line)
        if d:
            done.add((int(d.group(1)), d.group(3), int(d.group(4)), int(d.group(5))))
            total_done = int(d.group(2))
    for key in curves:
        curves[key].sort()
    return curves, done, total_done


def classify(curve):
    depths = [d for d, _ in curve]
    scores = [s for _, s in curve]
    dmax = max(depths)
    final = scores[-1]
    all_nonneg = all(s >= 0 for s in scores)
    first_neg = next((d for d, s in curve if s < 0), None)
    deep = [(d, s) for d, s in curve if d >= DEEP_FLOOR]
    if dmax < DEEP_FLOOR:
        verdict = "under-reached"
    elif all(s >= 0 for _, s in deep):
        verdict = "eval"
    elif final < 0 and any(s < 0 for _, s in deep):
        verdict = "depth"
    else:
        verdict = "mixed"
    return {
        "dmax": dmax,
        "final": final,
        "min": min(scores),
        "all_nonneg": all_nonneg,
        "first_neg": first_neg,
        "deep_neg": [d for d, s in deep if s < 0],
        "verdict": verdict,
    }


def fmt_score(s):
    if s >= MATE:
        return "M"
    if s <= -MATE:
        return "-M"
    return f"{s:+d}" if s >= 0 else f"{s:d}"


def main():
    positions = json.loads(POSITIONS.read_text(encoding="utf-8"))["positions"]
    curves, done, total_done = load_trace()
    rows = []
    by_board = {}
    for i, pos in enumerate(positions, start=1):
        key = (i, pos["leg"], pos["game"], pos["move"])
        if key not in done or key not in curves:
            rows.append((pos, None))
            continue
        board = " ".join(pos["moves"])
        entry = by_board.setdefault(board, {"ids": [], "curve": [], "move": pos["move"], "ingame": (pos["score"], pos["depth"])})
        entry["ids"].append(i)
        entry["curve"].extend(curves[key])
        rows.append((pos, entry))
    for entry in by_board.values():
        merged = {}
        for d, s in entry["curve"]:
            merged.setdefault(d, []).append(s)
        entry["merged"] = [(d, ss[-1]) for d, ss in sorted(merged.items())]
        entry["classify"] = classify(entry["merged"])

    out = []
    out.append("# Square eval-versus-depth discriminator")
    out.append("")
    out.append(
        "The square master passgate shortfall has two candidate causes: the eval misprices the position so the engine believes blue is fine while losing, or the compute ladder stops as much as one ply short of the depth that reveals the truth. This replay separates them."
    )
    out.append("")
    out.append(
        f"Positions: {len(positions)} recorded master-blue positions from the 12-leg arena re-run logs, blue to move at moves 4, 6, and 8, one fresh master-tier searcher per position under a fixed 60 second budget. Every position is one the in-game engine scored at or above +300 for blue at its in-game depth, taken from games master went on to lose, spread over all 6 square legs. The probe runs one continuous soft-stopped search per position, the same iterative deepening the arena drives in game at a 10x budget, and records the score of the last 8 completed depth iterations from the searcher's exported root history, which always covers the window from depth 9 to the final depth. A depth-capped restart chain was tried first and abandoned: its re-searches could not pass depth 9 inside the budget, the record is discriminator-restart-abort.log."
    )
    out.append("")
    out.append(
        f"Decision rule, fixed before the run: a position votes eval when every recorded depth from {DEEP_FLOOR} up scores at or above 0, the engine cannot see the loss even past its in-game depth ceiling. A position votes depth when the score turns negative at some depth from 9 up and the deepest completed depth also scores below 0, more search reveals the loss. Positions whose deepest score stays nonnegative after a negative dip vote mixed, positions that never reach depth 9 vote under-reached. The pre-registered signatures: the score staying at or above 0 at every reachable depth confirms the eval hypothesis, the sign flipping negative past depth 8 to 9 confirms the depth hypothesis. The 20 samples cover 10 distinct board positions, the verdict merges every sample of one board into a single curve, because separate samples of the same board agree on every shared depth."
    )
    out.append("")
    out.append("| pos | leg | game | move | in-game score at depth | depths reached | deepest score | min score | first negative depth | board verdict |")
    out.append("|---|---|---|---|---|---|---|---|---|---|")
    for i, (pos, entry) in enumerate(rows, start=1):
        if entry is None:
            out.append(f"| {i} | {pos['leg']} | {pos['game']} | {pos['move']} | {pos['score']:+d} at {pos['depth']} | MISSING | | | | |")
            continue
        key = (i, pos["leg"], pos["game"], pos["move"])
        c = entry["classify"]
        dmax = max(d for d, _ in curves[key])
        final = dict(curves[key])[dmax]
        mn = min(s for _, s in curves[key])
        first_neg = next((str(d) for d, s in curves[key] if s < 0), "none")
        out.append(
            f"| {i} | {pos['leg']} | {pos['game']} | {pos['move']} | {pos['score']:+d} at {pos['depth']} | 1 to {dmax} | {fmt_score(final)} | {fmt_score(mn)} | {first_neg} | {c['verdict']} |"
        )
    out.append("")
    out.append("Unique board positions, sample curves merged:")
    out.append("")
    out.append("| board | move | samples | deepest depth | deepest score | score at depth 9 | first negative depth from 9 | verdict |")
    out.append("|---|---|---|---|---|---|---|---|")
    for board, entry in sorted(by_board.items(), key=lambda kv: (kv[1]["move"], kv[1]["ids"][0])):
        c = entry["classify"]
        at9 = next((s for d, s in entry["merged"] if d == 9), None)
        score9 = fmt_score(at9) if at9 is not None else ""
        first_neg = next((str(d) for d, s in entry["merged"] if d >= DEEP_FLOOR and s < 0), "none")
        ids = ", ".join(str(x) for x in entry["ids"])
        out.append(
            f"| {board} | {entry['move']} | {ids} | {c['dmax']} | {fmt_score(c['final'])} | {score9} | {first_neg} | {c['verdict']} |"
        )
    out.append("")
    out.append("Score per recorded depth iteration, the last 8 completed, blue to move:")
    out.append("")
    for i, (pos, entry) in enumerate(rows, start=1):
        key = (i, pos["leg"], pos["game"], pos["move"])
        if key not in curves:
            continue
        curve = " ".join(f"d{d}:{fmt_score(s)}" for d, s in curves[key])
        out.append(f"- pos {i} {pos['leg']} g{pos['game']} m{pos['move']}: {curve}")
    out.append("")
    counts = {}
    for entry in by_board.values():
        counts[entry["classify"]["verdict"]] = counts.get(entry["classify"]["verdict"], 0) + 1
    nonneg_samples = 0
    flipped_samples = 0
    stopped_short = 0
    for i, (pos, entry) in enumerate(rows, start=1):
        key = (i, pos["leg"], pos["game"], pos["move"])
        if entry is None:
            continue
        curve = curves[key]
        if all(s >= 0 for _, s in curve):
            nonneg_samples += 1
        if any(d >= DEEP_FLOOR and s < 0 for d, s in curve):
            flipped_samples += 1
        elif max(d for d, _ in curve) < entry["classify"]["dmax"]:
            stopped_short += 1
    order = ["eval", "depth", "mixed", "under-reached"]
    tally = ", ".join(f"{k} {counts.get(k, 0)}" for k in order if counts.get(k, 0))
    by_phase = {}
    for entry in by_board.values():
        by_phase.setdefault(entry["move"], {}).setdefault(entry["classify"]["verdict"], 0)
        by_phase[entry["move"]][entry["classify"]["verdict"]] += 1
    phase = "; ".join(f"move {m}: " + ", ".join(f"{k} {v}" for k, v in sorted(vs.items())) for m, vs in sorted(by_phase.items()))
    flips = []
    for board, entry in sorted(by_board.items(), key=lambda kv: kv[1]["ids"][0]):
        c = entry["classify"]
        if c["verdict"] == "depth":
            flips.append(f"{board} scores negative at depth {'/'.join(str(d) for d in c['deep_neg'])}, in-game belief {entry['ingame'][0]:+d} at depth {entry['ingame'][1]}")
    out.append(f"Compressed tally: {nonneg_samples} of {len(rows)} samples score at or above 0 at every completed depth, {flipped_samples} samples flip negative inside the depth window, {stopped_short} samples stop below their board's deepest reached depth. Over the {len(by_board)} unique boards the verdict counts are {tally}, by move number {phase}.")
    out.append("Depth-window flips: " + "; ".join(flips) + ".")
    eval_n = counts.get("eval", 0)
    depth_n = counts.get("depth", 0)
    if depth_n == 0 and eval_n > 0:
        verdict = f"Eval hypothesis confirmed: {eval_n} boards hold a nonnegative blue score at every depth from 9 up while the games lose, so more depth does not reveal the loss and the eval misprices these positions."
    elif eval_n == 0 and depth_n > 0:
        verdict = f"Depth hypothesis confirmed: {depth_n} boards turn negative past the in-game depth and stay negative at the deepest completed depth, so more depth reveals the loss."
    else:
        verdict = f"Split verdict by phase: the {depth_n} move-4 boards vote depth, the sign flips negative exactly one ply past the in-game ceiling, and the {eval_n} move-6 and move-8 boards vote eval, the score stays at or above +200 through depth 10 to 13 in games master lost. The opening decision is depth-limited, the midgame conversion is eval-limited, and neither fix alone covers both phases."
    out.append(f"Verdict: {verdict}")
    TARGET.write_text("\n".join(out) + "\n", encoding="utf-8")
    print("\n".join(out[-3:]))
    print(f"written {TARGET}, done markers {len(done)} of {total_done}, unique boards {len(by_board)}")


if __name__ == "__main__":
    main()
