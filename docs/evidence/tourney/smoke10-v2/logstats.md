# logstats report

- dir: logs/tourny/20261004-015910-smoke10
- files: 31
- runs: 1 (1)
- series: 31
- games: 80
- m-lines: 4454

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| medium-1 | medium | 10 | 9 | 19 | 8 | 0 |
| medium-2 | medium | 10 | 6 | 16 | 12 | 0 |
| hard-1 | hard | 10 | 6 | 15 | 10 | 2 |
| hard-2 | hard | 10 | 4 | 12 | 12 | 2 |
| easy-1 | easy | 10 | 2 | 8 | 18 | 0 |
| easy-2 | easy | 10 | 2 | 8 | 18 | 0 |

## telemetry per tier

### tier easy

- moves: 1132
- depth: mean 6.5, median 7.0, max 64
- mean n: 3.3m, mean nps: 1.66m
- mean t: 1.85 s, mean alloc: 1.90 s, mean alloc/t: 1.02
- mean tt: 0.0%
- score: min -M2, max M1, mean -409.0 over 944 non-mate lines
- mate markers: +M 60, -M 128
- anomalies: d=1 non-first 75, zero nps 0, dup consecutive 0

### tier medium

- moves: 1406
- depth: mean 7.2, median 8.0, max 13
- mean n: 4.83m, mean nps: 2.5m
- mean t: 1.65 s, mean alloc: 1.81 s, mean alloc/t: 1.83
- mean tt: 13.2%
- score: min -M2, max M1, mean 382.4 over 1120 non-mate lines
- mate markers: +M 193, -M 93
- anomalies: d=1 non-first 127, zero nps 14, dup consecutive 0

### tier hard

- moves: 1916
- depth: mean 5.1, median 6.0, max 13
- mean n: 4.93m, mean nps: 2.93m
- mean t: 0.97 s, mean alloc: 1.27 s, mean alloc/t: 2.34
- mean tt: 9.0%
- score: min -M2, max M1, mean 41.5 over 1668 non-mate lines
- mate markers: +M 170, -M 78
- anomalies: d=1 non-first 196, zero nps 541, dup consecutive 0

## strength verdict

1. medium-1 (medium): series wins 9, game wins 19
2. medium-2 (medium): series wins 6, game wins 16
3. hard-1 (hard): series wins 6, game wins 15
4. hard-2 (hard): series wins 4, game wins 12
5. easy-1 (easy): series wins 2, game wins 8
6. easy-2 (easy): series wins 2, game wins 8

inversions:
- INVERSION: medium-1 (tier medium) ranks 1 above hard-1 (tier hard)
- INVERSION: medium-1 (tier medium) ranks 1 above hard-2 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above hard-1 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above hard-2 (tier hard)
zero-sum: wins 78, losses 78, draws 4: zero-sum holds

## parse integrity

- unparsable lines: 10
  - logs\tourny\20261004-015910-smoke10\summary.txt:1: "run 1"
  - logs\tourny\20261004-015910-smoke10\summary.txt:3: "finished 20261004-025534"
  - logs\tourny\20261004-015910-smoke10\summary.txt:4: ""
  - logs\tourny\20261004-015910-smoke10\summary.txt:5: "rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames"
  - logs\tourny\20261004-015910-smoke10\summary.txt:6: "1\tmedium-1\tmedium\t1240\t9\t19\t8\t0\t27"
  - logs\tourny\20261004-015910-smoke10\summary.txt:7: "2\thard-1\thard\t1107\t6\t15\t10\t2\t27"
  - logs\tourny\20261004-015910-smoke10\summary.txt:8: "3\tmedium-2\tmedium\t1102\t6\t16\t12\t0\t28"
  - logs\tourny\20261004-015910-smoke10\summary.txt:9: "4\thard-2\thard\t987\t4\t12\t12\t2\t26"
  - logs\tourny\20261004-015910-smoke10\summary.txt:10: "5\teasy-1\teasy\t802\t2\t8\t18\t0\t26"
  - logs\tourny\20261004-015910-smoke10\summary.txt:11: "6\teasy-2\teasy\t762\t2\t8\t18\t0\t26"
- unattributed m-lines (no pairing header): 0
- files without a full header: 1
  - logs\tourny\20261004-015910-smoke10\summary.txt
- unfinished series (no verdict): 0
- fold mismatches: 0
