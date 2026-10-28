# Solver audit design

Design against the closed square collapse investigation, playground/arena/out/square-collapse-investigation.md. The failure signature: square master out-searches every opponent yet loses through one-ply mate inversions, 14 of 31 hard 3+2 losses sat on a mate-for score refuted by a single VCF-tagged reply, 0 of 94 losses ever showed mate-on-self first. Two code truths carry the blame: the fail-only solver TT is depth-blind, and no code path ever asks whether the opponent holds a forced refutation before the engine commits to a mate-for move.

## Part A, the depth-blind fail-only TT

The defect, verified in tree. internal/vcf/solver.go:59-60 holds ttKeys and ttVerdicts per Solver instance. ttStore at solver.go:130-137 writes only fail verdicts, stamped with nothing. ttProbe at solver.go:122-128 returns a stored fail as authoritative with no depth knowledge. The same Solver instance serves the whole game: newBotSearcherFor at internal/server/match.go:557-565 creates one vcf and one vct solver per bot, reused by every Search attack pass at match.go:453 and by every StartPonder pass at match.go:495. A fail stored under a shallow plyCap round survives into every deeper probe, within one Solve's iterative rounds (solver.go:81-87 raises plyCap from 1 by 2) and across solves for the whole game. A node's true fail meaning is relative: attack at ply p under cap d proved no win within d-p plies, so reuse is sound only when the stored remaining depth covers the probe's remaining depth. The current code stores no depth at all.

Chosen fix: stamp every fail with the remaining depth it was proven under, and count a probe as a hit only when the stamp covers the probe's remaining depth.

1. Replace ttVerdicts with ttStamps [1<<SolverTTBits]uint16. Zero means empty, a nonzero value is a fail proven at that remaining depth. This drops the verdict array (saves 256 KiB per solver) and removes the present confusion where the stored fail byte 2 numerically equals resAbort.
2. Signatures take the remaining depth: ttProbe(hash uint64, remaining int) bool, hit iff ttStamps[slot] != 0 and ttKeys[slot] == hash and int(ttStamps[slot]) >= remaining. ttStoreFail(hash uint64, remaining int) writes the stamp. The win-store calls at search.go:24, search.go:68, search.go:90 are dead code today (the guard at solver.go:131 rejects non-fail results), delete them in the same commit.
3. Call sites: the probe at search.go:17 passes s.plyCap - ply. The fail stores at search.go:28, search.go:33, search.go:75, search.go:97 pass the same expression at their ply.
4. Cross-solve persistence stays: a deep fail remains valid for shallower probes by monotonicity, and the deeper-fail-covers-shallower-probe rule keeps the sound direction of reuse. Master's warm instances stop poisoning deep probes.

Tests, TDD order:
1. Red first: TestSolverStampBlocksStaleFail builds a position with a forced win beyond ply 3 but none within it, solves once with maxPly forced to 3 (fails, stamps), then raises maxPly and solves again on the same instance, the win must be found. This test fails on current HEAD.
2. Unit table over ttProbe and ttStoreFail: empty slot, hash mismatch, stamp below remaining, stamp at and above remaining.
3. Property in the oracle_test.go fuzz family: fresh-instance verdict equals stale-instance verdict across a cap ladder, seeded random boards, both geometries.
4. Mutation over internal/vcf with killers for the stamp comparison (hit with stamp below remaining must die), stamp zeroing, and the store-site remaining computation.

Bench, the evidence the original deferral demanded: playground/solverpass before and after on its openings corpus plus a long forced-line position, node counts and proved flags recorded under playground/solverpass/out/. Proved flags must not regress to false. Node counts may rise where unsound skips disappear, that rise is the fix working.

## Part B, the fresh refutation audit

Trigger: solverSearcher.Search at internal/server/match.go:461 returns the inner search's move with a mate-for score and no solver tag. Solver proofs at match.go:453 are true forced wins and need no audit. The audit fires when the tag is empty and st.Score >= config.EvalMateFloor.

New hub constant: EvalMateFloor = EvalMateMax - SearchMaxPly * EvalMateScoreStep in internal/config/config.go beside the mate family. internal/engine/tt.go:23 and internal/engine/stats.go:112 switch to it, one source.

Audit content: copy the board, apply the candidate, then run fresh cold solvers over the copy. After the mover's move the side to move is the opponent, so vcf.Solve on the copy asks exactly the opponent's question. The audit instances are constructed per fire, vcf.New(KindVCF) and vcf.New(KindVCT), never s.vcf or s.vct, so no table warmth crosses from the attack path, the part A class of bug cannot re-enter through the audit.

Verdict: the mover's claimed mate distance is myMatePly = (EvalMateMax - st.Score) / EvalMateScoreStep, the opponent proof length is out.Plies, the mover is on move, so the opponent kills first iff out.Plies < myMatePly. A decisive unit test with small constructed positions pins this parity table before any wiring, both directions plus the tie case where the mover mates first.

