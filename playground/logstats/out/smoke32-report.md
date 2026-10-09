# logstats report

- dir: logs/tourny/20261009-101129-smoke32
- files: 57
- runs: 1 (1)
- series: 57
- games: 147
- m-lines: 12415

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| master-2 | master | 14 | 10 | 24 | 12 | 2 |
| medium-2 | medium | 14 | 9 | 20 | 14 | 3 |
| hard-2 | hard | 14 | 8 | 19 | 14 | 3 |
| hard-1 | hard | 14 | 6 | 16 | 15 | 6 |
| medium-1 | medium | 14 | 6 | 16 | 17 | 4 |
| master-1 | master | 14 | 5 | 13 | 17 | 4 |
| easy-2 | easy | 14 | 4 | 11 | 17 | 10 |
| easy-1 | easy | 14 | 1 | 11 | 24 | 2 |

## telemetry per tier

### tier easy

- moves: 3451
- depth: mean 8.4, median 8.0, max 64
- mean n: 6.64m, mean nps: 1.51m
- mean t: 4.71 s, mean alloc: 5.05 s, mean alloc/t: 4.17
- mean tt: 16.4%
- score: min -M2, max M1, mean -448.8 over 3151 non-mate lines
- mate markers: +M 120, -M 180
- anomalies: d=1 non-first 128, zero nps 6, dup consecutive 0

### tier medium

- moves: 3244
- depth: mean 8.3, median 8.0, max 64
- mean n: 12.61m, mean nps: 2.66m
- mean t: 4.91 s, mean alloc: 5.45 s, mean alloc/t: 6.09
- mean tt: 14.7%
- score: min -M2, max M1, mean 111.0 over 2872 non-mate lines
- mate markers: +M 194, -M 178
- anomalies: d=1 non-first 86, zero nps 3, dup consecutive 0

### tier hard

- moves: 3054
- depth: mean 8.9, median 8.0, max 64
- mean n: 17.04m, mean nps: 4.52m
- mean t: 3.83 s, mean alloc: 5.40 s, mean alloc/t: 7.58
- mean tt: 17.7%
- score: min -M2, max M1, mean 302.5 over 2669 non-mate lines
- mate markers: +M 237, -M 148
- anomalies: d=1 non-first 69, zero nps 6, dup consecutive 0

### tier master

- moves: 2666
- depth: mean 8.7, median 8.0, max 64
- mean n: 34.16m, mean nps: 7.26m
- mean t: 3.97 s, mean alloc: 6.70 s, mean alloc/t: 12.05
- mean tt: 18.3%
- score: min -M2, max M1, mean 553.7 over 2256 non-mate lines
- mate markers: +M 258, -M 152
- anomalies: d=1 non-first 82, zero nps 3, dup consecutive 0

## strength verdict

1. master-2 (master): series wins 10, game wins 24
2. medium-2 (medium): series wins 9, game wins 20
3. hard-2 (hard): series wins 8, game wins 19
4. hard-1 (hard): series wins 6, game wins 16
5. medium-1 (medium): series wins 6, game wins 16
6. master-1 (master): series wins 5, game wins 13
7. easy-2 (easy): series wins 4, game wins 11
8. easy-1 (easy): series wins 1, game wins 11

inversions:
- INVERSION: medium-2 (tier medium) ranks 2 above hard-2 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above hard-1 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above master-1 (tier master)
- INVERSION: hard-2 (tier hard) ranks 3 above master-1 (tier master)
- INVERSION: hard-1 (tier hard) ranks 4 above master-1 (tier master)
- INVERSION: medium-1 (tier medium) ranks 5 above master-1 (tier master)
zero-sum: wins 130, losses 130, draws 34: zero-sum holds

## parse integrity

- unparsable lines: 12
  - logs\tourny\20261009-101129-smoke32\summary.txt:1: "run 1"
  - logs\tourny\20261009-101129-smoke32\summary.txt:3: "finished 20261010-052521"
  - logs\tourny\20261009-101129-smoke32\summary.txt:4: ""
  - logs\tourny\20261009-101129-smoke32\summary.txt:5: "rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames"
  - logs\tourny\20261009-101129-smoke32\summary.txt:6: "1\tmaster-2\tmaster\t1212\t10\t24\t12\t2\t38"
  - logs\tourny\20261009-101129-smoke32\summary.txt:7: "2\thard-2\thard\t1153\t8\t19\t14\t3\t36"
  - logs\tourny\20261009-101129-smoke32\summary.txt:8: "3\tmedium-2\tmedium\t1101\t9\t20\t14\t3\t37"
  - logs\tourny\20261009-101129-smoke32\summary.txt:9: "4\thard-1\thard\t987\t6\t16\t15\t6\t37"
  - logs\tourny\20261009-101129-smoke32\summary.txt:10: "5\tmaster-1\tmaster\t953\t5\t13\t17\t4\t34"
  - logs\tourny\20261009-101129-smoke32\summary.txt:11: "6\tmedium-1\tmedium\t950\t6\t16\t17\t4\t37"
  - logs\tourny\20261009-101129-smoke32\summary.txt:12: "7\teasy-2\teasy\t928\t4\t11\t17\t10\t38"
  - logs\tourny\20261009-101129-smoke32\summary.txt:13: "8\teasy-1\teasy\t716\t1\t11\t24\t2\t37"
- unattributed m-lines (no pairing header): 0
- files without a full header: 1
  - logs\tourny\20261009-101129-smoke32\summary.txt
- unfinished series (no verdict): 0
- fold mismatches: 0
