# crossarm gate

## gate 2+1

integrity: 13 dirs, bad lines 0, unfinished 0, fold mismatches 0

### rule 0 telemetry

| seat | tier | ponder | t mean | t max | t/alloc | verdict |
|---|---|---|---|---|---|---|
| easy-1 | easy | 0 | 0.00 | 0.00 | 0.97 | pass |
| easy-2 | easy | 0 | 0.00 | 0.00 | 0.97 | pass |
| grandmaster-1 | grandmaster | 1395 | 0.01 | 0.21 | 0.76 | pass |
| grandmaster-2 | grandmaster | 1346 | 0.01 | 0.22 | 0.77 | pass |
| hard-1 | hard | 1199 | 0.01 | 0.12 | 0.78 | pass |
| hard-2 | hard | 1265 | 0.01 | 0.12 | 0.77 | pass |
| master-1 | master | 727 | 0.01 | 0.12 | 0.78 | pass |
| master-2 | master | 733 | 0.01 | 0.12 | 0.76 | pass |
| medium-1 | medium | 735 | 0.00 | 0.02 | 0.97 | pass |
| medium-2 | medium | 824 | 0.00 | 0.05 | 0.97 | pass |

### floors

| tier | dirs | games | decisive | frac | 95% ci | covers 0.5 |
|---|---|---|---|---|---|---|
| hard | 5 | 101 | 93 | 0.473 | 0.371 to 0.575 | true |
| grandmaster | 5 | 105 | 93 | 0.505 | 0.404 to 0.607 | true |

### family reads

| family | floor | N | upper frac | lower frac | z rule A | z rule B | rule A | rule B |
|---|---|---|---|---|---|---|---|---|
| medium v easy | hard | 55 | 0.855 | 0.145 | -3.85 | 4.48 | quiet | certified |
| hard v medium | hard | 54 | 0.519 | 0.481 | 0.10 | 0.53 | quiet | unproven |
| master v hard | hard | 60 | 0.583 | 0.417 | -0.68 | 1.33 | quiet | unproven |
| grandmaster v master | grandmaster | 57 | 0.439 | 0.561 | 0.67 | -0.79 | quiet | unproven |

