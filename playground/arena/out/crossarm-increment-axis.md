# crossarm seat decomposition

## arm 2+1

dir logs/tourny/20261008-223429-full, files 56, series 56, games 146 (131 decisive, 15 drawn), zero-sum holds, bad lines 0, unfinished 0, fold mismatches 0


### standings

| seat | tier | series | games | red | blue |
|---|---|---|---|---|---|
| master-1 | master | 12-2 | 26-7-1 | 16-1-1 | 10-6-0 |
| master-2 | master | 9-4 | 21-14-2 | 15-3-1 | 6-11-1 |
| hard-1 | hard | 3-9 | 10-21-4 | 7-7-1 | 3-14-3 |
| hard-2 | hard | 6-5 | 16-15-4 | 11-6-2 | 5-9-2 |
| medium-1 | medium | 6-8 | 16-17-5 | 12-4-2 | 4-13-3 |
| medium-2 | medium | 6-6 | 16-18-2 | 10-6-1 | 6-12-1 |
| easy-1 | easy | 6-7 | 17-18-4 | 13-5-2 | 4-13-2 |
| easy-2 | easy | 2-9 | 9-21-8 | 7-8-5 | 2-13-3 |

### per-seat records by opponent tier

| seat | vs easy | vs medium | vs hard | vs master |
|---|---|---|---|---|
| master-1 | 7-3-1 | 8-2-0 | 8-0-0 | 3-2-0 |
| master-2 | 7-3-1 | 6-5-0 | 6-3-1 | 2-3-0 |
| hard-1 | 3-7-1 | 3-5-2 | 3-1-1 | 1-8-0 |
| hard-2 | 6-3-2 | 7-3-0 | 1-3-1 | 2-6-1 |
| medium-1 | 4-3-4 | 3-3-0 | 5-4-1 | 4-7-0 |
| medium-2 | 7-2-1 | 3-3-0 | 3-6-1 | 3-7-0 |
| easy-1 | 4-1-1 | 3-6-2 | 5-4-1 | 5-7-0 |
| easy-2 | 1-4-1 | 2-5-3 | 5-5-2 | 1-7-2 |

### series matrix, row winner over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 1-1 | 2-0 | 2-0 | 2-0 | 2-0 | 1-1 | 2-0 |
| master-2 | 1-1 |  | 2-0 | 1-1 | 1-1 | 1-1 | 2-0 | 1-0 |
| hard-1 | 0-2 | 0-2 |  | 1-0 | 0-2 | 1-0 | 0-2 | 1-1 |
| hard-2 | 0-2 | 1-1 | 0-1 |  | 2-0 | 1-1 | 1-0 | 1-0 |
| medium-1 | 0-2 | 1-1 | 2-0 | 0-2 |  | 1-1 | 1-1 | 1-1 |
| medium-2 | 0-2 | 1-1 | 0-1 | 1-1 | 1-1 |  | 2-0 | 1-0 |
| easy-1 | 1-1 | 0-2 | 2-0 | 0-1 | 1-1 | 0-2 |  | 2-0 |
| easy-2 | 0-2 | 0-1 | 1-1 | 0-1 | 1-1 | 0-1 | 0-2 |  |

### games matrix, row over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 3-2-0 | 4-0-0 | 4-0-0 | 4-1-0 | 4-1-0 | 3-3-0 | 4-0-1 |
| master-2 | 2-3-0 |  | 4-1-0 | 2-2-1 | 3-3-0 | 3-2-0 | 4-2-0 | 3-1-1 |
| hard-1 | 0-4-0 | 1-4-0 |  | 3-1-1 | 0-4-1 | 3-1-1 | 1-4-0 | 2-3-1 |
| hard-2 | 0-4-0 | 2-2-1 | 1-3-1 |  | 4-1-0 | 3-2-0 | 3-1-1 | 3-2-1 |
| medium-1 | 1-4-0 | 3-3-0 | 4-0-1 | 1-4-0 |  | 3-3-0 | 2-2-2 | 2-1-2 |
| medium-2 | 1-4-0 | 2-3-0 | 1-3-1 | 2-3-0 | 3-3-0 |  | 4-1-0 | 3-1-1 |
| easy-1 | 3-3-0 | 2-4-0 | 4-1-0 | 1-3-1 | 2-2-2 | 1-4-0 |  | 4-1-1 |
| easy-2 | 0-4-1 | 1-3-1 | 3-2-1 | 2-3-1 | 1-2-2 | 1-3-1 | 1-4-1 |  |

### per-seat telemetry, untagged searches

| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |
|---|---|---|---|---|---|---|---|---|---|
| master-1 | 855 | 252 | 8.4 | 9.0 | 3.43 | 4.33 | 0.79 | 8.52m | 27.71m |
| master-2 | 936 | 281 | 8.5 | 9.0 | 3.51 | 4.27 | 0.81 | 8.78m | 28.85m |
| hard-1 | 1240 | 40 | 9.2 | 8.0 | 2.50 | 3.17 | 0.77 | 5.79m | 13.58m |
| hard-2 | 1285 | 60 | 8.9 | 8.0 | 2.52 | 3.28 | 0.75 | 5.76m | 13.30m |
| medium-1 | 1631 | 45 | 8.8 | 8.0 | 2.96 | 3.10 | 0.96 | 2.75m | 7.93m |
| medium-2 | 1329 | 59 | 8.1 | 8.0 | 3.19 | 3.30 | 0.97 | 2.70m | 8.31m |
| easy-1 | 1448 | 0 | 8.0 | 8.0 | 3.18 | 3.35 | 0.95 | 1.28m | 4.02m |
| easy-2 | 1848 | 0 | 8.5 | 7.0 | 2.71 | 2.90 | 0.95 | 1.52m | 3.75m |

