# five-tier re-gate preregistration

Preregistered 2026-10-11, revised same day after a two-reviewer adversarial pass (14 and 13 findings, all confirmed findings folded in; this revision supersedes the first commit of this file in every rule). Supersedes the 8-seat framing of increment-axis-investigation.md follow-up 2: the demand re-spec grew the roster to 10 seats, so the ladder carries 4 adjacent families (easy-medium, medium-hard, hard-master, master-grandmaster). The master-master floor wait is lifted: the ponder off-lane fix landed 2026-10-11.

What carries from the parent unchanged: the crossarm fold as the only instrument, floors before edge reads, color-paired pooling, standings demoted to descriptive, and full 0.60-regime certification reserved for the closing ladder. What moves and why: the gate statistic becomes the decisive fraction (wins over decisive games), because identical-tier pairs give it an exact 0.5 null by symmetry, which kills the draw-regime dependence the W+D statistic carried; and the edge test becomes two-sample (arm against floor at combined variance) with the levels and power below recomputed honestly.

## instruments

The crossarm fold is the only instrument, extended before any gate arm runs: (1) a per-seat ponder telemetry section (adoption count, mean and max turn t over [PONDER] lines parsed with the boundary-clean ", t=" rule, tag counts), because the current fold drops tagged lines after counting them; (2) a gate mode that pools multiple run directories per arm, emits per-family decisive fractions with standard errors, the two-sample z against the pooled floor, and the rule verdicts. Until both land, no floor or gate arm starts. Hand-derived gate numbers are banned.

The tournament surface is the only drive surface. The bot series color law is strict per-game alternation from a host-red game 1 (internal/server/series.go NewBotSeries sets alternateRed, applyGame flips unconditionally; internal/tourney/runner.go mirrors it and cross-checks the billing), so the two host directions of one pairing form exact color pairs at each game index and the first-player effect cancels by construction, not merely in expectation.

## floors

Four floors: hard-hard at 2+1 and 3+2 (bands for easy-medium, medium-hard, hard-master), grandmaster-grandmaster at 2+1 and 3+2 (band for master-grandmaster, the 16-core pair-peak shape). No 1+0 or 10+5 floor: no rule below reads one. Each floor is assembled from labeled pair runs (2-participant runs, 2 bo11 series each) pooled by the crossarm gate mode, minimum 90 finished games per floor, run count logged. Statistic: the color-pair-pooled decisive fraction from one seat's perspective; the floor's role is (a) the surface-symmetry check, its own two-sided 95 percent binomial interval on the decisive fraction must cover 0.5, a miss voids the gate until root-caused, and (b) the empirical null center and variance for the edge tests. Cross-family variance transfer is the stated assumption: the null center is 0.5 for every family by the symmetry above, the floor supplies dispersion at the same control, and family-specific dispersion differences beyond the floor's are absorbed as unmodeled noise, disclosed in every verdict.

## gate arms

3 repetitions of the 10-seat 2+1 run (90 bo3 series, about 62 games and 55 decisive per family pooled) plus 1 repetition of the 10-seat 3+2 run (about 21 games, 19 decisive per family). Seat orders, preregistered verbatim (labels: e1 e2 m1 m2 h1 h2 M1 M2 G1 G2 for easy-1 through grandmaster-2):
- rep 1: e1 e2 m1 m2 h1 h2 M1 M2 G1 G2
- rep 2: G1 M1 h1 m1 e1 G2 M2 h2 m2 e2
- rep 3: e1 m1 h1 M1 G1 e2 m2 h2 M2 G2
- the 3+2 arm: M1 G1 e1 h1 m1 M2 G2 e2 h2 m2

The permutations interleave tiers so no same-tier pair stays adjacent and each label occupies different schedule positions per repetition; they land as four spec functions in internal/tourney before any repetition runs. This is a permutation, not the rotation the first revision claimed: rotation preserves tier-block contiguity, and position-linked systematics (the documented regime seam class) would keep a correlated sign across rotations.

The 10-seat 3+2 arm is not the parent's q2 discriminator: q2 asked for a second draw of the same 8-seat 3+2 configuration, a different population. This arm serves the 3+2 family reads below; the dedicated 8-seat q2 replicate stays a deferred follow-up unless the hard-master family read re-flags the anomaly.

## decision rules, fixed in advance

Rule 0, telemetry precondition, quantified: every [PONDER] adoption line in every pondering seat carries turn t below 1.0 s (per-arm max and mean emitted by the fold's ponder section; easy never ponders by construction), and no seat's untagged-search t-over-alloc arm mean exceeds 0.95. A violation voids the arm's gate numbers and routes to the fix chain. A passing rule 0 is necessary, not sufficient: the parent's t-line confinement bounds only the observable, so health is never proven, only not-disconfirmed. Parallel 1 is enforced by the conductor's static pair-peak core check, the same refusal that bound the quick arms.

Rule A, inversion alarm, per family, one-sided upper: fire iff z = (lower-tier decisive fraction minus floor center) over the combined null standard error 0.5*sqrt(1/N_e + 1/N_f) exceeds 2.496, the per-test cut at familywise 0.05 across the 8 alarm reads (4 families times 2 controls). A 2+1 fire fails the rescale tag. A 3+2 fire is a discriminator: the tag holds pending an investigation whose verdict decides, because the 3+2 arm holds about 19 decisive games per family and the parent's power accounting already established that 20-game reads fire on noise by construction. Exact-boundary outcomes count as not fired.

Rule B, strength certification, per family, one-sided: same statistic and level over the 8 certification reads, upper tier against its family floor. At 2+1 the threshold sits near a 0.71 decisive fraction; operating characteristics, stated honestly: about 44 percent power against a true 0.70-regime edge, about 67 percent against 0.75, about 84 percent against 0.80; at 3+2 about 30 percent against 0.75. So this gate certifies the 0.75 regime with useful power, reads the 0.70 regime marginally, and reports everything weaker as unproven, never as passed. 80 percent power against 0.70 needs roughly 300 decisive games per family, about 6 repetitions, which is the closing ladder's budget, not this gate's. Rule B never blocks the tag.

Rule C, cross-control precedence: a 2+1 rule A fire fails the tag regardless of 3+2; a 3+2 rule A fire without a 2+1 fire holds the tag pending investigation; rule B certifications at the two controls report independently with no precedence.

Rule D, partial arms and integrity: an arm that dies resumes whole per the standing law with the seam disclosed in its verdict; a fold that fails integrity (bad lines, unfinished series beyond resume, zero-sum break) voids the arm and it reruns whole. Ties resolve conservatively in every rule (no fire, no certification).

## position of the current quick arms

The 2026-10-11 five-tier smoke10 (1+0) and the smoke21 (2+1) that follows are plumbing and first-look arms: they prove the 10-seat roster runs at parallel 1 under the GOMEMLIMIT bound with healthy ponder telemetry, and their folds join the standing crossarm record as descriptive data. No tag decision and no band read rides them; the first revision's early-warning tripwire against existing bands is struck, no floor band exists at their read time.

## budget

A 10-seat 2+1 repetition is 90 series against the 8-seat arm's 56, a 1.6 factor on the one-night 8-seat unit: about 1.6 nights per repetition, roughly 5 for the three, 1.6 to 2 for the 3+2 arm, and each floor about half a night to one night by control for 2 to 4 total. About 9 to 11 serialized engine nights. The server mutation triage wave and the book corpus claim the same night slots; queue order: quick arms (running), crossarm extension (code, no engine time), smoke21, the four floors, then the three 2+1 repetitions and the 3+2 arm interleaved with the server re-gate as machine time allows, closing ladder last.
