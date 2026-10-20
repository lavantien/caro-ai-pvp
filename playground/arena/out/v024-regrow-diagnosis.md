# regrow diagnosis

Verdict: game generation is healthy, 308 games logged 4928 records over 1399 unique sub-16-ply positions with 12 distinct trunks, and the artifact collapsed inside the aggregation stage because 308 games over 12 trunks, 25.7 games per trunk, cannot put 20 majority-agreement visits on any position past ply 8, and that same starvation hands the solver overlay only quiet shallow candidates, hence 0 proofs.

## position census

The merged corpus db/book-corpus.txt, SHA256-identical to playground/bookgen/out/book-corpus-all.txt, header `F corpus v1 tier=master tc=3 games=300`, holds 308 G blocks, 4928 P records, exactly 16 per game with 0 illegal fallbacks, over 1399 unique position keys with plies 0 through 15 fully populated. Shard composition: out/book-corpus.txt 105 games, inst2 41 plus a 39-game b-segment, inst3 41 plus 41, inst4 41, inst5 through inst8 header-only stubs from parked fleet slots per scripts/bookgen-fleet.ps1:14-19. db/book-stats.txt agrees: positions 1399, floor_passing 22, trunks 12. The corpus holds 1399 unique positions against 22 artifact entries, so the collapse sits in the aggregation and floor stage, generation produced the mass.

## replay quantification

Distinct opening prefixes across the 308 games: 12 first moves (the forced cells), 35 second-move lines, 2.9 replies per trunk, 56 lines at ply 4, 91 at ply 8, 200 full 16-ply lines. 108 of 308 recorded lines are exact duplicates of another game and the most repeated line occurs 12 times, so 65% of games follow a line no other game played. Deterministic replay is ruled out.

The intended diversity mechanisms all exist and fire: forced first move cycling the 12 BookOpeningCells by game index (game.go:27-29, game.go:52-53, config.go:216), red seat alternation redSeat = i % 2 (corpus.go:387), and search effort variance from the game clock, the M-lines show alloc 22.5 to 27.4 s and 35m to 153m nodes per move at tc 3 (game.go:66-69, out/corpus-native.log tail).

Diversity did not fail, it works against the floor. Reply shares per trunk run 0.4 to 0.9, so on 7 of 12 trunks no second move reaches 20 picks out of the trunk's ~25 games and only 5 of 12 ply-1 positions pass the majority floor. The floor demands agreement, generation supplies branching, and 308 games cannot fund both past ply 8. Per-ply concentration at N = 308:

| ply | unique positions | raw visits 20 or more | top 6 visit counts |
|---|---|---|---|
| 1 | 12 | 12 | 29/29/29/28/28/24 |
| 2 | 35 | 5 | 27/24/23/23/23/19 |
| 4 to 7 | 56 | 2 | 23/23/19/18/14/12 |
| 8 | 91 | 2 | 21/20/19/17/14/11 |
| 12 | 139 | 0 | 17/15/14/11/11/9 |
| 15 | 177 | 0 | 12/10/9/8/8/6 |

## solver overlay

The overlay ran and followed its code path. overlayRun aggregates the corpus, keeps only floor-passing keys (overlay.go:188-189, floorPassingKeys at overlay.go:169-178), and probes each pending one with VCF then VCT under budget 1<<24 = 16777216 nodes (overlay.go:32-45, config.go:197). All 22 candidates returned no proof (out/overlay.log lines 1-23, 22 N lines in db/book-overlay.txt), and none looks like a budget casualty: the candidate set is the quiet consensus spine at plies 0 to 8, position 1 is the empty board, and a forced five-in-a-row needs at least 4 aligned own stones, which quiet positions under ply 9 do not offer. No-proof lines carry no node counts, so exhaustion cannot be read off the artifacts, but the cause is sufficient without it: positions that could hold proofs are tactical lines at ply 10 and beyond, where visits run 6 to 17 and never reach the floor, so the overlay never receives them. Records also stop at BookMaxPly 16 by design (game.go:80-82), so ply 15 is the deepest provable ply. The zero-proof outcome is the floor coupling working as preregistered, with no skipped branch and no wrong condition.

## aggregation and floor