### hard-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 344 | 9.9 | 3.69 | 4.49 | 0.82 |
| mid | 275 | 8.3 | 3.48 | 4.27 | 0.78 |
| late | 621 | 9.2 | 1.41 | 1.94 | 0.74 |

### hard-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s10 | 2 | easy-2 | easy | red | 66 | 4 | -M2 | M1 |
| s10 | 3 | easy-2 | easy | blue | 39 | 4 | -M2 | M1 |
| s15 | 1 | medium-1 | medium | blue | 43 | 4 | -M2 | M1 |
| s15 | 3 | medium-1 | medium | blue | 57 | 4 | -M2 | M1 |
| s19 | 1 | medium-2 | medium | blue | 43 | 4 | -M2 | M1 |
| s24 | 1 | master-1 | master | red | 86 | open 4 | -M2 | M1 |
| s24 | 2 | master-1 | master | blue | 73 | 4 | -M2 | M1 |
| s25 | 1 | master-2 | master | red | 68 | 4 | -M2 | M1 |
| s25 | 2 | master-2 | master | blue | 31 | 4 | -M2 | M1 |
| s32 | 1 | master-2 | master | blue | 41 | 4 | -M2 | M1 |
| s32 | 3 | master-2 | master | blue | 63 | open 4 | -M2 | M1 |
| s33 | 1 | master-1 | master | blue | 23 | open 4 | -M2 | M1 |
| s33 | 2 | master-1 | master | red | 110 | 4 | -M2 | M1 |
| s34 | 1 | hard-2 | hard | blue | 51 | 4 | -M2 | M1 |
| s42 | 1 | medium-1 | medium | red | 40 | open 4 | -52900 | M1 |
| s42 | 2 | medium-1 | medium | blue | 23 | 4 | -M2 | M1 |
| s47 | 1 | easy-2 | easy | red | 54 | 4 | -M2 | M1 |
| s04 | 1 | easy-1 | easy | blue | 21 | 4 | -M2 | M1 |
| s04 | 3 | easy-1 | easy | blue | 35 | 4 | -M2 | M1 |
| s53 | 1 | easy-1 | easy | red | 52 | 4 | -M2 | M1 |
| s53 | 2 | easy-1 | easy | blue | 21 | 4 | -M2 | M1 |

### hard-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 347 | 10.0 | 3.61 | 4.51 | 0.80 |
| mid | 294 | 8.5 | 3.67 | 4.48 | 0.82 |
| late | 644 | 8.4 | 1.41 | 2.08 | 0.70 |

### hard-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s11 | 3 | easy-2 | easy | blue | 43 | 4 | -M2 | M1 |
| s23 | 1 | hard-1 | hard | blue | 45 | 4 | -M2 | M1 |
| s23 | 2 | hard-1 | hard | red | 64 | open 4 | -M2 | M1 |
| s26 | 1 | master-1 | master | red | 48 | open 4 | -M2 | M1 |
| s26 | 2 | master-1 | master | blue | 25 | open 4 | -M2 | M1 |
| s27 | 1 | master-2 | master | red | 66 | 4 | -M2 | M1 |
| s27 | 2 | master-2 | master | blue | 41 | 4 | -M2 | M1 |
| s31 | 1 | master-1 | master | blue | 41 | 4 | -M2 | M1 |
| s31 | 2 | master-1 | master | red | 42 | 4 | -M2 | M1 |
| s34 | 2 | hard-1 | hard | blue | 67 | 4 | -M2 | M1 |
| s37 | 2 | medium-2 | medium | blue | 79 | open 4 | -M2 | M1 |
| s37 | 3 | medium-2 | medium | red | 46 | open 4 | -M2 | M1 |
| s41 | 1 | medium-1 | medium | red | 32 | 4 | -M2 | M1 |
| s46 | 2 | easy-2 | easy | blue | 21 | 4 | -M2 | M1 |
| s52 | 2 | easy-1 | easy | blue | 37 | 4 | -M2 | M1 |

### master-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 270 | 10.3 | 3.68 | 4.76 | 0.78 |
| mid | 247 | 8.7 | 4.29 | 5.22 | 0.82 |
| late | 338 | 6.6 | 2.59 | 3.32 | 0.76 |

### master-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s29 | 1 | master-2 | master | blue | 73 | 4 | -M2 | M1 |
| s29 | 3 | master-2 | master | blue | 53 | 4 | -M2 | M1 |
| s36 | 2 | medium-2 | medium | blue | 65 | 4 | -M2 | M1 |
| s40 | 2 | medium-1 | medium | blue | 47 | 4 | -M2 | M1 |
| s51 | 2 | easy-1 | easy | blue | 39 | 4 | -M2 | M1 |
| s06 | 2 | easy-1 | easy | red | 62 | 4 | -M2 | M1 |
| s06 | 3 | easy-1 | easy | blue | 43 | 4 | -M2 | M1 |

### medium-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 380 | 9.6 | 4.43 | 4.49 | 0.99 |
| mid | 336 | 8.5 | 4.43 | 4.47 | 0.99 |
| late | 915 | 8.6 | 1.82 | 2.02 | 0.94 |

