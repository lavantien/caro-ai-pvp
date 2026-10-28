# booktree thin build diagnosis

Input evidence, all local and gitignored: db/book-tree.txt (29 lines, the full expansion log), db/book-stats.txt, db/book.carobook (1706 bytes), playground/bookgen/out/tree-run.log, playground/arena/out/booktree-chain.poll.log. The run started 2026-10-28 11:41:49 and exited 0 at 11:45:39, 230 s wall clock, 301,990,272 search nodes, so the single-thread analyst rate is 1.31 Mnps. Every number below recomputes from those files.

## What ran

The chain script calls booktree-run.ps1 with its defaults, so -tier master and no -reverify. The sidecar header confirms: tier=master ply=8, total=268435456, critical=8388608, leaf=1048576, solver=16777216. The run was a master-bound build, not the grandmaster standard the book program now carries.

The log holds 6 expansions, then stop budget spent with frontier 4 (2 held):

| node | ply | side | children | analyses | score path |
|---|---|---|---|---|---|
| root | 0 | own | 1 (cell 119) | 8388608 | -1000 |
| after 119 | 1 | opp | 3 (121, 104, 119) | 8388608, 16777216, 33554432 | 200, 200, 1000 |
| after 121 | 2 | own | 1 (133) | same ladder | -300 x 3 |
| after 104 | 2 | own | 1 (101) | same ladder | 0 x 3 |
| after 119 | 2 | own | 1 (88) | same ladder | -300 x 3 |
| after 133 | 3 | opp | 2 (134, 135) | same ladder | 1000 x 3 |

Cumulative spend recomputes to 301,990,272 exactly, matching the stats artifact. Emission took the 4 red-side analyzed nodes (root plus the 3 ply-2 replies) and un-canonicalized them into 21 variant entries, 0 solver proofs, 0 frozen. The artifact is a spine fragment: one opening, 3 plies deep.

## Root causes, ordered by weight

1. Budget arithmetic. A non-frozen frontier node costs the geometric stability ladder base + 2x base + 4x base = 7 x base. Every node in this run took the critical base 2^23, so each node cost 58,720,256 nodes and the 2^28 total budget bought 4.57 nodes. The run processed 6 and overshot the cap by exactly 2^25 because nothing checks the budget inside a ladder. The routine class 2^20 never appears: own-node expansions carry exactly one move so bestGap stays structurally 0, opp-node opening gaps sit under critmargin 300, and critical marks every child. The criticality class as implemented cannot separate nodes, so BookTreeTotalNodes and BookTreeCriticalNodes are mutually inconsistent by about 500x for even the master spine.
2. Root seeding defect. The prereg correction states the root expands all 12 protocol openings. It cannot: engine movegen returns exactly one move, the geometry center, on an empty board (internal/engine/movegen.go:11), so the root ranking holds 1 move and the openSet filter in playground/bookgen/tree.go keeps only cell 119. The logged root expansion has 1 child. The tree tests at playground/bookgen/tree_test.go:108 and :403 feed fake analysts whose rankings already contain the 12 cells, so they certify the filter, never the engine contract. Red test for the fix lane: a root expansion driven by the real engineAnalyst must seed all 4 canonical opening orbits.
3. The stability ladder cannot fire in the opening. Freezing needs the latest score magnitude at or above 3000 milliunits; every observed opening score sits inside 1000. So 4 of every 7 nodes spent per node prove a non-freeze. A ladder gated on plausibility, or a per-node cost of 1x base with the ladder only when a freeze is reachable, cuts the per-node cost 7x with no opening-behavior change.
4. Reverify never ran. The prereg protocol includes the 8-thread reverify pass over contested owner nodes; -reverify defaults false and the chain does not pass it.
5. Driver gap for the regrow. The tree log is fingerprint-locked to tier and ply bound, so a grandmaster run needs a fresh -tree-log path, and booktree-run.ps1 does not expose that flag. The fix lane adds the pass-through or the gm run dies on the fingerprint check against db/book-tree.txt.

## Coverage math at the grandmaster standard

Assumption chain: analyst rate 1.31 Mnps measured above; own nodes expand 1 move, opp nodes expand top-k 4 replies that canonical dedup collapses to about 3, matching the logged 3 and 2; opening scores stay under the freeze thresholds so ladders run full cost; solver passes stay near the observed 64 nodes on quiet opening positions.

Spine book, the natural frontier-held stop with the root seeding fixed: 4 openings x 15 ladders to the ply-16 bound plus 1x base bound freezes is about 3.6G nodes, 46 minutes. It yields roughly 32 owner positions and 100 to 200 entries covering one line per opening. That proves the machinery at the grandmaster bound, nothing more: any opponent deviation leaves the book on move 2.

Real coverage, every opp reply developed toward the bound: the explored set grows about 3x per 2 plies, so full-k to ply 16 is about 3^8 = 6561 ladders, 385G nodes, 82 hours at 1.31 Mnps. With the ladder trimmed to 1x base plus conditional re-analysis, the same tree costs about 55G nodes, 12 hours. Intermediate shapes price linearly in between: full-k to ply 12 then spines is about 43G with the ladder as shipped, 9 hours.

## Proposed constants and night plan

Fix order first, then money. The root seeding defect and the driver -tree-log pass-through are prerequisites, and the ladder gating decides the budget tier, so the fix lane lands those before any night fires.

| plan | BookTreeTotalNodes | wall clock at 1.31 Mnps | nights | shape |
|---|---|---|---|---|
| validation | 2^34 | 3.6 h | 1 | gm spine book, frontier-held stop, proves the 16-ply machinery |
| coverage, ladder trimmed | 2^36 | 15 h | 2 | full-k to about ply 15, resumable across nights |
| coverage, ladder as shipped | 2^38 | 58 h | 6 | full-k to ply 16 with the 7x ladder |

The builder resumes from its append-only log and the budget spans nights, so the total is raised once to the plan's target and each night continues the same tree. Recommended: fix lane first, then the 2^36 trimmed-ladder plan as the funded default, escalation to 2^38 only if the trimmed ladder loses freezes the arena arms later miss. Tier flag grandmaster on every run, -reverify on the terminal night, emission gated on the prereg's validation chain (bookval short, confirm, cross) before any gate night.

Emission today also writes visits 1 on every tree entry because every own gap is 0 and the visit weight floors at 1. Real visits per the grandmaster standard need the weight keyed to measured score spread at the owner node, which rides the same fix lane as the gap semantics.

## Defect list for the fix lane

1. Root seeding: real engine ranking cannot seed the 12 protocol openings, movegen center-only on the empty board. Red test: engineAnalyst-driven root expansion seeds 4 canonical orbits.
2. Criticality class dead: own-node bestGap is structurally 0, so every node pays the critical base. Red test: an own node with a clear ranking spread produces routine-class children.
3. Budget not checked inside the stability ladder, overshoot 2^25 observed. Red test: a ladder starting above the remaining budget stops at the cap.
4. booktree-run.ps1 lacks a -tree-log pass-through, so the gm run cannot take a fresh log.
5. Reverify opt-in only, the chain never passes it.
