# Square strength fix lane report

Post-fix analysis of the 2026-10-31 12-leg arena re-run, run on the frozen binary built after the solver audit fixes (fail-memo depth stamps 4939c52, fresh solver audit 9289217, both 2026-10-28). The hex-medium-tc1 leg was still running when this report was written, every other leg quoted here carries a committed verdict. All numbers come from playground/sqstrength/analyze.py over the arena-rerun logs, output at playground/sqstrength/out/taxonomy.md.

## 1. Verdicts and seat splits

wd counts a draw as a pass, matching the gate.

| leg | verdict | n | pooled wd | master red | master blue |
|---|---|---|---|---|---|
| square hard 2+1 | FAIL | 60 | 0.500 | 0.700 (17-4-9) | 0.300 (4-5-21) |
| square hard 3+2 | FAIL | 40 | 0.350 | 0.400 (7-1-12) | 0.300 (4-2-14) |
| square medium 2+1 | FAIL | 20 | 0.450 | 0.700 (7-0-3) | 0.200 (1-1-8) |
| square medium 3+2 | FAIL | 60 | 0.617 | 0.800 (20-4-6) | 0.433 (10-3-17) |
| square easy 2+1 | PASS | 60 | 0.917 | 0.933 | 0.900 |
| square easy 3+2 | FAIL | 60 | 0.850 | 0.900 | 0.800 |
| hex hard 2+1 | PASS | 40 | 0.775 | 0.900 | 0.650 |
| hex hard 3+2 | PASS | 20 | 0.850 | 1.000 | 0.700 |

The refaudit fixes moved the one-ply mate reversal class from 8 to 14 of 31 losses on the square hard legs to 3 to 4 of 26 to 30 losses, so that program worked, but the seat splits it left behind are the same as the pre-fix investigation recorded: master blue 0.300 versus hard at 2+1 both before and after.

## 2. Master trails hard from both seats

Reading the same games from the opponent's side gives the cleaner statement. Hard holding red scored 26 of 30 W+D (0.867) against master blue at 2+1 and 16 of 20 (0.800) at 3+2, while master holding red scored 0.700 and 0.400 against hard blue. Hard holding blue scored 0.433 and 0.650, master holding blue 0.300 and 0.300. Hard outperforms master from the same seat in both seats, at both controls. The 3+2 red-seat difference is 0.80 versus 0.40 over 20 games each, about 2.8 standard deviations apart, not noise.

Medium shows the same flatness one rung down: master red 0.775 pooled versus medium red 0.725, master blue 0.375 versus medium blue 0.325. Master is equal to medium, not above it. Only easy sits clearly below.

The opening lines are not the difference. On the identical red line H8 H5 J6 J8, hard converted 20-4-3 pooled while master converted 8-1-6. On the identical blue line H6 I6 I7 J5, master scored 3-4-18 while hard scored 6-1-8. Both engines play the same small set of openings (master red played H8 then H5 in 40 of the 48 hard-leg games the per-line tables cover, master blue opened H6 in 25 of the 26 counted 2+1 games), so the divergence happens after move 8, in midgame conversion.

The red seat dominates decided games against the solver tiers: red won 38 of 51 decided games versus hard at 2+1, 21 of 37 at 3+2, 15 of 19 versus medium at 2+1, 37 of 53 at 3+2, roughly 69 percent pooled, while against easy the seats are even (31 of 59 and 27 of 54). Strong red attacks beat whoever holds blue, and master's defense holds no better than hard's.

## 3. Loss taxonomy

| leg | losses | slow bleed | one-ply mate reversal | was-mating collapse | eval crash | blue flip median | blue early flips | red flip median |
|---|---|---|---|---|---|---|---|---|
| square hard 2+1 | 30 | 26 | 3 | 0 | 1 | 6 | 20 of 21 | 3 |
| square hard 3+2 | 26 | 20 | 4 | 1 | 1 | 6 | 14 of 14 | 3 |
| square medium 2+1 | 11 | 8 | 2 | 0 | 1 | 6 | 6 of 8 | 3 |
| square medium 3+2 | 23 | 20 | 0 | 2 | 1 | 6 | 13 of 17 | 19 |
| hex hard 2+1 | 9 | 8 | 1 | 0 | 0 | 60 | 0 of 7 | 1 |
| hex hard 3+2 | 3 | 2 | 0 | 0 | 1 | 48 | 0 of 3 | none |

