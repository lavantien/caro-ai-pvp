# ponder adoption: logs\tourny

source: logs\tourny
series log files: 1111

definition: opportunities are all moves by a ponder-capable tier (config PonderThreads > 0) in a series where that tier shows at least one adopted [PONDER] answer. The logs carry no lane start or stop record, so a tier with ponder disabled all series would not appear here, and the count includes the seat's first move of a game where no ponder ran, bounded by the game count.

## adoption rate

| tier | tc | opportunities | adopted | rate | adopted t q1/med/q3 s |
|---|---|---|---|---|---|
| grandmaster | 1+0 | 2640 | 345 | 13.1% | 0.00/0.00/0.01 |
| grandmaster | 2+1 | 22245 | 3194 | 14.4% | 0.00/0.00/0.02 |
| grandmaster | 3+2 | 10499 | 1508 | 14.4% | 0.00/0.00/0.02 |
| hard | 1+0 | 11346 | 302 | 2.7% | 0.00/0.00/0.01 |
| hard | 2+1 | 27804 | 2897 | 10.4% | 0.00/0.00/0.02 |
| hard | 3+2 | 22129 | 1786 | 8.1% | 0.00/0.00/0.02 |
| master | 1+0 | 5644 | 704 | 12.5% | 0.00/0.00/0.01 |
| master | 2+1 | 16132 | 2185 | 13.5% | 0.00/0.00/0.01 |
| master | 3+2 | 5880 | 760 | 12.9% | 0.00/0.00/0.00 |
| medium | 1+0 | 10501 | 332 | 3.2% | 0.00/0.00/0.00 |
| medium | 2+1 | 23019 | 1949 | 8.5% | 0.00/0.00/0.00 |
| medium | 3+2 | 14766 | 491 | 3.3% | 0.00/0.00/0.00 |

## adopted vs normal searches

| tier | tc | depth adopted | depth normal | nodes adopted | nodes normal | adopted mate share |
|---|---|---|---|---|---|---|
| grandmaster | 1+0 | 8/9/10 | 4/8/10 | 10670000/12170000/14300000 | 109565/9090000/12680000 | 11.9% |
| grandmaster | 2+1 | 7/9/10 | 5/8/10 | 11000000/25150000/32637500 | 5820000/15440000/28920000 | 8.7% |
| grandmaster | 3+2 | 7/9/10 | 5/8/10 | 21985000/39360000/52445000 | 13205000/27490000/47665000 | 9.6% |
| hard | 1+0 | 7/9/9 | 1/8/9 | 3135000/3640000/4300000 | 48/5010000/7942500 | 9.6% |
| hard | 2+1 | 7/9/10 | 5/8/10 | 4270000/8610000/11270000 | 3925000/10470000/18040000 | 9.7% |
| hard | 3+2 | 7/9/10 | 5/8/10 | 7320000/13660000/18230000 | 7290000/15990000/28155000 | 7.3% |
| master | 1+0 | 8/9/10 | 5/8/10 | 6280000/8610000/13295000 | 3490000/8570000/12282500 | 11.6% |
| master | 2+1 | 7/9/10 | 5/8/10 | 6750000/15190000/21950000 | 5260000/14500000/25750000 | 8.9% |
| master | 3+2 | 7/9/10 | 6/8/10 | 18775000/30750000/42197500 | 13357500/27985000/46325000 | 11.7% |
| medium | 1+0 | 7/8/9 | 3/8/9 | 1610000/1870000/2250000 | 61440/3920000/5310000 | 8.4% |
| medium | 2+1 | 6/8/9 | 5/8/9 | 1940000/4130000/5740000 | 2750000/7590000/11500000 | 8.0% |
| medium | 3+2 | 7/9/10 | 5/8/10 | 3695000/7080000/9925000 | 5650000/11590000/18420000 | 9.6% |

## adoption law clauses against PonderAdoptFraction 0.5, PonderStableIters 3, PonderScoreDropMargin 300

clause 1 (depth adequacy, form A: adopted depth at or above the normal-search median depth for the same tier, tc, and grant bucket) is the only clause the logs can decide. Form B (ponder elapsed at or above half the grant) needs ponder elapsed, which an adopted line does not carry: t is turn wall.

| tier | tc | adopted evaluable | passes form A |
|---|---|---|---|
| grandmaster | 1+0 | 345 | 238 |
| grandmaster | 2+1 | 3194 | 2378 |
| grandmaster | 3+2 | 1508 | 1012 |
| hard | 1+0 | 302 | 167 |
| hard | 2+1 | 2897 | 1948 |
| hard | 3+2 | 1786 | 1333 |
| master | 1+0 | 704 | 463 |
| master | 2+1 | 2185 | 1547 |
| master | 3+2 | 760 | 457 |
| medium | 1+0 | 332 | 150 |
| medium | 2+1 | 1949 | 1398 |
| medium | 3+2 | 491 | 340 |

clause 2 (best move stable over the last 3 completed iterations), clause 3 (score drop within 300 milliunits across recent iterations), and clause 4 (no best-move change in the final iteration) need the root-iteration history ring that the v0.25 engine layer introduces. Not decidable from logs.
