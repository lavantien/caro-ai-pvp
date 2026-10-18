# pikafish search pipeline (zread summary)

source: https://zread.ai/official-pikafish/Pikafish/11-alpha-beta-search-pipeline (generated from src/search.cpp with line citations)
fetched: 2026-10-18 via WebFetch

## summary as fetched

Framework: negamax alpha-beta with iterative deepening, aspiration windows, PVS-style null-window pruning. search<>() templated on NodeType (Root / PV / NonPV). PvNode = nodeType != NonPV, rootNode = nodeType == Root, allNode = !(PvNode || cutNode).

Iterative deepening and aspiration:
- rootDepth 1..MAX_PLY carrying previousScore forward.
- delta = 10 + threadIdx % 8 + |meanSquaredScore| / 39605, window centered on averageScore.
- Fail-high/low re-searches grow delta about 34.4 percent per re-search (44 delta / 128).
- Repeated fail-highs reduce effective depth: max(1, rootDepth - failedHighCnt - 3*(searchAgainCounter+1)/4).
- Xiangqi-specific: no stalemate; draw scores get VALUE_DRAW - 1 + (nodes & 0x2) to avoid 3-fold blindness.

Search stack: flat Stack array with 7 sentinels below and 2 above; per-ply fields pv, continuationHistory, excludedMove, staticEval, statScore, moveCount, inCheck, ttPv, followPV, cutoffCnt, reduction.

Steps (constants from the page):
- Step 2 repetition via pos.rule_judge(); abort/max-ply checks. Step 3 mate distance pruning, alpha/beta tightened to mated_in(ply) / mate_in(ply+1). Step 4 TT probe.
- Step 5 corrected static eval: NNUE + 4 correction histories, clamped.
- Step 6 TT cutoff when ttDepth >= depth and bound matches; GHI guard at high rule60 counts.
- Step 8 razoring: eval < alpha - 709*depth^2 -> qsearch.
- Step 9 child futility: eval - margin >= beta, depth < 15 (6 if seekMate); returns (716*beta + 308*eval)/1024.
- Step 10 null move: condition staticEval >= beta - 8*depth - 50*improving + 187 plus major material; R = 8 + depth/3 + max((staticEval - beta)/256, 0); verification search at depth >= 15 via nmpMinPly.
- Step 11 IIR: no TT move, depth >= 6, not allNode -> depth - 1.
- Step 12 ProbCut: probCutBeta = beta + 251 - 66*improving, depth >= 3, qsearch pre-check then search at depth - (improving ? 5 : 3), stored BOUND_LOWER. Step 13 TT ProbCut: BOUND_LOWER entry at ttDepth >= depth-4 with value >= beta + 470.
- Step 15 LMP: moveCount >= (3 + depth^2)/(2 - improving) skips quiets. History pruning threshold -2995*depth; SEE pruning at -35*lmrDepth^2.
- Step 16 singular extensions: excluded search at newDepth/2; singularBeta = ttValue - (44 + 72*(ttPv && !PvNode))*depth/69; double/triple extension margins below that; -3 negative extensions at cut nodes; gated by is_shuffling(). Multi-cut at depth >= 5 when value >= beta.
- Step 18 LMR: depth >= 2, moveCount > 1, logarithmic base reduction adjusted by ttPv -2363, cutNode +3251 (+1048 without TT move), ttCapture +1571, child cutoffCnt adds +256..+1280, TT move -2730, history term -946*statScore/8192, allNode scaling +r*256/(256*depth+256); searched depth max(1, min(newDepth - r/1024, newDepth + 2)) + PvNode; full-depth re-search when the reduced search beats alpha.
- Step 20 PV re-search full window search<PV>(..., -beta, -alpha, newDepth, false); newDepth forced >= 1 if the TT move would enter qsearch with a decisive score.
- Steps 21-22: on threads.stop values are discarded (VALUE_ZERO) to protect TT/PV. Root moves update averageScore/meanSquaredScore via exponential moving averages feeding aspiration sizing.

Hindsight adjustment at node entry: depth++ if priorReduction >= 3 and not opponentWorsening; depth-- if priorReduction >= 2 and depth >= 2 and staticEval + (ss-1)->staticEval > 193.

Optimism: optimism[us] = 92 * avg / (|avg| + 95), negated for opponent. Correction value blends 5 tables: (4547*pawnCV + 3804*minorCV + 8213*(nonPawnW + nonPawnB) + cntCV) / 131072, clamped, added to raw static eval.

Move ordering (MovePicker): TT move -> captures (SEE + capture history) -> quiets (main history + continuation history + pawn history).

Qsearch resolves captures and checks at depth 0.

Time and ponder: totalTime = optimum() * fallingEval * reduction * bestMoveInstability * highBestMoveEffort; fallingEval = (16.93 + 2.73*(prevAvg - best) + 0.8*(iterValue - best)) / 100; instability = 0.96 + 1.63*bestMoveChanges/threads. Search halts past min(totalTime, maximum()) or enters ponder mode if applicable. If elapsed < 26 percent of budget, increaseDepth lets helper threads search one ply deeper.

## extraction notes

- All numeric constants above are SPSA-tuned xiangqi values; the shape, not the numbers, is the transferable part.
- increasing-depth helper policy (elapsed < 26 percent of budget -> +1 ply) and the searchAgainCounter depth backoff are the two least-commonly-copied pieces.
