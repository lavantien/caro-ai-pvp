# pikafish wiki: what is nnue

source: https://www.pikafish.com/wiki/en/faq/nnue.html
fetched: 2026-10-18 via webReader

## content as fetched

NNUE (Efficiently Updatable Neural Network) is a neural network architecture suited to inference on CPUs. All top xiangqi engines use NNUE. NNUE is only an evaluation network, responsible solely for evaluating positions. The search used by top xiangqi engines is still traditional alpha-beta pruning.

Origin: designed for shogi by Yu Nasu, applied in YaneuraOu 2018, ported to Stockfish June 2019 by Nodchip. Core idea: board inputs change very little per move, so update incrementally instead of recomputing.

Feature sets (chess example): HalfKP = (our king square, piece square, piece type, piece color), 64x64x5x2 = 40960 features. HalfKA includes kings as ordinary pieces, 64x64x6x2 = 49152. HalfKAv2 merges the king-pair self-features, 64x64x11 = 45056. HalfKAv2_hm exploits horizontal mirror symmetry, king squares halved, 32x64x11 = 22528.

Accumulator: first layer output (22528 -> 2560 in SFNNv8) stored per perspective, 2 accumulators. A move adds or subtracts only the weight columns of changed features.

Pikafish footnote (release-era numbers, page published 2026-09-30): "Pikafish's NNUE has 25920 features in total (=6x6x90x8). 25920->2048(L1)->15(L2)->32(L3)->1" and "Pikafish currently uses 2048" for L1.

## notes

- Wiki release-era shape (25920 -> 2048 -> 15 -> 32 -> 1) versus master src/nnue/nnue_architecture.h read 2026-10-18 (dual feature sets PSQ K6R2NC2 + FullThreats 45547 dims, L1 1024, L2 32, L3 32, sparse first layer, skip connection). Master is the dev truth, the wiki page reflects the documented release lineage. Both recorded here with dates.
- The horizontal-mirror trick (HalfKAv2_hm) halves king-square features. For our game, the board has 8-fold dihedral symmetry and we deliberately do not canonicalize the book by it (CAROBOOK1 probe simplicity). Any future feature-crafted eval or net could exploit 8-fold symmetry in feature indexing, an axis chess never had beyond 1 axis of mirroring.
