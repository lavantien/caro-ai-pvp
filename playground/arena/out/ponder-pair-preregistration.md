# ponder pair pre-registration

Fixed before any game runs, per the gate law. Governs the v0.26 roadmap item: the ponder arena pair, noponder versus pondering seats at the gated control, and the tag decision that follows it. Supersedes no earlier protocol, the smoke arms and their rules stay as recorded.

## premise

Pondering is in tree and ships at every tier above easy: the engine StartPonder and StopPonder lane, the [PONDER] M-lines, and the 4-clause adoption gate landed 2026-10-08, the adoption machinery and lane hardening closed 2026-10-22, and the widths are committed in the tier table (internal/config/config.go:92-95). What no run has measured is strength: whether a seat that ponders at its tier width beats the same seat with the lane off. The recorded telemetry says the machinery is healthy and the lever is the gate, not the prediction: adoption 12.5 to 14.4 percent across tiers and controls over 214,354 moves, the true prediction hit rate 67 to 73 percent flat everywhere, gate-limited not prediction-limited (playground/logmine/out/ponder-adoption-logmine.md, ponder-hitrate-logmine.md, banked 2026-10-22). The t-inflation the three smoke arms replicated in full-contention pairs was root-caused and fixed 2026-10-10, the off-lane pass under a pure stop flag, with zero stalls over 149 post-fix ponder lines and worst turn t 4.71 s (devlog 2026-10-10, ponderscan-three-arms.md), and the 2026-10-18 re-gate measured grandmaster adopted t max 0.22 s (regate-2plus1-gate.md rule 0). The pair is the separation evidence the chain has queued since the v0.25 close, whose entry records "the arena pair queues behind the weights chain, evidence pending", and the v0.28 gates queue behind this pair per the adoption ordering.

## trigger and sequencing

Three conditions, all recorded: the weights chain resolves first, its deciding pairs evalpair-21 and evalpair-32 are the pending state (evaladopt-terminal.status, re-registered 2026-10-24), the machine is free of any engine-timing batch, then the pair runs as the machine's one engine-timing batch, detached under a native launcher with the GOMEMLIMIT tournament bound on the launch target and resume from the append-only pair log on any interruption. One frozen arena binary drives every block, the bin/arena-freeze.exe pattern the gate chains use. The order against the booktree validation chain is recorded nowhere, see the open questions.

## the control

2+1, tc index 1 into config.TimeControls (internal/config/config.go:40). The 4 controls are not interchangeable and the recorded evidence picks one. 1+0 is excluded: the v0.20 arena program established it as thread-decided, the smoke10 arm ran analysis-only for that reason, and grandmaster adoption drops to 13.1 percent there because opponent turns run about a second, so the lane is mostly off and a read at 1+0 measures thread speed, not ponder. 10+5 is the confirmation control at about 20 minutes per game, too dear for a 60-game block. Between 2+1 and 3+2 the telemetry shows no ponder-surface difference at the subject tier, grandmaster adoption 14.4 percent at both controls over 22,245 and 10,499 opportunities, so the wider increment buys nothing for this question. 2+1 is where the strength instruments are calibrated: the v0.19 reference ladder, the committed floor-gm21, and the increment-axis instrument of 3 color-paired 2+1 repetitions plus a 3+2 discriminator (increment-axis-investigation.md, closed 2026-10-10). The pair inherits that instrument's primary control.

## the seats

grandmaster versus grandmaster-noponder, both seats already registered (playground/arena/main.go:104, :117). The noponder seat differs in exactly one field, PonderThreads 0 against 8. Everything else is TierGrandmaster verbatim (internal/config/config.go:95): 8 search cores, 2 GiB TT, VCF, VCT, the book probed at 16 plies, and the scaffold eval weights in force per the evaladopt terminal state. Grandmaster is the subject tier on 3 recorded grounds. First, the 2026-10-10 re-spec accepted consequence: a subset-width ponder undershoots the full-width grant ring's reference depths, so non-grandmaster adoption leans on the gate's time-adequacy clause, and grandmaster is the only tier whose ponder width equals its search width, 8 against 8, so its ponder lane reproduces the mover's own search shape while a master pair would blend ponder weakness with gate refusal. Second, grandmaster carries the highest ponder surface in the telemetry, 14.4 percent adoption and 67.5 percent hit at 2+1. Third, the noponder seats exist only for master and grandmaster (main.go:115, :117) and no master-master floor was ever committed (v024-bookgate-verdict.md), while grandmaster has floor-gm21 to read against.

Room shape: the pair books the peak max(a.Cores+b.PonderThreads, b.Cores+a.PonderThreads) (internal/config/config.go:101) = max(8+0, 8+8) = 16 = MachineCores (config.go:117), the whole machine, parallel 1, the same worst-case contention the tournament law already funds, with 2 TTs at 2 GiB each inside the GOMEMLIMIT bound. The admission ledger books it per the v0.26 law: cores acquired at the search window, the ponder width booked on the opponent's turn, no partial grants, never a width cut, and the noponder seat books width 0 so the room grant is exactly 16 at the peak. The book state is shared: both seats probe the same artifact at 16 plies, book hits leave the ponder lane unarmed (accepted record 2026-10-18), so the book removes ponder surface symmetrically and never differs between the seats.

## stopping rule

