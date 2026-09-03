# Round-robin tournament report

status: completed

started 2026-09-03T13:56:57.130Z; generated 2026-09-03T15:04:00.387Z

## Run

- git: d2db55cb682e7f82d4d435621e1db10db0920f17
- build: Release | node v26.3.0 | 20 CPUs | win32
- argv: `--games-per-pairing 5 --label initial-5`
- games: 60/60 played | 2436 moves | 4015s elapsed

## Determinism

Openings are splitmix64-seeded from base seed 20260821: each game uses seed = base + global game index (20260822-20260881). Pairing order is fixed, games run sequentially, colors swap every second game.

The `t=`, `nps=` statline columns and the achieved depth under a soft time budget are wall-clock evidence: they vary with machine load and are not part of the deterministic oracle. Multi-threaded levels (L3+) also vary in node counts run to run. Depth-capped levels (L1, L2) are deterministic.

## Ladder

| pairing | games | draws | higher wins | decisive | higher win rate | 95% CI |
|---|---|---|---|---|---|---|
| L1vL1 | 5 | 1 | L1=4 | 4 | 100.0% | 51.0%-100.0% |
| L1vL5 | 5 | 0 | L5=3 | 5 | 60.0% | 23.1%-88.2% |
| L1vL4 | 5 | 0 | L4=4 | 5 | 80.0% | 37.5%-96.4% |
| L1vL3 | 5 | 0 | L3=4 | 5 | 80.0% | 37.5%-96.4% |
| L1vL2 | 5 | 0 | L2=4 | 5 | 80.0% | 37.5%-96.4% |
| L2vL5 | 5 | 0 | L5=4 | 5 | 80.0% | 37.5%-96.4% |
| L2vL4 | 5 | 0 | L4=3 | 5 | 60.0% | 23.1%-88.2% |
| L2vL3 | 5 | 0 | L3=1 | 5 | 20.0% | 3.6%-62.5% |
| L3vL5 | 5 | 0 | L5=4 | 5 | 80.0% | 37.5%-96.4% |
| L3vL4 | 5 | 0 | L4=2 | 5 | 40.0% | 11.8%-76.9% |
| L4vL5 | 5 | 0 | L5=4 | 5 | 80.0% | 37.5%-96.4% |
| L5vL5 | 5 | 0 | L5=5 | 5 | 100.0% | 56.5%-100.0% |

Adjacent steps: L1vL2 80.0%, L2vL3 20.0%, L3vL4 40.0%, L4vL5 80.0%

Verdict: non-monotonic (some adjacent step favors the lower level)

## L1vL1 (5 games)

Wins L1=4, draws=1, avg moves=75.4, avg 15.3s
End reasons: {"draw":1,"win":4}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 377 | 1.78 | 2 | 2365.98 | 24549.39 | 6.5% | 0.2578 | 0 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 1 | 20260822 | L1 | L1 | none | draw | 253 | 58.7 |
| 2 | 20260823 | L1 | L1 | red | win | 27 | 3.7 |
| 3 | 20260824 | L1 | L1 | red | win | 29 | 4.9 |
| 4 | 20260825 | L1 | L1 | red | win | 37 | 5.7 |
| 5 | 20260826 | L1 | L1 | red | win | 31 | 3.6 |

## L1vL5 (5 games)

Wins L1=2, L5=3, draws=0, avg moves=40.8, avg 80.4s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 102 | 1.75 | 2 | 2796.97 | 36588.09 | 16.1% | 0.2943 | 0 | 0/0 |
| L5 Grandmaster | 102 | 4.32 | 4 | 727449.55 | 176113.04 | 44.5% | 0.4092 | 15 | 86/36 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 6 | 20260827 | L1 | L5 | red | win | 43 | 98.5 |
| 7 | 20260828 | L5 | L1 | red | win | 31 | 49.6 |
| 8 | 20260829 | L1 | L5 | blue | win | 64 | 112.0 |
| 9 | 20260830 | L5 | L1 | blue | win | 48 | 113.3 |
| 10 | 20260831 | L1 | L5 | blue | win | 18 | 28.5 |

## L1vL4 (5 games)

