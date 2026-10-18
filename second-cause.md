# second cause: engine study of pikafish and fairy-stockfish

Date: 2026-10-18. Status: complete research pass, primary sources read and archived under `ref/`.

## 1. what this is

`docs/plans/first-cause-rebuild.md` is the first cause of the current codebase: the decision to rebuild from zero and the laws that governed it. This file is the second cause: the design decisions we can borrow from two mature open engines, studied at source level, mapped onto the v0.21 to v0.30 ladder. The studied projects are Pikafish (xiangqi, a Stockfish derivative specialized into one game, like us) and Fairy-Stockfish (a multi-variant framework, unlike us). Both were checked against live sources on 2026-10-18, wiki pages, repository files at master, and generated docs with file and line citations. Every claim below traces to a file under `ref/`, which holds the fetched material verbatim with source URLs and fetch dates. Nothing here relies on memory alone.

Scope per the request: how each project designs its engine, its ponder, its book, and which techniques we can yoink. The game here is a 16x16 gomoku variant with exact-five wins, overlines nullified, and both-ends blocked lines dead, so every borrowed idea is judged against that game and against our standing laws: arena evidence gates every strength-bearing change, no artificial handicap, config hub for constants, no host hardware numbers in prose.

## 2. the two projects at a glance

| aspect | pikafish | fairy-stockfish |
| --- | --- | --- |
| lineage | Stockfish fork specialized for xiangqi | Stockfish fork generalized to many games |
| goal | strongest single-game engine, one rule set | one engine, rules as data, dozens of games |
| board | 9x10 fixed | up to 12x10 in the largeboard build |
| eval | NNUE only, dual feature sets, threat-aware | handcrafted PSQT by default, optional NNUE per variant |
| search | Stockfish pipeline retuned for xiangqi | Stockfish pipeline with per-rule modulations |
| ponder | UCI ponder, stopOnPonderhit, TT-based reply recovery | same base, nothing added for variants |
| book | none in the engine, GUI-side | none in the engine, GUI-side |
| generality cost | none, single game | over 2x slowdown on chess, 50 to 200 Elo for largeboard |
| tests | signature, reprosearch, perft, ptnml/SPRT culture | perft, regression, protocol, reprosearch, signature |

## 3. pikafish engine design

### 3.1 search

Negamax alpha-beta with iterative deepening, aspiration windows, and PVS null-window search. The pipeline is the Stockfish shape with xiangqi-tuned constants: razoring at `eval < alpha - 709*depth^2`, child futility returning a blended value, null move with `R = 8 + depth/3 + max((staticEval - beta)/256, 0)` plus a verification search at depth 15, internal iterative reduction when no TT move exists, ProbCut with its own TT shortcut, late move pruning at `(3 + depth^2)/(2 - improving)` moves, singular extensions with double and triple extension margins, and LMR with a logarithmic base reduction adjusted by a dozen history and node-shape terms. Two pieces are uncommon and worth naming: hindsight adjustment restores depth at node entry when the parent's reduction looks wrong in the new node's light (`depth++` when the parent reduced 3 or more and the opponent is not worsening), and repeated aspiration fail-highs back the searched depth off by `failedHighCnt` and a `searchAgainCounter` term instead of re-searching the same failing window forever. Aspiration deltas grow by 44/128, about 34 percent, per re-search, centered on an exponential moving average of root scores rather than the last raw score. All numbers are SPSA output for xiangqi, the shape is the transferable part. Source: `ref/pikafish-search-pipeline.md`.

### 3.2 time management

Two layers. The strategic layer computes `optimumTime` and `maximumTime` per move from the clocks: a log model of remaining time and ply scaled by an `originalTimeAdjust` factor computed once per game from the starting clock, a hard ceiling of `0.8237 * remaining - moveOverhead` so no move can consume more than about 82 percent of what is left, a symmetric behind-in-time shrink, and a nodes-as-time mode that converts the whole clock into a node budget decremented by nodes actually searched. The tactical layer scales the optimum by four multipliers after each completed iteration: falling score extends time (range 0.610 to 1.860), best-move age shortens it (a move stable across 17 depths cuts the budget to 0.670 of nominal), cross-thread best-move churn extends it (`0.96 + 1.63 * bestMoveChanges / threads`), and root effort concentration shortens it when over 94 percent of nodes went into one move. Hard checks run on the main thread only, throttled to about every 512 nodes. Source: `ref/pikafish-timeman.md`, `ref/pikafish-timeman-zread.md`.

