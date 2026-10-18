# pikafish search.h structures

source: https://github.com/official-pikafish/Pikafish/blob/master/src/search.h
fetched: 2026-10-18 via zread read_file (full file, verbatim excerpts)
license: GPL-3.0 (Stockfish/Pikafish), quoted for study

## facts

- NodeType template params: NonPV, PV, Root.
- RootMove: score, previousScore, averageScore, meanSquaredScore, uciScore, inexactLower/inexactUpper bound flags, previousScoreExact, selDepth, effort (u64 node effort on the move), pv + previousPV. Sorts by score then previousScore descending.
- `bool extract_ponder_from_tt(const TranspositionTable& tt, Position& pos);` ponder move recovery from the TT at bestmove time.
- LimitsType: time/inc per color, npmsec, movetime, movestogo, depth, mate, perft, infinite, nodes, searchmoves, startTime, `bool ponderMode`.
- SearchManager (main thread only): TimeManagement tm, double originalTimeAdjust, int callsCnt, `std::atomic_bool ponder`, iterValue[4], previousTimeReduction, bestPreviousScore, bestPreviousAverageScore, `bool stopOnPonderhit`, update callbacks (onUpdateNoMoves/Full/Iter, onBestmove, onStart).
- Worker (one per thread): histories (Butterfly mainHistory, LowPlyHistory, captureHistory, continuationCorrectionHistory, TTMoveHistory, SharedHistories& sharedHistory per NUMA node, continuationHistory[2][2]), nodes + bestMoveChanges as RelaxedAtomic, selDepth, nmpMinPly, optimism[COLOR_NB], rootPos/rootState/rootMoves/rootDepth, seekMate flag, lastIterationIdxPV, threadIdx/numaThreadIdx/numaTotal + NumaReplicatedAccessToken, accumulatorStack + refreshTable (NNUE accumulator caches).
- SharedState bundles options, threads, tt, per-NUMA SharedHistories map, `LazyNumaReplicatedSystemWide<Eval::NNUE::Network>`.

## extraction notes

- Ponder plumbing surfaces: LimitsType.ponderMode in, atomic ponder + stopOnPonderhit in SearchManager, extract_ponder_from_tt on RootMove, bestmove callback carries (move, ponder) strings.
- bestMoveChanges is aggregated across all threads (used by the bestMoveInstability time factor).
- optimism per color and correction history blending happen before the move loop (see search pipeline notes).
