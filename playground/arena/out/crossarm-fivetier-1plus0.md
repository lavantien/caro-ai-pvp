# crossarm seat decomposition

## arm 1+0

dir logs/tourny/20261011-135728-smoke10, files 90, series 90, tc 1+0, games 220 (217 decisive, 3 drawn), zero-sum holds, bad lines 0, unfinished 0, fold mismatches 0


### standings

| seat | tier | series | games | red | blue |
|---|---|---|---|---|---|
| grandmaster-1 | grandmaster | 13-5 | 29-14-0 | 18-4-0 | 11-10-0 |
| grandmaster-2 | grandmaster | 8-9 | 25-22-1 | 17-5-0 | 8-17-1 |
| master-1 | master | 12-6 | 27-17-0 | 19-4-0 | 8-13-0 |
| master-2 | master | 10-8 | 23-20-1 | 16-5-0 | 7-15-1 |
| hard-1 | hard | 10-7 | 24-19-1 | 15-9-0 | 9-10-1 |
| hard-2 | hard | 11-7 | 26-18-0 | 17-5-0 | 9-13-0 |
| medium-1 | medium | 9-8 | 23-20-1 | 12-7-1 | 11-13-0 |
| medium-2 | medium | 10-8 | 22-21-0 | 13-9-0 | 9-12-0 |
| easy-1 | easy | 2-16 | 9-34-0 | 6-16-0 | 3-18-0 |
| easy-2 | easy | 3-14 | 9-32-2 | 4-16-2 | 5-16-0 |

### per-seat records by opponent tier

| seat | vs easy | vs medium | vs hard | vs master | vs grandmaster |
|---|---|---|---|---|---|
| grandmaster-1 | 8-0-0 | 6-2-0 | 6-4-0 | 5-6-0 | 4-2-0 |
| grandmaster-2 | 7-1-1 | 6-5-0 | 7-4-0 | 3-8-0 | 2-4-0 |
| master-1 | 8-1-0 | 5-5-0 | 5-4-0 | 2-3-0 | 7-4-0 |
| master-2 | 8-2-1 | 2-6-0 | 3-6-0 | 3-2-0 | 7-4-0 |
| hard-1 | 8-0-0 | 5-5-1 | 3-3-0 | 6-4-0 | 2-7-0 |
| hard-2 | 8-0-0 | 5-5-0 | 3-3-0 | 4-4-0 | 6-6-0 |
| medium-1 | 7-3-0 | 2-2-0 | 6-4-1 | 3-6-0 | 5-5-0 |
| medium-2 | 6-5-0 | 2-2-0 | 4-6-0 | 8-1-0 | 2-7-0 |
| easy-1 | 3-3-0 | 4-7-0 | 0-8-0 | 2-8-0 | 0-8-0 |
| easy-2 | 3-3-0 | 4-6-0 | 0-8-0 | 1-8-1 | 1-7-1 |

### series matrix, row winner over column

| seat | grandmaster-1 | grandmaster-2 | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 |  | 2-0 | 1-1 | 1-1 | 2-0 | 0-2 | 1-1 | 2-0 | 2-0 | 2-0 |
| grandmaster-2 | 0-2 |  | 0-2 | 0-2 | 1-1 | 2-0 | 1-1 | 1-1 | 2-0 | 1-0 |
| master-1 | 1-1 | 2-0 |  | 1-1 | 1-1 | 1-1 | 2-0 | 0-2 | 2-0 | 2-0 |
| master-2 | 1-1 | 2-0 | 1-1 |  | 0-2 | 1-1 | 1-1 | 0-2 | 2-0 | 2-0 |
| hard-1 | 0-2 | 1-1 | 1-1 | 2-0 |  | 1-1 | 0-1 | 1-1 | 2-0 | 2-0 |
| hard-2 | 2-0 | 0-2 | 1-1 | 1-1 | 1-1 |  | 1-1 | 1-1 | 2-0 | 2-0 |
| medium-1 | 1-1 | 1-1 | 0-2 | 1-1 | 1-0 | 1-1 |  | 1-1 | 2-0 | 1-1 |
| medium-2 | 0-2 | 1-1 | 2-0 | 2-0 | 1-1 | 1-1 | 1-1 |  | 1-1 | 1-1 |
| easy-1 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 0-2 | 1-1 |  | 1-1 |
| easy-2 | 0-2 | 0-1 | 0-2 | 0-2 | 0-2 | 0-2 | 1-1 | 1-1 | 1-1 |  |

### games matrix, row over column