### medium-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s14 | 2 | medium-2 | medium | blue | 43 | 4 | -M2 | M1 |
| s16 | 1 | hard-2 | hard | red | 40 | open 4 | -M2 | M1 |
| s16 | 2 | hard-2 | hard | blue | 43 | 4 | -M2 | M1 |
| s17 | 1 | master-1 | master | red | 96 | 4 | -M2 | M1 |
| s17 | 2 | master-1 | master | blue | 73 | 4 | -M2 | M1 |
| s18 | 2 | master-2 | master | blue | 25 | 4 | -M2 | M1 |
| s02 | 1 | easy-1 | easy | blue | 39 | 4 | -M2 | M1 |
| s39 | 1 | master-2 | master | blue | 37 | 4 | -M2 | M1 |
| s39 | 3 | master-2 | master | blue | 45 | 4 | -M2 | M1 |
| s40 | 1 | master-1 | master | blue | 113 | 4 | -M2 | M1 |
| s40 | 3 | master-1 | master | blue | 47 | 4 | -M2 | M1 |
| s41 | 2 | hard-2 | hard | red | 78 | open 4 | -M2 | M1 |
| s41 | 3 | hard-2 | hard | blue | 83 | 4 | -M2 | M1 |
| s43 | 2 | medium-2 | medium | red | 66 | 4 | -M2 | M1 |
| s43 | 3 | medium-2 | medium | blue | 99 | 4 | -M2 | M1 |
| s55 | 2 | easy-1 | easy | blue | 79 | 4 | -M2 | M1 |
| s08 | 3 | easy-2 | easy | blue | 45 | 4 | -M2 | M1 |

### medium-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 360 | 9.5 | 4.36 | 4.44 | 0.98 |
| mid | 317 | 7.9 | 4.25 | 4.32 | 0.98 |
| late | 652 | 7.5 | 2.03 | 2.17 | 0.95 |

### medium-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s14 | 1 | medium-1 | medium | blue | 65 | 4 | -M2 | M1 |
| s14 | 3 | medium-1 | medium | blue | 125 | 4 | -M2 | M1 |
| s19 | 2 | hard-1 | hard | blue | 41 | 4 | -M2 | M1 |
| s20 | 1 | hard-2 | hard | red | 118 | 4 | -M2 | M1 |
| s20 | 2 | hard-2 | hard | blue | 31 | 4 | -M2 | M1 |
| s21 | 1 | master-1 | master | red | 40 | open 4 | -M2 | M1 |
| s21 | 2 | master-1 | master | blue | 23 | open 4 | -M2 | M1 |
| s22 | 1 | master-2 | master | red | 100 | 4 | 0 | M1 |
| s35 | 1 | master-2 | master | blue | 65 | 4 | -M2 | M1 |
| s35 | 2 | master-2 | master | red | 78 | 4 | -M2 | M1 |
| s36 | 1 | master-1 | master | blue | 59 | 4 | -M2 | M1 |
| s36 | 3 | master-1 | master | blue | 39 | 4 | -M2 | M1 |
| s37 | 1 | hard-2 | hard | blue | 41 | 4 | -M2 | M1 |
| s38 | 1 | hard-1 | hard | blue | 33 | 4 | -M2 | M1 |
| s38 | 2 | hard-1 | hard | red | 106 | 4 | -M2 | M1 |
| s03 | 1 | easy-1 | easy | blue | 37 | 4 | -M2 | M1 |
| s43 | 1 | medium-1 | medium | red | 136 | 4 | -M2 | M1 |
| s09 | 1 | easy-2 | easy | blue | 57 | 4 | -M2 | M1 |

### draws by tier pair

easy-easy: 1
easy-medium: 5
easy-hard: 3
easy-master: 2
hard-medium: 2
hard-hard: 1
hard-master: 1

## arm 3+2

dir logs/tourny/20261009-101129-smoke32, files 56, series 56, games 147 (130 decisive, 17 drawn), zero-sum holds, bad lines 0, unfinished 0, fold mismatches 0


### standings

| seat | tier | series | games | red | blue |
|---|---|---|---|---|---|
| master-1 | master | 5-7 | 13-17-4 | 8-7-2 | 5-10-2 |
| master-2 | master | 10-2 | 24-12-2 | 17-2-1 | 7-10-1 |
| hard-1 | hard | 6-6 | 16-15-6 | 11-5-2 | 5-10-4 |
| hard-2 | hard | 8-5 | 19-14-3 | 14-3-1 | 5-11-2 |
| medium-1 | medium | 6-6 | 16-17-4 | 11-6-2 | 5-11-2 |
| medium-2 | medium | 9-5 | 20-14-3 | 12-4-2 | 8-10-1 |
| easy-1 | easy | 1-11 | 11-24-2 | 9-7-2 | 2-17-0 |
| easy-2 | easy | 4-7 | 11-17-10 | 7-7-5 | 4-10-5 |

### per-seat records by opponent tier

| seat | vs easy | vs medium | vs hard | vs master |
|---|---|---|---|---|
| master-1 | 5-3-4 | 6-2-0 | 1-8-0 | 1-4-0 |
| master-2 | 8-3-0 | 5-5-1 | 7-3-1 | 4-1-0 |
| hard-1 | 5-2-3 | 2-7-1 | 3-2-1 | 6-4-1 |
| hard-2 | 7-2-1 | 5-5-1 | 2-3-1 | 5-4-0 |
| medium-1 | 5-4-2 | 3-3-0 | 5-4-1 | 3-6-1 |
| medium-2 | 6-3-2 | 3-3-0 | 7-3-1 | 4-5-0 |
| easy-1 | 3-2-0 | 2-8-0 | 2-7-1 | 4-7-1 |
| easy-2 | 2-3-0 | 5-3-4 | 2-5-3 | 2-6-3 |

### series matrix, row winner over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 0-2 | 0-2 | 0-2 | 2-0 | 1-1 | 1-0 | 1-0 |
| master-2 | 2-0 |  | 1-0 | 2-0 | 0-1 | 1-1 | 2-0 | 2-0 |
| hard-1 | 2-0 | 0-1 |  | 1-1 | 1-1 | 0-2 | 1-0 | 1-1 |
| hard-2 | 2-0 | 0-2 | 1-1 |  | 1-1 | 1-1 | 2-0 | 1-0 |
| medium-1 | 0-2 | 1-0 | 1-1 | 1-1 |  | 1-1 | 2-0 | 0-1 |
| medium-2 | 1-1 | 1-1 | 2-0 | 1-1 | 1-1 |  | 2-0 | 1-1 |
| easy-1 | 0-1 | 0-2 | 0-1 | 0-2 | 0-2 | 0-2 |  | 1-1 |
| easy-2 | 0-1 | 0-2 | 1-1 | 0-1 | 1-0 | 1-1 | 1-1 |  |

