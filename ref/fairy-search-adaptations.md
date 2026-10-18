# fairy-stockfish variant-specific search adaptations

source: https://zread.ai/fairy-stockfish/Fairy-Stockfish/20-alpha-beta-search-with-variant-specific-pruning (generated from src/search.cpp)
fetched: 2026-10-18 via WebFetch

## summary as fetched

Futility pruning: depth cutoff shrinks by 3 plies when blast_on_capture (atomic-style); margin multiplied upward for check counting, mandatory capture, single-piece extinction, and no-checking variants, pruning stays conservative where tactics swing quickly.

Null move pruning: skipped when the opponent holds only pawns (zugzwang risk); disabled for piece-flipping mechanics; static-eval threshold gains a bonus when no double-step pawn region exists; must-capture variants apply a larger depth reduction on the null-move search.

SEE pruning: threshold increases by 10 for flag-region variants (racing kings); duck chess fully exempt because blocking moves can be necessary despite material loss.

ProbCut: SEE threshold adjusted for captures-to-hand variants (crazyhouse) to reflect material gained via drops.

Singular extensions: depth gate becomes depth >= 7 - 2 * (count<KING>() == 1), so multi-king variants begin singular searches two plies earlier.

Check extension: only when depth > 6 and |staticEval| > 100cp, targeting genuinely tactical positions.

Losing-chess capture extension: in must-capture variants, captures extended when in check or when only one capture exists.

LMR: statScore adds gating history at double weight for gating variants; divisor drops by 4434 when captures_to_hand, so history influences reductions more strongly in drop games.

Variant game-end detection: at non-root nodes pos.is_game_end(variantResult, ply) checks piece extinction, flag reaching, material-count thresholds, variant checkmate rules, returns ply-adjusted win/loss. No legal moves -> pos.checkmate_value(ply) with per-variant mate values, stalemate distinguished accordingly. Qsearch repeats game-end, repetition, mate/stalemate detection. TT avoids GHI by checking rule50 counts, adjusts mate scores to distance from node, handles variant TB scores.

Bughouse: at completed depth >= 8 under XBOARD sends time/otim clocks to partner plus states: dead (being mated, partner must help), sit (winning, conserve time), fast (virtual mate, play fast), x (clear).

Connect games: not addressed on this page.

## notes

- The pattern is per-rule modulation of standard techniques, never per-variant forks of the search: each variant flag tweaks a threshold, a margin, or gates a technique off. Our tier capability seats (novct, nosolver, nobook arena overrides) are the same pattern applied to engine capabilities.
- Connect-n games get zero special handling in the search: they ride generic eval plus the connectN game-end detector in position.cpp. Confirms the gap our threat eval and VCF/VCT solvers fill.