| seat | grandmaster-1 | grandmaster-2 | master-1 | master-2 | hard-1 | hard-2 | medium-1 | medium-2 | easy-1 | easy-2 |
|---|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 |  | 4-2-0 | 3-3-0 | 2-3-0 | 4-0-0 | 2-4-0 | 2-2-0 | 4-0-0 | 4-0-0 | 4-0-0 |
| grandmaster-2 | 2-4-0 |  | 1-4-0 | 2-4-0 | 3-2-0 | 4-2-0 | 3-3-0 | 3-2-0 | 4-0-0 | 3-1-1 |
| master-1 | 3-3-0 | 4-1-0 |  | 2-3-0 | 3-2-0 | 2-2-0 | 4-1-0 | 1-4-0 | 4-0-0 | 4-1-0 |
| master-2 | 3-2-0 | 4-2-0 | 3-2-0 |  | 1-4-0 | 2-2-0 | 2-2-0 | 0-4-0 | 4-2-0 | 4-0-1 |
| hard-1 | 0-4-0 | 2-3-0 | 2-3-0 | 4-1-0 |  | 3-3-0 | 2-3-1 | 3-2-0 | 4-0-0 | 4-0-0 |
| hard-2 | 4-2-0 | 2-4-0 | 2-2-0 | 2-2-0 | 3-3-0 |  | 2-3-0 | 3-2-0 | 4-0-0 | 4-0-0 |
| medium-1 | 2-2-0 | 3-3-0 | 1-4-0 | 2-2-0 | 3-2-1 | 3-2-0 |  | 2-2-0 | 4-1-0 | 3-2-0 |
| medium-2 | 0-4-0 | 2-3-0 | 4-1-0 | 4-0-0 | 2-3-0 | 2-3-0 | 2-2-0 |  | 3-3-0 | 3-2-0 |
| easy-1 | 0-4-0 | 0-4-0 | 0-4-0 | 2-4-0 | 0-4-0 | 0-4-0 | 1-4-0 | 3-3-0 |  | 3-3-0 |
| easy-2 | 0-4-0 | 1-3-1 | 1-4-0 | 0-4-1 | 0-4-0 | 0-4-0 | 2-3-0 | 2-3-0 | 3-3-0 |  |

### per-seat telemetry, untagged searches

| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |
|---|---|---|---|---|---|---|---|---|---|
| grandmaster-1 | 988 | 244 | 7.3 | 9.0 | 1.50 | 1.87 | 0.68 | 4.98m | 8.94m |
| grandmaster-2 | 1152 | 256 | 7.2 | 8.0 | 1.49 | 1.81 | 0.68 | 5.06m | 9.18m |
| master-1 | 1013 | 206 | 7.4 | 9.0 | 1.54 | 1.93 | 0.69 | 4.21m | 7.53m |
| master-2 | 1050 | 195 | 7.0 | 8.0 | 1.47 | 1.80 | 0.70 | 4.36m | 7.34m |
| hard-1 | 1056 | 208 | 6.9 | 8.0 | 1.43 | 1.79 | 0.65 | 2.85m | 5.07m |
| hard-2 | 940 | 226 | 7.5 | 9.0 | 1.60 | 1.98 | 0.71 | 3.00m | 5.55m |
| medium-1 | 1147 | 223 | 6.6 | 8.0 | 1.77 | 1.79 | 0.78 | 1.47m | 3.28m |
| medium-2 | 1127 | 199 | 6.7 | 8.0 | 1.85 | 1.87 | 0.83 | 1.60m | 3.47m |
| easy-1 | 864 | 0 | 6.7 | 7.0 | 1.92 | 1.97 | 0.96 | 1.14m | 2.25m |
| easy-2 | 1133 | 0 | 5.8 | 6.0 | 1.57 | 1.61 | 0.86 | 1.11m | 1.97m |

### ponder adoption telemetry

| seat | ponder | t mean | t max | vcf | vct |
|---|---|---|---|---|---|
| grandmaster-1 | 169 | 0.01 | 0.07 | 63 | 12 |
| grandmaster-2 | 176 | 0.01 | 0.08 | 68 | 12 |
| master-1 | 146 | 0.01 | 0.08 | 56 | 4 |
| master-2 | 131 | 0.01 | 0.07 | 53 | 11 |
| hard-1 | 143 | 0.01 | 0.08 | 55 | 10 |
| hard-2 | 159 | 0.01 | 0.08 | 54 | 13 |
| medium-1 | 176 | 0.00 | 0.01 | 47 | 0 |
| medium-2 | 156 | 0.00 | 0.01 | 43 | 0 |
| easy-1 | 0 | 0.00 | 0.00 | 0 | 0 |
| easy-2 | 0 | 0.00 | 0.00 | 0 | 0 |

### draws by tier pair

easy-master: 1
easy-grandmaster: 1
hard-medium: 1

## cross-arm synthesis

### same-tier mutuals, first seat's perspective

| pair | 1+0 |
|---|---|
| easy-1 v easy-2 | s1-1 g3-3-0 |
| medium-1 v medium-2 | s1-1 g2-2-0 |
| hard-1 v hard-2 | s1-1 g3-3-0 |
| master-1 v master-2 | s1-1 g2-3-0 |
| grandmaster-1 v grandmaster-2 | s2-0 g4-2-0 |

### adjacent-tier aggregates, upper tier's perspective

| pair | 1+0 |
|---|---|
| easy v medium | s3-5 g8-13-0 |
| medium v hard | s4-3 g10-10-1 |
| hard v master | s5-3 g10-8-0 |
| master v grandmaster | s6-2 g14-8-0 |

