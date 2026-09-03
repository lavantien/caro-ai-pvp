# Round-robin tournament report

status: completed

started 2026-09-03T16:03:39.902Z; generated 2026-09-03T18:06:36.777Z

## Run

- git: 848d03044b1cbb610e39145f3d2571516537ab39
- build: Release | node v26.3.0 | 20 CPUs | win32
- argv: `--pairings 1v5 --games-per-pairing 100 --label l1v5-100`
- games: 100/100 played | 4321 moves | 7369s elapsed

## Determinism

Openings are splitmix64-seeded from base seed 20260821: each game uses seed = base + global game index (20260822-20260921). Pairing order is fixed, games run sequentially, colors swap every second game.

The `t=`, `nps=` statline columns and the achieved depth under a soft time budget are wall-clock evidence: they vary with machine load and are not part of the deterministic oracle. Multi-threaded levels (L3+) also vary in node counts run to run. Depth-capped levels (L1, L2) are deterministic.

## Ladder

| pairing | games | draws | higher wins | decisive | higher win rate | 95% CI |
|---|---|---|---|---|---|---|
| L1vL5 | 100 | 3 | L5=80 | 97 | 82.5% | 73.7%-88.8% |

Adjacent steps: none

Verdict: inconclusive (fewer than 3 adjacent steps measured)

## L1vL5 (100 games)

Wins L1=17, L5=80, draws=3, avg moves=43.21, avg 73.7s
End reasons: {"win":97,"draw":3}

| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |
|---|---|---|---|---|---|---|---|---|---|
| L1 Novice | 2144 | 1.74 | 2 | 2195.2 | 27138.81 | 10.8% | 0.2398 | 0 | 0/0 |
| L5 Grandmaster | 2177 | 4.06 | 4 | 618094.62 | 150155.33 | 43.0% | 0.3981 | 445 | 1715/708 |