Every hard and medium square loss ends on an opponent VCF kill, 60 of 60 on the hard legs. Flip is the first master move scoring below -300 with no later recovery: as blue, master's own evaluation turns permanently negative at its 3rd move (move 6 overall) in the great majority of square losses, and as red at its 2nd move (move 3, right after the book H8 and the opponent's H6 reply). Hex losses flip late, median 48 to 60, zero early flips.

Game 4 of square hard 2+1 (log lines 696 to 839) is the typical blue loss: master blue evaluates +300 at M2 and +1000 at M4 while building the I-file counter ladder, hard's J6 at M5 drops master to -1400 at M6, and the score decays -3000, -4000, -5400 until the VCF kill at move 91. The engine believes it is winning at the exact moment the position has already turned.

## 4. Hex versus square contrast

Draw rate is the structural difference: 23 of 40 games drew on hex hard 2+1 and 14 of 20 at 3+2, every hex draw a full 397-move board, against 9 of 60, 3 of 40, 1 of 20, and 7 of 60 on square. On hex the blue seat survives to the board cap and draws count as passes, master blue 0.650 and 0.700 ride 10 and 6 draws in 20 games. On square the blue seat gets attacked to a decision, 15 percent draws.

Master searches shallower on hex (5.6 to 6.9 mean depth) than square (8.0 to 8.6) and wins anyway, so depth is not what holds hex. What holds hex is that the blue position is genuinely defensible there, and the engine's early evaluations agree: master blue first-3-move scores average +1222 to +2600 on hex and the games indeed hold. On square master blue's early scores (+225 to +1466) are the same illusion in games it loses 70 percent of.

The book exists only for square by config (BookArtifactPathFor returns a path only for the square variant), and master's square book fired 35 times in 60 games at 2+1, always as red, 0 times as blue in 180 square games.

## 5. Ranked hypotheses

H1, square eval misprices the first-move initiative, direction opposite to outcomes. Every searched empty-board M1 on square scores negative for red: -300 for hard across the legs, -200 to -1000 for medium, while red empirically wins about 69 percent of decided games against hard and medium. After H8 H6, master's own M3 search scores -300 to -1000 for red in losses. As blue, master trusts +207 to +1466 early scores, chooses counter ladders over containment, and is lost by move 6. Because all tiers share one eval, extra depth deepens the misread instead of correcting it, which matches the time-control gradient: master red fell 0.70 to 0.40 and blue stayed 0.30 from 2+1 to 3+2 while hex rose 0.90 to 1.00 and 0.65 to 0.70. On hex the same negative red pricing matches reality, which is why the identical eval passes there. Evidence: arena-rerun-square-hard-tc1.log lines 697 to 704, arena-rerun-square-medium-tc1.log lines 103 to 930 (M1 scores), taxonomy flip medians, section 2 cross-seat table.

H2, master's compute shape buys under a ply of depth, flattening the tier ladder. Master reaches mean depth 8.1 to 8.6 on 15 to 27 million nodes per move, hard reaches 7.2 to 8.3 on 9 to 23 million, medium 7.5 to 8.3 on 8 to 14 million, so master's 2 to 3x node spend over medium buys +0.03 to +0.87 ply and over hard +0.1 to +0.7 ply depending on seat and control. A fraction of a ply should be worth a few points of win rate, and that is what the results show: the master versus medium versus hard strength ladder is empirically flat on square (section 2) while the gate expects a clear margin. The shortfall is search efficiency and eval quality, not a timing fault: 0 overruns of alloc in every leg, no illegal moves, ponder adopt rates and adopted-move score deltas match hard (49 versus 55 percent adopt, -356 versus -224 mean delta at 2+1).

H3, solver-pass mate proofs are exempt from the refutation audit and a residue is false. Square hard 3+2 game 17 (log lines 2166 to 2204): master's M15 H10 and M17 H7 come from VCT passes claiming M9 then M7, hard answers M18 G5 with a counter attack, master's M19 search returns -11100 and it is VCF-mated at M22. Hard's own M10 VCT claim of M7 in the same game was also false (its M12 self-evaluation is -9000). Counts: 1 to 4 solver-tagged mate claims per square leg collapse below -10000 or to mate-against within 2 of master's own moves, hex shows 0 of 26, and 16 master mate claims sit unconverted in medium 3+2 losses. Small next to the bleed class but a real correctness residue in the exact class the audit was built for, entering through the exemption that treats solver proofs as sound by construction.

