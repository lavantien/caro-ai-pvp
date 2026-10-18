# crossarm gate

## gate 3+2

integrity: 11 dirs, bad lines 0, unfinished 0, fold mismatches 0

### rule 0 telemetry

| seat | tier | ponder | t mean | t max | t/alloc | verdict |
|---|---|---|---|---|---|---|
| easy-1 | easy | 0 | 0.00 | 0.00 | 0.96 | pass |
| easy-2 | easy | 0 | 0.00 | 0.00 | 0.96 | pass |
| grandmaster-1 | grandmaster | 771 | 0.01 | 0.16 | 0.77 | pass |
| grandmaster-2 | grandmaster | 737 | 0.01 | 0.13 | 0.78 | pass |
| hard-1 | hard | 852 | 0.01 | 0.09 | 0.77 | pass |
| hard-2 | hard | 934 | 0.01 | 0.10 | 0.76 | pass |
| master-1 | master | 175 | 0.01 | 0.14 | 0.79 | pass |
| master-2 | master | 160 | 0.01 | 0.08 | 0.80 | pass |
| medium-1 | medium | 271 | 0.00 | 0.06 | 0.96 | pass |
| medium-2 | medium | 220 | 0.00 | 0.03 | 0.97 | pass |

### floors

| tier | dirs | games | decisive | frac | 95% ci | covers 0.5 |
|---|---|---|---|---|---|---|
| hard | 5 | 104 | 90 | 0.556 | 0.452 to 0.659 | true |
| grandmaster | 5 | 97 | 87 | 0.552 | 0.447 to 0.657 | true |

### family reads

| family | floor | N | upper frac | lower frac | z rule A | z rule B | rule A | rule B |
|---|---|---|---|---|---|---|---|---|
| medium v easy | hard | 16 | 0.812 | 0.188 | -2.71 | 1.89 | quiet | unproven |
| hard v medium | hard | 19 | 0.526 | 0.474 | -0.65 | -0.23 | quiet | unproven |
| master v hard | hard | 20 | 0.550 | 0.450 | -0.85 | -0.04 | quiet | unproven |
| grandmaster v master | grandmaster | 20 | 0.400 | 0.600 | 0.39 | -1.22 | quiet | unproven |