### games matrix, row over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 1-4-0 | 1-4-0 | 0-4-0 | 4-0-0 | 2-2-0 | 3-2-1 | 2-1-3 |
| master-2 | 4-1-0 |  | 3-2-1 | 4-1-0 | 2-3-1 | 3-2-0 | 4-2-0 | 4-1-0 |
| hard-1 | 4-1-0 | 2-3-1 |  | 3-2-1 | 2-3-0 | 0-4-1 | 3-1-1 | 2-1-2 |
| hard-2 | 4-0-0 | 1-4-0 | 2-3-1 |  | 2-2-1 | 3-3-0 | 4-1-0 | 3-1-1 |
| medium-1 | 0-4-0 | 3-2-1 | 3-2-0 | 2-2-1 |  | 3-3-0 | 4-1-0 | 1-3-2 |
| medium-2 | 2-2-0 | 2-3-0 | 4-0-1 | 3-3-0 | 3-3-0 |  | 4-1-0 | 2-2-2 |
| easy-1 | 2-3-1 | 2-4-0 | 1-3-1 | 1-4-0 | 1-4-0 | 1-4-0 |  | 3-2-0 |
| easy-2 | 1-2-3 | 1-4-0 | 1-2-2 | 1-3-1 | 3-1-2 | 2-2-2 | 2-3-0 |  |

### per-seat telemetry, untagged searches

| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |
|---|---|---|---|---|---|---|---|---|---|
| master-1 | 1082 | 295 | 8.4 | 8.0 | 4.83 | 6.28 | 0.77 | 8.04m | 34.81m |
| master-2 | 991 | 298 | 8.4 | 9.0 | 5.26 | 6.76 | 0.77 | 7.73m | 38.78m |
| hard-1 | 1566 | 68 | 9.2 | 8.0 | 3.79 | 5.08 | 0.74 | 4.96m | 17.28m |
| hard-2 | 1340 | 80 | 8.9 | 8.0 | 4.28 | 5.49 | 0.78 | 4.51m | 18.65m |
| medium-1 | 1657 | 50 | 8.4 | 8.0 | 4.89 | 5.10 | 0.97 | 2.82m | 12.82m |
| medium-2 | 1471 | 66 | 8.4 | 8.0 | 5.31 | 5.54 | 0.97 | 2.69m | 13.37m |
| easy-1 | 1280 | 0 | 7.9 | 8.0 | 5.49 | 5.75 | 0.96 | 1.32m | 7.18m |
| easy-2 | 2171 | 0 | 8.7 | 8.0 | 4.24 | 4.64 | 0.94 | 1.62m | 6.32m |

### hard-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 363 | 10.0 | 5.62 | 7.06 | 0.79 |
| mid | 307 | 8.1 | 5.60 | 6.98 | 0.79 |
| late | 896 | 9.3 | 2.43 | 3.63 | 0.70 |

### hard-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s15 | 1 | medium-1 | medium | blue | 43 | 4 | -M2 | M1 |
| s19 | 1 | medium-2 | medium | blue | 53 | 4 | -M2 | M1 |
| s19 | 3 | medium-2 | medium | blue | 55 | 4 | -M2 | M1 |
| s24 | 1 | master-1 | master | red | 68 | open 4 | -M2 | M1 |
| s25 | 1 | master-2 | master | red | 44 | open 4 | -M2 | M1 |
| s32 | 1 | master-2 | master | blue | 33 | 4 | -M2 | M1 |
| s32 | 3 | master-2 | master | blue | 41 | 4 | -M2 | M1 |
| s34 | 1 | hard-2 | hard | blue | 41 | 4 | -M2 | M1 |
| s34 | 3 | hard-2 | hard | blue | 69 | 4 | -M2 | M1 |
| s38 | 1 | medium-2 | medium | red | 44 | 4 | -M2 | M1 |
| s38 | 2 | medium-2 | medium | blue | 61 | 4 | -M2 | M1 |
| s42 | 1 | medium-1 | medium | red | 130 | 4 | -M2 | M1 |
| s42 | 2 | medium-1 | medium | blue | 61 | 4 | -M2 | M1 |
| s47 | 3 | easy-2 | easy | red | 56 | 4 | -M2 | M1 |
| s04 | 3 | easy-1 | easy | blue | 49 | 4 | -M2 | M1 |

### hard-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 353 | 10.0 | 5.72 | 7.08 | 0.81 |
| mid | 301 | 8.2 | 5.79 | 7.34 | 0.79 |
| late | 686 | 8.5 | 2.87 | 3.86 | 0.77 |

### hard-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s11 | 1 | easy-2 | easy | blue | 29 | 4 | -M2 | M1 |
| s16 | 1 | medium-1 | medium | blue | 55 | 4 | 0 | M1 |
| s16 | 2 | medium-1 | medium | red | 32 | 4 | -M2 | M1 |
| s20 | 1 | medium-2 | medium | blue | 57 | open 4 | -M2 | M1 |
| s20 | 3 | medium-2 | medium | blue | 55 | 4 | -M2 | M1 |
| s23 | 1 | hard-1 | hard | blue | 75 | open 4 | -M2 | M1 |
| s23 | 3 | hard-1 | hard | blue | 41 | 4 | -M2 | M1 |
| s27 | 1 | master-2 | master | red | 92 | 4 | -M2 | M1 |
| s27 | 2 | master-2 | master | blue | 31 | 4 | -M2 | M1 |
| s30 | 1 | master-2 | master | blue | 41 | 4 | -M2 | M1 |
| s30 | 3 | master-2 | master | blue | 117 | 4 | -M2 | M1 |
| s34 | 2 | hard-1 | hard | blue | 105 | 4 | -M2 | M1 |
| s37 | 2 | medium-2 | medium | blue | 51 | 4 | -M2 | M1 |
| s52 | 1 | easy-1 | easy | red | 58 | 4 | -M2 | M1 |

