# Arena 6-leg regate

Post-fix master pass gates over both seats after the square clean-window eval correction and the blue-side square book entries, one frozen binary bin\arena-freeze.exe 14227968 bytes. Thresholds: hard 0.60, medium 0.75, easy 0.90, wald interval at gateZ 1.96 with block extension, then the point rule. Hex medium 2+1 is the hex calibration guard, its clean-window weight is 0 by construction.

| leg | re-run | verdict | n | wd | lb | ub | cap |
|---|---|---|---|---|---|---|---|
| square hard 2+1 | FAIL | FAIL | 60 | 0.517 | 0.390 | 0.643 | true |
| square hard 3+2 | FAIL | FAIL | 60 | 0.483 | 0.357 | 0.610 | true |
| square medium 2+1 | FAIL | FAIL | 60 | 0.667 | 0.547 | 0.786 | true |
| square medium 3+2 | FAIL | FAIL | 60 | 0.683 | 0.566 | 0.801 | true |
| square easy 3+2 | FAIL | PASS | 40 | 0.975 | 0.927 | 1.000 | |
| hex medium 2+1 | PASS | PASS | 20 | 0.950 | 0.854 | 1.000 | |

2 of 6 legs pass against 1 of 6 on the same legs in the 12-leg re-run.
