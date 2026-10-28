# Anti blindness rnd, topic map and preregistration

Campaign order 2026-10-28: measure blindness surfaces of the engine under the philosophy that the engine never operates via degradation. Every candidate measure is additive capability, more probing, more verification, more table. Width cuts, reduced parallelism, pruned vision are rejected by law and only recorded as rejected.

Source failure, playground/arena/out/square-collapse-investigation.md: square master loses to solver-bearing lower tiers through one-ply mate inversions while out-searching them 43 to 120 percent on nodes. 14 of 31 hard 3+2 losses sat on a mate-for score refuted by one VCF-tagged reply, 0 of 94 losses ever showed mate-on-self first.

## Surfaces

S1 solver TT staleness. internal/vcf/solver.go ttKeys and verdict array, one Solver instance per bot for the whole game at internal/server/match.go:557-565, reused by attack passes at match.go:453 and ponder passes at match.go:495. A fail stored under a shallow plyCap round stays authoritative for deeper probes. The stamp fix is landing in the part A lane, this campaign measures around it and records the internal/vcf commit it runs against.

S2 root mate-for halt. internal/engine/smp.go runWorker halts the root loop the moment score reaches EvalMateMax minus one step, no opponent refutation is ever asked at that point. The search commits to the first mate-for iteration it completes. Measurement: replay search depth sweep at flip positions, record the depth at which the mate-against truth becomes visible or stays invisible.

S3 shared TT replacement and generation semantics. internal/engine/tt.go store keeps a same generation entry only when deeper, an old generation entry is always replaced, direct mapped, one slot per index, lockless XOR signature. Every new search or ponder bumps the generation at smp.go setupJob, so the shallow depth 1 and 2 writes of each new search can evict deep sound entries of the previous generation at hot slots. Entries stay sound across searches, stones only accumulate. Measurement: hit rate trajectories across search sequences from the public stats, plus the code reading itself.

S4 parallel width thrash. All workers of an instance share the one table at smp.go newSMPPonder. Concurrent writers at equal depth from different subtrees alternate on colliding slots, the same generation depth rule only protects deeper entries. Measurement: TTHitPermille and detection at fixed table across worker counts 1, 2, 4, 6.

S5 ponder warmth interplay. Ponder runs ponderWidth workers on the same table with its own generation bump, its entries become old generation warmth for the next search, hit able until evicted by S3. Ponder also runs solver passes on the shared solver instance, feeding S1. Measurement: cold versus sequentially warmed replay sequences.

S6 TT scale and warmth. Search table scales by tier, 0 to 768 MiB in the demand spec. The solver table does not, SolverTTBits 18 is 262144 slots on every tier, so tier scale blindness through the solver is a warmth effect, more solves per game leave more stale fails, not a size effect. Measurement: table size sweep and instance warmth sweep as separate axes.

S7 geometry. Square carries 4 line axes and short forced kills, hex carries 3 and a 0.594 draw rate at the board cap. The 7 hex easy losses flip late. Measurement: the same probes on both geometries, square flip set and hex loss set.

## Preregistered questions

Q1 From the settled dataset, playground/gateforensic/out/moves.parquet: do master moves at flip positions carry different tt_pct, hf_pct, depth, or ponder tags than matched non flip master moves, per arm and seat.

Q2 At recorded flip positions, does a replayed engine still score mate-for, and how does that blind share move with search TT size (0, 32 MiB, 128 MiB, 768 MiB) and with instance warmth, cold per position versus carried across a whole game.

Q3 Does worker count alone at fixed table (1, 2, 4, 6) change the blind share or the hit rate at the same positions.

Q4 From code, the exact replacement and generation semantics of the shared TT, stated with line anchors, plus where a deep forced line entry can die to shallow traffic, plus the solver warmth model.

Q5 Which additive measures remove each blindness class, ranked by measured effect from this campaign, with cost.

## Instruments

Dataset folds through duckdb in python, extract_flips.py then mine_q1.py, outputs as csv under out/. Replay probes as a nested go module importing internal engine, rules, vcf, config, bounded budgets, no arena. Findings memo at findings.md.
