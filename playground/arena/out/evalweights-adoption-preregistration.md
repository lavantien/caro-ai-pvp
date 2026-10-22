# eval weights adoption pre-registration

Fixed before any refit or arena game runs, per the gate law. Governs the replacement of the scaffold evaluation constants with organic fitted weights.

## premise

The ruleset is a 16x16 variant that exists nowhere else, so expert priors over pattern class values cannot exist. first-cause.md:59 forbids arbitrary hardcoded sets for exactly this reason and :77 mandates zero hard code. The shipped values, PatternClassWeights = {0, 100, 500, 1200, 4000, 15000} at internal/config/config.go:163-167 and :266 plus EvalTempo = 1000 at :161, consumed at internal/engine/eval.go:69, are bootstrap scaffolding. The 2026-10-19 fit (playground/evalfit/out/evalfit-master-gm.txt, 395 grandmaster+master games, 31165 positions) already disagrees structurally: weights 137, 162, 483, 484, 1561 at scale 3638.55, validation loss 0.68447 against the 0.69001 scaffold baseline, with three and four collapsing to near equal and open two far heavier relative to the scaffold. The scaffold stays in force until this protocol's gates pass.

## trigger and sequencing

All 3 must hold: the bookgen corpus reaches 1500 games and the day-8 merge, overlay, and book chain complete. The playground/evalfit refresh (outcome-labeled data path plus the SHADE challenger) lands. The machine is free of any engine-timing batch. The refit then runs as the next engine-timing batch, before the v0.25 ponder arena pair and the v0.28 master gates, so every later gate measures the final weights. One batch on the machine at a time.

## refit spec

Corpus: grandmaster and master games only, the strongest available play. The bookgen corpus files carry no outcomes (playground/bookgen/corpus.go:396-400 writes G, P, and E lines with no winner field), so the data comes from the db game path plus the refresh lane's outcome-labeled master self-play generator, together at least 1500 games. Fitter: gradient descent primary with the SHADE challenger, the 2 percent DE-versus-GD tolerance check preserved. Integer conversion: round to nearest int, class 0 pinned at 0, drift recorded in the artifact as weights-int-drift already is (evalfit-master-gm.txt:14), and any drift above 5 percent of the largest weight is rejected. Artifact: the existing out/ txt field set verbatim so diffs stay readable, baseline loss measured on the same holdout with scaffold weights.

## tempo decision point

Open until the refit numbers exist, then fixed in the outcome section before any arena game.

Keep EvalTempo at 1000: cost is zero. Risk is a distorted ratio, tempo would sit at 64 percent of the top class under the fitted vector against 6.7 percent under the scaffold, inflating the side-to-move bias relative to every class.

Refit tempo as a sixth free parameter: cost is one extra fit dimension plus rewriting the EvalTempo == EvalMilliUnit pin at internal/config/config_test.go:485-486 and the tempo-anchored tests it feeds. Risk is conflating red-first seat advantage with side-to-move tempo in the outcome labels.

Derive tempo from the fitted scale ratio: cost is one derived constant in the config hub. Risk is carrying the fit's residual noise into a term the search depends on at every leaf.

## consumer enumeration

Every consumer of the weight magnitudes, mate band, and ordering interlocks, from grep over internal/ and playground/. Required action per entry for any swap.

