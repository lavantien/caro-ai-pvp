# Round-robin tournament report

status: aborted (game 56 (L5vL5): run killed externally mid-game (55/60 games persisted))

started 2026-09-03T11:48:03.444Z; generated 2026-09-03T13:52:31.480Z

## Run

- git: 9307987a04e0886605d2f9abe3b1e0f1f7b53d03
- build: Release | node v26.3.0 | 20 CPUs | win32
- argv: `--games-per-pairing 5 --label initial-5`
- games: 55/60 played | 2289 moves | 3393s elapsed

## Determinism

Openings are splitmix64-seeded from base seed 20260821: each game uses seed = base + global game index (20260822-20260881). Pairing order is fixed, games run sequentially, colors swap every second game.

The `t=`, `nps=` statline columns and the achieved depth under a soft time budget are wall-clock evidence: they vary with machine load and are not part of the deterministic oracle. Multi-threaded levels (L3+) also vary in node counts run to run. Depth-capped levels (L1, L2) are deterministic.

## Ladder

| pairing | games | draws | higher wins | decisive | higher win rate | 95% CI |
|---|---|---|---|---|---|---|
| L1vL1 | 5 | 1 | L1=4 | 4 | 100.0% | 51.0%-100.0% |
| L1vL5 | 5 | 0 | L5=4 | 5 | 80.0% | 37.5%-96.4% |
| L1vL4 | 5 | 0 | L4=5 | 5 | 100.0% | 56.5%-100.0% |
| L1vL3 | 5 | 0 | L3=5 | 5 | 100.0% | 56.5%-100.0% |
| L1vL2 | 5 | 0 | L2=3 | 5 | 60.0% | 23.1%-88.2% |
| L2vL5 | 5 | 0 | L5=5 | 5 | 100.0% | 56.5%-100.0% |
| L2vL4 | 5 | 0 | L4=3 | 5 | 60.0% | 23.1%-88.2% |
| L2vL3 | 5 | 0 | L3=2 | 5 | 40.0% | 11.8%-76.9% |
| L3vL5 | 5 | 0 | L5=3 | 5 | 60.0% | 23.1%-88.2% |
| L3vL4 | 5 | 0 | L4=2 | 5 | 40.0% | 11.8%-76.9% |
| L4vL5 | 5 | 0 | L5=4 | 5 | 80.0% | 37.5%-96.4% |

Adjacent steps: L1vL2 60.0%, L2vL3 40.0%, L3vL4 40.0%, L4vL5 80.0%

Verdict: non-monotonic (some adjacent step favors the lower level)

## L1vL1 (5 games)

Wins L1=4, draws=1, avg moves=75.4, avg 13.1s
End reasons: {"draw":1,"win":4}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 377 | 1.72 | 2 | 2221.26 | 24900.87 | 5.5% | 0.2218 | 0 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 1 | 20260822 | L1 | L1 | none | draw | 253 | 48.2 |
| 2 | 20260823 | L1 | L1 | red | win | 27 | 3.5 |
| 3 | 20260824 | L1 | L1 | red | win | 29 | 4.6 |
| 4 | 20260825 | L1 | L1 | red | win | 37 | 5.5 |
| 5 | 20260826 | L1 | L1 | red | win | 31 | 3.5 |

## L1vL5 (5 games)

Wins L1=1, L5=4, draws=0, avg moves=36.2, avg 65.5s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 90 | 1.82 | 2 | 2158.99 | 46558.25 | 11.8% | 0.1644 | 0 | 0/0 |
| L5 Grandmaster | 91 | 4.59 | 4 | 924136.21 | 221402.85 | 46.5% | 0.4053 | 20 | 72/25 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 6 | 20260827 | L1 | L5 | red | win | 51 | 126.5 |
| 7 | 20260828 | L5 | L1 | red | win | 29 | 29.7 |
| 8 | 20260829 | L1 | L5 | blue | win | 38 | 61.8 |
| 9 | 20260830 | L5 | L1 | red | win | 31 | 57.5 |
| 10 | 20260831 | L1 | L5 | blue | win | 32 | 52.1 |

## L1vL4 (5 games)

Wins L4=5, draws=0, avg moves=61.6, avg 76s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 153 | 1.73 | 2 | 2707.73 | 27125.07 | 9.6% | 0.2791 | 0 | 0/0 |
| L4 Advanced | 155 | 3.5 | 3 | 208950.19 | 92842.68 | 37.8% | 0.3321 | 20 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 11 | 20260832 | L1 | L4 | blue | win | 54 | 65.4 |
| 12 | 20260833 | L4 | L1 | red | win | 155 | 203.3 |
| 13 | 20260834 | L1 | L4 | blue | win | 42 | 49.8 |
| 14 | 20260835 | L4 | L1 | red | win | 29 | 24.0 |
| 15 | 20260836 | L1 | L4 | blue | win | 28 | 37.7 |

## L1vL3 (5 games)

Wins L3=5, draws=0, avg moves=28.4, avg 13.5s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 70 | 1.91 | 2 | 1969.37 | 48947.45 | 8.2% | 0.1165 | 0 | 0/0 |
| L3 Intermediate | 72 | 3.73 | 4 | 75969.41 | 83168.69 | 32.1% | 0.2084 | 16 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 16 | 20260837 | L1 | L3 | blue | win | 22 | 7.7 |
| 17 | 20260838 | L3 | L1 | red | win | 37 | 16.2 |
| 18 | 20260839 | L1 | L3 | blue | win | 34 | 19.0 |
| 19 | 20260840 | L3 | L1 | red | win | 29 | 14.7 |
| 20 | 20260841 | L1 | L3 | blue | win | 20 | 9.7 |

## L1vL2 (5 games)

