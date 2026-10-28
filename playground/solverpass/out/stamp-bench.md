# Solver TT stamp bench

Before: unfixed depth-blind fail-only TT, commit f333747 tree.
After: remaining-depth stamped fails, same tree plus the stamp commit.
Command: go run . (5 openings) plus the forced-line position F9 E9 G9 A1 H9 P16 I7 B15 I8 O2.

| position | kind | before nodes | after nodes | before proved | after proved |
|---|---|---|---|---|---|
| H8 | VCF | 32 | 32 | false | false |
| H8 | VCT | 32 | 32 | false | false |
| H8 H9 | VCF | 32 | 32 | false | false |
| H8 H9 | VCT | 32 | 32 | false | false |
| H8 I9 | VCF | 32 | 32 | false | false |
| H8 I9 | VCT | 32 | 32 | false | false |
| 6 stones | VCF | 32 | 32 | false | false |
| 6 stones | VCT | 32 | 32 | false | false |
| 12 stones | VCF | 32 | 32 | false | false |
| 12 stones | VCT | 531 | 589 | false | false |
| forced line | VCF | 17 | 17 | true | true |
| forced line | VCT | 126 | 126 | true | true |

Proved flags never regress. The single node rise, 531 to 589 on the 12-stone VCT position, is the fix working: fails stamped under a shallow remaining depth no longer authorize deeper probes, so those subtrees re-search instead of being skipped on an unsound memo.

Staged equivalence differential, unfixed tree, 80 clustered random boards per kind at production budget, cap-3-then-full staged instance versus fresh instance: VCF 0 mismatches over 5 wins, VCT 0 mismatches over 9 wins at iter 60 of 80. On the unfixed tree every stored fail is a completed depth-independent proof because cap exhaustion returns resAbort and aborts are never stored, so the depth-blind probe cannot poison a verdict today. The stamp converts that implicit invariant into an explicit mechanism and TestSolverStagedCapLadderMatchesFresh in internal/vcf/solver_test.go guards the equivalence over both geometries and both kinds.
