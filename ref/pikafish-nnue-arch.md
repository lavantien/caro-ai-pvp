# pikafish nnue architecture

source: https://github.com/official-pikafish/Pikafish/blob/master/src/nnue/nnue_architecture.h and src/nnue/features/full_threats.h
fetched: 2026-10-18 via zread read_file (full files, verbatim excerpts)
license: GPL-3.0 (Stockfish/Pikafish), quoted for study

## architecture facts (nnue_architecture.h)

- Dual feature sets: `ThreatFeatureSet = Features::FullThreats` and `PSQFeatureSet = Features::K6R2NC2`. The zread architecture-overview page states the network is HalfKAv2_hm plus FullThreats (dual feature set), contrasting with chess Stockfish's HalfKAv2_hm only.
- Layer sizes: L1 = 1024 (transformed feature dims), L2 = 32, L3 = 32, LayerStacks = 32.
- Layer chain: AffineTransformSparseInput<L1, L2> -> SqrClippedReLU and ClippedReLU pair (concatenated outputs, FC_0_OUTPUTS * 2 wide) -> AffineTransform<64, L3> -> SqrClippedReLU + ClippedReLU pair (L3*2 wide) -> AffineTransform<L2*2 + L3*2, 1> scalar output.
- Skip connection: output adds `fc_0_out[L2-2] - fc_0_out[L2-1]` directly to the final scalar.
- Output scaling: quantized so 1.0 == 600 * OutputScale, computed via int64 intermediate to make overflow impossible.
- Network identity: get_hash_value() (embedded in the eval file, includes dims and layer hashes) and get_content_hash() (whole-network content hash, std::hash specialization).

## full_threats.h facts

- Dimensions = 45547 sparse input features, MaxActiveDimensions = 64, index type u16.
- Index built from (perspective, attacker piece, from, to, threatened piece, mirror flag): make_index(Color, Piece attkr, Square from, Square to, Piece attkd, bool mirror).
- Dirty-update type DirtyThreats; append_changed_indices with prefetch base pointer and stride, i.e. incremental accumulator updates prefetch the exact weight rows being touched.
- Dual-perspective update helper append_changed_indices_both with independent mirror flags per color.

## extraction notes

- The FullThreats set encodes attacker->defender threat relations (piece pair with squares) as first-class NNUE inputs. This is xiangqi-specific structure injected into the net: the engine tells the network what a threat is instead of making it learn from raw placement.
- Net is tiny by chess standards (1024->32->32->1 with sparse input): CPU-only, per-search incremental updates, L1 accumulator per thread with NNUE accumulator caches (refreshTable in search.h).
- K6R2NC2 PSQ set: king + 6 ray/2 knight/2 cannon style bucketing (name from features/k6r2nc2.h), xiangqi piece mix.
