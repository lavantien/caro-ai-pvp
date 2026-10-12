# crossarm seat decomposition

## arm 2+1

dir logs/tourny/20261011-192828-full, files 90, series 90, tc 2+1, games 222 (208 decisive, 14 drawn), zero-sum holds, bad lines 0, unfinished 0, fold mismatches 0


### standings

| seat | tier | series | games | red | blue |
|---|---|---|---|---|---|
| grandmaster-1 | grandmaster | 14-2 | 32-11-2 | 19-4-1 | 13-7-1 |
| grandmaster-2 | grandmaster | 15-3 | 30-11-6 | 17-3-2 | 13-8-4 |
| master-1 | master | 11-6 | 26-18-3 | 15-3-3 | 11-15-0 |
| master-2 | master | 11-6 | 25-17-2 | 16-6-0 | 9-11-2 |
| hard-1 | hard | 9-8 | 25-18-3 | 17-4-2 | 8-14-1 |
| hard-2 | hard | 10-8 | 25-19-2 | 17-5-1 | 8-14-1 |
| medium-1 | medium | 8-9 | 21-21-4 | 12-10-2 | 9-11-2 |
| medium-2 | medium | 7-11 | 17-24-1 | 12-10-0 | 5-14-1 |
| easy-1 | easy | 1-17 | 2-35-2 | 0-19-1 | 2-16-1 |
| easy-2 | easy | 1-17 | 5-34-3 | 2-17-2 | 3-17-1 |

### per-seat records by opponent tier

| seat | vs easy | vs medium | vs hard | vs master | vs grandmaster |
|---|---|---|---|---|---|
| grandmaster-1 | 8-0-0 | 7-2-1 | 7-3-0 | 6-5-1 | 4-1-0 |
| grandmaster-2 | 8-1-1 | 8-2-0 | 7-1-4 | 6-3-1 | 1-4-0 |
| master-1 | 8-1-0 | 5-5-1 | 6-5-0 | 3-2-0 | 4-5-2 |
| master-2 | 8-0-0 | 8-1-1 | 3-6-1 | 2-3-0 | 4-7-0 |
| hard-1 | 8-0-0 | 6-4-0 | 3-3-0 | 5-5-1 | 3-6-2 |
| hard-2 | 8-0-0 | 7-4-0 | 3-3-0 | 6-4-0 | 1-8-2 |
| medium-1 | 8-0-1 | 2-2-0 | 5-6-0 | 3-6-2 | 3-7-1 |
| medium-2 | 8-0-1 | 2-2-0 | 3-7-0 | 3-7-0 | 1-8-0 |
| easy-1 | 2-3-1 | 0-8-0 | 0-8-0 | 0-8-0 | 0-8-1 |
| easy-2 | 3-2-1 | 0-8-2 | 0-8-0 | 1-8-0 | 1-8-0 |

### series matrix, row winner over column

| seat | grandmaster-1 | grandmaster-2 | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 |  | 2-0 | 1-0 | 1-1 | 1-1 | 2-0 | 1-0 | 2-0 | 2-0 | 2-0 |
| grandmaster-2 | 0-2 |  | 1-1 | 2-0 | 2-0 | 2-0 | 2-0 | 2-0 | 2-0 | 2-0 |
| master-1 | 0-1 | 1-1 |  | 1-1 | 1-1 | 2-0 | 1-1 | 1-1 | 2-0 | 2-0 |
| master-2 | 1-1 | 0-2 | 1-1 |  | 1-0 | 0-2 | 2-0 | 2-0 | 2-0 | 2-0 |
| hard-1 | 1-1 | 0-2 | 1-1 | 0-1 |  | 1-1 | 1-1 | 1-1 | 2-0 | 2-0 |
| hard-2 | 0-2 | 0-2 | 0-2 | 2-0 | 1-1 |  | 1-1 | 2-0 | 2-0 | 2-0 |
| medium-1 | 0-1 | 0-2 | 1-1 | 0-2 | 1-1 | 1-1 |  | 1-1 | 2-0 | 2-0 |
| medium-2 | 0-2 | 0-2 | 1-1 | 0-2 | 1-1 | 0-2 | 1-1 |  | 2-0 | 2-0 |
| easy-1 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 |  | 1-1 |
| easy-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 1-1 |  |

### games matrix, row over column