### 3.3 transposition table

One global table, deliberately racy across threads: synchronizing would cost more Elo than the occasional collision costs, and the code says so in prose. Entries carry move, value, static eval, depth, bound, and a pv flag, copied out non-atomically. Aging is a u8 generation bumped by `new_search()` at every root search, so yesterday's entries, including ponder-era writes, decay by generation instead of being cleared. The one unusual member is `TTWriter::penalize(int)`, which decrements an entry's stored depth when the search decides the entry misled it, a feedback channel into replacement policy that demotes rather than deletes. Source: `ref/pikafish-tt-h.md`.

### 3.4 threads and numa

Lazy SMP over a three-layer thread stack. Each thread owns a Worker allocated on 2 MB huge pages inside a job on that thread, so the memory is NUMA-local at creation. Root selection after the search is by vote: every thread votes for its own best root move with weight `(score - worstScore + 14)`, mates and tablebase conversions take priority with the shortest mate preferred, and the winning move is taken from the best thread that voted for it, not necessarily the main thread. Helper threads that are far ahead of the time budget (`elapsed < 26 percent` of it) search one ply deeper than nominal. NUMA replication covers the network and shared histories per node, and a system-wide shared-memory wrapper deduplicates the NNUE weights across multiple engine processes on one host. Sources: `ref/pikafish-threading-numa.md`, `ref/pikafish-search-h.md`.

### 3.5 evaluation

NNUE only, CPU inference, incremental accumulators. Master reads with two input feature sets: a piece-square set (`K6R2NC2`, the xiangqi piece mix) and `FullThreats`, 45547 sparse features encoding attacker to defender threat relations with from and to squares, updated incrementally with dirty diffs and weight-row prefetch. The network is small: sparse input into L1 of 1024, two hidden layers of 32 and 32 with squared-clipped and clipped activation pairs concatenated, a scalar output with a skip connection straight from the first hidden layer, quantized so the output range is fixed by construction. The release-era wiki documents a larger lineage shape, 25920 features into 2048 into 15 into 32 into 1. Both are recorded with dates in the ref files. The design point that matters to us is not the net, it is the feature philosophy: xiangqi told the network what a threat is instead of making it learn threats from raw placement. Sources: `ref/pikafish-nnue-arch.md`, `ref/pikafish-wiki-nnue.md`.

### 3.6 testing culture

The repo ships `signature.sh` (deterministic node signature per network and build), `reprosearch.sh` (search reproducibility), perft, and instrumented builds. The wiki documents ptnml, pentanomial pairing of the two games an opening contributes, the statistically correct treatment for unbalanced openings, plus SPRT practice on paired games with varied openings. Source: `ref/pikafish-wiki-ptnml.md`.

## 4. fairy-stockfish engine design

### 4.1 rules as data

The whole ruleset is a struct of about 180 fields, `variant.h`, from board geometry through piece movement regions, drops, promotions, castling geometry, walling, flips, chasing, extinction, flag races, and the connect family: `connectN`, `connectHorizontal`, `connectVertical`, `connectDiagonal`, `connectRegion1` and `2`, `connectNxN` (a tight square), `collinearN` (a line with gaps), `connectValue`. Built-in variants are concrete structs in `variant.cpp`, user variants load at runtime from an ini file parsed by `parser.cpp`, with inheritance between sections. Tic-tac-toe is 3x3 with pocket drops and `connectN = 3`, connect four is 6x7 with `enclosingDrop = top` for gravity and `connectN = 4`. The design is rule-centric, not variant-centric: each rule is handled independently, so a new variant is a new combination and improvements carry across combinations. The cost is measured, not assumed: over 2x slowdown on plain chess versus Stockfish, 50 to 200 Elo lost to the largeboard build. Sources: `ref/fairy-variant-h.md`, `ref/fairy-variants-ini-connect.md`, `ref/fairy-wiki-code.md`, `ref/fairy-wiki-strength.md`.

