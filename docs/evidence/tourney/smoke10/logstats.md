# logstats report

- dir: tourney-logs
- files: 30
- runs: 1 (1)
- series: 30
- games: 76
- m-lines: 2802

## participants

| participant | tier | series | series wins | wins | losses | draws |
| --- | --- | --- | --- | --- | --- | --- |
| hard-1 | hard | 10 | 10 | 20 | 5 | 0 |
| medium-2 | medium | 10 | 5 | 13 | 13 | 0 |
| hard-2 | hard | 10 | 5 | 12 | 12 | 0 |
| easy-2 | easy | 10 | 4 | 10 | 16 | 0 |
| easy-1 | easy | 10 | 3 | 12 | 15 | 0 |
| medium-1 | medium | 10 | 3 | 9 | 15 | 0 |

## telemetry per tier

### tier easy

- moves: 896
- depth: mean 3.0, median 2.0, max 8
- mean n: 37.98k, mean nps: 1850.59m
- mean t: 0.03 s, mean alloc: 0.03 s, mean alloc/t: 1.02
- mean tt: 0.0%
- score: min -M2, max M1, mean 248.6 over 846 non-mate lines
- mate markers: +M 31, -M 19
- anomalies: d=1 non-first 153, zero nps 0, dup consecutive 0

### tier medium

- moves: 962
- depth: mean 3.1, median 3.0, max 9
- mean n: 61.8k, mean nps: 3046.76m
- mean t: 0.03 s, mean alloc: 0.03 s, mean alloc/t: 1.00
- mean tt: 22.7%
- score: min -M2, max M1, mean 122.6 over 920 non-mate lines
- mate markers: +M 28, -M 14
- anomalies: d=1 non-first 143, zero nps 0, dup consecutive 0

### tier hard

- moves: 944
- depth: mean 3.1, median 3.0, max 9
- mean n: 114.06k, mean nps: 2102.33m
- mean t: 0.03 s, mean alloc: 0.03 s, mean alloc/t: 1.00
- mean tt: 26.7%
- score: min -M2, max M1, mean 953.0 over 883 non-mate lines
- mate markers: +M 47, -M 14
- anomalies: d=1 non-first 139, zero nps 0, dup consecutive 0

## strength verdict

1. hard-1 (hard): series wins 10, game wins 20
2. medium-2 (medium): series wins 5, game wins 13
3. hard-2 (hard): series wins 5, game wins 12
4. easy-2 (easy): series wins 4, game wins 10
5. easy-1 (easy): series wins 3, game wins 12
6. medium-1 (medium): series wins 3, game wins 9

inversions:
- INVERSION: medium-2 (tier medium) ranks 2 above hard-2 (tier hard)
- INVERSION: easy-2 (tier easy) ranks 4 above medium-1 (tier medium)
- INVERSION: easy-1 (tier easy) ranks 5 above medium-1 (tier medium)
zero-sum: wins 76, losses 76, draws 0: zero-sum holds

## parse integrity

- unparsable lines: 0
- unattributed m-lines (no pairing header): 0
- files without a full header: 0
- unfinished series (no verdict): 0
- fold mismatches: 0