H4, the book is inert where the gate bleeds. db/book.carobook is 1706 bytes, 4 canonical entries, one red-side trunk about 3 plies deep with priors from 110 games (db/book-stats.txt, db/book-tree.txt). Blue-to-move positions are not stored, so master's blue opening is re-derived by a 22 million node search every game and converges on the same H6 ladders, the H6 I6 I7 J5 line alone took 25 of 60 blue games across the hard legs and scored 3-4-18. This is an enabler for H1 rather than an independent cause: the same tree expansion runs on the same eval, so blue book entries help only if the tree's 268 million node assessment of blue defenses beats the in-game search's.

## 6. Fix plan inside the laws

Every item is additive or a correctness fix on the pre-registered capability-removal ladder, no tier shape reduction, no handicapping, square-only by construction except item 2, which is a correctness fix shared with hex.

1. H1, per-geometry eval calibration of the tempo pricing. Run square-specific candidate weight vectors through the existing arena machinery, -evalweights and -evalbase seats with scripts/evaladopt-arms.ps1, deciding games direct, adoption only on arena feedback per the eval weight evolution law, no self-play floor gates. Target metric: the pooled master-blue score on square hard 2+1, with the cross-arm instrument watching hex legs for regression. Test surface: weight-parsing unit tables already exist, the adoption arms are the evidence chain, plus a playground capability-perturbation run per the inversion research law before any tier admission. Removed if the arms show no gain.
2. H3, close the solver-proof exemption. First replay the recorded g17 M17 position in playground/solverpass to decide between an unsound proof and a follow-up failure. If unsound, fix the defend reply enumeration or the pattern classification at that node, with a red-first unit test from the replayed position. Independently, extend auditRefuted in internal/server/match.go to also audit solver-pass returns at the root when the grant allows, same cold-solver construction as the inner-search audit. Test surface: solversearcher tests for the audit-on-proof path, the g17 replay as a pinned unit, mutation killers on the new gate. Cost is one extra cold pass per proof, 1 to 3 per game.
3. H4, emit blue-side book entries. Extend the fixed-budget tree expansion's emit step to store blue-to-move canonical entries so the probe hits as blue, keep BookMaxPly 8. Test surface: booksearcher tests for blue-side probes and the past-ply bound, book round-trip tests, arena rerun of the square legs. Removed if the rerun regresses.
4. H2, depth conversion, conditional. Only if the H1 experiment shows the eval is not the cause: profile SMP node efficiency at 6 threads on square midgame positions, then widen pruning or ordering to convert the node surplus into ply. Heavier and less certain than 1 to 3, so it waits for the discriminator.

## 7. Cheapest discriminating experiment

Take 20 recorded positions from square master-blue losses at moves 4 to 8 (the H8 H6 H5 X family from the logs), run the frozen binary on each with a fixed 60 second budget, about 10x the in-game spend, and record the score sign and depth per position. H1 predicts the blue score stays at or above 0 at every reachable depth while the positions lose 70 percent in play, condemning the eval. H2 predicts the sign turns negative as depth passes the in-game 8 to 9, condemning the search's depth ceiling. About 40 single-engine minutes, no arena, no batch, runnable the moment the frozen batch finishes. The g17 solver replay in item 2 is the companion discriminator for H3 at minutes of cost.

## Reading log

read playground/arena/out/arena-rerun-square-*.log grep winner/verdict/M-lines - loss tally, taxonomy, game traces
read playground/arena/out/arena-rerun-hex-hard-tc1.log grep winner/M-lines - hex contrast leg
read playground/arena/out/arena-rerun-hex-hard-tc2.log grep winner - second hex leg
read playground/arena/out/square-collapse-investigation.md full - pre-fix record and prior seat splits
read playground/refaudit/design.md full - fix program scope and landed parts
read playground/arena/main.go full - seat construction, play loop, ponder wiring
read playground/arena/hexgates.go full - gate math, thresholds, wd definition
read internal/config/config.go grep tiers, clock, book paths - tier shapes and book config
read internal/server/stats.go lines 1 to 64 - M-line field semantics
read internal/server/match.go lines 380 to 560 - solverSearcher, audit, ponder passes
read internal/server/booksearcher.go full - book probe conditions, artifact loading
read internal/vcf/search.go full - attack, forcedBlock, defend, threatReplies
read internal/vcf/solver.go grep ttStamps - stamp fix presence
read db/book-stats.txt, db/book-tree.txt full - artifact coverage
read scripts/arena-rerun.ps1 full - freeze law, leg order, resume
read playground/gateforensic/parse.py full - log regexes reused by the analyzer
read playground/gateforensic/config.py full - arm labels, mate score encoding
wrote playground/sqstrength/analyze.py, playground/sqstrength/out/taxonomy.md - evidence artifacts