### master-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 243 | 10.5 | 6.45 | 7.71 | 0.84 |
| mid | 232 | 8.6 | 7.20 | 9.05 | 0.80 |
| late | 607 | 7.4 | 3.27 | 4.64 | 0.73 |

### master-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s12 | 3 | easy-2 | easy | blue | 85 | 4 | -M2 | M1 |
| s21 | 1 | medium-2 | medium | blue | 29 | open 4 | -M2 | M1 |
| s21 | 2 | medium-2 | medium | red | 40 | open 4 | -53000 | M1 |
| s24 | 2 | hard-1 | hard | red | 82 | open 4 | -M2 | M1 |
| s24 | 3 | hard-1 | hard | blue | 21 | 4 | -M2 | M1 |
| s26 | 1 | hard-2 | hard | blue | 37 | 4 | -M2 | M1 |
| s26 | 2 | hard-2 | hard | red | 24 | 4 | -M2 | M1 |
| s28 | 2 | master-2 | master | blue | 73 | 4 | -M2 | M1 |
| s28 | 3 | master-2 | master | red | 36 | 4 | -M2 | M1 |
| s29 | 1 | master-2 | master | blue | 35 | 4 | -M2 | M1 |
| s29 | 2 | master-2 | master | red | 58 | 4 | -M2 | M1 |
| s31 | 1 | hard-2 | hard | red | 142 | 4 | -M2 | M1 |
| s31 | 2 | hard-2 | hard | blue | 43 | 4 | -M2 | M1 |
| s33 | 1 | hard-1 | hard | red | 98 | 4 | -M2 | M1 |
| s33 | 2 | hard-1 | hard | blue | 45 | 4 | -M2 | M1 |
| s51 | 2 | easy-1 | easy | blue | 67 | 4 | -M2 | M1 |
| s06 | 1 | easy-1 | easy | blue | 33 | 4 | -M2 | M1 |

### medium-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 370 | 9.9 | 7.00 | 7.09 | 0.99 |
| mid | 349 | 8.4 | 6.99 | 7.05 | 0.99 |
| late | 938 | 7.8 | 3.27 | 3.59 | 0.95 |

### medium-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s14 | 1 | medium-2 | medium | red | 64 | 4 | -M2 | M1 |
| s15 | 2 | hard-1 | hard | blue | 55 | 4 | -M2 | M1 |
| s15 | 3 | hard-1 | hard | red | 96 | 4 | -M2 | M1 |
| s17 | 1 | master-1 | master | red | 34 | 4 | -M2 | M1 |
| s17 | 2 | master-1 | master | blue | 33 | 4 | -M2 | M1 |
| s18 | 2 | master-2 | master | blue | 43 | 4 | -M2 | M1 |
| s39 | 1 | master-2 | master | blue | 37 | 4 | -M2 | M1 |
| s40 | 1 | master-1 | master | blue | 43 | 4 | -M2 | M1 |
| s40 | 2 | master-1 | master | red | 88 | 4 | -M2 | M1 |
| s41 | 1 | hard-2 | hard | blue | 43 | 4 | -M2 | M1 |
| s41 | 3 | hard-2 | hard | blue | 49 | 4 | -M2 | M1 |
| s43 | 1 | medium-2 | medium | blue | 109 | open 4 | -M2 | M1 |
| s43 | 3 | medium-2 | medium | blue | 127 | 4 | -M2 | M1 |
| s49 | 1 | easy-2 | easy | red | 90 | 4 | -M2 | M1 |
| s49 | 3 | easy-2 | easy | red | 38 | 4 | -M2 | M1 |
| s55 | 2 | easy-1 | easy | blue | 49 | 4 | -M2 | M1 |
| s08 | 1 | easy-2 | easy | blue | 47 | 4 | -M2 | M1 |

### medium-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 370 | 9.9 | 7.08 | 7.21 | 0.98 |
| mid | 337 | 8.1 | 7.41 | 7.51 | 0.99 |
| late | 764 | 7.8 | 3.53 | 3.85 | 0.95 |

### medium-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s14 | 2 | medium-1 | medium | red | 88 | 4 | -M2 | M1 |
| s14 | 3 | medium-1 | medium | blue | 175 | 4 | -M2 | M1 |
| s20 | 2 | hard-2 | hard | blue | 41 | 4 | -M2 | M1 |
| s22 | 1 | master-2 | master | red | 72 | 4 | -M2 | M1 |
| s22 | 2 | master-2 | master | blue | 43 | 4 | -M2 | M1 |
| s35 | 1 | master-2 | master | blue | 45 | 4 | -M2 | M1 |
| s36 | 1 | master-1 | master | blue | 33 | 4 | -M2 | M1 |
| s36 | 2 | master-1 | master | red | 76 | open 4 | -M2 | M1 |
| s37 | 1 | hard-2 | hard | blue | 55 | 4 | -M2 | M1 |
| s37 | 3 | hard-2 | hard | blue | 153 | 4 | -M2 | M1 |
| s03 | 1 | easy-1 | easy | blue | 67 | 4 | -M2 | M1 |
| s43 | 2 | medium-1 | medium | blue | 43 | 4 | -M2 | M1 |
| s48 | 2 | easy-2 | easy | blue | 165 | 4 | -M2 | M1 |
| s48 | 3 | easy-2 | easy | red | 118 | 4 | -M2 | M1 |

