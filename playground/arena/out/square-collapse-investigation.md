# Square passgate collapse investigation

Final over all 12 gate legs. The fill batch closed with 6 failed arms recorded, the last leg square easy tc 2 landed at wd 0.850 n 60 and switched the dataset from provisional to settled. All folds run from playground/gateforensic/out through the committed query set, ledger clean at 0 unaccounted lines across 13 sources, 480 games, 70,638 moves, every failed verdict W+D and interval recomputes to the arena's own verdict line exactly, win-sign consistency 355 of 355 decided games, all 20 integrity checks pass. Full tables live in gateforensic-report.md, queries in playground/gateforensic/sql, datasets in moves.parquet and events.parquet.

| leg | verdict | n | wd | lb | ub | cap |
|---|---|---|---|---|---|---|
| hex hard 2+1 | PASS | 20 | 0.850 | 0.694 | 1.0 |  |
| hex hard 3+2 | PASS | 20 | 0.950 | 0.854 | 1.0 |  |
| hex medium 2+1 | PASS | 20 | 0.800 | 0.625 | 0.975 |  |
| hex medium 3+2 | PASS | 20 | 0.850 | 0.694 | 1.0 |  |
| hex easy 2+1 | FAIL | 60 | 0.883 | 0.802 | 0.965 | true |
| hex easy 3+2 | PASS | 20 | 1.000 | 1.0 | 1.0 |  |
| square hard 2+1 | FAIL | 60 | 0.483 | 0.357 | 0.610 | true |
| square hard 3+2 | FAIL | 60 | 0.483 | 0.357 | 0.610 | true |
| square medium 2+1 | FAIL | 40 | 0.525 | 0.370 | 0.680 | false |
| square medium 3+2 | FAIL | 20 | 0.350 | 0.141 | 0.559 | false |
| square easy 2+1 | PASS | 60 | 0.933 | 0.870 | 0.996 |  |
| square easy 3+2 | FAIL | 60 | 0.850 | 0.760 | 0.940 | true |

Failed rows carry the arena verdict numbers. Pass rows show the recomputed W+D from q1_arm_wd over the settled games, the arena DONE line carries no interval. Thresholds: hard 0.60, medium 0.75, easy 0.90. Two geometries, one ruleset, one binary: hex clears 5 of 6 rungs with one straddle, square fails 5 of 6 with one pass.

## 1. Red-first or seat-symmetric

Square hard 2+1: master red 0.667 (18W-2D-10L, CI 0.498-0.835), master blue 0.300 (7-2-21, CI 0.136-0.464), pooled 0.483. Square medium 2+1: red 0.750 (14-1-5), blue 0.300 (4-2-14), pooled 0.525. Square medium 3+2: red 0.600, blue 0.100, pooled 0.350. So at 2+1 the pooled failure is mostly the blue seat, master red alone sits at or above threshold versus hard. But square hard 3+2 collapses in both seats, red 0.500 and blue 0.467, so the anomaly is not only first-move advantage. Square easy fails seat-symmetrically at 3+2, red 0.867 (21-5-4) and blue 0.833 (24-1-5), and passes 2+1 with both seats at 0.900 or better. Hex shows no blue collapse anywhere, blue W+D 0.70 to 1.00.

## 2. Compute parity

On untagged search moves master out-searches every square opponent. Square hard 2+1: master mean depth 7.74 on 18.55m nodes at 6.33m nps versus hard 7.76 on 12.94m at 4.47m, 43 percent more nodes at equal depth and 42 percent higher nps. Square hard 3+2: master 8.17 on 32.02m versus hard 8.39 on 22.03m, hard reaches 0.22 ply deeper but on 31 percent fewer nodes. Square medium 2+1: master 8.61 on 18.73m versus medium 8.33 on 8.51m, better depth on 2.2 times the nodes. Square easy 3+2: master 9.7 on 33.0m at 6.33m nps versus easy 7.38 on 6.66m. Hex parity runs the same direction, milder, master 6.15 on 9.53m versus hard 6.26 on 7.09m at tc 1. Compute is not equalized in the opponents' favor anywhere, so the losses are not a nodes or speed deficit.

## 3. Where square losses turn

