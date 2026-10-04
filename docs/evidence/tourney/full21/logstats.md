CGO_ENABLED=1 go -C playground/logstats build -o ../../bin/logstats.exe .
bin/logstats.exe logs/tourny/20261004-212858-ui
# logstats report

- dir: logs\tourny\20261004-212858-ui
- files: 31
- runs: 1 (1)
- series: 31
- games: 71
- m-lines: 5309

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| medium-1 | medium | 10 | 9 | 18 | 4 | 2 |
| medium-2 | medium | 10 | 6 | 14 | 6 | 5 |
| hard-1 | hard | 10 | 6 | 12 | 9 | 2 |
| hard-2 | hard | 10 | 5 | 14 | 8 | 3 |
| easy-1 | easy | 10 | 1 | 3 | 18 | 1 |
| easy-2 | easy | 10 | 0 | 3 | 19 | 1 |

## telemetry per tier

### tier easy

- moves: 1186
- depth: mean 6.7, median 7.0, max 64
- mean n: 5.01m, mean nps: 1.29m
- mean t: 3.92 s, mean alloc: 4.02 s, mean alloc/t: 1.42
- mean tt: 0.0%
- score: min -M2, max M1, mean -1919.5 over 1000 non-mate lines
- mate markers: +M 31, -M 155
- anomalies: d=1 non-first 22, zero nps 1, dup consecutive 0

### tier medium

- moves: 2236
- depth: mean 7.9, median 7.0, max 64
- mean n: 6.31m, mean nps: 2.25m
- mean t: 2.94 s, mean alloc: 3.31 s, mean alloc/t: 4.18
- mean tt: 15.4%
- score: min -M2, max M1, mean 54.3 over 2022 non-mate lines
- mate markers: +M 178, -M 36
- anomalies: d=1 non-first 73, zero nps 2, dup consecutive 0

### tier hard

- moves: 1887
- depth: mean 8.1, median 8.0, max 64
- mean n: 9.5m, mean nps: 3.86m
- mean t: 2.50 s, mean alloc: 3.49 s, mean alloc/t: 3.74
- mean tt: 16.9%
- score: min -M2, max M1, mean 1175.9 over 1616 non-mate lines
- mate markers: +M 186, -M 85
- anomalies: d=1 non-first 69, zero nps 16, dup consecutive 0

## strength verdict

1. medium-1 (medium): series wins 9, game wins 18
2. medium-2 (medium): series wins 6, game wins 14
3. hard-1 (hard): series wins 6, game wins 12
4. hard-2 (hard): series wins 5, game wins 14
5. easy-1 (easy): series wins 1, game wins 3
6. easy-2 (easy): series wins 0, game wins 3

inversions:
- INVERSION: medium-1 (tier medium) ranks 1 above hard-1 (tier hard)
- INVERSION: medium-1 (tier medium) ranks 1 above hard-2 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above hard-1 (tier hard)
- INVERSION: medium-2 (tier medium) ranks 2 above hard-2 (tier hard)
zero-sum: wins 64, losses 64, draws 14: zero-sum holds

## parse integrity

- unparsable lines: 10
  - logs\tourny\20261004-212858-ui\summary.txt:1: "run 1"
  - logs\tourny\20261004-212858-ui\summary.txt:3: "finished 20261004-235904"
  - logs\tourny\20261004-212858-ui\summary.txt:4: ""
  - logs\tourny\20261004-212858-ui\summary.txt:5: "rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames"
  - logs\tourny\20261004-212858-ui\summary.txt:6: "1\tmedium-1\tmedium\t1313\t9\t18\t4\t2\t24"
  - logs\tourny\20261004-212858-ui\summary.txt:7: "2\tmedium-2\tmedium\t1209\t6\t14\t6\t5\t25"
  - logs\tourny\20261004-212858-ui\summary.txt:8: "3\thard-2\thard\t1131\t5\t14\t8\t3\t25"
  - logs\tourny\20261004-212858-ui\summary.txt:9: "4\thard-1\thard\t1069\t6\t12\t9\t2\t23"
  - logs\tourny\20261004-212858-ui\summary.txt:10: "5\teasy-1\teasy\t659\t1\t3\t18\t1\t22"
  - logs\tourny\20261004-212858-ui\summary.txt:11: "6\teasy-2\teasy\t619\t0\t3\t19\t1\t23"
- unattributed m-lines (no pairing header): 0
- files without a full header: 1
  - logs\tourny\20261004-212858-ui\summary.txt
- unfinished series (no verdict): 0
- fold mismatches: 0