Wins L1=1, L4=4, draws=0, avg moves=41.4, avg 47.5s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 103 | 1.78 | 2 | 2562.58 | 35067.11 | 11.9% | 0.2256 | 0 | 0/0 |
| L4 Advanced | 104 | 3.74 | 4 | 213435.83 | 97369.37 | 40.4% | 0.3085 | 17 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 11 | 20260832 | L1 | L4 | red | win | 51 | 58.9 |
| 12 | 20260833 | L4 | L1 | red | win | 71 | 98.7 |
| 13 | 20260834 | L1 | L4 | blue | win | 28 | 24.3 |
| 14 | 20260835 | L4 | L1 | red | win | 29 | 26.5 |
| 15 | 20260836 | L1 | L4 | blue | win | 28 | 29.2 |

## L1vL3 (5 games)

Wins L1=1, L3=4, draws=0, avg moves=31.4, avg 17.6s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 78 | 1.87 | 2 | 2018.55 | 44986.56 | 8.6% | 0.1443 | 0 | 0/0 |
| L3 Intermediate | 79 | 3.66 | 4 | 72072.38 | 69472.46 | 35.4% | 0.2382 | 14 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 16 | 20260837 | L1 | L3 | blue | win | 22 | 9.0 |
| 17 | 20260838 | L3 | L1 | blue | win | 48 | 30.6 |
| 18 | 20260839 | L1 | L3 | blue | win | 38 | 23.1 |
| 19 | 20260840 | L3 | L1 | red | win | 29 | 14.5 |
| 20 | 20260841 | L1 | L3 | blue | win | 20 | 10.6 |

## L1vL2 (5 games)

Wins L1=1, L2=4, draws=0, avg moves=26.6, avg 10.3s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 66 | 1.89 | 2 | 2731.77 | 48945.89 | 12.2% | 0.1921 | 0 | 0/0 |
| L2 Beginner | 67 | 3.21 | 3 | 24144.13 | 42887.82 | 30.0% | 0.3258 | 0 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 21 | 20260842 | L1 | L2 | red | win | 35 | 16.4 |
| 22 | 20260843 | L2 | L1 | red | win | 17 | 7.1 |
| 23 | 20260844 | L1 | L2 | blue | win | 18 | 7.1 |
| 24 | 20260845 | L2 | L1 | red | win | 25 | 8.4 |
| 25 | 20260846 | L1 | L2 | blue | win | 38 | 12.4 |

## L2vL5 (5 games)

Wins L2=1, L5=4, draws=0, avg moves=43.4, avg 75.6s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 108 | 2.81 | 3 | 15188.06 | 29382.47 | 31.9% | 0.3766 | 0 | 0/0 |
| L5 Grandmaster | 109 | 4.23 | 4 | 584796.27 | 171733.36 | 43.8% | 0.3494 | 27 | 82/52 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 26 | 20260847 | L2 | L5 | blue | win | 30 | 39.1 |
| 27 | 20260848 | L5 | L2 | red | win | 23 | 35.6 |
| 28 | 20260849 | L2 | L5 | red | win | 109 | 230.1 |
| 29 | 20260850 | L5 | L2 | red | win | 29 | 35.0 |
| 30 | 20260851 | L2 | L5 | blue | win | 26 | 38.0 |

## L2vL4 (5 games)

Wins L2=2, L4=3, draws=0, avg moves=43.2, avg 70s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 108 | 2.84 | 3 | 18795.69 | 37875.76 | 35.7% | 0.3443 | 0 | 0/0 |
| L4 Advanced | 108 | 3.65 | 4 | 247004.75 | 90893.46 | 40.7% | 0.3674 | 16 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 31 | 20260852 | L2 | L4 | blue | win | 34 | 64.4 |
| 32 | 20260853 | L4 | L2 | red | win | 45 | 57.7 |
| 33 | 20260854 | L2 | L4 | blue | win | 36 | 49.1 |
| 34 | 20260855 | L4 | L2 | blue | win | 66 | 115.3 |
| 35 | 20260856 | L2 | L4 | red | win | 35 | 63.7 |

## L2vL3 (5 games)