### 4.2 search modulation per rule

Search is Stockfish's with per-rule threshold tweaks, never per-variant forks: futility margins widen for must-capture and no-check variants, null move is skipped against pawn-only opponents and disabled for flipping games, SEE thresholds rise for flag-race variants and vanish entirely for duck chess, ProbCut's SEE accounts for drop-recaptured material in crazyhouse, singular extensions start 2 plies earlier in multi-king games, LMR history gets double-weighted gating history in gating variants. Game-end detection is a first-class search step (`pos.is_game_end` at non-root nodes) returning ply-adjusted values, and quiescence repeats it. Bughouse even has a partner protocol with state flags (dead, sit, fast) sent at completed depth 8. Connect games get none of this: they ride the generic eval and the connectN detector. Sources: `ref/fairy-search-adaptations.md`, `ref/fairy-wiki-code.md`.

### 4.3 evaluation

Handcrafted by default: per-variant piece values and piece-square tables, shared pawn structure, material imbalance, and endgame code reused where rules allow. NNUE is an optional per-variant upgrade loaded by filename convention (the file name must start with the variant name), trained with a generalized nnue-pytorch fork on data from a Fairy-Stockfish-based generator, with kingless and restricted-king variants handled by making the king feature optional in the Half(K)A set. One file per variant, no multi-variant nets in the documented model. Where nets exist, they close Elo distances nothing else closes, shogi gains over 1000 Elo, and the engine goes pure NNUE for speed reasons. Sources: `ref/fairy-wiki-nnue.md`, `ref/fairy-wiki-strength.md`.

### 4.4 protocols and bindings

UCI, UCCI, USI, UCI-cyclone, and CECP/XBoard, so any GUI can drive it. The engine is also a library: pyffish (Python) and ffish.js drive movegen, SAN, FEN, and game-end checks for every variant, which is how servers like pychess-variants consume it. Variant support has hard entry criteria: two players, alternating turns, perfect and complete information, rectangular board, at most 12x10. Source: `ref/fairy-readme.md`, `ref/fairy-wiki-faq.md`.

### 4.5 strength facts relevant to us

The strength table lists relative Elo with error margins where they exist and flags every result derived from fewer than 100 games. Connect four and tic-tac-toe are absent from it entirely, and breakthrough is the only placement-family entry. Neither project has demonstrated strength in a connect-n game, and the 16x16 board exceeds the framework's hard cap, so there is nothing here to compete against and nothing to port whole. Our differentiation is the solver-assisted threat search, which both engines lack any analogue of for this game family.

## 5. ponder in both

Both engines inherit the same ponder design, Pikafish documents it most completely.

Protocol shape: the GUI sends `go ponder`, the engine searches the predicted position without any time bound, the GUI either sends `ponderhit` (the prediction was right, the search converts into the real search) or `stop` (wrong, throw it away). The engine never self-stops a ponder: `check_time()` returns immediately while pondering is active, only `ponderhit` or `stop` ends it. The reply to the previous `bestmove` carries the expected opponent reply as `bestmove M ponder P`, so the GUI knows what position to ponder.

The graceful handoff is `stopOnPonderhit`. During ponder the soft time check still runs after each iteration. When the soft limit is crossed, the engine does not stop, it sets `stopOnPonderhit = true` and keeps searching. If `ponderhit` then arrives, the search stops instantly and the result is used, with the knowledge that it ran past its nominal soft budget. If the prediction was wrong, everything is discarded anyway.

Time accounting rewards pondering: with the `Ponder` option on, `optimumTime += optimumTime / 4` on every move, a 25 percent grant increase justified because time spent pondering is off the clock. The TT makes the wrong-prediction case cheap: the ponder search wrote a whole generation of entries, and the next real search starts warm regardless.

