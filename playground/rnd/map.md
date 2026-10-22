# rnd topic map

Each topic mapped against the shipped engine on 2026-10-22. Verdicts: fits (seam exists, prototype justified), ruled out (reason from the code), needs prototype (open question a cheap experiment answers). Prototype lanes referenced by name run under playground/rnd.

| topic | verdict | carrier |
| --- | --- | --- |
| fft / convolution | ruled out | none |
| n-body pairwise | ruled out | none |
| sorting regimes | ruled out for now, measurement seam exists | none |
| root-parallel trial evaluation | needs prototype only on scaling evidence | rnd-search (deferred) |
| mcts | needs prototype only inside an nn architecture change | rnd-nneval |
| simulated annealing | ruled out, differential evolution owns the niche | none |
| broad-phase / spatial hashing | already shipped, nothing to add | none |
| wavefront threat maps | needs prototype | rnd-search |
| wfc / procedural diversity | fits, cheap version first | bookgen regrow |
| nn evaluation interface | fits as a documented seam, adoption gated | rnd-nneval |
| gpu batched eval | needs prototype, standing revisit trigger | rnd-nneval |
| ant colony optimization | ruled out | none |
| bayesian optimization | fits, highest value tuning lane | rnd-tune |
| applied game theory | fits, analysis first | rnd-gametheory |

## fft / convolution

Ruled out. Pattern extraction is a nonlinear classification: each 9-cell window in each of 4 directions maps through a 262144-entry table to a threat class and a win-in-1 mask, built by emulating windows on a real board (pattern/table.go:23-48). The classes depend on nonlinear predicates: blocked ends, exact run lengths, realization. A convolution is linear and cannot express them. The table lookup costs 0.26 ns (playground/gpuoffload/ANALYSIS.md:25) against 256 cells times 4 directions at 16x16 scale, far under any transform floor. The evaluator is incremental: makeMove and unmakeMove touch only the 4 directions times 9 windows through the changed cell (engine/eval.go:100-142), and eval() itself is 2 array reads and a subtract (engine/eval.go:68-70). Nothing for a transform to win. If a neural eval ever lands its conv layers use matmul, that is the nneval lane, not this one.

## n-body pairwise

Ruled out. There is no pairwise interaction to prune. Stone interaction is local along 4 directions and already handled by bitboards: candidate generation dilates the occupied set by Chebyshev rings (engine/movegen.go:37), the solver shifts bitboards per direction (vcf/threat.go:56-109). No O(n squared) term exists anywhere in the hot path.

## sorting regimes

Ruled out for now, one measurement seam stays open. Ordering is layered: transposition move at SearchOrderTT, 2 killers, then incremental static cell sums plus history (engine/movegen.go:84-99), history aging on overflow (engine/movegen.go:116-132). Moves are not fully sorted: pickMax selects the next best on demand because beta cutoffs arrive early (engine/movegen.go:103-114). The static layer costs 2 array reads from the incremental evaluator, so a fancier sort has almost nothing to save. The seam: stats.FirstMoveFailHighPermille already measures ordering quality per search (engine/search.go:252-254). If arena logs ever show first-move fail-high dropping at shallow plies, a bucket sort of the static layer is the prototype to try. Until then no evidence demands it.

## root-parallel trial evaluation

Needs prototype only if scaling evidence demands it. The shipped shape is lazy smp: every worker runs the full iterative deepening root loop on the same board, they share one TT, and collection picks the deepest completed iteration with earliest sequence number (engine/smp.go:354-395, 301-352). Root splitting would remove duplicated root work but each root subtree then runs alone after a cutoff elsewhere, and the TT sharing benefit that lazy smp lives on weakens. Engines in this class keep lazy smp for that reason. The decision input is a scaling curve, which engine/racescale_test.go already exercises. Defer until a tier reports poor depth gain per added worker.

## mcts

Needs prototype only inside an architecture change. As a plain playout policy against the shipped alpha-beta plus exact VCF and VCT solvers, uct sampling loses: the shipped search reaches deep forced lines the sampler cannot, and gomoku class engines without neural evaluation lose to alpha-beta in every public precedent. The interesting form is mcts as the batching owner for a neural eval, lc0 shaped, which playground/gpuoffload/ANALYSIS.md:45-48 already names as the architecture class where gpu pays. That decision belongs to the nneval lane verdict, not to a standalone mcts trial. Playout speed and correctness live in rnd-search as the mcts core only.

## simulated annealing

Ruled out. The offline tuning problems are low dimensional and smooth enough: 6 pattern weights and a scale (playground/evalfit fits them by differential evolution, playground/evalfit/evolve.go:52-98), 3 PID gains per row (playground/clocktune grid sweep). Differential evolution already owns this niche, and bayesian optimization is the sample-starved upgrade for expensive arena objectives. Annealing adds nothing over either.

## broad-phase / spatial hashing

Already shipped. Candidate move generation is the broad-phase: dilate the occupied bitboard by SearchRingRadius rings, intersect with free in-region cells, then drop cells whose incremental static score is zero for both sides (engine/movegen.go:31-78). When the opponent holds a live four the set narrows to four-class cells, a forced-move superset (engine/movegen.go:40-51). Dirty-region tracking is exactly the incremental evaluator: a move touches only the windows through that cell (engine/eval.go:100-142). The one unexplored piece is maintaining the dilated ring incrementally across plies instead of recomputing dilation each node, but dilation is 2 bitboard iterations and is not on any measured profile. Nothing to add.

