# pikafish ponder move recovery from tt

source: src/search.cpp of https://github.com/official-pikafish/Pikafish (raw fetch of master, 2026-10-18), function RootMove::extract_ponder_from_tt plus the call site in start_searching()
fetched: 2026-10-18 via WebFetch over raw.githubusercontent.com (extraction summary, quoted fragments under 125 chars)
license: GPL-3.0, quoted for study

## mechanism as fetched

Comment block: "Called in case we have no ponder move before exiting the search," for instance when the search stops during a fail high at root. It tries hard to produce a ponder move so a GUI in ponder mode has something to think about.

Signature: bool RootMove::extract_ponder_from_tt(const TranspositionTable& tt, Position& pos)

1. Asserts the PV holds exactly one valid move.
2. Plays that move on the position with TT access enabled.
3. Unless the move triggers a draw rule ("if (!pos.rule_judge(_, 1))"), probes the table for the resulting position and, if the entry holds a legal move, appends it via "pv.push_back(ttData.move);".
4. Undoes the move and reports success only when a ponder candidate was found: "return pv.size() > 1;"

Call site in start_searching(), after best-thread selection and score storage, comment "Send PV info if it has changed since last output in iterative_deepening()":

"if (bestThread->rootMoves[0].pv.size() == 1 && bestThread->rootMoves[0].extract_ponder_from_tt(tt, rootPos))"

On success uciPvSent = false so the PV is re-emitted, and the ponder string is built from the second PV entry: "ponder = UCIEngine::move(bestThread->rootMoves[0].pv[1]);"

## notes

- A 1-move PV (fail-high at root, truncated TT line) still yields a ponder move: make the best move, probe the TT for the reply, use it if legal. The GUI always gets a ponderable position; no second search needed.
- Our ponder lane arms on PVLen >= 2 (the plan's v0.25 gate). This function is the exact fallback for the PVLen == 1 case: probe our TT after the book/solver/single-move answer to still arm the lane. Zero search cost, reuses warmth we already paid for.
