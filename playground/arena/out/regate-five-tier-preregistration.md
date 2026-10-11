# five-tier re-gate preregistration

Preregistered 2026-10-11 before any gate arm runs. Supersedes the 8-seat framing of increment-axis-investigation.md follow-up 2: the demand re-spec grew the roster to 10 seats (grandmaster-1, grandmaster-2), so the ladder now carries 4 adjacent families (easy-medium, medium-hard, hard-master, master-grandmaster) and the gate reads all 4. The power accounting, the color-paired pooling, and the floor-band law carry over unchanged from the investigation. The master-master floor wait is lifted: the ponder off-lane fix landed 2026-10-11, so same-tier ponder pairs no longer absorb known defect variance.

## instruments

The crossarm fold (playground/crossarm, make crossarm) is the only measurement instrument; hand-rolled extraction stays retired. Every number the gate reads comes from a fold with zero bad lines, zero unfinished series, zero fold mismatches, and zero-sum holding. The tournament surface is the only drive surface. A 10-seat run is 90 bo3 series, each pairing hosted both directions; a pair run is 1 pairing, 2 series, both host directions. The two host directions of one pairing form the color-paired block: game j of the X-hosted series pairs with game j of the Y-hosted series, canceling the first-player effect in expectation. Explicit in-series red alternation is rejected as an instrument change: the series color law (loser takes red) is part of the standing surface and the double round robin already delivers both colors per pairing, so the block design rides it with zero new machinery.

## floors

Identical-tier pair runs, 100 games per pair (target count of finished games, series partition immaterial to the fold), per control, never transferred across controls. The band statistic: the two-sided 95 percent binomial interval on the color-pair-pooled success fraction, success = win or draw from one seat's perspective, which centers the band at the same-tier W+D rate rather than 0.5 so draw-rate regimes are absorbed into the null.

Floor set: hard-hard at 1+0, 2+1, 3+2, 10+5 (the canonical middle-of-ladder band, one per draw-rate regime), plus grandmaster-grandmaster at 2+1 and 3+2 only (the top of the ladder is the newest, widest-ponder shape and its noise regime is unmeasured; 1+0 is recorded-not-gated and 10+5 rides provisional gains). Six floor runs total, serialized engine batches, detached, resume-on-dead.

## gate arms

3 repetitions of the 10-seat 2+1 run plus 1 repetition of the 10-seat 3+2 run (the q2 discriminator, not deferred). Seat slots are permuted per repetition by a fixed preregistered permutation committed before the runs (a rotation of the roster slice, no runtime RNG), so seat labels cannot stay coupled to schedule positions across repetitions; the permutation lands as a driver spec argument when the wave opens. Standings are demoted to descriptive output everywhere; no gate rule reads them.

Pooled per family per control: the color-paired W+D record of the upper tier over the lower, across both seats of both tiers and all repetitions.

## decision rules, fixed in advance

Rule 0, telemetry precondition: every arm must pass the ponder health invariant (adoption lines carry near-zero turn t, no stall signature in any seat's telemetry) before any rule below reads. A violation aborts the gate to the fix chain; gate numbers from a sick arm are void.

Rule A, inversion test, per family per gated control: the lower tier's pooled W+D fraction over the upper must sit inside the floor band at that control (hard-hard band for easy-medium, medium-hard, hard-master; grandmaster-grandmaster band for master-grandmaster). Any outside-band inversion fails the rescale tag and reopens the tier behind the inversion research law.

Rule B, strength certification, per family per gated control: the upper tier's pooled W+D fraction clears the floor band's upper bound at one-sided 95 percent with Bonferroni across the 4 families (familywise 0.0125 each) = certified in the 0.70 regime. Inside the band = unproven at this power, reported as unproven, never as passed; rule B alone never blocks the tag. Full 0.60-regime certification (~150 games per family) stays reserved for the closing ladder round.

Rule C: the 3+2 discriminator reads rules A and B at 3+2 with its own floor bands; a 3+2 inversion fails the tag even when 2+1 passes, because the two controls carry independent draws of the same ladder.

Rule D: an arm lost to a host-side death is resumed whole per the standing law; a resumed arm is folded normally and its seam disclosed in the verdict doc, as the smoke21 seam was.

## position of the current quick arms

The 2026-10-11 five-tier smoke10 (1+0) and the smoke21 (2+1) that follows it are plumbing and first-look arms: they prove the 10-seat roster runs at parallel 1 under the GOMEMLIMIT bound with healthy ponder telemetry, and their folds feed the standing crossarm record as descriptive data. Rule A may be read against the existing floor bands as an early-warning tripwire; no tag decision rides them. The rescale tag decision rides the designed gate above. The ponder tag rides its separate arena pair (master-noponder versus master); the grandmaster tag rides its own strength gate at v0.28 rungs.

## budget

Each 10-seat 2+1 repetition is roughly one night detached at parallel 1, the 3+2 discriminator similar or longer, each 100-game pair run roughly half a night to a night by control. Six floors plus four gate arms is about 8 to 10 serialized engine nights. The server mutation triage wave and the book corpus claim the same night slots, so the queue order is explicit: quick arms first (running), then the floor set, then the 3 gate repetitions plus discriminator interleaved with the server re-gate as machine time allows, closing ladder last.
