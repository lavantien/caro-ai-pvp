# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). Levels are strength-based: depth caps L1=2 .. L5=50, VCF solver from L3 (solver depth ladder 2/4/12), parallel search from L3 (2 threads). Time fraction is a secondary cap.

Per-move statlines are logged during AI vs AI matches:

```
M 0 red  h9  d=4  n=185.3K  nps=124K  tt= 27% s=+10 thr=4 t=1.5s alloc=0.3s[VCF]
```

Format: `M<move> <player> <pos> d=<depth> n=<nodes> nps=<nps> tt=<hit%> s=<score> thr=<threads> t=<time> alloc=<allocated_time>[VCF][PONDER]`

- `thr`: number of search threads used
- `alloc`: time budget allocated by time manager (vs `t` which is actual elapsed)
- `[VCF]`: present when move was found by VCF solver (pre-search)
- `[PONDER]`: present when the opponent played the reply the mover had pondered

## Benchmark commands

```bash
node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
node scripts/uci-probe.mjs --mode parity   # fixed-position, fixed-depth probes
node scripts/uci-probe.mjs --mode speed    # single-thread NPS on a fixed position
```

The tournament runner randomizes openings (seeded), swaps colors every game, reports per-color and per-reason breakdowns with a 95% Wilson score interval, and writes `tournament-summary.json`.

## Go-era baseline (v8.1.0, 2026-08-21, archived)

Command: `node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821`
Source artifacts: `docs/artifacts/go-baseline/`

```
A (L1 Novice):       5/20
B (L5 Grandmaster): 16/20
Draws: 1 | Errored: 0
A win rate (decisive games): 23.8% 95% CI [10.6%, 45.1%]
Avg moves: 50.45
Avg time: 135.6s
```

### Typical statlines from the archived run

| Level | Depth | Nodes | Threads | Think time |
|-------|-------|-------|---------|------------|
| L5 mid-game | 4-14 | 0.4M-5M | 8 | 2-11s |
| L5 endgame (VCF converting) | 0 (solver) | 0 | 8 | <0.1s |
| L1 | 2 | 1K-150K | 1 | 0.05-0.6s |
