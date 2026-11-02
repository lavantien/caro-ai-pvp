# Arena 12-leg re-run

Master pass gates over both seats after the solver audit fixes, one frozen binary bin\arena-freeze.exe 14224896 bytes. Thresholds: hard 0.60, medium 0.75, easy 0.90, wald interval at gateZ 1.96 with block extension, then the point rule.

| leg | verdict | n | wd | lb | ub | cap |
|---|---|---|---|---|---|---|
| square hard 2+1 | FAIL | 60 | 0.500 | 0.373 | 0.627 | true |
| square hard 3+2 | FAIL | 40 | 0.350 | 0.202 | 0.498 | false |
| square medium 2+1 | FAIL | 20 | 0.450 | 0.232 | 0.668 | false |
| square medium 3+2 | FAIL | 60 | 0.617 | 0.494 | 0.740 | false |
| square easy 2+1 | PASS | 60 | 0.917 | 0.847 | 0.987 | |
| square easy 3+2 | FAIL | 60 | 0.850 | 0.760 | 0.940 | true |
| hex hard 2+1 | PASS | 40 | 0.775 | 0.646 | 0.904 | |
| hex hard 3+2 | PASS | 20 | 0.850 | 0.694 | 1.000 | |
| hex medium 2+1 | PASS | 60 | 0.817 | 0.719 | 0.915 | |
| hex medium 3+2 | PASS | 60 | 0.800 | 0.699 | 0.901 | |
| hex easy 2+1 | PASS | 60 | 0.900 | 0.824 | 0.976 | |
| hex easy 3+2 | PASS | 20 | 1.000 | 1.000 | 1.000 | |

7 of 12 legs pass. A still-failing square leg gets engine fixes only inside the capability-removal ladder under the never-degrade law, the reference-model tier redesign stays a post-close product option.