The Wald block-extension law of 538a722, the committed gate instrument: blocks of 20 games, at most 3 blocks, the Wald 95 percent interval at gateZ 1.96, lower bound at or above the threshold passes, upper bound below fails, a straddle extends one block, and the cap falls to the point rule (playground/arena/hexgates.go:55-77). The statistic is the house floor estimator verbatim, the ponder seat's color-pair-pooled decisive fraction, frac ± 1.96 times sqrt(frac times (1 minus frac) over decisive games), the same derivation as floor-gm21 (regate-2plus1-gate.md), and the threshold is 0.5: under the null of equal seats every decisive game is a fair coin, whatever the draw rate. The 62.1 percent same-base red tilt (rnd/gametheory/REPORT.md verdict 1) cancels inside the per-game color alternation, each seat takes red in half the pooled games, and the ptnml pair tally records the color-paired structure as a read, not the decision statistic. Wald over SPRT: the rnd campaign adopted SPRT early stop as a tuning-harness method, but no gate in this repo has ever run it, while the Wald block law is implemented, capped, and has run every passgate leg since 538a722. Power, stated before any game: at the 60-game cap with the floor-gm21 draw share, 93 of 105 decisive, the decisive count is near 53 and the half-width near 0.13, so only an edge at or above about 0.63 certifies by interval, a true 0.60 edge passes at the cap with probability about 0.9, and a wash resolves at the cap by the point rule. The 2+1 clock books about 5 minutes of move time per game, 2 minutes plus 1 second per own move over a 60-move game, so the full cap costs about 5 machine-hours.

## opening protocol

The arena pair protocol as it ships: red and blue alternate per game (playground/arena/main.go:287-290), the first move comes from the engine or the book, and pair mode carries no trunk rotation. Same-base pairs concentrate on one trunk, 109 of 120 same-base games sat on the H8 H6 H5 line with 9 exact full-move repeats under lazy SMP (rnd/gametheory/REPORT.md section 5), so the concentration is a recorded fact, not an assumption to wave off. It stays acceptable here because it is symmetric: each seat takes red every other game inside a ptnml pair, so the trunk adds variance, never a seat bias. The bookgen diversity alarm (playground/bookgen/corpus.go:69) applies as a read rule on the pair log: the distinct first moves are counted, a single-trunk log carries a coverage caveat into the verdict, and no caveat re-opens the decision rule.

## ponder health

The smoke rule 3 carried verbatim onto this pair: every M-line tagged [PONDER] must be a legal move, carry t under 1 second, carry alloc equal to that turn's budget draw, and derive nps from ponder nodes over ponder elapsed. Any illegal move, engine panic, or mismatch between the ponder outcome lines (miss, reject, adopt) and the seat tallies blocks the tag and routes to root-causing before any rerun. The regate rule 0 record for the subject tier is adopted t max 0.22 s at t/alloc 0.77, and an adopted t above 1 second is the old t-signature class, a defect under the 2026-10-10 fix, never noise. The run records adoption and hit rate against the banked band, 14.4 percent and 67.5 percent at 2+1, as telemetry with no new threshold.

## verdict read

Pass: the lower bound reaches 0.5 at any block, or the cap straddle resolves by the point rule with the decisive fraction at or above 0.5. The tag proceeds, pondering stays shipped at the tier widths, and the v0.28 gates run with the lane on. A pass that arrives only through the point rule is recorded as unproven, and the gate-tuning question reopens on it as the next ponder work, the 4 clauses and their constants PonderAdoptFraction 0.5, PonderStableIters 3, PonderScoreDropMargin 300, PonderDepthHistory 8 (internal/config/config.go:123-126) being the levers the gametheory report names. Fail: the upper bound falls below 0.5 at any block, or the cap straddle resolves below 0.5. The tag blocks, the strength contribution is negative at the shipped widths, root-causing opens on the gate constants and the lane, and no rerun happens before a fix lands. Extend: a straddle before block 3, one more block of 20 appended to the same pair log. One pair, one control, one tier decides the tag for every ponder width, per the roadmap line.

## outcome

| item | value | verdict |
| --- | --- | --- |
| control | | |
| blocks run | | |
| ponder decisive frac, lb, ub, n | | |
| ptnml | | |
| adoption over arms | | |
| hit rate | | |
| adopted t max | | |
| distinct first moves | | |
| health | | |
| decision | | |

## hub rulings

Ruled 2026-11-05 under docs/plans/final-close.md, closing the 3 questions above.

The weights chain condition resolves by cut: evalpair-21 and evalpair-32 are dropped because the fitted weights were derived under an eval that the clean-window and boxed-ladder terms replaced and the fitted set was already rejected at both controls, so the scaffold stays in force and nothing weights-shaped gates this pair.

The pair takes the machine after the booktree funded build, the book validation arms, and the final square passgate, between the v0.25 and v0.26 tags. Both seats probe the book at 16 plies, so the pair reads the funded artifact, never the 22-entry trunk artifact.

No book or weight swap is scheduled before v0.28. If one lands anyway, the pair re-runs whole on the new artifact before any tag that depends on it.

A point-rule pass triggers one 3+2 confirmation pair of 20 games, recorded as evidence for the reopened gate-tuning question. The v0.26 verdict rides the 2+1 arm alone.
