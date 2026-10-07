# v0.23 master gate table and verdict

Decision run: the 2026-10-07 22:01 detached chain (run 2, the idle-machine rerun; run 1 and its contention caveat are recorded in v023-master-preregistration.md). 4 pairs, 20 games each, red alternating so each seat holds red 10 times. Raw pair logs under logs/archive/arena-v023-p*.log (local per the logs law).

| pair | tc | seats | W-L-D | red wins | blue wins |
| --- | --- | --- | --- | --- | --- |
| 1 | 2+1 | master | 12-7-1 | 9 | 3 |
| 1 | 2+1 | hard | 7-12-1 | 6 | 1 |
| 2 | 3+2 | master | 8-11-1 | 6 | 2 |
| 2 | 3+2 | hard | 11-8-1 | 7 | 4 |
| 3 | 3+2 | master (equal floor, 40 games) | 19-19-2 | 19 | 0 |
| 4 | 3+2 | master | 8-11-1 | 6 | 2 |
| 4 | 3+2 | master-novct | 11-8-1 | 8 | 3 |

## rule applications

Rule 1 (master beats hard outside the floor band at both 2+1 and 3+2: at least 13 of 20 and at least 3 blue-seat wins at each): miss at both controls. 2+1: 12 of 20 with 3 blue-seat wins, one game under the bar. 3+2: 8 of 20 with 2 blue-seat wins, and hard took the pair outright, including 4 blue-seat wins of its own against master's 8 threads.

Rule 2 (floor calibration): the pair 3 identical-engine floor at 3+2 shows 0 blue-seat wins over 40 games (19-19-2, both draws and every win from the red seat), matching the 1+0 and 10+5 floors. The about-zero prior stands, so rule 1's blue-win clause reads unadjusted, and hard's 4 blue-seat wins at 3+2 are the strength signature running the wrong way.

Rule 3 (capability monotonicity at the top): violated. master-novct beat master 11-8 at 3+2 with 3 blue-seat wins, outside the 0-blue floor. A VCT-stripped master outranking full master contradicts the v0.20 matrix (solver passes measured neutral to positive) and blocks the tag; routed to investigation with rule 4.

Rule 4 (a rule 1 miss blocks the tier and records the re-derivation trigger): applied. The tier is blocked as strength-unverified; the re-derivation round is the standing v0.29 reserve trigger.

## verdict

BLOCKED. Master does not clear hard at either control under the preregistered bars, and the top-of-ladder capability order inverts with VCT stripped. The v0.23 tier code (constant, migration v9, admission ledger) stays in tree untagged; the strength re-derivation runs before any ship decision, and the v0.28 pass thresholds (W+D 60%+ vs hard) bind after it and after the tier re-scale.

## caveats

Pair 2 game 4 (a draw at the 256-move cap) overlapped a brief local contention window from an over-matched test regex; the miss margins (12 vs 13 at 2+1, 8 vs 13 at 3+2) are far wider than one game, so nothing flips on it. This chain measured master against the shipped hard (128 MiB table); the demand-spec re-scale to 1 GiB hard raises the bar further, recorded in the plan.
