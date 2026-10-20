# v0.24 book gate verdict

Artifact db/book.carobook, 22 majority entries from 308 master-tier self-play games at tc 3, solver overlay 0 proofs, per book-gate-preregistration.md. Six arms run serialized, 20 games each, ptnml rows verbatim in the bookgate and evalfit logs under this directory. Scoring statistic per the five-tier preregistration: the color-pair-pooled decisive fraction from the book seat's perspective, against the committed same-tier floor bands.

## book arms

| arm | ptnml | book decisive frac | committed band | verdict |
|---|---|---|---|---|
| grandmaster 2+1 (gm21) | 1-0-8-1-0 | 10/19 = 0.526 | 0.404 to 0.607 (floor-gm21) | inside band |
| grandmaster 3+2 (gm32) | 1-1-4-0-4 | 7/19 = 0.368 | 0.447 to 0.657 (floor-gm32) | below band, gate fail |
| master 2+1 (master21) | 1-0-7-0-2 | 9/20 = 0.450 | none committed | unbandable, descriptive |
| master 3+2 (master32) | 2-0-3-1-4 | 7/18 = 0.389 | none committed | unbandable, descriptive |

Points shares for the record: gm21 10.5/20, gm32 7.5/20, master21 9/20, master32 7.5/20.

## evalfit side-gate

Base grandmaster against the derived weights seat 137,162,483,484,1561.

| arm | ptnml | weights decisive frac | floor lower bound | verdict |
|---|---|---|---|---|
| tc 1 2+1 | 6-0-4-0-0 | 4/20 = 0.20 | 0.404 | reject |
| tc 2 3+2 | 5-0-5-0-0 | 5/20 = 0.25 | 0.447 | reject |

## verdict

The gate fails on gm32: the book seat's 0.368 sits below the floor-gm32 lower bound 0.447, three decisive games below the band edge. Per the preregistered rule the builder is at fault, the fix and regrow run before v0.24 tags, and no arm reinterprets its band after results. The gm21 inside-band read carries no force while the gate fails. The master arms expose a preregistration gap recorded honestly: only four floors were ever committed (hard-hard and grandmaster-grandmaster at both controls, regate-2plus1-gate.md and regate-3plus2-gate.md), no master-master floor exists, so the master reads stay descriptive, both pointing the same direction as the failing 3+2 arm, and the regrow round must either commit a master floor or preregister the master band derivation before its arms rerun. The evalfit weights variant is rejected at both controls; the base weights stand.

Diagnostic direction for the regrow, not part of the verdict: the artifact holds 22 majority entries and zero solver proofs from a tc 3 master-tier corpus, the book side holds 10.5/20 points at 2+1 and falls to 7.5/20 at 3+2, so the corpus lines hurt most exactly where the opponent has clock to punish book exits. The builder law stands: deep self-play agreement over the visit floor with the solver overlay, no hand curation.