Wins L1=2, L2=3, draws=0, avg moves=26, avg 9.7s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 65 | 1.89 | 2 | 2786.45 | 52441.3 | 12.3% | 0.1835 | 0 | 0/0 |
| L2 Beginner | 65 | 3.31 | 4 | 24692.43 | 46053.2 | 31.5% | 0.3194 | 0 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 21 | 20260842 | L1 | L2 | red | win | 35 | 15.9 |
| 22 | 20260843 | L2 | L1 | red | win | 17 | 6.1 |
| 23 | 20260844 | L1 | L2 | blue | win | 18 | 6.8 |
| 24 | 20260845 | L2 | L1 | red | win | 25 | 7.8 |
| 25 | 20260846 | L1 | L2 | red | win | 35 | 12.1 |

## L2vL5 (5 games)

Wins L5=5, draws=0, avg moves=32, avg 59.4s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 79 | 3.14 | 3 | 17571.01 | 40284.15 | 36.8% | 0.3315 | 0 | 0/0 |
| L5 Grandmaster | 81 | 4.6 | 4 | 939504.07 | 223868.14 | 45.8% | 0.3638 | 24 | 57/30 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 26 | 20260847 | L2 | L5 | blue | win | 24 | 39.8 |
| 27 | 20260848 | L5 | L2 | red | win | 35 | 82.0 |
| 28 | 20260849 | L2 | L5 | blue | win | 44 | 83.1 |
| 29 | 20260850 | L5 | L2 | red | win | 25 | 52.8 |
| 30 | 20260851 | L2 | L5 | blue | win | 32 | 39.5 |

## L2vL4 (5 games)

Wins L2=2, L4=3, draws=0, avg moves=34.8, avg 53s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 87 | 3.05 | 3 | 22831.89 | 46068 | 38.6% | 0.324 | 0 | 0/0 |
| L4 Advanced | 87 | 4.14 | 4 | 326180.28 | 126338.25 | 37.9% | 0.3414 | 15 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 31 | 20260852 | L2 | L4 | blue | win | 34 | 52.8 |
| 32 | 20260853 | L4 | L2 | red | win | 35 | 47.6 |
| 33 | 20260854 | L2 | L4 | blue | win | 38 | 60.8 |
| 34 | 20260855 | L4 | L2 | blue | win | 34 | 51.3 |
| 35 | 20260856 | L2 | L4 | red | win | 33 | 52.5 |

## L2vL3 (5 games)

Wins L2=3, L3=2, draws=0, avg moves=27.8, avg 22.9s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 70 | 3.04 | 3 | 19424.49 | 45267.89 | 35.6% | 0.2722 | 0 | 0/0 |
| L3 Intermediate | 69 | 3.72 | 4 | 76164.97 | 81347.66 | 35.5% | 0.2334 | 4 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 36 | 20260857 | L2 | L3 | red | win | 25 | 15.9 |
| 37 | 20260858 | L3 | L2 | red | win | 23 | 16.9 |
| 38 | 20260859 | L2 | L3 | red | win | 17 | 16.0 |
| 39 | 20260860 | L3 | L2 | red | win | 43 | 50.7 |
| 40 | 20260861 | L2 | L3 | red | win | 31 | 15.2 |

## L3vL5 (5 games)

Wins L3=2, L5=3, draws=0, avg moves=66, avg 191.4s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L3 Intermediate | 165 | 3.03 | 3 | 53855.87 | 38828.79 | 40.3% | 0.3932 | 8 | 0/0 |
| L5 Grandmaster | 165 | 4.17 | 4 | 692881.91 | 158871.28 | 52.9% | 0.5066 | 24 | 140/96 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 41 | 20260862 | L3 | L5 | red | win | 61 | 209.3 |
| 42 | 20260863 | L5 | L3 | red | win | 37 | 88.5 |
| 43 | 20260864 | L3 | L5 | red | win | 85 | 260.0 |
| 44 | 20260865 | L5 | L3 | red | win | 91 | 223.9 |
| 45 | 20260866 | L3 | L5 | blue | win | 56 | 175.3 |

## L3vL4 (5 games)

Wins L3=3, L4=2, draws=0, avg moves=35.4, avg 64.9s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L3 Intermediate | 89 | 3.29 | 3 | 72696.8 | 56232.15 | 40.4% | 0.3093 | 10 | 0/0 |
| L4 Advanced | 88 | 3.95 | 4 | 242198.95 | 106242.65 | 43.6% | 0.3079 | 9 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 46 | 20260867 | L3 | L4 | red | win | 27 | 29.9 |
| 47 | 20260868 | L4 | L3 | blue | win | 34 | 50.9 |
| 48 | 20260869 | L3 | L4 | red | win | 33 | 67.3 |
| 49 | 20260870 | L4 | L3 | red | win | 47 | 118.0 |
| 50 | 20260871 | L3 | L4 | blue | win | 36 | 58.6 |

## L4vL5 (5 games)

Wins L4=1, L5=4, draws=0, avg moves=34.2, avg 109.1s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L4 Advanced | 85 | 3.95 | 4 | 211350.8 | 85276.21 | 39.6% | 0.3487 | 6 | 0/0 |
| L5 Grandmaster | 86 | 4.61 | 5 | 775076.85 | 178012.15 | 49.7% | 0.4252 | 19 | 70/44 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 51 | 20260872 | L4 | L5 | blue | win | 36 | 121.4 |
| 52 | 20260873 | L5 | L4 | red | win | 23 | 74.4 |
| 53 | 20260874 | L4 | L5 | blue | win | 30 | 90.3 |
| 54 | 20260875 | L5 | L4 | red | win | 27 | 69.6 |
| 55 | 20260876 | L4 | L5 | red | win | 55 | 189.8 |

## Anomalies

- timeout-fallback moves: 0
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