| # | seed | red | blue | winner | reason | moves | seconds |
|---|---|---|---|---|---|---|---|
| 1 | 20260822 | L1 | L5 | blue | win | 58 | 102.8 |
| 2 | 20260823 | L5 | L1 | red | win | 35 | 57.0 |
| 3 | 20260824 | L1 | L5 | blue | win | 72 | 137.7 |
| 4 | 20260825 | L5 | L1 | red | win | 17 | 33.3 |
| 5 | 20260826 | L1 | L5 | blue | win | 34 | 40.4 |
| 6 | 20260827 | L5 | L1 | red | win | 17 | 20.7 |
| 7 | 20260828 | L1 | L5 | none | draw | 254 | 375.3 |
| 8 | 20260829 | L5 | L1 | red | win | 23 | 54.0 |
| 9 | 20260830 | L1 | L5 | red | win | 65 | 119.7 |
| 10 | 20260831 | L5 | L1 | red | win | 25 | 41.0 |
| 11 | 20260832 | L1 | L5 | blue | win | 68 | 124.9 |
| 12 | 20260833 | L5 | L1 | blue | win | 52 | 89.2 |
| 13 | 20260834 | L1 | L5 | blue | win | 24 | 30.0 |
| 14 | 20260835 | L5 | L1 | red | win | 29 | 43.4 |
| 15 | 20260836 | L1 | L5 | blue | win | 42 | 76.6 |
| 16 | 20260837 | L5 | L1 | red | win | 35 | 33.4 |
| 17 | 20260838 | L1 | L5 | none | draw | 254 | 367.0 |
| 18 | 20260839 | L5 | L1 | red | win | 23 | 26.9 |
| 19 | 20260840 | L1 | L5 | blue | win | 52 | 61.2 |
| 20 | 20260841 | L5 | L1 | red | win | 27 | 46.7 |
| 21 | 20260842 | L1 | L5 | blue | win | 16 | 24.6 |
| 22 | 20260843 | L5 | L1 | blue | win | 30 | 43.6 |
| 23 | 20260844 | L1 | L5 | red | win | 65 | 121.9 |
| 24 | 20260845 | L5 | L1 | red | win | 71 | 103.9 |
| 25 | 20260846 | L1 | L5 | blue | win | 24 | 42.0 |
| 26 | 20260847 | L5 | L1 | red | win | 21 | 45.3 |
| 27 | 20260848 | L1 | L5 | blue | win | 32 | 58.5 |
| 28 | 20260849 | L5 | L1 | red | win | 17 | 38.9 |
| 29 | 20260850 | L1 | L5 | blue | win | 36 | 60.1 |
| 30 | 20260851 | L5 | L1 | blue | win | 52 | 132.0 |
| 31 | 20260852 | L1 | L5 | blue | win | 34 | 50.8 |
| 32 | 20260853 | L5 | L1 | red | win | 19 | 20.1 |
| 33 | 20260854 | L1 | L5 | blue | win | 28 | 68.8 |
| 34 | 20260855 | L5 | L1 | red | win | 45 | 74.3 |
| 35 | 20260856 | L1 | L5 | blue | win | 26 | 58.4 |
| 36 | 20260857 | L5 | L1 | red | win | 19 | 33.7 |
| 37 | 20260858 | L1 | L5 | blue | win | 24 | 46.6 |
| 38 | 20260859 | L5 | L1 | red | win | 25 | 20.6 |
| 39 | 20260860 | L1 | L5 | blue | win | 10 | 16.5 |
| 40 | 20260861 | L5 | L1 | red | win | 29 | 39.2 |
| 41 | 20260862 | L1 | L5 | red | win | 59 | 134.2 |
| 42 | 20260863 | L5 | L1 | blue | win | 24 | 46.1 |
| 43 | 20260864 | L1 | L5 | blue | win | 38 | 68.3 |
| 44 | 20260865 | L5 | L1 | red | win | 23 | 48.7 |
| 45 | 20260866 | L1 | L5 | blue | win | 48 | 33.1 |
| 46 | 20260867 | L5 | L1 | blue | win | 20 | 47.5 |
| 47 | 20260868 | L1 | L5 | blue | win | 30 | 59.8 |
| 48 | 20260869 | L5 | L1 | red | win | 43 | 39.6 |
| 49 | 20260870 | L1 | L5 | red | win | 55 | 100.2 |
| 50 | 20260871 | L5 | L1 | red | win | 23 | 44.6 |
| 51 | 20260872 | L1 | L5 | blue | win | 72 | 139.6 |
| 52 | 20260873 | L5 | L1 | red | win | 27 | 40.6 |
| 53 | 20260874 | L1 | L5 | blue | win | 26 | 50.5 |
| 54 | 20260875 | L5 | L1 | red | win | 31 | 40.2 |
| 55 | 20260876 | L1 | L5 | blue | win | 20 | 23.3 |
| 56 | 20260877 | L5 | L1 | red | win | 25 | 31.9 |
| 57 | 20260878 | L1 | L5 | blue | win | 36 | 76.3 |
| 58 | 20260879 | L5 | L1 | red | win | 29 | 66.3 |
| 59 | 20260880 | L1 | L5 | red | win | 35 | 83.8 |
| 60 | 20260881 | L5 | L1 | red | win | 33 | 41.7 |
| 61 | 20260882 | L1 | L5 | red | win | 25 | 45.1 |
| 62 | 20260883 | L5 | L1 | red | win | 29 | 38.7 |
| 63 | 20260884 | L1 | L5 | none | draw | 254 | 380.9 |
| 64 | 20260885 | L5 | L1 | red | win | 29 | 47.4 |
| 65 | 20260886 | L1 | L5 | blue | win | 38 | 52.6 |
| 66 | 20260887 | L5 | L1 | red | win | 21 | 28.6 |
| 67 | 20260888 | L1 | L5 | red | win | 75 | 131.0 |
| 68 | 20260889 | L5 | L1 | red | win | 35 | 61.2 |
| 69 | 20260890 | L1 | L5 | blue | win | 44 | 53.4 |
| 70 | 20260891 | L5 | L1 | blue | win | 30 | 76.3 |
| 71 | 20260892 | L1 | L5 | blue | win | 120 | 209.3 |
| 72 | 20260893 | L5 | L1 | red | win | 25 | 78.7 |
| 73 | 20260894 | L1 | L5 | blue | win | 30 | 69.4 |
| 74 | 20260895 | L5 | L1 | red | win | 39 | 74.6 |
| 75 | 20260896 | L1 | L5 | blue | win | 16 | 27.5 |
| 76 | 20260897 | L5 | L1 | red | win | 67 | 120.4 |
| 77 | 20260898 | L1 | L5 | blue | win | 20 | 16.2 |
| 78 | 20260899 | L5 | L1 | red | win | 25 | 55.6 |
| 79 | 20260900 | L1 | L5 | blue | win | 26 | 27.2 |
| 80 | 20260901 | L5 | L1 | red | win | 35 | 28.9 |
| 81 | 20260902 | L1 | L5 | blue | win | 30 | 42.7 |
| 82 | 20260903 | L5 | L1 | red | win | 25 | 31.1 |
| 83 | 20260904 | L1 | L5 | red | win | 57 | 133.3 |
| 84 | 20260905 | L5 | L1 | red | win | 51 | 102.3 |
| 85 | 20260906 | L1 | L5 | blue | win | 28 | 60.8 |
| 86 | 20260907 | L5 | L1 | red | win | 31 | 28.4 |
| 87 | 20260908 | L1 | L5 | red | win | 59 | 143.1 |
| 88 | 20260909 | L5 | L1 | red | win | 125 | 244.9 |
| 89 | 20260910 | L1 | L5 | blue | win | 56 | 111.9 |
| 90 | 20260911 | L5 | L1 | red | win | 41 | 81.2 |
| 91 | 20260912 | L1 | L5 | blue | win | 34 | 59.9 |
| 92 | 20260913 | L5 | L1 | red | win | 17 | 18.9 |
| 93 | 20260914 | L1 | L5 | blue | win | 34 | 52.4 |
| 94 | 20260915 | L5 | L1 | red | win | 21 | 40.7 |
| 95 | 20260916 | L1 | L5 | blue | win | 54 | 93.0 |
| 96 | 20260917 | L5 | L1 | blue | win | 28 | 92.7 |
| 97 | 20260918 | L1 | L5 | blue | win | 30 | 56.3 |
| 98 | 20260919 | L5 | L1 | blue | win | 24 | 51.3 |
| 99 | 20260920 | L1 | L5 | blue | win | 26 | 36.3 |
| 100 | 20260921 | L5 | L1 | red | win | 41 | 66.8 |

## Anomalies

- timeout-fallback moves: 15
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