One mechanism covers the awkward case where the search ends with a single-move PV, for example after a fail-high at root: `RootMove::extract_ponder_from_tt` plays the best move, probes the TT for the resulting position, and appends the stored move as the ponder reply if it is legal, so the GUI always has something to ponder at zero search cost. Source: `ref/pikafish-extract-ponder.md`.

Fairy-Stockfish adds nothing variant-specific to ponder. Its searcher emits the ponder move from the PV exactly as Stockfish does, and its variant game-end logic already runs inside a ponder search like any other.

## 6. books in both

Neither engine contains an opening book. The FAQ, README, option lists, and wikis of both projects surface no book facility at all. In this lineage books are a GUI concern: the GUI owns a book file (Polyglot format de facto), picks weighted random moves from it while positions match, and the engine is a pure position-in, bestmove-out service. The engine contract stays clean, and book policy (weighting, floors, learning) lives where the game state and the match format live.

Our design diverges on purpose and the study supports the divergence rather than weakening it. We are the server and the GUI and the engine in one process, the book is a master-tier capability gated by the identity law rather than a match-tool artifact, and the artifact is ours to define: CAROBOOK1, fixed-width sorted entries, stone-mask keys, majority with a visit floor plus solver-proven overrides. What the study adds is the short list of book v2 candidates if the v0.24 arena gates ever show line-repeat losses: weighted selection among the top lines for diversity (we chose deterministic reproducibility), and per-event book learning that penalizes lines that lost in the current match, the standard anti-repeat mechanism of GUI books.

## 7. techniques to take

Class legend: take means it rides an existing planned wave now, candidate means strength-bearing and therefore arena-gated before it ships, recorded means written down for a future decision, not scheduled.

| technique | source | lands in | class |
| --- | --- | --- | --- |
| stopOnPonderhit semantics: soft limit crossed during ponder defers the stop to ponderhit, the run continues | pikafish timeman | v0.25 ponder lane, marks runs that crossed the soft limit as time-inadequate for adoption clause 1 | take |
| ponder reply recovered from the TT when the answer carries a single-move PV | pikafish search.cpp | v0.25 arm condition, TT fallback beside `PVLen >= 2` for book hits and solver proofs | take |
| ponder run has no time bound, only the owner stops it | pikafish check_time | v0.25 design already states this, the study confirms it | take, confirms |
| ponder grant bonus, +25 percent on the per-move grant while the seat's lane is armed | pikafish timeman | candidate knob `PonderGrantFactor` in the config hub | candidate |
| four soft-stop signals: falling score, best-move age, cross-thread best-move churn, root effort concentration | pikafish timeman and search | clock PID feed signals and adoption clause 1 inputs, all measurable in our engine today | candidate |
| best-thread voting for the root move, weight `score - worst + 14`, shortest mate priority | pikafish thread pool | SMP root selection, first verify what our SMP root selection does today | candidate |
| TT entry depth demotion on misleading entries (penalize) | pikafish tt.h | TT replacement policy | candidate |
| aspiration windows, delta growth 44/128 per re-search, EMA-centered | pikafish search | absent in our engine by design, our engine has no instability machinery to interact with it | candidate, large |
| helper threads one ply deeper when elapsed is under 26 percent of budget | pikafish search | SMP helper policy | candidate, small |
| pentanomial pairing of each opening's 2 seat games in arena verdicts | pikafish wiki ptnml | arena verdict math for tier edges, the v0.28 gate, and the v0.30 closing ladder | take, tooling |
| strength reporting with explicit pair counts, margins, and a flag on thin samples | fairy wiki strength page | README evidence tables | take, docs |
| reproducible search test: same binary, same position, byte-identical output across runs | both, tests/reprosearch.sh | verify we have one, add if missing | take, tests |
| 8-fold symmetric feature indexing precedent (chess halves its king squares by mirroring) | pikafish wiki nnue, HalfKAv2_hm | any future feature-crafted eval, our board has 8 symmetry axes where chess has 1 | recorded |
| weighted book picks and per-event line penalty learning | GUI book practice in this lineage | CAROBOOK2, only if v0.24 gates show line-repeat losses | recorded |