Wins L2=4, L3=1, draws=0, avg moves=29.6, avg 24.2s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L2 Beginner | 75 | 2.95 | 3 | 20116.03 | 47217.36 | 36.1% | 0.3 | 0 | 0/0 |
| L3 Intermediate | 73 | 3.63 | 4 | 66016.35 | 70485.64 | 40.4% | 0.2185 | 5 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 36 | 20260857 | L2 | L3 | red | win | 29 | 19.8 |
| 37 | 20260858 | L3 | L2 | red | win | 27 | 18.3 |
| 38 | 20260859 | L2 | L3 | red | win | 23 | 26.3 |
| 39 | 20260860 | L3 | L2 | blue | win | 40 | 34.7 |
| 40 | 20260861 | L2 | L3 | red | win | 29 | 22.1 |

## L3vL5 (5 games)

Wins L3=1, L5=4, draws=0, avg moves=46.2, avg 122.3s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L3 Intermediate | 115 | 3.34 | 3 | 55031.69 | 48629.47 | 41.4% | 0.296 | 7 | 0/0 |
| L5 Grandmaster | 116 | 4.61 | 5 | 792184.38 | 181468.19 | 48.1% | 0.4395 | 17 | 102/69 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 41 | 20260862 | L3 | L5 | red | win | 39 | 96.7 |
| 42 | 20260863 | L5 | L3 | red | win | 63 | 147.1 |
| 43 | 20260864 | L3 | L5 | blue | win | 44 | 118.2 |
| 44 | 20260865 | L5 | L3 | red | win | 29 | 73.5 |
| 45 | 20260866 | L3 | L5 | blue | win | 56 | 175.9 |

## L3vL4 (5 games)

Wins L3=3, L4=2, draws=0, avg moves=33, avg 61s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L3 Intermediate | 83 | 3.41 | 4 | 86393.79 | 59323.04 | 41.1% | 0.342 | 10 | 0/0 |
| L4 Advanced | 82 | 3.99 | 4 | 247367.14 | 109267.34 | 42.7% | 0.2934 | 9 | 0/0 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 46 | 20260867 | L3 | L4 | red | win | 27 | 30.8 |
| 47 | 20260868 | L4 | L3 | blue | win | 30 | 49.8 |
| 48 | 20260869 | L3 | L4 | red | win | 25 | 46.6 |
| 49 | 20260870 | L4 | L3 | red | win | 47 | 119.4 |
| 50 | 20260871 | L3 | L4 | blue | win | 36 | 58.3 |

## L4vL5 (5 games)

Wins L4=1, L5=4, draws=0, avg moves=37, avg 121.2s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L4 Advanced | 92 | 4.16 | 4 | 218035.4 | 90792.51 | 37.4% | 0.3459 | 6 | 0/0 |
| L5 Grandmaster | 93 | 4.84 | 5 | 862621.66 | 188345.92 | 48.5% | 0.4448 | 20 | 76/49 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 51 | 20260872 | L4 | L5 | blue | win | 36 | 120.6 |
| 52 | 20260873 | L5 | L4 | red | win | 35 | 123.6 |
| 53 | 20260874 | L4 | L5 | blue | win | 30 | 97.9 |
| 54 | 20260875 | L5 | L4 | red | win | 29 | 78.2 |
| 55 | 20260876 | L4 | L5 | red | win | 55 | 185.9 |

## L5vL5 (5 games)

Wins L5=5, draws=0, avg moves=39.2, avg 157.7s
End reasons: {"win":5}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L5 Grandmaster | 196 | 4.79 | 5 | 801959.51 | 173001.25 | 50.3% | 0.4555 | 28 | 166/95 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 56 | 20260877 | L5 | L5 | red | win | 21 | 71.2 |
| 57 | 20260878 | L5 | L5 | red | win | 57 | 250.2 |
| 58 | 20260879 | L5 | L5 | red | win | 31 | 137.2 |
| 59 | 20260880 | L5 | L5 | blue | win | 54 | 257.1 |
| 60 | 20260881 | L5 | L5 | red | win | 33 | 72.6 |

## Anomalies

- timeout-fallback moves: 2
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