Square losses versus hard and medium turn early and violently. Median turning ply: hard 2+1 16, hard 3+2 17, medium 2+1 25, medium 3+2 14, with 16 of 31, 16 of 31, 8 of 19, and 7 of 13 losses flipping inside the first 20 moves. Hex losses turn late, medians 60, 68, 81.5, and 98 on the four multi-loss hex legs, the lone hex hard 3+2 loss flipped at 23, and 0 of 21 hex losses across all legs flipped inside 20 moves. The reversal is a single opponent ply, not attrition: square hard 3+2, 14 of 31 losses had master at a mate-for score immediately before the final flip, and in 9 of 31 the opponent's single reply produced a mate-against score. Square hard 2+1: 8 of 31 was-mating, 4 one-ply reply mates. Medium 2+1: 6 of 19, medium 3+2: 7 of 13 was-mating. Master never saw mate-on-self as its first negative score on these arms, 0 cases across the 94 losses. A mate-for-master evaluation collapsing to mate-against in one opponent ply means the forced kill was already on the board and master's search scored the position as winning, a forced-line blindness, and the VCF tag on the killing reply in 4 to 7 losses per square hard arm points at the square VCF defense as the hole. Square easy 3+2 does not fit that shape: its 9 losses flip at median 74 with 7 of 9 late, 4 of 9 was-mating, 1 one-ply reply mate, 0 VCF-tagged killing replies.

## 4. Phase where the edge dissolves

Square hard: master ahead share 0.443 early, 0.461 mid, 0.429 late, behind share 0.529 mid, and getting-mated share exceeds mating share from mid on, 0.140 versus 0.127 mid and 0.073 versus 0.038 late at 2+1. Square medium 2+1 mirrors it, ahead 0.451 early, clamped mean turning negative late. On square the master is not ahead at any phase versus hard, the deficit exists from move 1 and compounds. Square easy keeps the edge in both controls: ahead 0.690 early and 0.774 mid with 0.508 mating share mid at 2+1, ahead 0.698 early, 0.807 mid, 0.548 late at 3+2. So the early-collapse mechanism belongs to hard and medium opposition on square, not the variant. Hex is different in kind: master ahead share 0.513 to 0.563 early on every hex arm, hex mid and late drift below 0.5 on the medium legs too, but the 0.594 draw rate absorbs the drift and the games end at the board cap.

## 5. Does the hex easy 2+1 miss share the mechanism

No. Hex easy 2+1 lost 7 of 60 with 33 draws, flips at median 68 with 6 of 7 late, only 1 loss from a mate-for score, 0 one-ply reply mates, wd 0.883 with CI 0.802-0.965 straddling the 0.90 threshold. The archived first read lost 3 of 20, all late flips, wd 0.850 CI 0.694-1.000. The shape matches the other hex arms, slow late erosion into draws, not the square early forced kill. Square easy 3+2 lands between the two families: edge retained through every phase, seat-symmetric, late flips, yet 9 losses and a cap miss at 0.850 CI 0.760-0.940. Three of its 9 losses carry a ponder-adopted move as the first master move after the flip, a signal absent from hex but present at 7 of 31 on square hard 2+1. The square hard and medium failures stay decisive with upper bounds 0.610 and 0.680 below their thresholds.

## 6. Length and draw profile

Hex: 107 draws in 180 games, 0.594, every draw exactly 397 moves, the full board. Square: 18 draws in 300 games, 0.06, every draw exactly 256 moves. Square decided games end fast on hard and medium, means 56 to 66 moves, while square easy runs longer, means 47 to 73, and square medium 3+2 drew none in 20 games and sits at 0.350, the worst leg. Ponder verdicts show no material asymmetry, master adopt share 39.7 percent versus easy on square easy 3+2 and 33 to 39 percent elsewhere.

## Arena labeling defect

Every square leg's verdict line reads hex-passgates: because passVerdictLine in playground/arena/hexgates.go hardcodes the prefix regardless of geometry. The gate math is unaffected, the forensic parser accepts the family prefix generically.

## Follow-ups

1. Replay the 14 square one-ply mate reversals, 9 on hard 3+2, from recorded positions and audit the square VCF defense and mate-distance scoring for the missed forced kills.
2. Probe the blue-seat opening deficit on square 2+1 with a red-blue mirror instrument, the red seat clears the hard threshold on its point estimate while blue collapses to 0.300.
3. Fund or close the two straddling easy rungs separately, hex easy 2+1 CI 0.802-0.965 and square easy 3+2 CI 0.760-0.940, both draw-heavy late-flip shapes, neither shares the hard or medium mechanism.
4. Audit ponder adoption on the move after a score flip, 3 of 9 losses on square easy 3+2 and 7 of 31 on square hard 2+1 carry it, before any ponder gate constant moves.
5. Fix the geometry-blind verdict prefix in the arena.