### draws by tier pair

easy-medium: 4
easy-hard: 4
easy-master: 4
hard-medium: 2
hard-hard: 1
hard-master: 1
master-medium: 1

## arm 1+0

dir logs/tourny/20261010-052633-smoke10, files 56, series 56, games 141 (140 decisive, 1 drawn), zero-sum holds, bad lines 0, unfinished 0, fold mismatches 0


### standings

| seat | tier | series | games | red | blue |
|---|---|---|---|---|---|
| master-1 | master | 8-6 | 19-13-0 | 12-3-0 | 7-10-0 |
| master-2 | master | 11-3 | 25-11-0 | 16-2-0 | 9-9-0 |
| hard-1 | hard | 9-5 | 20-16-0 | 13-6-0 | 7-10-0 |
| hard-2 | hard | 9-5 | 20-18-0 | 14-5-0 | 6-13-0 |
| medium-1 | medium | 5-9 | 14-18-1 | 8-9-0 | 6-9-1 |
| medium-2 | medium | 4-10 | 13-22-0 | 9-8-0 | 4-14-0 |
| easy-1 | easy | 5-9 | 15-21-0 | 11-7-0 | 4-14-0 |
| easy-2 | easy | 5-9 | 14-21-1 | 8-9-1 | 6-12-0 |

### per-seat records by opponent tier

| seat | vs easy | vs medium | vs hard | vs master |
|---|---|---|---|---|
| master-1 | 6-2-0 | 7-3-0 | 5-4-0 | 1-4-0 |
| master-2 | 7-5-0 | 7-3-0 | 7-2-0 | 4-1-0 |
| hard-1 | 6-5-0 | 8-3-0 | 4-2-0 | 2-6-0 |
| hard-2 | 8-4-0 | 6-4-0 | 2-4-0 | 4-6-0 |
| medium-1 | 5-4-1 | 4-0-0 | 4-6-0 | 1-8-0 |
| medium-2 | 5-4-0 | 0-4-0 | 3-8-0 | 5-6-0 |
| easy-1 | 2-3-0 | 6-3-0 | 5-7-0 | 2-8-0 |
| easy-2 | 3-2-0 | 2-7-1 | 4-7-0 | 5-5-0 |

### series matrix, row winner over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 0-2 | 1-1 | 1-1 | 2-0 | 1-1 | 2-0 | 1-1 |
| master-2 | 2-0 |  | 2-0 | 1-1 | 2-0 | 1-1 | 2-0 | 1-1 |
| hard-1 | 1-1 | 0-2 |  | 2-0 | 2-0 | 2-0 | 1-1 | 1-1 |
| hard-2 | 1-1 | 1-1 | 0-2 |  | 1-1 | 2-0 | 2-0 | 2-0 |
| medium-1 | 0-2 | 0-2 | 0-2 | 1-1 |  | 2-0 | 1-1 | 1-1 |
| medium-2 | 1-1 | 1-1 | 0-2 | 0-2 | 0-2 |  | 0-2 | 2-0 |
| easy-1 | 0-2 | 0-2 | 1-1 | 0-2 | 1-1 | 2-0 |  | 1-1 |
| easy-2 | 1-1 | 1-1 | 1-1 | 0-2 | 1-1 | 0-2 | 1-1 |  |

### games matrix, row over column

| seat | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|
| master-1 |  | 1-4-0 | 2-2-0 | 3-2-0 | 4-0-0 | 3-3-0 | 4-0-0 | 2-2-0 |
| master-2 | 4-1-0 |  | 4-0-0 | 3-2-0 | 4-1-0 | 3-2-0 | 4-2-0 | 3-3-0 |
| hard-1 | 2-2-0 | 0-4-0 |  | 4-2-0 | 4-2-0 | 4-1-0 | 3-3-0 | 3-2-0 |
| hard-2 | 2-3-0 | 2-3-0 | 2-4-0 |  | 2-2-0 | 4-2-0 | 4-2-0 | 4-2-0 |
| medium-1 | 0-4-0 | 1-4-0 | 2-4-0 | 2-2-0 |  | 4-0-0 | 2-2-0 | 3-2-1 |
| medium-2 | 3-3-0 | 2-3-0 | 1-4-0 | 2-4-0 | 0-4-0 |  | 1-4-0 | 4-0-0 |
| easy-1 | 0-4-0 | 2-4-0 | 3-3-0 | 2-4-0 | 2-2-0 | 4-1-0 |  | 2-3-0 |
| easy-2 | 2-2-0 | 3-3-0 | 2-3-0 | 2-4-0 | 2-3-1 | 0-4-0 | 3-2-0 |  |

### per-seat telemetry, untagged searches

| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |
|---|---|---|---|---|---|---|---|---|---|
| master-1 | 597 | 156 | 8.3 | 9.0 | 1.57 | 1.92 | 0.74 | 8.45m | 14.44m |
| master-2 | 684 | 187 | 7.8 | 9.0 | 1.53 | 1.84 | 0.74 | 8.22m | 14.22m |
| hard-1 | 962 | 50 | 7.4 | 9.0 | 1.26 | 1.55 | 0.67 | 4.49m | 6.81m |
| hard-2 | 1010 | 59 | 7.5 | 9.0 | 1.26 | 1.55 | 0.70 | 4.62m | 6.86m |
| medium-1 | 984 | 28 | 6.4 | 8.0 | 1.34 | 1.37 | 0.76 | 2.21m | 3.83m |
| medium-2 | 983 | 23 | 6.9 | 8.0 | 1.46 | 1.48 | 0.80 | 2.31m | 4.19m |
| easy-1 | 970 | 0 | 6.7 | 8.0 | 1.46 | 1.51 | 0.80 | 1.17m | 2.10m |
| easy-2 | 1084 | 0 | 6.1 | 7.0 | 1.39 | 1.43 | 0.78 | 1.20m | 2.10m |

