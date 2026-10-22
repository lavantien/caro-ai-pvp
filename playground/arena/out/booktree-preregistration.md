# book tree expansion pre-registration

Fixed before any build run, per the gate law. User directive 2026-10-22: self-play corpus games at 10+5 are rejected as the book generation mechanism and the bookgen fleet is stopped. This protocol governs the replacement: a best-first tree expansion over the opening space with fixed node-count analysis, backed up by negamax, validated by arena pairs afterward.

## premise

Why corpus self-play was wrong as the generator: one full game yields one line of which only the first plies are book-relevant, a 10+5 game costs about 20 minutes of wall clock, deterministic seats repeat lines unless randomness injects weak moves, the win or loss label is a noisy signal dominated by the first-player effect, the clock spends the same budget on trivial and critical positions, and both seats share the same blind spots so errors bake into the book. The tree builder instead spends fixed node budgets exactly where the opening contest is unresolved. The corpus mechanism closed at 284 recorded games (legacy 110, inst2 b55 and d55, inst3 c55 and d9), retired by this protocol.

## ruleset symmetry law

The ruleset is invariant under the full 8-element dihedral group of the 16x16 board: identity, rotations 90, 180, 270, horizontal, vertical, and both diagonal reflections. Each transform maps the 4 line directions onto line directions, preserves 5-in-line continuity, overline nullification, and both-ends close-block semantics window-locally, and preserves the red second-move constraint because Chebyshev distance is an isometry invariant (internal/rules/legality.go:9, :47). Stones keep their color under a transform, so a transform maps a legal position to a legal position with the same side to move. On the cell encoding cell = row * 16 + col the 8 transforms are the standard (r, c) relabelings, for example rot90 is (r, c) to (c, 15 - r). The 12 forced opening cells BookOpeningCells (internal/config/config.go:218) fall into exactly 3 symmetry orbits: the center 2x2 {119, 120, 135, 136}, the diagonal ring {102, 105, 150, 153}, and the orthogonal ring {116, 123, 87, 167}, each of size 4.

## analysis space and the artifact decision

The builder analyzes in canonical space: a node's identity is the exact 2x4-word stone-mask key minimal over its 8 transforms, so transpositions and symmetric duplicates merge into one node and the structure is a DAG, as the directive requires. The artifact stays CAROBOOK1 version 1 with exact uncanonicalized keys (internal/config/config.go:187-206): emission un-canonicalizes by writing, for every stored line, one entry per root-opening variant it covers. The 3 canonical openings each have orbit size 4, so emission multiplies entries by 4.

Price of the two options, verified: canonical keys in the artifact would need probe-side canonicalization in internal/book/book.go:183-208 Probe and its callers plus a format version bump, and would put an 8-transform key minimization on the book-hit hot path that is zero-alloc today. Exact keys with 4x emission cost nothing at run time and one build-time transform pass, bounded by the entry count: at 100k canonical entries the artifact is 100k times 4 times 75 bytes (config.go:190-195) = 30 MiB against 7.5 MiB canonical, both far under any binding budget, and the realistic 8-to-16-ply opening tree is an order smaller. Exact keys win: zero engine consumers change, the loader contract (strict ascending, occupied-move rejection, source domain) is untouched, no BookSidecarVersion or format bump. Consumers that change are builder-side only.

## seeding law

Root is the empty board. Level 1 expands the 3 canonical opening orbits, each represented by its minimal cell. Every deeper node below the tier ply bound (master BookMaxPly 8, grandmaster 16, internal/config/config.go:75-76) is expandable. Red's second move honors the Chebyshev constraint by construction because all candidate moves come from the engine or movegen, both of which enforce legality.

## expansion policy