| consumer | role | swap effect | action |
| --- | --- | --- | --- |
| internal/engine/eval.go:36 | default weights source | the swap itself | new literals in one commit |
| internal/engine/eval.go:69 | score = own sum minus opp sum plus tempo | class swap changes every score, tempo term unchanged unless tempo refits | re-run eval tests |
| internal/engine/movegen.go:40-46 | minStatic = PatternWeightFour when the opponent holds a live four | behavior-affecting: the threshold is relative, under fitted weights sub-Four cell sums can cross Four, 483 + 137 = 620 > 484 | re-derive that the threshold still admits a superset of the forced set and stays above every quiet-only cell maximum, pin the inequality beside config_test.go:517, forced_test.go:72-73 must still hold |
| internal/engine/movegen.go:65-73 | static = own plus opp per-cell weighted sums, feeds ordering | magnitudes rescale | none beyond the bound below |
| internal/config/config_test.go:514-517 | ordering bound, PatternWeightOpenFour * 2 * PatternDirections + SearchHistoryMax < SearchOrderKiller2 | fitted: 1561 * 8 + 1048576 = 1061064 < 536870911, holds | verify once on the final integers |
| internal/config/config_test.go:488-495 | strict ascent up the forcing order, class 0 is 0 | fitted 137 < 162 < 483 < 484 < 1561 holds | verify once |
| internal/config/config_test.go:497-498 | mate band floor above max window sum | fitted: 4 * 256 * 1561 = 1598464 against a mate floor near 1.07e9, holds | verify once |
| internal/pattern/table_test.go:171-176 | weights length equals class count, nondecreasing | holds | verify once |
| internal/config/config_test.go:485-486 | EvalTempo pinned to EvalMilliUnit | binds only under a tempo refit | rewrite the pin only as part of the tempo decision |
| internal/engine/eval_test.go:78,164-167, internal/engine/evalweights_test.go:75-84,137-142, internal/server/evalweights_test.go:37-60 | tempo-anchored expectations | change only under a tempo refit | update with the tempo commit |
| internal/server/match.go:527-529 | seat construction carries t.EvalWeights into NewTieredEval or NewEval | mechanism unchanged | none |
| internal/config/config.go:68 | per-seat EvalWeights override | arm mechanism, unchanged | none |
| internal/engine/search.go:59,220,259,335, internal/server/match.go:512, internal/server/stats.go:53,148-166 | mate band and milliunit rendering, EvalMateMax, EvalMateScoreStep, EvalMilliUnit | weight-independent | none, listed to close the set |
| playground/arena/main.go:34,62,68,125-126 | -evalweights and -evalbase flags register a derived seat | seam already shipped, zero new code | none |
| db/book.carobook | majority lines recorded under scaffold weights, solver proofs ruleset-true | majority quality is weight-conditioned | re-gate book hold, see below |

## arena arms and decision rules

Seats: master tier with the book enabled on both seats, the shipped shape. The fitted seat registers through -evalweights with the fitted integers over -evalbase master. Pairs: fitted versus scaffold at 2+1 and 3+2, 20 games per pair, red alternating. Floors first: master,master under each weight set, 5 dirs of 20 games per control, scored as the color-pair-pooled decisive fraction with the Wald 95 percent interval per the v0.24 regrow law, the interval must cover 0.5 or the pool extends by another 5-dir block before anything else runs, at most 2 extensions, after which the floor is recorded unmeasurable and this protocol pauses for re-registration instead of extending again.

Rules fixed now. Adopt only if the fitted pair's decisive fraction lower bound clears its floor band upper bound, or the pair ties inside noise (intervals overlap) while the fit validation loss strictly beats the scaffold baseline on the same holdout. A loss outside noise rejects the fitted set. Every non-adopting outcome, including a tie inside noise without the loss improvement, consumes the single iterate. Exactly one iterate is allowed, more data then refit, before the scaffold stands. The tie-break is the pooled decisive fraction itself, higher wins a tie inside noise when the loss condition also holds. The book hold rides the re-gate: master book versus master-nobook at 2+1 and 3+2, 20 games each, must stay inside its floor band, a book failure under new weights opens a book regrow decision and blocks adoption until resolved.

## adoption mechanics and rollback

One commit swaps the literals at internal/config/config.go:163-167 and :266 with the tempo decision applied, every consumer row above gets its listed verification, and the full tier re-gate ladder runs before any tag: all 4 tiers at all 4 time controls, zero inversions required at 2+1, 3+2, and 10+5, 1+0 recorded as thread-decided, because the default weights are global across every tier. Rollback: any inversion or book-hold failure reverts the literals to the scaffold values in one revert commit and the finding is recorded here. Strength gates already banked against scaffold weights, v0.28 rungs included, re-run against the final weights.

## outcome

| item | value | verdict |
| --- | --- | --- |
| refit artifact | | |
| integer drift | | |
| tempo decision | | |
| validation loss | | |
| floor fitted 2+1 | | |
| floor fitted 3+2 | | |
| floor scaffold 2+1 | | |
| floor scaffold 3+2 | | |
| pair 2+1 | | |
| pair 3+2 | | |
| book hold | | |
| decision | | |
| re-gate ladder | | |