## wavefront threat maps

Needs prototype. Solver move ordering today is per-cell and local: class rank in the high bits, count of directions reaching the kind floor in the low bits (vcf/threat.go:158-176 threatScore). There is no spatial field, no notion of threat concentration or distance between attacker and defender resources. A multi-source BFS distance field over threat-carrying cells could reorder defender refutation enumeration and attacker sequencing on deep VCT plies, where node budget exhaustion at SolverNodeBudget (config.go:288) decides outcomes. Node counts are deterministic and load immune, so the experiment is cheap and machine independent. Seams: vcf/search.go attack and defend loops consuming threatScore, oracle cover in vcf/oracle_test.go. This is the rnd-search lane.

## wfc / procedural diversity

Fits, cheap version first. Corpus diversity today is 12 fixed opening cells cycled by game index (playground/bookgen/game.go:27-29 over config.BookOpeningCells) plus red alternation. Beyond ply 1 the corpus is deterministic master-tier play, the known narrowing risk. The corpus keys on stone masks so transpositions merge by construction, diversity has to come from the tree shape. Two candidate levers in cost order. First, small eval-weight perturbation seats for corpus generation only: the arena already registers weighted seats (playground/arena/main.go:56+), bookgen seats are built the same way through server.NewBotSearcher (playground/bookgen/game.go:32). Second, a constraint sampler over the first 4 plies, wfc style, seeding from the 12 cells with symmetry expansion. Full wfc is overkill for a 2 deep skeleton. Measure as unique positions per 100 games at ply 8 and 16. Benefit lands at the next book regrow, not the running one.

## mcts playouts + nn evaluation, the interface seam

Fits as a documented seam, adoption gated by arena evidence. The eval call site is a single leaf return in negamax (engine/search.go:319-321). At that point the feature vector is free: per cell per direction class pairs in ev.wpair (engine/eval.go:24-25), side sums, four counts, per-cell sums. Natural input is 256 cells times 4 directions of class-pair codes, roughly 1 KiB int8, or 2 side planes of 6 class channels. The cost law is the blocker: the shipped eval is O(1) incremental, a forward pass is 3 or more orders dearer, so the search shape must batch leaves, which is the lc0 architecture change, not a drop-in. Any adoption runs the strength gate, per the inversion research law. Numbers land in rnd-nneval.

## gpu batched eval, cpu search

Needs prototype, this is the standing revisit trigger. The prior assessment measured the floors: 7.069 µs kernel launch, 23.4 GB/s pinned PCIe (playground/gpuoffload/ANALYSIS.md:14-19), and closed every per-node offload because one node costs 0.7 µs total. It left the trigger: uniform throughput-shaped evaluation. The structural constraint for cpu search plus gpu eval: lazy smp workers each own a latency-bound DFS (engine/smp.go:354-395), a gpu batch needs one batching owner, so this forces either root-parallel subtrees or an mcts owner, never lazy smp as is. The prototype must answer: batched matmul evals per second on cpu and gpu, transfer-inclusive breakeven batch size, and whether speculative leaf batching inside a DFS preserves enough cutoffs to keep depth. That is the rnd-nneval lane, and its verdict gates any architecture talk.

## ant colony optimization

Ruled out. Pheromone construction suits high dimensional discrete combinatorics, routing and scheduling. The repo's optimization problems are 6 dimensional weight fitting and 3 dimensional gain tuning, both already served by differential evolution and grids, and both better served by bayesian optimization when objectives get expensive. No discrete construction space exists in the engine to exploit.

## bayesian heuristics as bayesian optimization

Fits, highest value tuning lane. The expensive black-box objectives are exactly where sampling cost hurts: arena win rate per weight vector costs minutes per game, clock drain deviation per gain set costs a sweep, ponder adoption quality costs a night. Grid sweeps cannot afford the evaluations (playground/clocktune runs a fixed grid, config.go:43-47 holds 4 authored rows). A gaussian-process optimizer with expected improvement buys the same confidence for a fraction of the games, and the seam already exists: weighted arena seats parameterize the objective (playground/arena/main.go:28+), clock has the NewGameClockWithGains tuning seam (internal/clock, per the clocktune contract), ponder constants config.go:102-106 are a future surface. The rnd-tune lane builds the harness and validates it against grid and random on synthetic objectives first.

## applied game theory

Fits, analysis first, strength changes only through arena gates. Three concrete questions. First, first-move advantage: quantify red versus blue win rate with confidence intervals from the accumulated arena logs, same-tier pairs only, to decide whether any balancing lever is even discussable. Second, the book as a game object: the committed law is majority plus solver override per position (config.go:221-222), which is pure strategy: an opponent who memorizes the book meets fully deterministic play. A mixed strategy over top 2 moves at probe time (internal/server/booksearcher.go) is a strength-bearing change needing arena evidence, the quantification comes first: what fraction of floor-passing positions hold a single-move majority. Third, ponder allocation as a bandit: the lane already predicts the opponent reply and scores hits (internal/server/ponder.go, AdoptPonder), framing time allocation as a Thompson sampler over prediction-hit rates is a design question answerable from logged M-lines. The rnd-gametheory lane runs all three on existing data, no new games.