### hard-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 355 | 9.7 | 1.57 | 1.91 | 0.82 |
| mid | 273 | 8.3 | 1.52 | 1.84 | 0.82 |
| late | 334 | 4.3 | 0.71 | 0.92 | 0.39 |

### hard-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s15 | 2 | medium-1 | medium | red | 68 | 4 | -13000 | M1 |
| s23 | 2 | hard-2 | hard | blue | 21 | 4 | -M2 | M1 |
| s25 | 1 | master-2 | master | red | 68 | 4 | -20600 | M1 |
| s25 | 2 | master-2 | master | blue | 75 | 4 | -10400 | M1 |
| s32 | 1 | master-2 | master | blue | 31 | 4 | -M2 | M1 |
| s32 | 2 | master-2 | master | red | 66 | 4 | -17300 | M1 |
| s33 | 1 | master-1 | master | blue | 31 | 4 | -M2 | M1 |
| s33 | 2 | master-1 | master | red | 62 | 4 | -12400 | M1 |
| s34 | 1 | hard-2 | hard | blue | 85 | 4 | -22100 | M1 |
| s38 | 2 | medium-2 | medium | blue | 81 | 4 | -18000 | M1 |
| s42 | 2 | medium-1 | medium | blue | 41 | 4 | -M2 | M1 |
| s47 | 1 | easy-2 | easy | red | 70 | 4 | -7200 | M1 |
| s47 | 3 | easy-2 | easy | red | 76 | 4 | -24200 | M1 |
| s04 | 1 | easy-1 | easy | blue | 21 | 4 | -M2 | M1 |
| s04 | 3 | easy-1 | easy | blue | 55 | 4 | -M2 | M1 |
| s53 | 2 | easy-1 | easy | blue | 21 | 4 | -M2 | M1 |

### hard-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 375 | 9.6 | 1.59 | 1.90 | 0.83 |
| mid | 306 | 8.1 | 1.43 | 1.72 | 0.82 |
| late | 329 | 4.4 | 0.72 | 1.00 | 0.44 |

### hard-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s11 | 1 | easy-2 | easy | blue | 37 | 4 | -M2 | M1 |
| s16 | 1 | medium-1 | medium | blue | 45 | 4 | -M2 | M1 |
| s16 | 2 | medium-1 | medium | red | 30 | 4 | -M2 | M1 |
| s20 | 2 | medium-2 | medium | red | 92 | double 4 | -39100 | M1 |
| s23 | 1 | hard-1 | hard | blue | 25 | open 4 | -M2 | M1 |
| s23 | 3 | hard-1 | hard | blue | 43 | 4 | -M2 | M1 |
| s26 | 2 | master-1 | master | blue | 41 | 4 | -M2 | M1 |
| s27 | 1 | master-2 | master | red | 48 | 4 | -11600 | M1 |
| s27 | 2 | master-2 | master | blue | 77 | 4 | -M2 | M1 |
| s30 | 1 | master-2 | master | blue | 63 | 4 | -15100 | M1 |
| s31 | 1 | master-1 | master | blue | 43 | 4 | -M2 | M1 |
| s31 | 2 | master-1 | master | red | 88 | double 4 | -48300 | M1 |
| s34 | 2 | hard-1 | hard | blue | 81 | 4 | -23700 | M1 |
| s34 | 3 | hard-1 | hard | red | 58 | 4 | -M2 | M1 |
| s37 | 2 | medium-2 | medium | blue | 21 | 4 | -M2 | M1 |
| s46 | 2 | easy-2 | easy | blue | 127 | 4 | -28000 | M1 |
| s52 | 2 | easy-1 | easy | blue | 21 | 4 | -M2 | M1 |
| s05 | 1 | easy-1 | easy | blue | 71 | 4 | -19900 | M1 |

### master-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 251 | 10.0 | 1.69 | 2.01 | 0.84 |
| mid | 210 | 8.3 | 1.72 | 2.10 | 0.80 |
| late | 136 | 5.2 | 1.11 | 1.46 | 0.49 |

### master-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s12 | 1 | easy-2 | easy | blue | 21 | 4 | -M2 | M1 |
| s12 | 2 | easy-2 | easy | red | 88 | 4 | -M2 | M1 |
| s21 | 1 | medium-2 | medium | blue | 43 | 4 | -M2 | M1 |
| s21 | 3 | medium-2 | medium | blue | 23 | 4 | -M2 | M1 |
| s24 | 1 | hard-1 | hard | blue | 35 | double 4 | -M2 | M1 |
| s24 | 2 | hard-1 | hard | red | 38 | open 4 | -M2 | M1 |
| s26 | 1 | hard-2 | hard | blue | 37 | 4 | -M2 | M1 |
| s26 | 3 | hard-2 | hard | blue | 91 | 4 | -14700 | M1 |
| s28 | 1 | master-2 | master | red | 36 | open 4 | -M2 | M1 |
| s28 | 2 | master-2 | master | blue | 31 | 4 | -18600 | M1 |
| s29 | 1 | master-2 | master | blue | 27 | 4 | -16800 | M1 |
| s29 | 3 | master-2 | master | blue | 53 | 4 | -M2 | M1 |
| s36 | 2 | medium-2 | medium | blue | 29 | 4 | -M2 | M1 |

