# Round-robin tournament report

status: completed

started 2026-09-03T11:29:15.257Z; generated 2026-09-03T11:32:20.362Z

## Run

- git: 2dc311a341d245b0e04e9d77521905e79694f71d (dirty working tree)
- build: Release | node v26.3.0 | 20 CPUs | win32
- argv: `--pairings 1v1 --games-per-pairing 2 --tc 1+0 --label smoke-l1v1`
- games: 2/2 played | 84 moves | 7s elapsed

## Determinism

Openings are splitmix64-seeded from base seed 20260821: each game uses seed = base + global game index (20260822-20260823). Pairing order is fixed, games run sequentially, colors swap every second game.

The `t=`, `nps=` statline columns and the achieved depth under a soft time budget are wall-clock evidence: they vary with machine load and are not part of the deterministic oracle. Multi-threaded levels (L3+) also vary in node counts run to run. Depth-capped levels (L1, L2) are deterministic.

## Ladder

| pairing | games | draws | higher wins | decisive | higher win rate | 95% CI |
|---|---|---|---|---|---|---|
| L1vL1 | 2 | 0 | L1=2 | 2 | 100.0% | 34.2%-100.0% |

Adjacent steps: none

Verdict: inconclusive (fewer than 3 adjacent steps measured)

## L1vL1 (2 games)

Wins L1=2, draws=0, avg moves=42, avg 3.5s
End reasons: {"win":2}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 84 | 1.5 | 2 | 1435.02 | 35500.82 | 11.1% | 0.448 | 0 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 1 | 20260822 | L1 | L1 | red | win | 43 | 3.6 |
| 2 | 20260823 | L1 | L1 | red | win | 41 | 3.3 |

## Anomalies

- timeout-fallback moves: 1
- games hitting the move cap: 0
- errored games: 0
- L1vL1 and L5vL5 are calibration pairings: near-balanced results are expected there, decisive skew in cross pairings.

Timeout-fallback moves, draws, and max-move games are recorded, not fatal; any other game failure aborts the run with a partial summary.

## Artifacts

- `run.log`: every banner and statline verbatim, plus the backend `move-statline` log lines.
- `summary.json`: this report's source data (schema v1).
- `matches.db`: sqlite archive of games and moves. Example query:

```sql
SELECT difficulty, COUNT(*) vcf_moves, AVG(vcf_depth) avg_chain
FROM moves WHERE move_type = 'vcf' GROUP BY difficulty ORDER BY difficulty;
```