Best-first over the frontier, priority order: nodes on the current backed-up principal variation first, then backed-up score closeness to 0 (contested positions), then shallower ply. At a node where the mover is the side the book advises (the book owner), expand the engine's best move only. At an opponent node, expand the top BookTreeOpponentK = 4 replies by root score. Node budgets by criticality class: BookTreeLeafNodes = 1 << 20 for routine leaves, BookTreeCriticalNodes = 1 << 23 for leaves whose parent margin is inside BookTreeCriticalMarginMilli = 300, BookTreeReverifyNodes = 1 << 24 for the root reverify pass. All constants land in the config hub, never literals in the builder.

## reproducibility law

Every leaf analysis runs single-thread (engine.New with an explicit TT size) under a fixed node budget, never wall clock, so a leaf verdict is a pure function of position, budget, weights, and engine version, independent of machine load. Each leaf gets a fresh engine instance so no TT state crosses leaves and the artifact is bit-reproducible regardless of scheduling. Tie-breaks are deterministic: moves and nodes order by exact key then cell index. The 8-thread reverify pass is not bit-reproducible: lazy SMP workers share one TT (internal/engine/smp.go:99-101) so worker interleaving shifts the tree, disclosed here, its outputs are recorded deltas and can never silently mutate the artifact, only flag or confirm.

## node budget seam

No node-count deadline exists today: internal/engine/deadline.go provides only wall-clock FixedBudget (:17-39) and Stoppable (:41-51), while the search checks dl.Exceeded() every SearchNodeCheckInterval = 2048 nodes (internal/engine/search.go:304-307, internal/config/config.go:173). The seam: deadline.go gains type NodeBudget with an atomic count and an Add method implementing Deadline by count against limit, and the search loop feeds the interval batch through a single type assertion per search. The VCF and VCT solvers are already node-count native (playground/bookgen/overlay.go:32-45 passes budgets directly into vcf.Solve). This is strength-neutral budget plumbing: the search semantics are unchanged, only the stop source differs, gated by unit tests (exact-budget stop, no stop under budget, composition with Stop, zero allocs), no arena round, same as the Stoppable precedent.

## root ranking seam

Top-k opponent replies need a ranked root list. SearchStats carries the RootMoves and RootScores iteration ring (internal/engine/stats.go:25-27, filled at internal/engine/search.go:227-230), which records the best move per completed depth, not a ranking. The seam: expose the existing final-iteration per-root-move scores as a ranked top-k surface on the engine, plumbing only, the root loop already scores every root move. Single-thread, fresh-TT, fixed-node, so the ranking is deterministic.

## backup law

Negamax over the DAG: a node's score is the max over children of minus the child score, draws score 0, mate values ride the engine band (EvalMateMax minus ply times EvalMateScoreStep, internal/config/config.go:158-159) so a solver-proven win backs up as a mate score with distance, consistent with the engine's own band consumers. Backed-up scores drive expansion priority and the pruning margin. A node whose every child is frozen (solver-proven or stable) is itself frozen with its backed-up value.

## stopping laws

A leaf freezes when VCF or VCT proves it (budget BookSolverNodeBudget = 1 << 24, the overlay prober pattern at playground/bookgen/overlay.go:32-45), in either direction: a proven win freezes the subtree and emits its proof move with source BookSourceSolver = 2, a proven loss freezes the line so the parent stops honoring it. A quiet leaf freezes when stable and lopsided: BookTreeStableIters = 3 re-analyses at budgets 2^k, 2^(k+1), 2^(k+2) whose scores agree within BookTreeStableMarginMilli = 150 and whose latest magnitude is at least BookTreeLopsidedMilli = 3000. Everything else stays on the frontier until the frontier is empty or the global node budget BookTreeTotalNodes is spent.

## pruning and weighting

Emission keeps at a book-owner node every move within BookTreePruneMarginMilli = 300 of the backed-up best, weighted by softmin over the score gaps, so variety survives without weak moves: the corpus round's one-move-majority problem (74.8 percent single-move at depth 2 and up) is exactly what the margin fixes. Opponent nodes emit nothing, the book advises one side only.

## solver overlay and emit