aggregate groups records by full position id and counts per move (overlay.go:47-58), majorityOf takes the most-picked move with a lowest-cell tiebreak (overlay.go:60-69), and the floor demands the majority count itself reach 20 (overlay.go:169-178). No key mismatch exists, logger and aggregator share keyOf (corpus.go:138-147), and ply accounting is exact, 4928 = 308 x 16 records with plies 0-15 populated. The only loss beyond sizing is majority versus raw visits: 34 positions hold 20 or more raw visits, 12 of them split picks across replies so no single move reaches 20, leaving 22. Back of envelope for diverse games: every game contributes exactly one visit per ply, so ply p supports at most N/20 floor-passing positions, 15 at N = 308, while the observed tree already holds 35 positions at ply 2 and 56 at ply 4. The pipeline keeps everything it should, the corpus is undersized for floor 20 over 12 trunks.

## builder fix

No change to the aggregation or overlay logic. The fix is corpus mass plus two operational repairs. Regrow to about 1500 games with an explicit -games target so the fingerprint records the real number, the current header says games=300 while the file holds 308 (corpus.go:73-75). Start the overlay sidecar fresh: the on-disk overlay files carry the pre-a968713 5-field header, current readOverlay demands 7 fields with solver and corpus pins (overlay.go:81-84, overlay.go:110) and refuses them, and a968713 changed the format without bumping BookSidecarVersion, so both formats claim v1 (config.go:201). Regenerating the overlay with current code over a new or deleted out path is enough. Optional hardening: bump BookSidecarVersion on any future format change.

No forced-move or symmetry additions belong in the builder. Forced first moves already exist as the committed BookOpeningCells constant, and forcing replies at later plies would be hand curation of lines, which the builder law forbids.

## regrow plan

Game count. Root branching is the 12 forced opening cells. One game funds exactly one visit per ply, so a position passes floor 20 when its share of all games reaches 20/N. Perfect consensus over all 12 trunks through ply 15 needs only N = 20 x 12 = 240 games, and 308 already exceeds that bound, so coverage fails on reply splitting alone. Measured shares put the ply-15 top line at 12/308 = 3.9% and the ply-15 top 6 at 1.9%, so the consensus spine through ply 15 clears 20 near N = 20/0.019 = 1050. Line counts keep growing as N rises and the 35% duplicate rate thins, so target 1500 games, 5x the current corpus. That funds 125 games per trunk and every trunk position through ply 2 passes with reply shares of 0.16 or better. Master reads plies 0 to 7 only (TierMaster.BookMaxPly 8, config.go:75) and grandmaster plies 0 to 15 (config.go:76), so the depth growth directly serves the failing gm32 arm while the master subset closes its 7 trunk holes at ply 2.

Diversity mechanism. Same fleet, same law: forced 12-cell rotation, red alternation, clock-driven budgets, b-segment re-arm windows (scripts/bookgen-fleet.ps1). At tc 3 the M-lines show about 24 s per engine move, so 1500 games costs roughly 5x the 2026-10-20 fleet run in wall clock across the 3 running slots. If that cost is unacceptable the one principled lever is trunk concentration: halving BookOpeningCells to 6 cells doubles per-trunk mass and halves the needed N at the cost of root coverage, a config decision to preregister before the run, and it is trunk config, not line curation.

Master floor. The regate gates derived their bands as same-tier pools with a Wald 95% interval on the pooled decisive fraction: gm21 held 105 games, 93 decisive, frac 0.505, interval 0.505 +- 1.96 x sqrt(0.505 x 0.495 / 93) = 0.404 to 0.607, and gm32 held 97 games, 87 decisive, frac 0.552, interval 0.552 +- 0.104 = 0.447 to 0.657 (regate-2plus1-gate.md, regate-3plus2-gate.md). The regrow round must commit the missing master floors before its arms rerun, with the same derivation: master-nobook vs master-nobook, about 100 games per control, 5 arena dirs of 20, pooled color-pair decisive fraction with the Wald interval, verified to cover 0.5, committed as floor-master21 and floor-master32. The arms then read against them under the preregistered rule: a book seat below its floor band's lower bound fails the gate and the builder is at fault.
