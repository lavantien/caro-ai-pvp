# search: algorithmic prototypes against the shipped engine

Four experiments over 212 master self-play games and 35 tactical positions. Trust anchor: the lane's reimplementation of internal/vcf reproduces the shipped solver exactly (identical verdicts and node counts, 11298/11298 and 22871/22871 across two corpus states), so the A/B numbers compare against a verified mirror. Per-position artifacts under out/, benchmarks in bench_test.go (in-loop timing proved eliminable by the optimizer, so measurements live in testing.B).

## verdicts

1. wavefront threat-distance ordering for VCF: drop. Ordering-only variants move nodes -0.1 to +4.3 percent with no verdict change. Pruning attacker candidates by threat distance saves 19-26 percent of nodes but misses 6-14 percent of wins. The sound two-phase (prune, then full fallback) costs 2.3x because most VCF probes end not-found and pay the fallback. The wavefront build alone is 13.5 us, 1-9x a whole solve. If revisited, target the VCT move set where threes widen the candidates, before concluding.

2. spatial-hash incremental candidates (broad-phase): drop. Parity exact over 3376 corpus moves (0 mismatches, 0 key failures), but from-scratch 4-word dilation is 45 ns versus 118 ns for count-based incremental make+unmake. The shipped movegen is already the cheaper structure at 16x16.

3. convolution screen versus table extraction: drop. A rolling dual-sum convolution over own/free planes is an exact superset on all positions (20/20 recall, 0 false positives) and 1.6x faster than a full-board table scan (4.4 versus 6.9 us), but nothing consumes full-board re-extraction: the shipped evaluator is incremental per move and the solver prefilters with bitboard dirReach at lower cost than any convolution.

4. UCT core: prototype kept, not adopted. 2266 playouts/s under fleet load, tree to depth 3, exact board restore and legality asserted per playout in tests. Roughly 3 orders of magnitude under the alpha-beta node rate, consistent with classical gomoku results without a learned leaf evaluator. The core stands ready as the search shell if a batched evaluator ever appears (the nneval lane's architecture-class condition). Strength testing queued post-fleet.

All wall-clock numbers are coarse (the bookgen fleet owned 24 of 32 cores). Precise timing reruns queue post-fleet.