None of the take rows are strength-bearing: rows 1 to 4 ride the already-gated v0.25 wave, and rows 11 to 13 are tooling, docs, and tests. Every candidate row goes through an arena round under the inversion research law before any of it ships, and row 7 additionally stays inside the existing TT size laws.

## 8. techniques rejected

| rejected | why |
| --- | --- |
| NNUE evaluation and its training pipeline | the GPU offload assessment closed 2026-10-07 with no benefit at current scope, the handcrafted threat eval is the product, and the net is only as good as a data pipeline we have no budget to build and retrain per strength wave |
| rules-as-data variant framework | we have exactly one rule set, our config hub is the right size for that truth, fairy's measured 2x slowdown is the price of generality we do not need |
| skill levels and strength limiting | violates the no-handicap law, an engine never runs below its tier shape |
| engine-side book as a match tool | both engines prove the clean contract, but our book is a tier capability inside the server, the divergence is the identity law, not an oversight |
| NUMA replication and process-shared weights | single-node host, one process per tournament, nothing to replicate across |
| multi-protocol front ends | we have our own room API and SSE surface, UCI dialects would be dead code |
| racy unlocked table access as a stance | their trade is Elo-measured for chess-shaped trees at millions of nodes per second, ours is a Go engine under `GOMEMLIMIT` with different contention behavior, we keep our own concurrency discipline and revisit only with measurements |

## 9. corrections and confirmations for the current plan

The v0.25 ponder wave design in the ladder plan survives this study intact, with 3 refinements, none of which change its gates: the arm condition gains the TT fallback from row 3 of the take table, the adoption law's clause 1 gains the soft-limit-crossed marker from row 1 as an explicit input rather than an implicit one, and the grant bonus of row 4 is the one new knob, config-hubbed and swept in the same arena night as `PonderAdoptFraction` if the first gate run motivates it. The v0.24 book wave is confirmed as designed, including the deliberate divergence from GUI-side book practice. The arena verdict upgrade to pentanomial pairing is pure tooling, lands before the v0.28 gate where tight thresholds bind hardest, and changes no gate semantics, only the variance of the estimator behind them.

## 10. source index

All fetched 2026-10-18. Each ref file carries its URL, fetch method, and verbatim content or extraction.

| ref file | source |
| --- | --- |
| ref/pikafish-wiki-home.md | https://www.pikafish.com/wiki/en/ |
| ref/pikafish-wiki-nnue.md | https://www.pikafish.com/wiki/en/faq/nnue.html |
| ref/pikafish-wiki-ptnml.md | https://www.pikafish.com/wiki/en/faq/ptnml.html |
| ref/pikafish-search-pipeline.md | zread page on src/search.cpp, official-pikafish/Pikafish |
| ref/pikafish-timeman.md | src/timeman.cpp, official-pikafish/Pikafish, full source |
| ref/pikafish-timeman-zread.md | zread time management page |
| ref/pikafish-search-h.md | src/search.h, official-pikafish/Pikafish, full source |
| ref/pikafish-tt-h.md | src/tt.h, official-pikafish/Pikafish, full source |
| ref/pikafish-threading-numa.md | zread threading and NUMA page |
| ref/pikafish-nnue-arch.md | src/nnue/nnue_architecture.h and src/nnue/features/full_threats.h, full source |
| ref/pikafish-extract-ponder.md | src/search.cpp ponder recovery, raw master fetch |
| ref/fairy-readme.md | README.md, fairy-stockfish/Fairy-Stockfish, full source |
| ref/fairy-variant-h.md | src/variant.h, fairy-stockfish/Fairy-Stockfish, full source |
| ref/fairy-variants-ini-connect.md | src/variants.ini connect-family sections |
| ref/fairy-wiki-nnue.md | wiki NNUE page |
| ref/fairy-wiki-strength.md | wiki Playing-strength page |
| ref/fairy-wiki-code.md | wiki Understanding-the-code page |
| ref/fairy-wiki-faq.md | wiki FAQ page |
| ref/fairy-search-adaptations.md | zread variant-specific search page |
