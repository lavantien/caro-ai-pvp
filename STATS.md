# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). Levels are strength-based: depth caps L1=2 .. L5=50, VCF solver from L3, parallel search from L4. Time fraction is a secondary cap.

Per-move statlines are logged during AI vs AI matches:

```
M 0 red  h9  d=4  n=185.3K  nps=124K  tt= 27% s=+10 thr=4 t=1.5s alloc=0.3s[VCF]
```

Format: `M<move> <player> <pos> d=<depth> n=<nodes> nps=<nps> tt=<hit%> s=<score> thr=<threads> t=<time> alloc=<allocated_time>[VCF]`

- `thr`: number of search goroutines used
- `alloc`: time budget allocated by time manager (vs `t` which is actual elapsed)
- `[VCF]`: present when move was found by VCF solver (pre-search)

## Benchmark commands

```bash
node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+0 --seed 20260821
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
```

The tournament runner randomizes openings (seeded), swaps colors every game, reports per-color and per-reason breakdowns with a 95% Wilson score interval, and writes `tournament-summary.json`.

## L1 (Novice) vs L5 (Grandmaster), 3+0, 20 games

Command: `node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+0 --seed 20260821`
Source artifacts: `tournament.txt`, `tournament-summary.json` (2026-08-21)

```
A (L1 Novice):       4/20
B (L5 Grandmaster): 16/20
Draws: 0 | Errored: 0
Red color wins: 12 | Blue color wins: 8
End reasons: 20 x win (no timeouts at 3+0: the allocator scales with the remaining clock)
A win rate (decisive games): 20.0% 95% CI [8.1%, 41.6%]
Avg moves: 34.5
Avg time: 96.3s
```

L5 wins the matchup decisively. Games are decided on the board across varied seeded openings; the previous artifact's contradiction (published 60% for L1 against an 89%-for-L5 log) is resolved: strength ordering and reported numbers now come from the same run.

### Typical statlines from this run

| Level | Depth | Nodes | Threads | Think time |
|-------|-------|-------|---------|------------|
| L5 mid-game | 4-14 | 0.4M-5M | 8 | 2-11s |
| L5 endgame (VCF converting) | 0 (solver) | 0 | 8 | <0.1s |
| L1 | 2 | 1K-150K | 1 | 0.05-0.6s |

### Interpretation notes

- Red's 12-8 color edge is the first-move advantage on a 16x16 board; color-swapped pairs cancel it in the A/B totals.
- L1's 4 wins are upsets from sharp seeded openings where a depth-2 engine can still execute a forced win it can see.
- At 3+0 neither level flags: the time allocator spends a bounded fraction of the remaining clock, so "3+0" here measures play under a tight but managed budget. Flag-fall adjudication exists and is exercised by the API tests.
