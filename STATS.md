# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). Levels are strength-based: depth caps L1=2 .. L5=50, VCF solver from L3 (solver depth ladder 2/4/12), parallel search from L3 (2 threads). Time fraction is a secondary cap.

Per-move statlines are logged during AI vs AI matches:

```
M 0 red  h9  d=4  n=185.3K  nps=124K  tt= 27% s=+10 thr=4 t=1.5s alloc=0.3s[VCF]
```

Format: `M<move> <player> <pos> d=<depth> n=<nodes> nps=<nps> tt=<hit%> s=<score> thr=<threads> t=<time> alloc=<allocated_time>[VCF][PONDER]`

- `thr`: number of search threads used
- `alloc`: time budget allocated by time manager (vs `t` which is actual elapsed)
- `[VCF]`: present when move was found by VCF solver (pre-search); these moves log `d=0 n=0` because the solver short-circuits alpha-beta
- `[PONDER]`: present when the opponent played the reply the mover had pondered

## Benchmark commands

```bash
node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
node scripts/uci-probe.mjs --mode parity   # fixed-position, fixed-depth probes
node scripts/uci-probe.mjs --mode speed    # single-thread NPS on a fixed position
```

The tournament runner randomizes openings (seeded), swaps colors every game, reports per-color and per-reason breakdowns with a 95% Wilson score interval, and writes `tournament-summary.json`.

## Round-robin benchmark

```bash
node scripts/run-round-robin.mjs                                      # full run: 12 pairings x 20 games, 3+2
node scripts/run-round-robin.mjs --pairings 1v1 --games-per-pairing 2 --tc 1+0 --label smoke-l1v1
```

Fixed pairing order: 1v1 first (fail-fast smoke pairing), then every cross pairing strong-vs-weak first (1v5, 1v4, ... 4v5), 5v5 last (calibration). Colors swap every second game, game N uses opening seed `base + N` (default base 20260821), and games run sequentially: parallel games would contaminate the wall-clock columns being probed. Builds Release by default (`--build Debug` overrides; Debug is roughly 4x slower and gates soft-budget iterations one depth earlier).

Each run writes `docs/artifacts/tournaments/<label>/`:

- `run.log`: every banner and statline verbatim, plus the backend `move-statline` log lines (double evidence per move)
- `summary.json`: schema-versioned source data: per-pairing games, per-level side aggregates (depth/nodes/nps/tt, VCF chain stats, ponder depth/hits), ladder with adjacent-level steps, totals, anomalies
- `report.md`: rendered report: run provenance, determinism statement, ladder, monotonicity verdict, per-pairing engine side-by-side tables
- `matches.db`: sqlite archive of every game and move (gitignored; the three text artifacts are committed)

Fail-fast contract: any game failure other than a legitimate timeout adjudication aborts the run (exit 1, `status: "aborted"`, partial artifacts kept). A flagged player surfaces as HTTP 400 on the next move request, so the runner confirms against a state GET before recording the result. Timeout-fallback moves, draws, and max-move games are counted as anomalies, not failures. `summary.json` and `report.md` are rewritten atomically after every completed pairing, so a crashed long run keeps its partial evidence.

Determinism: openings, pairing order, color assignment, and scheduling are deterministic (`seed = base + global game index`). The `t=`/`nps=` columns and the achieved depth under a soft time budget are wall-clock evidence that vary with machine load, and node counts for L3+ (Lazy SMP) vary run to run; L1/L2 are depth-capped and fully deterministic. Probe fields (`ponderDepth`/`ponderNodes`/`ponderHit`, `vcfDepth`/`vcfNodes`) ride in the `ai-move` response without changing statline bytes.

Duration: a full 240-game run at 3+2 takes roughly 4-10 hours on a desktop; extrapolate from a filtered smoke run first. Committed examples: `docs/artifacts/tournaments/smoke-l1v1/` and `smoke-l5v5/`.
