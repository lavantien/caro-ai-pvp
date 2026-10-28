# Square passgate collapse investigation

Draft over the 11 resolved gate legs plus the archived hex easy tc 1 first read. The 12th leg, square easy tc 2, is still playing: its rows below are provisional from the streaming log, 23 finished games at the last dataset refresh, and no conclusion rests on it. All folds run from playground/gateforensic/out through the committed query set, ledger clean at 0 unaccounted lines across 13 sources and 67,970 move rows at that refresh, every verdict W+D and interval recompute matches the arena's own verdict lines exactly, and the win-sign check passed at 100 percent. Full tables live in gateforensic-report.md.

## 1. Red-first or seat-symmetric

Square hard 2+1: master red 0.667 (18W-2D-10L, CI 0.498-0.835), master blue 0.300 (7-2-21, CI 0.136-0.464), pooled 0.483. Square medium 2+1: red 0.750 (14-1-5), blue 0.300 (4-2-14), pooled 0.525. Square medium 3+2: red 0.600, blue 0.100, pooled 0.350. So at 2+1 the pooled failure is mostly the blue seat, master red alone would sit at or above threshold versus hard. But square hard 3+2 collapses in both seats, red 0.500 and blue 0.467, so the anomaly is not only first-move advantage. Hex shows no blue collapse anywhere, blue W+D 0.70 to 1.00. Square easy 2+1 passed at 0.933 with both seats at 0.900 or better.

## 2. Compute parity

On untagged search moves master out-searches every square opponent. Square hard 2+1: master mean depth 7.74 on 18.55m nodes at 6.33m nps versus hard 7.76 on 12.94m at 4.47m, 43 percent more nodes at equal depth and 42 percent higher nps. Square hard 3+2: master 8.17 on 32.02m versus hard 8.39 on 22.03m, hard reaches 0.22 ply deeper but on 31 percent fewer nodes. Square medium 2+1: master 8.61 on 18.73m versus medium 8.33 on 8.51m, better depth on 2.2 times the nodes. Hex parity is similar in direction but milder, master 6.15 on 9.53m versus hard 6.26 on 7.09m at tc 1. Compute is not equalized in the opponents' favor anywhere, so the square losses are not a nodes or speed deficit.

## 3. Where square losses turn

Square losses turn early and violently. Median turning ply: hard 2+1 16, hard 3+2 17, medium 2+1 25, medium 3+2 14, with 16 of 31, 16 of 31, 8 of 19, and 7 of 13 losses flipping inside the first 20 moves. Hex losses turn late, medians 60, 68, 81.5, and 98 on the four multi-loss hex legs, the lone hex hard 3+2 loss flipped at 23, and 0 of 21 hex losses across all legs flipped inside 20 moves. The reversal is a single opponent ply, not attrition: in square hard 3+2, 14 of 31 losses had master at a mate-for score immediately before the final flip, and in 9 of 31 the opponent's single reply produced a mate-against score. Square hard 2+1: 8 of 31 was-mating, 4 one-ply reply mates. Medium 2+1: 6 of 19 and medium 3+2: 7 of 13 was-mating. Master never saw mate-on-self as its first negative score on square, 0 cases across the 94 losses of the four failed square arms. A mate-for-master evaluation collapsing to mate-against in one opponent ply means the forced kill was already on the board and master's search scored the position as winning, a forced-line blindness, and the VCF tag on the killing reply in 4 to 7 losses per square hard arm points at the square VCF defense as the hole.

## 4. Phase where the edge dissolves

Square hard: master ahead share 0.443 early, 0.461 mid, 0.429 late, behind share 0.529 mid, and getting-mated share exceeds mating share from mid on, 0.140 versus 0.127 mid and 0.073 versus 0.038 late at 2+1. Square medium 2+1 mirrors it, ahead 0.451 early, clamped mean turning negative late. So on square the master is not ahead at any phase versus hard, the deficit exists from move 1 and compounds. Hex is different in kind: master ahead share 0.513 to 0.563 early on every hex arm while square hard and medium start at 0.443 to 0.510, hex mid and late drift below 0.5 on the medium legs too, but the 0.594 draw rate absorbs the drift and the games end at the board cap. Square easy 2+1 keeps the edge everywhere, ahead 0.690 early and 0.774 mid with 0.508 mating share mid, which localizes the failure to hard and medium opposition on square, not the square variant as a whole.

## 5. Does the hex easy 2+1 miss share the mechanism

No. Hex easy 2+1 lost 7 of 60 with 33 draws, flips at median 68 with 6 of 7 late, only 1 loss from a mate-for score, 0 one-ply reply mates, wd 0.883 with CI 0.802-0.965 straddling the 0.90 threshold. The archived first read lost 3 of 20, all late flips, wd 0.850 CI 0.694-1.000. The shape matches the other hex arms, slow late erosion into draws, not the square early forced kill. The hex miss is marginal, the square failures are decisive with upper bounds 0.610 and 0.680 below their thresholds.

## 6. Length and draw profile

Hex: 107 draws in 180 games, 0.594, every draw exactly 397 moves, the full board. Square: 14 draws in 259 finished games, 0.054, every draw exactly 256 moves. Square decided games end fast, means 47 to 66 moves, medians 35 to 61, minima 21 to 31, while hex decided means run 77 to 126. Square medium 3+2 drew none in 20 games and sits at 0.350, the worst leg. Ponder verdicts show no material asymmetry, master adopt share 33 percent versus hard 35 percent on square hard 2+1.

## Arena labeling defect

Every square leg's verdict line reads hex-passgates: because passVerdictLine in playground/arena/hexgates.go hardcodes the prefix regardless of geometry. The gate math is unaffected, the forensic parser accepts the family prefix generically.

## Follow-ups

1. Replay the 14 square one-ply mate reversals, 9 on hard 3+2, from recorded positions and audit the square VCF defense and mate-distance scoring for the missed forced kills.
2. Probe the blue-seat opening deficit on square 2+1 with a red-blue mirror instrument, the red seat clears the hard threshold on its point estimate while blue collapses to 0.300.
3. Fund or close the hex easy 2+1 straddle separately, CI 0.802-0.965, it is a different mechanism and the leg is underpowered at 60 games.
4. Fix the geometry-blind verdict prefix in the arena.
5. Rerun scripts/gateforensic.ps1 -Step all when square easy tc 2 lands and finalize this file with the resolved 12th leg.
