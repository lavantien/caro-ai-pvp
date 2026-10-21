# ponder hit rate: logs\tourny

source: logs\tourny
series log files: 1111

method: within each game, compare pv[1] of turn n (the predicted reply) against the played cell of turn n+1. Opportunities are turns of a ponder-capable tier (config PonderThreads > 0) with a pv of at least 2 cells, in a series where that tier shows at least one adopted [PONDER] answer. Adopted-given-hit requires the ponderer's next move in the same game to carry [PONDER]. Caveats: the pv truncates near the search horizon so not-in-pv overstates true distance, a wrong prediction can still warm the tree, and turns where the ponder lane was idle by admission are indistinguishable from gate refusals in the logs.

## hit rate and conditional adoption

| tier | tc | predictions | hits | hit rate | adopted after hit | adopted / hits |
|---|---|---|---|---|---|---|
| grandmaster | 1+0 | 1170 | 860 | 73.5% | 345 | 40.1% |
| grandmaster | 2+1 | 11486 | 7753 | 67.5% | 3194 | 41.2% |
| grandmaster | 3+2 | 5388 | 3680 | 68.3% | 1508 | 41.0% |
| hard | 1+0 | 5325 | 3935 | 73.9% | 302 | 7.7% |
| hard | 2+1 | 15878 | 11052 | 69.6% | 2897 | 26.2% |
| hard | 3+2 | 13002 | 8688 | 66.8% | 1786 | 20.6% |
| master | 1+0 | 2769 | 2005 | 72.4% | 704 | 35.1% |
| master | 2+1 | 8649 | 5900 | 68.2% | 2185 | 37.0% |
| master | 3+2 | 3196 | 2148 | 67.2% | 760 | 35.4% |
| medium | 1+0 | 5723 | 4171 | 72.9% | 332 | 8.0% |
| medium | 2+1 | 14101 | 9813 | 69.6% | 1949 | 19.9% |
| medium | 3+2 | 9273 | 6314 | 68.1% | 491 | 7.8% |

## miss distance, share of misses by where the actual reply sat in the predicting pv

| tier | tc | misses | pv3 | pv4 | pv5+ | not-in-pv |
|---|---|---|---|---|---|---|
| grandmaster | 1+0 | 310 | 15.8% | 11.6% | 11.0% | 61.6% |
| grandmaster | 2+1 | 3733 | 18.8% | 11.0% | 14.6% | 55.6% |
| grandmaster | 3+2 | 1708 | 18.3% | 9.2% | 17.2% | 55.4% |
| hard | 1+0 | 1390 | 17.5% | 12.5% | 12.5% | 57.5% |
| hard | 2+1 | 4826 | 17.5% | 12.0% | 15.5% | 55.0% |
| hard | 3+2 | 4314 | 17.9% | 11.3% | 20.1% | 50.7% |
| master | 1+0 | 764 | 16.5% | 10.3% | 11.3% | 61.9% |
| master | 2+1 | 2749 | 19.4% | 10.8% | 13.7% | 56.2% |
| master | 3+2 | 1048 | 15.2% | 10.9% | 18.7% | 55.2% |
| medium | 1+0 | 1552 | 16.5% | 11.2% | 16.0% | 56.3% |
| medium | 2+1 | 4288 | 18.1% | 13.1% | 18.2% | 50.7% |
| medium | 3+2 | 2959 | 16.5% | 12.8% | 24.3% | 46.3% |