### medium-1 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 329 | 9.4 | 1.85 | 1.88 | 0.98 |
| mid | 282 | 7.1 | 1.64 | 1.66 | 0.88 |
| late | 373 | 3.2 | 0.67 | 0.69 | 0.47 |

### medium-1 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s15 | 1 | hard-1 | hard | red | 64 | 4 | -28200 | M1 |
| s15 | 3 | hard-1 | hard | red | 140 | 4 | -15900 | M1 |
| s17 | 1 | master-1 | master | red | 86 | 4 | -25700 | M1 |
| s17 | 2 | master-1 | master | blue | 59 | 4 | -15800 | M1 |
| s18 | 2 | master-2 | master | blue | 33 | 4 | -21000 | M1 |
| s18 | 3 | master-2 | master | red | 62 | 4 | -20000 | M1 |
| s39 | 1 | master-2 | master | blue | 39 | 4 | -M2 | M1 |
| s39 | 2 | master-2 | master | red | 44 | 4 | -14000 | M1 |
| s40 | 1 | master-1 | master | blue | 35 | 4 | -19500 | M1 |
| s40 | 2 | master-1 | master | red | 68 | 4 | -13500 | M1 |
| s41 | 1 | hard-2 | hard | blue | 43 | 4 | -M2 | M1 |
| s41 | 2 | hard-2 | hard | red | 66 | 4 | -29900 | M1 |
| s42 | 1 | hard-1 | hard | blue | 39 | 4 | -M2 | M1 |
| s42 | 3 | hard-1 | hard | blue | 41 | 4 | -M2 | M1 |
| s49 | 2 | easy-2 | easy | blue | 55 | 4 | -M2 | M1 |
| s49 | 3 | easy-2 | easy | red | 36 | 4 | -M2 | M1 |
| s55 | 1 | easy-1 | easy | red | 56 | 4 | -M2 | M1 |
| s55 | 2 | easy-1 | easy | blue | 83 | 4 | -14600 | M1 |

### medium-2 phase telemetry

| phase | moves | d mean | t mean | alloc mean | t/alloc |
|---|---|---|---|---|---|
| early | 349 | 9.5 | 1.84 | 1.87 | 0.99 |
| mid | 292 | 7.7 | 1.70 | 1.72 | 0.96 |
| late | 342 | 3.5 | 0.87 | 0.89 | 0.47 |

### medium-2 losses

| series | game | opponent | tier | color | moves | won by | own final s | opp final s |
|---|---|---|---|---|---|---|---|---|
| s14 | 1 | medium-1 | medium | blue | 43 | 4 | -M2 | M1 |
| s14 | 2 | medium-1 | medium | red | 38 | open 4 | -M2 | M1 |
| s19 | 1 | hard-1 | hard | red | 82 | 4 | -22000 | M1 |
| s19 | 2 | hard-1 | hard | blue | 49 | 4 | -M2 | M1 |
| s20 | 1 | hard-2 | hard | red | 64 | 4 | -24900 | M1 |
| s20 | 3 | hard-2 | hard | red | 60 | 4 | -M2 | M1 |
| s21 | 2 | master-1 | master | blue | 39 | 4 | -M2 | M1 |
| s22 | 2 | master-2 | master | blue | 33 | 4 | -M2 | M1 |
| s35 | 1 | master-2 | master | blue | 45 | 4 | -M2 | M1 |
| s35 | 2 | master-2 | master | red | 32 | 4 | -9900 | M1 |
| s36 | 1 | master-1 | master | blue | 41 | 4 | -M2 | M1 |
| s36 | 3 | master-1 | master | blue | 33 | 4 | -11400 | M1 |
| s37 | 1 | hard-2 | hard | blue | 43 | 4 | -M2 | M1 |
| s37 | 3 | hard-2 | hard | blue | 61 | 4 | -M2 | M1 |
| s38 | 1 | hard-1 | hard | blue | 37 | 4 | -M2 | M1 |
| s38 | 3 | hard-1 | hard | blue | 63 | 4 | -26500 | M1 |
| s03 | 2 | easy-1 | easy | red | 94 | 4 | -9600 | M1 |
| s03 | 3 | easy-1 | easy | blue | 91 | 4 | -8900 | M1 |
| s43 | 1 | medium-1 | medium | red | 70 | 4 | -14200 | M1 |
| s43 | 2 | medium-1 | medium | blue | 41 | 4 | -M2 | M1 |
| s54 | 1 | easy-1 | easy | red | 68 | 4 | -11000 | M1 |
| s54 | 2 | easy-1 | easy | blue | 59 | 4 | -M2 | M1 |

### draws by tier pair

easy-medium: 1

## cross-arm synthesis

### same-tier mutuals, first seat's perspective

| pair | 2+1 | 3+2 | 1+0 |
|---|---|---|---|
| easy-1 v easy-2 | s2-0 g4-1-1 | s1-1 g3-2-0 | s1-1 g2-3-0 |
| medium-1 v medium-2 | s1-1 g3-3-0 | s1-1 g3-3-0 | s2-0 g4-0-0 |
| hard-1 v hard-2 | s1-0 g3-1-1 | s1-1 g3-2-1 | s2-0 g4-2-0 |
| master-1 v master-2 | s1-1 g3-2-0 | s0-2 g1-4-0 | s0-2 g1-4-0 |

### adjacent-tier aggregates, upper tier's perspective

| pair | 2+1 | 3+2 | 1+0 |
|---|---|---|---|
| easy v medium | s2-5 g5-11-5 | s2-5 g7-11-4 | s4-4 g8-10-1 |
| medium v hard | s3-4 g8-10-2 | s5-3 g12-7-2 | s1-7 g7-14-0 |
| hard v master | s1-7 g3-14-1 | s4-3 g11-8-1 | s3-5 g6-12-0 |

