# v0.24 regrow pre-registration

Fixed before any new run, per the gate law. Supersedes nothing in book-gate-preregistration.md: the arms and decision rule carry over verbatim, this round adds the corpus target, the sidecar version repair, and the missing master floors.

Corpus: 1500 games at tc 3 on master-tier seats, same fleet law as the 308-game round (forced 12-cell opening rotation over BookOpeningCells, red seat alternation, clock-driven budgets, b-segment re-arm). BookOpeningCells stays 12 cells, full root coverage, no trunk halving this round. BookVisitFloor stays 20. Ply bounds stay committed: master BookMaxPly 8, grandmaster 16. The run records its -games target in the corpus fingerprint so target and actual count are both honest. The overlay sidecar restarts fresh under BookSidecarVersion 2, repairing the a968713 format change that never bumped the version; the old 5-field v1 files are refused, never migrated.

Master floors, committed before any arm reruns: master-nobook vs master-nobook pools, 5 arena dirs of 20 games per control at tc 1 (2+1) and tc 2 (3+2), scored as the color-pair-pooled decisive fraction with the Wald 95 percent interval, the same derivation as floor-gm21 and floor-gm32 in regate-2plus1-gate.md and regate-3plus2-gate.md: frac ± 1.96 times sqrt(frac times (1 minus frac) over decisive games). The interval must cover 0.5; if it does not, the pool extends by another 5-dir block before anything else runs. Results commit as floor-master21 and floor-master32.

Arms rerun: the same four book pairs from book-gate-preregistration.md, 20 games each, serialized, against the rebuilt artifact. The evalfit weights variant stays rejected from the first round, no rerun, base weights stand. Decision rule unchanged: a book seat whose color-pair-pooled decisive fraction falls below its same-tier floor band lower bound fails the gate, the builder is at fault, and no arm reinterprets its band after seeing results.

Sequencing: corpus regrow, artifact rebuild with the fresh overlay, master floor pools, then arms. One engine-timing batch on the machine at a time.