The existing overlay solver pass and emit phase (playground/bookgen/overlay.go, emit.go) are reused: the tree builder emits its intermediate as the same key-move-visit-source stream the emit phase consumes, visits derived from the weighting (source 1 entries) with solver proofs overriding as source 2 per the standing precedence law. The corpus, merge, and stats phases retire for generation, corpus files become priors below.

## corpus disposition

The 284 corpus games become expansion priors only: their position-move pairs seed the top-k reply ranking as tie-break bonuses (real master-play first), never as verdicts, since the files carry no outcomes (playground/bookgen/corpus.go:396-400). After the build, the files archive in place as history. The 1500-game target and its fingerprint clause are void.

## validation plan

Floors first, unchanged from the v0.24 regrow law: master-nobook pools at 2+1 and 3+2, 5 dirs of 20 games per control, color-pair-pooled decisive fraction with the Wald 95 percent interval covering 0.5, capped extensions. Arms: book versus no-book at 1+0 and 2+1 for volume, 20 games per pair per control, then the 10+5 confirmation pair, 20 games. A book seat failing its floor band blocks the artifact, the builder is at fault. Cross-check arm: the 100 highest-weight lines re-analyzed under perturbed settings, half node budget, single-thread versus SMP, and the -evalweights perturbed seat (playground/arena/main.go:125-126), any line whose best move flips beyond the prune margin is dropped from the artifact and recorded, blind-spot detection rather than blind-spot shipping.

## outcome

| item | value | verdict |
| --- | --- | --- |
| canonical entries | | |
| emitted entries | | |
| solver proofs | | |
| stable frozen | | |
| frontier at stop | | |
| total nodes | | |
| build wall clock | | |
| cross-check flips | | |
| floor 2+1 | | |
| floor 3+2 | | |
| pair 1+0 | | |
| pair 2+1 | | |
| pair 10+5 | | |
| decision | | |

## corrections 2026-10-23

The adversarial pair over the landed builder disproved four body claims, corrected here without rewriting the body above. The symmetry law is wrong as stated: the 12 BookOpeningCells span 4 orbits, not 3 of size 4, since 116's orbit has 8 members mostly outside the list and 87 is not isometric to 116 (pinned by TestOpeningCellsTrueOrbitStructure), so the root expands all 12 protocol openings and canonical keys merge orbit mates downstream. The emission multiplier is 4 to 8, not a flat 4: single-stone openings reduce to 4 variants but deeper positions emit up to 8 distinct variant keys, still non-binding on artifact size. The weighted multi-move emission paragraph is superseded: CAROBOOK1 stores exactly one move per strictly ascending key, so each owner node emits its best move with the score gap driving a confidence-scaled visit weight (BookTreeVisitScale, BookTreeVisitFloor) and contested nodes feed the reverify pass, a true multi-move variety needs a format bump. The ranking premise is wrong as stated: the root loop runs PVS, so only move 0 carries an exact full-window score and tail moves carry fail-soft null-window bounds, consumers must enable SetExactRootRanking (2bd6e50) which searches every root move full-window under the engine's own extension law, and the ranking is exact only in that mode. The stability ladder is geometric: budgets base, 2x base, 4x base, the arithmetic base, 2x, 3x shape was a build defect. The solver stop is mover-direction only: proofs freeze wins for the side to move, losses arise through cascade backup with mate distances stepped by one EvalMateScoreStep per ply, there is no loss-direction probe. Reproducibility rests entirely on the fresh-engine-per-leaf discipline: resetForSearch clears history, killers, and the evaluator but not the TT, so any TT reuse between leaves silently breaks bit-reproducibility. Line citations refreshed: BookOpeningCells at internal/config/config.go:233, shared TT construction at internal/engine/smp.go:105-107, node check at internal/engine/search.go:355-368. Two constants landed as provisional owner decisions unvalued in the body: BookTreeTotalNodes = 1<<28 and BookTreeVisitScale = 1000, plus BookTreeVisitFloor = 1. The artifact meta field CorpusGames carries the prior-games count under its format-v1 fixed name.
