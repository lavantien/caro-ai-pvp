# logstats report

- dir: logs/tourny/20261010-052633-smoke10
- files: 57
- runs: 1 (1)
- series: 57
- games: 141
- m-lines: 7777

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| master-2 | master | 14 | 11 | 25 | 11 | 0 |
| hard-1 | hard | 14 | 9 | 20 | 16 | 0 |
| hard-2 | hard | 14 | 9 | 20 | 18 | 0 |
| master-1 | master | 14 | 8 | 19 | 13 | 0 |
| easy-1 | easy | 14 | 5 | 15 | 21 | 0 |
| easy-2 | easy | 14 | 5 | 14 | 21 | 1 |
| medium-1 | medium | 14 | 5 | 14 | 18 | 1 |
| medium-2 | medium | 14 | 4 | 13 | 22 | 0 |

## telemetry per tier

### tier easy

- moves: 2054
- depth: mean 6.4, median 8.0, max 64
- mean n: 2.1m, mean nps: 1.18m
- mean t: 1.43 s, mean alloc: 1.47 s, mean alloc/t: 1.02
- mean tt: 11.6%
- score: min -M2, max M1, mean -569.4 over 1842 non-mate lines
- mate markers: +M 107, -M 105
- anomalies: d=1 non-first 431, zero nps 2, dup consecutive 0

### tier medium

- moves: 2018
- depth: mean 6.6, median 8.0, max 64
- mean n: 3.91m, mean nps: 2.21m
- mean t: 1.37 s, mean alloc: 1.44 s, mean alloc/t: 1.50
- mean tt: 13.0%
- score: min -M2, max M1, mean -451.4 over 1793 non-mate lines
- mate markers: +M 104, -M 121
- anomalies: d=1 non-first 443, zero nps 3, dup consecutive 0

### tier hard

- moves: 2081
- depth: mean 7.3, median 8.0, max 13
- mean n: 6.48m, mean nps: 4.32m
- mean t: 1.19 s, mean alloc: 1.59 s, mean alloc/t: 2.32
- mean tt: 16.4%
- score: min -M2, max M1, mean 781.8 over 1788 non-mate lines
- mate markers: +M 182, -M 111
- anomalies: d=1 non-first 345, zero nps 0, dup consecutive 0

### tier master

- moves: 1624
- depth: mean 7.9, median 9.0, max 64
- mean n: 13.53m, mean nps: 7.69m
- mean t: 1.27 s, mean alloc: 1.96 s, mean alloc/t: 2.21
- mean tt: 18.9%
- score: min -M2, max M1, mean 1435.4 over 1332 non-mate lines
- mate markers: +M 218, -M 74
- anomalies: d=1 non-first 187, zero nps 1, dup consecutive 0

## strength verdict

1. master-2 (master): series wins 11, game wins 25
2. hard-1 (hard): series wins 9, game wins 20
3. hard-2 (hard): series wins 9, game wins 20
4. master-1 (master): series wins 8, game wins 19
5. easy-1 (easy): series wins 5, game wins 15
6. easy-2 (easy): series wins 5, game wins 14
7. medium-1 (medium): series wins 5, game wins 14
8. medium-2 (medium): series wins 4, game wins 13

inversions:
- INVERSION: hard-1 (tier hard) ranks 2 above master-1 (tier master)
- INVERSION: hard-2 (tier hard) ranks 3 above master-1 (tier master)
- INVERSION: easy-1 (tier easy) ranks 5 above medium-1 (tier medium)
- INVERSION: easy-1 (tier easy) ranks 5 above medium-2 (tier medium)
- INVERSION: easy-2 (tier easy) ranks 6 above medium-1 (tier medium)
- INVERSION: easy-2 (tier easy) ranks 6 above medium-2 (tier medium)
zero-sum: wins 140, losses 140, draws 2: zero-sum holds

## parse integrity

- unparsable lines: 12
  - logs\tourny\20261010-052633-smoke10\summary.txt:1: "run 1"
  - logs\tourny\20261010-052633-smoke10\summary.txt:3: "finished 20261010-092945"
  - logs\tourny\20261010-052633-smoke10\summary.txt:4: ""
  - logs\tourny\20261010-052633-smoke10\summary.txt:5: "rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames"
  - logs\tourny\20261010-052633-smoke10\summary.txt:6: "1\tmaster-2\tmaster\t1298\t11\t25\t11\t0\t36"
  - logs\tourny\20261010-052633-smoke10\summary.txt:7: "2\tmaster-1\tmaster\t1184\t8\t19\t13\t0\t32"
  - logs\tourny\20261010-052633-smoke10\summary.txt:8: "3\thard-2\thard\t1039\t9\t20\t18\t0\t38"
  - logs\tourny\20261010-052633-smoke10\summary.txt:9: "4\thard-1\thard\t1036\t9\t20\t16\t0\t36"
  - logs\tourny\20261010-052633-smoke10\summary.txt:10: "5\teasy-1\teasy\t939\t5\t15\t21\t0\t36"
  - logs\tourny\20261010-052633-smoke10\summary.txt:11: "6\teasy-2\teasy\t863\t5\t14\t21\t1\t36"
  - logs\tourny\20261010-052633-smoke10\summary.txt:12: "7\tmedium-1\tmedium\t846\t5\t14\t18\t1\t33"
  - logs\tourny\20261010-052633-smoke10\summary.txt:13: "8\tmedium-2\tmedium\t795\t4\t13\t22\t0\t35"
- unattributed m-lines (no pairing header): 0
- files without a full header: 1
  - logs\tourny\20261010-052633-smoke10\summary.txt
- unfinished series (no verdict): 0
- fold mismatches: 0
