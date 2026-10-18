# fairy-stockfish wiki: nnue

source: https://github.com/fairy-stockfish/Fairy-Stockfish/wiki/NNUE
fetched: 2026-10-18 via WebFetch

## content as fetched

Default evaluation is handcrafted; variant-specific NNUE files are optional upgrades. Technique originated in shogi, adopted by Stockfish, ported to Fairy-Stockfish with generalizations for different board sizes, arbitrary piece types, variants without kings, and variants with piece drops.

Training: generalized fork of the official Stockfish trainer, variant-nnue-pytorch (github.com/fairy-stockfish/variant-nnue-pytorch). Data generated with variant-nnue-tools (Fairy-Stockfish based). Documentation on the variant-nnue-pytorch wiki; nets submitted via form and hosted externally, public list of networks page tracks the current strongest per variant.

Feature sets: HalfKA (v2.x) requires FSF 14+; Half(K)A (v2.5+) requires 14.0.1+ and mainly applies to variants without one royal king per side (antichess, horde) plus restricted-king games (xiangqi, janggi); the (K) makes the king feature optional. Old HalfKP (v1.x) and shogi nn.bin files need 13.1. Specifics deferred to variant-nnue-pytorch wiki Technical-details (HalfKAv2 architecture).

EvalFile: loads at runtime from file or compiled into the binary, no functional difference. The NNUE filename must start with the variant name: the filename is how the engine decides whether a net applies to the selected variant. Matching net engages automatically on variant selection; "NNUE evaluation using ... enabled" confirms.

One file per variant, keyed by filename prefix. Prebuilt NNUE releases exist for xiangqi, janggi, makruk.

## notes

- Variant applicability by filename prefix is a simple, checkable convention; our analog is the CAROBOOK1 magic plus meta line carrying generator tag and version, validated at load.
- The trainer story is the expensive part: engine-side integration is small, data generation and training are the project. Any future net of ours inherits that ratio.
