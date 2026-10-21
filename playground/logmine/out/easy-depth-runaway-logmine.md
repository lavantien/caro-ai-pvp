# easy tier depth runaway: logs\tourny

source: logs\tourny
series log files: 1111
threshold: depth >= 30 on easy moves
engine caps: SearchMaxPly 64, EvalMateMax 1073741824, EvalMateScoreStep 16

## population

| moves | at depth cap 64 | mate scores | near-mate | zero scores | normal | depth q1/med/q3 | move no q1/med/q3 | nodes med | pv len med | ebf med |
|---|---|---|---|---|---|---|---|---|---|---|
| 626 | 590 | 41 | 0 | 585 | 0 | 64/64/64 | 247/250/253 | 444/6860/205790 | 3/6/9 | 2.0/2.0/2.0 |

by time control: 1+0 15, 2+1 298, 3+2 313

## example moves

| file | game | move | tc | depth | nodes | score | pv len | ebf |
|---|---|---|---|---|---|---|---|---|
| run1_s10_easy-2-vs-hard-1.txt | 1 | 243 | 2+1 | 64 | 2380000 | +0 | 14 | 2.0 |
| run1_s10_easy-2-vs-hard-1.txt | 1 | 245 | 2+1 | 64 | 643920 | +0 | 12 | 2.0 |
| run1_s10_easy-2-vs-hard-1.txt | 1 | 247 | 2+1 | 64 | 119630 | +0 | 10 | 2.0 |
| run1_s10_easy-2-vs-hard-1.txt | 1 | 249 | 2+1 | 64 | 1490 | +0 | 8 | 2.0 |
| run1_s10_easy-2-vs-hard-1.txt | 1 | 251 | 2+1 | 64 | 6430 | +0 | 6 | 2.0 |

verdict: cap-bound ladder on collapsed branching with draw scores, not a mate ladder. The engine's depth cap is SearchMaxPly 64 and no move sits above it: the iterative ladder runs to the hard cap on boards whose branching collapsed (median ebf 2.0). Mate-band scores step 16 per ply from EvalMateMax, so the mate-scored subset is the mate-distance ladder; the zero-scored subset is a quiet forced tail the ladder walks to the cap without a mate. Both are endgame shapes, not move generator defects.
