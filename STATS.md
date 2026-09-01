# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). Levels are strength-based: depth caps L1=2 .. L5=50, VCF solver from L3, parallel search from L4. Time fraction is a secondary cap.

Per-move statlines are logged during AI vs AI matches:

```
M 0 red  h9  d=4  n=185.3K  nps=124K  tt= 27% s=+10 thr=4 t=1.5s alloc=0.3s[VCF]
```

Format: `M<move> <player> <pos> d=<depth> n=<nodes> nps=<nps> tt=<hit%> s=<score> thr=<threads> t=<time> alloc=<allocated_time>[VCF]`

- `thr`: number of search threads used
- `alloc`: time budget allocated by time manager (vs `t` which is actual elapsed)
- `[VCF]`: present when move was found by VCF solver (pre-search)

## Benchmark commands

```bash
node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
node scripts/uci-probe.mjs --mode parity   # fixed-position, fixed-depth probes
node scripts/uci-probe.mjs --mode speed    # single-thread NPS on a fixed position
```

The tournament runner randomizes openings (seeded), swaps colors every game, reports per-color and per-reason breakdowns with a 95% Wilson score interval, and writes `tournament-summary.json`.

## C#/.NET 10 port baseline (v9.0.0, 2026-09-01)

Command: `node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821`
Source artifacts: `docs/artifacts/csharp-port/`

```
A (L1 Novice):       7/20
B (L5 Grandmaster): 14/20
Draws: 1 | Errored: 0
Red color wins: 14 | Blue color wins: 5
End reasons: 19 x win, 1 x max-moves
A win rate (decisive games): 33.3% 95% CI [17.2%, 54.6%]
Avg moves: 43.6
Avg time: 83.8s
```

L5 wins the matchup clearly; the interval covers the Go-era baseline below,
so the port shows no significant strength change at this sample size.

UCI probes (single thread, hash 256, Release build):

| Probe | Result |
|-------|--------|
| Fixed-depth parity, 4 positions x depths 4/8 | depth-4 bestmove + score match the Go engine exactly; depth-8 matches on bestmove for 7/8 positions, the last differs by one aspiration re-search (score within ~30cp) |
| Depth-9 nodes on midgame-quiet | 4,148,684 vs Go 4,119,518 (+0.7%), identical across 3 runs |
| Single-thread NPS | ~55K vs Go ~73K (0.76x) |
| 8-thread NPS (L5 midgame) | ~227K, inside the Go range (90-260K) after swapping the per-shard reader-writer lock for a plain monitor |

### Interpretation notes

- Unstable sorts (C# `List.Sort` vs Go pdqsort) reorder equal-scored moves, so
  node counts drift slightly; fixed-depth root moves agree except where an
  aspiration-window re-search opens a different path first.
- The single-thread gap (0.76x) is JIT-vs-AOT on the branchy hot loop; the
  tournament result shows it does not change outcomes at these time controls.
- Red's 14-5 color edge is the first-move advantage on a 16x16 board;
  color-swapped pairs cancel it in the A/B totals.

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
