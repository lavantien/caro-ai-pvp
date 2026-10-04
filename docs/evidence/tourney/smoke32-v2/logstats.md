# logstats report

- dir: logs/tourny/20261004-025535-smoke32
- files: 31
- runs: 1 (1)
- series: 31
- games: 83
- m-lines: 7806

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| medium-1 | medium | 10 | 8 | 17 | 5 | 6 |
| hard-1 | hard | 10 | 6 | 16 | 6 | 5 |
| hard-2 | hard | 10 | 5 | 13 | 13 | 2 |
| medium-2 | medium | 10 | 4 | 12 | 11 | 4 |
| easy-2 | easy | 10 | 1 | 7 | 17 | 5 |
| easy-1 | easy | 10 | 1 | 4 | 17 | 6 |

## telemetry per tier

### tier easy

- moves: 2976
- depth: mean 6.8, median 6.0, max 64
- mean n: 8.31m, mean nps: 1.89m
- mean t: 4.72 s, mean alloc: 4.97 s, mean alloc/t: 2.50
- mean tt: 0.0%
- score: min -M2, max M1, mean -636.0 over 2805 non-mate lines
- mate markers: +M 51, -M 120
- anomalies: d=1 non-first 124, zero nps 6, dup consecutive 0

### tier medium

- moves: 2617
- depth: mean 8.3, median 7.0, max 64
- mean n: 14.37m, mean nps: 3.26m
- mean t: 4.72 s, mean alloc: 5.23 s, mean alloc/t: 4.77
- mean tt: 16.2%
- score: min -M2, max M1, mean 297.4 over 2369 non-mate lines
- mate markers: +M 155, -M 93
- anomalies: d=1 non-first 94, zero nps 6, dup consecutive 0

### tier hard

- moves: 2213
- depth: mean 8.7, median 8.0, max 64
- mean n: 21.58m, mean nps: 5.5m
- mean t: 4.04 s, mean alloc: 5.82 s, mean alloc/t: 6.83
- mean tt: 16.5%
- score: min -M2, max M1, mean 865.9 over 1921 non-mate lines
- mate markers: +M 194, -M 98
- anomalies: d=1 non-first 77, zero nps 11, dup consecutive 0

## strength verdict

1. medium-1 (medium): series wins 8, game wins 17
2. hard-1 (hard): series wins 6, game wins 16
3. hard-2 (hard): series wins 5, game wins 13
4. medium-2 (medium): series wins 4, game wins 12
5. easy-2 (easy): series wins 1, game wins 7
6. easy-1 (easy): series wins 1, game wins 4

inversions:
- INVERSION: medium-1 (tier medium) ranks 1 above hard-1 (tier hard)
- INVERSION: medium-1 (tier medium) ranks 1 above hard-2 (tier hard)
zero-sum: wins 69, losses 69, draws 28: zero-sum holds

## parse integrity

- unparsable lines: 10
  - logs\tourny\20261004-025535-smoke32\summary.txt:1: "run 1"
  - logs\tourny\20261004-025535-smoke32\summary.txt:3: "finished 20261004-081748"
  - logs\tourny\20261004-025535-smoke32\summary.txt:4: ""
  - logs\tourny\20261004-025535-smoke32\summary.txt:5: "rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames"
  - logs\tourny\20261004-025535-smoke32\summary.txt:6: "1\tmedium-1\tmedium\t1273\t8\t17\t5\t6\t28"
  - logs\tourny\20261004-025535-smoke32\summary.txt:7: "2\thard-1\thard\t1240\t6\t16\t6\t5\t27"
  - logs\tourny\20261004-025535-smoke32\summary.txt:8: "3\tmedium-2\tmedium\t1016\t4\t12\t11\t4\t27"
  - logs\tourny\20261004-025535-smoke32\summary.txt:9: "4\thard-2\thard\t1009\t5\t13\t13\t2\t28"
  - logs\tourny\20261004-025535-smoke32\summary.txt:10: "5\teasy-2\teasy\t759\t1\t7\t17\t5\t29"
  - logs\tourny\20261004-025535-smoke32\summary.txt:11: "6\teasy-1\teasy\t703\t1\t4\t17\t6\t27"
- unattributed m-lines (no pairing header): 0
- files without a full header: 1
  - logs\tourny\20261004-025535-smoke32\summary.txt
- unfinished series (no verdict): 0
- fold mismatches: 0
