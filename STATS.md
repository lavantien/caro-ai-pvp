# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). L5 = full strength.

Per-move statlines are logged during AI vs AI matches:

```
M 0 red  h9  d=4  n=185.3K  nps=124K  tt= 27% s=+10 thr=4 t=1.5s alloc=0.3s[VCF]
```

Format: `M<move> <player> <pos> d=<depth> n=<nodes> nps=<nps> tt=<hit%> s=<score> thr=<threads> t=<time> alloc=<allocated_time>[VCF]`

- `thr`: number of search goroutines used
- `alloc`: time budget allocated by time manager (vs `t` which is actual elapsed)
- `[VCF]`: present when move was found by VCF solver (pre-search)

## Benchmark Commands

```bash
node scripts/run-tournament.mjs --games 4 --red 5 --blue 5 --tc 3+2
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
```

## L1 (Novice) vs L5 (Grandmaster), 3+0, 20 games (color-swapped)

```
A (L1 Novice):       12/20 (60.0%)
B (L5 Grandmaster):   8/20 (40.0%)
Draws: 0
Avg moves: 26.2
Avg time: 294.2s
```

Source: `tournament.txt`

Note: L1 wins are largely due to time-pressure advantage (5% budget = fast moves, less clock pressure). L5 achieves deeper search but consumes more time per move. Score dominance comes from TT-sharing parallel search at L5.

### L5 Per-Move Stats (typical mid-game)

| Metric | Range |
|--------|-------|
| Depth | 4-14 |
| Nodes | 1.6M - 21.4M |
| NPS | 310K - 860K |
| TT Hit Rate | 1% - 27% |
| Threads | Pow2((N-2)/2) |
| Think Time | 1.4s - 30.0s |

### L1 Per-Move Stats (typical)

| Metric | Range |
|--------|-------|
| Depth | 3-5 |
| Nodes | 100K - 235K |
| NPS | 100K - 155K |
| TT Hit Rate | 50% - 64% (high due to shallow search reuse) |
| Threads | 1 |
| Think Time | 1.5s - 1.6s (capped by 5% time budget) |