On a refutation the candidate is demoted: one re-search with the candidate excluded, bounded by the remaining deadline, then return the new move with its own honest stats and an empty tag. The M-line format stays frozen, no new tag, the log simply shows a normal search line. One exclusion, never a loop.

Exclusion seam: engine gains SearchExcluded(b, dl, exclude rules.Move) (rules.MoveNone means no exclusion) as the sibling of SearchDepth at internal/engine/search.go:92 and internal/engine/smp.go:245, root move generation in both engines filters the excluded cell. The tierSearcher and singleSearcher adapters in internal/server/match.go pass it through.

Budget: RefAuditNodeBudget in the config hub, provisional value SolverNodeBudget, the same family the attack passes use at match.go:453. Time: the audit runs inside the remaining grant under the same SolverBudgetShare shaping and is skipped when the grant is under SolverMinGrantMs, matching the existing guard at match.go:441.

Hooks:
1. solverSearcher.Search after match.go:461, synchronous, no new goroutine, the existing deadline bounds it.
2. The ponder adoption gate in internal/server/ponder.go, the clause block at ponder.go:59-69: an adopted move whose producing stats claim the mate zone runs the same audit before adoption, a refutation discards the adoption and falls through to the existing wake-and-search path. This subsumes the ponder-after-flip follow-up: the flip correlation gets its check at the exact point of failure, no new gate constant ships, per the law that constants wait for measured rates. If perturbation data later shows adoption-side misses beyond the audit's reach, a dedicated clause returns with evidence.

Cost, from the settled dataset: fires only on mate-for claims by the search. The at-flip counts per arm run 6 of 19 to 14 of 31 losses, plus confirmed-win mates that pass the audit. Each fire costs one cold VCF plus VCT pass, the same work every solver-bearing opponent already pays every move inside its share, so the mover adds at most one opponent-move-class cost on a subset of its own moves. The grant guard bounds the low-clock case.

Tier policy: the audit lives in solverSearcher, so medium, hard, master and grandmaster get it. Easy keeps its shape: it has no solver stack and no tier below it runs one, its expected place is the ladder bottom. One code path, additive capability only, no tier ever runs below its shape, the no-handicap law holds.

Geometry: the audit copies the board and runs the shared solver, the geometry flows from b.Geom, hex and square take the same path, internal/vcf/hex_test.go is the precedent and the arena re-runs cover both.

Tests: the parity table test first, then solverSearcher tests with a fake inner claiming mate-for over a refutable line, the audit must demote and re-search, plus the confirmed-win case passing through unchanged, plus the grant-guard skip. Race coverage through the existing solverSearcher suite, the audit is synchronous. Mutation over internal/server solverSearcher label with killers on the trigger comparison, the parity comparison, and the exclusion filter.

## Part C, the ponder-after-flip clause

Folded into part B as described above: the audit at the adoption point is the measured fix, no constant ships without the perturbation experiment. Recorded as closed-by-subsumption.

## Part D, the verdict prefix rider

playground/arena/hexgates.go:130-134 hardcodes the hex-passgates prefix in both return strings while the gate already knows env.variant at hexgates.go:100 and :145. Thread the variant into passVerdictLine and print variant-passgates in both strings. scripts/passgates-fill.ps1 and scripts/hex-passgates.ps1 generalize their Match regexes from the fixed hex-passgates prefix to the (hex|square)-passgates family. The forensic parser already accepts the family prefix generically, verified during the investigation.

## Evidence and serialization

Both parts are strength-bearing, the inversion research law governs, no tier constant moves anywhere in this design.

1. Part A lands: tests, mutation, solverpass bench committed.
2. Arena batch 1, one frozen binary, detached: re-run square hard 2+1 and 3+2 passgate legs plus hex easy 2+1, the mechanism legs, Wald extend, same driver.
3. Part B lands on top of A: tests, mutation, race.
4. Arena batch 2, fresh freeze: square medium 2+1 and 3+2, square easy 3+2, and any batch 1 leg whose shape B touches.
5. The full 12-leg re-run decides, its table is the v0.24 close evidence.

One engine-timing batch on the machine at a time, each batch freezes its own HEAD, the frozen-binary law per batch.

## Files the implementer touches

internal/vcf/solver.go and internal/vcf/search.go plus their tests, internal/config/config.go and config_test.go for EvalMateFloor and RefAuditNodeBudget, internal/engine/search.go, smp.go, stats.go, tt.go and root tests for the exclusion seam and the floor switch, internal/server/match.go, ponder.go, solversearcher_test.go, ponder_test.go for the audit wiring, playground/arena/hexgates.go and the two gate scripts for part D, playground/solverpass for the bench artifact. Zero comments in every touched file, full de-comment on touch per the repo law.