| seat | grandmaster-1 | grandmaster-2 | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 |  | 4-1-0 | 3-2-1 | 3-3-0 | 3-2-0 | 4-1-0 | 3-2-1 | 4-0-0 | 4-0-0 | 4-0-0 |
| grandmaster-2 | 1-4-0 |  | 2-2-1 | 4-1-0 | 3-1-2 | 4-0-2 | 4-1-0 | 4-1-0 | 4-0-1 | 4-1-0 |
| master-1 | 2-3-1 | 2-2-1 |  | 3-2-0 | 2-3-0 | 4-2-0 | 2-3-1 | 3-2-0 | 4-0-0 | 4-1-0 |
| master-2 | 3-3-0 | 1-4-0 | 2-3-0 |  | 3-2-1 | 0-4-0 | 4-0-1 | 4-1-0 | 4-0-0 | 4-0-0 |
| hard-1 | 2-3-0 | 1-3-2 | 3-2-0 | 2-3-1 |  | 3-3-0 | 3-2-0 | 3-2-0 | 4-0-0 | 4-0-0 |
| hard-2 | 1-4-0 | 0-4-2 | 2-4-0 | 4-0-0 | 3-3-0 |  | 3-3-0 | 4-1-0 | 4-0-0 | 4-0-0 |
| medium-1 | 2-3-1 | 1-4-0 | 3-2-1 | 0-4-1 | 2-3-0 | 3-3-0 |  | 2-2-0 | 4-0-0 | 4-0-1 |
| medium-2 | 0-4-0 | 1-4-0 | 2-3-0 | 1-4-0 | 2-3-0 | 1-4-0 | 2-2-0 |  | 4-0-0 | 4-0-1 |
| easy-1 | 0-4-0 | 0-4-1 | 0-4-0 | 0-4-0 | 0-4-0 | 0-4-0 | 0-4-0 | 0-4-0 |  | 2-3-1 |
| easy-2 | 0-4-0 | 1-4-0 | 1-4-0 | 0-4-0 | 0-4-0 | 0-4-0 | 0-4-1 | 0-4-1 | 3-2-1 |  |

### per-seat telemetry, untagged searches

| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |
|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 | 1204 | 316 | 8.1 | 8.0 | 3.46 | 4.37 | 0.77 | 6.80m | 21.97m |
| grandmaster-2 | 1451 | 391 | 8.9 | 8.0 | 2.96 | 3.72 | 0.78 | 7.25m | 19.53m |
| master-1 | 1180 | 315 | 8.5 | 9.0 | 3.35 | 4.26 | 0.78 | 5.71m | 17.73m |
| master-2 | 1127 | 264 | 7.9 | 8.0 | 3.48 | 4.32 | 0.79 | 5.65m | 18.70m |
| hard-1 | 1246 | 322 | 8.0 | 8.0 | 3.25 | 4.12 | 0.78 | 3.69m | 11.40m |
| hard-2 | 1101 | 300 | 8.7 | 8.0 | 3.72 | 4.56 | 0.80 | 3.92m | 13.90m |
| medium-1 | 1379 | 313 | 8.2 | 8.0 | 4.04 | 4.17 | 0.96 | 2.06m | 7.43m |
| medium-2 | 1070 | 203 | 7.7 | 8.0 | 4.28 | 4.37 | 0.97 | 2.06m | 8.25m |
| easy-1 | 1118 | 0 | 7.0 | 7.0 | 3.78 | 3.88 | 0.97 | 1.13m | 4.04m |
| easy-2 | 1249 | 0 | 6.7 | 6.0 | 3.65 | 3.75 | 0.97 | 1.36m | 4.56m |

### ponder adoption telemetry

| seat | ponder | t mean | t max | vcf | vct |
|---|---|---|---|---|---|
| grandmaster-1 | 195 | 0.01 | 0.14 | 98 | 23 |
| grandmaster-2 | 258 | 0.02 | 0.35 | 118 | 15 |
| master-1 | 208 | 0.01 | 0.09 | 84 | 23 |
| master-2 | 176 | 0.01 | 0.18 | 78 | 10 |
| hard-1 | 222 | 0.02 | 0.36 | 82 | 18 |
| hard-2 | 211 | 0.01 | 0.07 | 78 | 11 |
| medium-1 | 245 | 0.00 | 0.07 | 68 | 0 |
| medium-2 | 145 | 0.00 | 0.32 | 58 | 0 |
| easy-1 | 0 | 0.00 | 0.00 | 0 | 0 |
| easy-2 | 0 | 0.00 | 0.00 | 0 | 0 |

### draws by tier pair

easy-easy: 1
easy-medium: 2
easy-grandmaster: 1
hard-master: 1
master-medium: 2
grandmaster-medium: 1
grandmaster-hard: 4
grandmaster-master: 2

## cross-arm synthesis

### same-tier mutuals, first seat's perspective

| pair | 2+1 |
|---|---|
| easy-1 v easy-2 | s1-1 g2-3-1 |
| medium-1 v medium-2 | s1-1 g2-2-0 |
| hard-1 v hard-2 | s1-1 g3-3-0 |
| master-1 v master-2 | s1-1 g3-2-0 |
| grandmaster-1 v grandmaster-2 | s2-0 g4-1-0 |

### adjacent-tier aggregates, upper tier's perspective

| pair | 2+1 |
|---|---|
| easy v medium | s0-8 g0-16-2 |
| medium v hard | s3-5 g8-13-0 |
| hard v master | s3-4 g11-9-1 |
| master v grandmaster | s2-5 g8-12-2 |

