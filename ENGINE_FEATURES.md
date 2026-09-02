# Caro AI Engine Features

**Board:** 16x16 (256 intersections)
**Rule:** Caro (exactly-5 to win, open rule for red's second move)

---

## 1. Overview

This document provides comprehensive technical documentation of the AI engine's theoretical foundations, algorithmic structures, and design patterns. It serves as a reference for understanding the engine architecture without implementation details.

### Design Philosophy

The engine follows principles from state-of-the-art game-playing systems:
- **Rapfi** - Gomoku/Renju engine with BitKey pattern system
- **Stockfish** - Chess engine with proven alpha-beta optimizations
- **Computer Chess Theory** - Decades of research in game tree search

### Performance Target

- **Parallelism:** Lazy SMP with power-of-2 worker threads (largest power of 2 <= (ProcessorCount-2)/2)
- **Runtime:** .NET 10 with Server GC, CancellationToken cancellation, task-based concurrency

---

## 2. Search Architecture

### 2.1 Lazy SMP Parallel Search

Lazy SMP with all-equal workers sharing a single sharded TT — no master/slave split. Each worker runs its own iterative deepening loop independently.

**Core Principle:**
- All workers are identical: fresh local search board, shared TT, fresh heuristics
- Each worker runs iterative deepening from depth (1 + workerID % 2) to maxDepth
- Workers cooperate via shared TT — standard Lazy SMP pattern
- Best move selected by deepest completed depth; ties broken by score

**Worker Distribution:**
- Worker count: largest power of 2 <= (ProcessorCount-2)/2 (e.g., 20 cores -> 8 workers)
- Workers dispatched per-search as long-running tasks with a concurrent result bag
- Each worker maintains independent heuristics (killers, history)
- Shared TT provides inter-worker cooperation via hash move hints
- Ponder searches run the same machinery during the opponent's turn with
  fresh heuristics and no soft limit (see 6.3)

**Result Selection:**
- Deepest completed depth wins
- Score breaks ties at same depth
- Workers that complete more depths naturally contribute more results

**Advantages:**
- Shared TT eliminates redundant search across workers
- No master/slave coordination overhead
- CancellationToken provides clean cancellation
- Sharded TT avoids false sharing between workers

### 2.2 Principal Variation Search (PVS)

PVS is an enhancement to alpha-beta search that uses null-window searches to prove moves are suboptimal quickly.

**Implementation Scope:**
- Both paths (sequential `SearchPosition` and parallel Lazy SMP workers) share the same `SearchRoot`/`AlphaBeta` core with null-window searches and re-searches

**Algorithm Structure (Sequential Path):**
1. Search first move with full alpha-beta window
2. For remaining moves, search with null-window (alpha, alpha+1) or (beta-1, beta)
3. If null-window fails high, re-search with full window

**Theoretical Basis:**
- First move is usually best (good move ordering)
- Most moves can be disproven with minimal search
- Re-searches are rare with good ordering

**Complexity Reduction:**
- Standard alpha-beta: O(b^(d/2))
- PVS with good ordering: approaches O(b^(d/2)) with smaller constants

### 2.3 Aspiration Windows

Aspiration windows narrow the alpha-beta bounds around the expected score, reducing search effort.

**Mechanism:**
- Root search uses window centered on previous iteration's score
- Initial window size: ±AspirationWindow (from SearchHeuristicConstants)
- Failed searches widen incrementally: delta doubles each attempt (up to 3 widenings)
- Fail-high widens beta only; fail-low widens alpha only (single-direction)
- Full-window fallback after max widenings

**Benefits:**
- More cutoffs in alpha-beta
- Faster iterations in iterative deepening
- Better time usage estimation

### 2.4 Quiescence Search

Quiescence search extends the search at horizon positions to avoid tactical blunders.

**Purpose:**
- Evaluate only "quiet" positions (no immediate threats)
- Continue searching through forcing sequences
- Prevent horizon effect (bad moves hidden at depth limit)

**Implementation Characteristics:**
- Filters to only four-forcing moves (creates or blocks a four) via `GetTacticalCandidates`
- Stand-pat pruning skipped when opponent has forcing threats (forced response handling)
- Separate qsPly tracking prevents depth confusion with rootDepth
- Depth limit (4 quiescence plies) to prevent explosion
- Span-based zero-allocation candidate generation in hot path

### 2.5 Adaptive Late Move Reduction (LMR)

LMR reduces search depth for moves that are statistically less likely to be best.

**Reduction Factors:**
- Move ordering position (later moves reduced more)
- Current depth (deeper positions can afford more reduction)
- Move type (quiet moves reduced, threats not)
- Position improvement (improving positions reduced less)

**Adaptive Components:**
- Reduction varies by ply from root
- Node type (PV vs non-PV) affects reduction
- History scores modulate reduction

---

## 3. Transposition Table System

Single sharded transposition table shared across all search paths. In parallel search, all workers share the same TT instance via a per-shard monitor lock.

### 3.1 Sharded Lock Architecture

**Shard Distribution:**
- 16 independent segments, each protected by a plain monitor (`lock (shard.Gate)`)
- Hash-based index calculation: `shardIndex = (hash >> 32) & 0xF`
- The port benchmarked `ReaderWriterLockSlim` against the monitor and kept the
  monitor: the per-slot critical sections are short enough that reader-writer
  bookkeeping cost more than the exclusivity it saved (see STATS.md)

**Depth-Age Replacement:**
- Priority formula: depth - 8 * age
- Same-hash entries: deeper entry always kept (shallow overwrites rejected)
- Different-hash entries: lower priority entry rejected
- Age increments once per official move via `IncrementAge()`; ponder
  searches write under the current age without bumping it

**Stats Tracking:**
- `probes` and `hits` atomic counters for hit rate computation
- Shared across all workers in parallel search
- Hit rate = total hits / total probes from all workers

### 3.2 Entry Structure

Each TT entry stores:
- **Hash Key** - Position identification (truncated)
- **Depth** - Search depth of stored result
- **Bound Type** - Exact, lower bound (beta cutoff), or upper bound (alpha cutoff)
- **Score** - Position evaluation
- **Best Move** - Principal variation move
- **Age** - Search generation, used by the depth-age replacement formula

### 3.3 Shared TT Write Policy

All workers in Lazy SMP share a single TT instance. Writes are coordinated via
per-shard monitor locks to prevent data races. Each worker writes at all depths
to the shared TT, providing cross-worker move hints via hash moves.
The depth-age replacement strategy handles entry quality naturally.

---

## 4. Move Ordering System

### 4.1 Ordering Priority

Move ordering is critical for alpha-beta efficiency. The engine uses staged generation with strict priority:

| Priority | Stage | Description |
|----------|-------|-------------|
| 1 | TT_MOVE | Transposition table move, searched unconditionally first |
| 2 | WINNING_MOVE | Creates winning position (open four, double threat) |
| 3 | MUST_BLOCK | Mandatory defense against opponent's open four or five threat |
| 4 | THREAT_CREATE | Creates threats (open three, broken four) |
| 5 | KILLER_COUNTER | Killer moves and counter-move responses combined |
| 6 | QUIET | All remaining quiet moves, sorted by history/killer/continuation/center/proximity score |

### 4.2 Staged Move Picker

Moves are generated and scored in stages, allowing early termination on cutoffs.

**One picker:** `MovePicker` stages generation so cutoffs skip whole stages, and an exact-dedup bitmap prevents a move from yielding twice. `OrderMoves` is the all-at-once wrapper used at the root.

**Stage Sequence:**
1. **TT_MOVE** - Single move from the transposition table, yielded first
2. **WINNING_MOVE** - Completes an exact five
3. **MUST_BLOCK** - Opponent would complete a five here
4. **THREAT_CREATE** - Four/three creation or denial, sorted by additive sub-scores below
5. **KILLER_COUNTER** - Killer moves and the counter-move response, in that order
6. **QUIET** - History/killer/continuation score + center bias + proximity

An exact-dedup bitmap prevents a move from yielding twice across stages.

**Scoring (`MovePicker.cs`):** the threat stage scores each placement with
additive sub-scores for own and opponent shapes; the quiet stage sums history,
killer, continuation, center, and proximity terms:

| Term | Score |
|------|-------|
| Own open four | +700,000 |
| Own four | +400,000 |
| Own flex three | +300,000 |
| Opponent open four | +500,000 |
| Opponent four | +350,000 |
| Opponent flex three | +200,000 |
| Killer 1 / Killer 2 (quiet stage) | +500,000 / +400,000 |
| Quiet history | 2x butterfly history, capped at 300,000 |
| Continuation history | up to +30,000 |
| Quiet center bias | (28 - manhattan distance to center) x 100 |
| Quiet proximity | 10 x occupied cells in the 5x5 neighborhood |

Stage order, not the numeric scores, is what separates winning/must-block from
threats: the threat stage only sees moves that survived the earlier stages.

### 4.3 Continuation History

Tracks move pairs across consecutive plies to identify good move sequences.

**Structure:**
- Dimensions: [player, previous_cell, current_cell]
- Score range: 0 to +30,000
- Keyed on the immediately preceding move (move pairs, not longer sequences)

**Update Mechanism:**
- Bonus for moves causing cutoffs: depth^2 * 3
- Bounded updates prevent overflow (capped at 30,000)

**Usage:**
- Contributes to quiet move scoring in the move picker

### 4.4 Counter-Move History

Captures move-response patterns: which moves work well against specific opponent moves.

**Structure:**
- Dimensions: [player, opponent_move, our_move]
- Mirrors continuation history bounds
- Integrates with move picker scoring

**Purpose:**
- Captures tactical responses
- Improves ordering in forced sequences
- Complements continuation history

### 4.5 Killer Moves

Stores moves that caused beta cutoffs at sibling nodes.

**Characteristics:**
- Two slots per ply
- FIFO replacement (oldest evicted)
- Moves likely good at sibling nodes

**Scoring:**
- Fixed score for killer moves
- Combined with history for quiet moves

### 4.6 History Heuristic

General-purpose move ordering based on past performance.

**Butterfly History:**
- Tracks move performance globally
- Dimensions: [player, x, y] (stones have no from-square, so cells are keyed directly)
- Long-term statistics across game

**Update Policy:**
- Successful cutoffs: depth^2 bonus
- Values clamped to 1,000,000

---

## 5. Evaluation System

### 5.1 Line-Window Pattern Analysis

`PatternWindow.cs` classifies threats from 11-cell line windows centered on a
stone: any exact five through the center plus both end-check cells fits in the
window. All analysis is allocation-free array work on the extracted line.

**Primitives:**
- `extractLine` - read the 11 cells relative to a player (own/empty/opponent)
- `spanThrough` - maximal contiguous own run containing the center, with an
  optional single fill cell (this is what makes split fours visible)
- `lineCompletions` - empty cells adjacent to the span whose fill makes an
  exact five through the center (at most two candidates exist)
- `maxCompsAfterFill` - best completion count reachable by one more stone;
  >=2 means the shape can become an open four (flex three class)

### 5.2 Pattern4 Classification

Combined 4-direction threat classification for each position. Enum values are
distinct members of one `Pattern4` type in `Pattern4.cs`; each direction is
classified independently and summed into `PlayerPattern4` counts.

**Pattern Categories:**

| Category | Value | Description |
|----------|-------|-------------|
| P4None | 0 | No significant pattern |
| P4Flex1 | 1 | Single stone |
| P4Flex2 | 3 | Open two (both ends empty) |
| P4Block2 | 4 | Blocked two |
| P4Flex3 | 5 | Three that can still become an open four (includes broken threes like .X.XX.) |
| P4Block3 | 6 | Three with a single continuation |
| P4Flex4 | 7 | Four with two completion squares (includes split fours like .XX.XX.) |
| P4Block4 | 8 | Four with one completion square |
| P4Exactly5 | 9 | Win condition |
| P4Overline | 10 | Invalid (exactly-5 rule) |

**Caro-Specific Rules:**
- Overlines (6+) don't count as wins (P4Overline)
- Blocked fours can still win (opponent can't block both ends)
- Double threats are not enum classes; they are derived from the counts during evaluation (see 5.3)
- ClassifyStone skips directions anchored by a same-color stone so gapped clusters are counted once

### 5.3 Combination Bonuses

Evaluation has no separate cache; threat combinations are read directly from the
`PlayerPattern4` counts. The highest matching category wins (`Evaluation.cs`):

| Combination | Bonus |
|------------|-------|
| Exactly5 | 30,000 (= WinScore) |
| Flex4 present | 15,000 |
| Double blocked four | 14,000 |
| Blocked four + open three | 13,000 |
| Double open three | 12,000 |

Below those thresholds, patterns score linearly: Flex4 10,000, Block4 5,000,
Flex3 1,000, Block3 100, Flex2 100, Block2 30, plus 10 per stone and a center
bonus of 2 * (BoardSize - manhattan distance to center).

### 5.4 Scoring System

Position evaluation combines multiple factors:

**Score Hierarchy:**

| Constant | Value | Purpose |
|----------|-------|---------|
| Infinity | 100,000 | Initial alpha/beta bounds |
| WinScore | 30,000 | Terminal win (mate-distance adjusted: WinScore - ply) |
| MaxEval | 25,000 | Non-win evaluation clamp |
| fiveScore | 30,000 (= WinScore) | Static eval for five-in-a-row |

**Score Boundaries:**
- Evaluation scores clamped to ±25,000 (MaxEval)
- Terminal win score: 30,000 (WinScore), reduced by ply from root for mate-distance preference
- Mate scores stored in TT with ply adjustment to normalize across depths

**Zero-Sum Property:**
- Evaluation is strictly zero-sum: `Evaluate(board, Red) == -Evaluate(board, Blue)`
- Score = `playerScore - opponentScore` + `centerBonus(player) - centerBonus(opponent)`
- Removed asymmetric defense multiplier for correct alpha-beta bounds

---

## 6. Time Management

### 6.1 Phase-Aware Time Allocation

`AllocateTime` in `TimeManager.cs` divides the remaining clock by a phase
divisor (25 early, 30 after move 25), adds a fraction of the increment, and
caps the result at 40% of the remaining clock.

**Safety Features:**
- Hard bound never negative; any live clock gets at least a 100ms floor
- Soft bound at 80% of optimal stops iterative deepening early when the
  next depth will not finish
- 50ms reserve always kept back from the hard bound
- The server adjudicates flag fall, and the engine's per-move budget stays
  under the remaining clock

### 6.2 Time Control Support

| Control | Initial | Increment | Use Case |
|---------|---------|-----------|----------|
| Bullet | 1 min | 0 sec | Speed games |
| Blitz | 3 min | 0 sec | Quick games |
| Blitz | 3 min | 2 sec | Quick games |
| Rapid | 7 min | 5 sec | Standard games |
| Rapid | 10 min | 0 sec | Standard games |
| Classical | 15 min | 10 sec | Long games |

### 6.3 Pondering

Grandmaster (L5) bots search on the opponent's clock. When an L5 move
commits, the session reads the opponent's predicted reply from the TT entry
the search just stored (`PredictReply`) and launches a background search on
the resulting position (`StartPonder`, owned by the mover's `MinimaxAI`).

**Mechanics:**
- Shares the AI's TT, uses a fresh `SearchHeuristics`, never bumps the TT
  age, and never touches the AI's official stats
- `SoftLimitMs` disabled: iterative deepening runs until the wall-clock cap
  or `MaxDepth`. The cap is derived per move from the opponent's live
  remaining clock (they must move or flag within it), so it scales with the
  time control instead of a fixed constant
- The prediction comes from a searched PV node, so legality (including the
  open rule) is inherent; entries without depth or pointing at occupied
  cells are rejected, and turns without a prediction simply do not ponder

**Hit handling (continuation search):**
- The pondered move is never played directly. Whatever the opponent
  replied, the next `ai-move` runs the normal budgeted search; pondering
  pays off through the TT the background search warmed (hash moves,
  cutoffs, deeper iterations inside the same budget)
- A hit (the opponent played the predicted reply and the ponder completed
  at least one depth) is recorded for stats only: `[PONDER]` statline tag
  on the real search, `ponder_depth`/`ponder_nodes` persisted whenever a
  ponder preceded the move, hit or miss
- Instant-move adoption was tried and removed by measurement: against
  mid-level opponents whose think time clears a time-based adoption gate,
  L5 cashed in depth-4-5 pondered moves instead of its depth-6-8 searches
  and lost the matchup (L3 13-7, L4 11-9 at 1+0). The warm-table approach
  keeps full search strength on every move

**Lifecycle and teardown:**
- At most one ponder per session (the latest mover); every applied move
  stops the previous ponderer before starting the mover's
- Joins under the session mutex are safe: the ponder task takes no
  session or store locks, and cancellation is node-granular (the ID loop,
  alpha-beta, quiesce, and VCF all poll `ShouldStop`, which honors the
  cancelled context inline)
- Every teardown path (game over, flag fall, undo, delete, janitor sweep,
  shutdown) funnels through `DisposeAI`, which joins the ponder before
  `tt.Dispose` nils the shard slices — a straggler search would otherwise
  panic
- `CARO_DISABLE_PONDER=1` disables pondering process-wide

**Trade-off:** in L5-vs-L5 games both sides can search at once (one
pondering, one on the clock), contending for cores; wall-clock budgets
absorb the contention.

---

## 7. Domain-Specific Features

### 7.1 Board Representation

Immutable board design with pre-computed AI optimization data.

**Architecture:**
- Board is immutable - operations return new instances
- Cell uses struct with Player field for value semantics
- Pre-computed bitboards and hash updated incrementally

**Performance Optimization:**
- BitBoards: `uint64[4]` arrays (256 bits for 16x16 board)
- Hash: Zobrist-style XOR updated on each move; dedicated null-move key prevents TT poisoning
- O(1) access during AI search instead of O(n^2) iteration

**Memory Layout:**
```
BitIndex = y * 16 + x
UlongIndex = BitIndex / 64
BitOffset = BitIndex % 64
```

**Trade-offs:**
- Immutable Board copies 256 cells on PlaceStone
- SearchBoard uses make/unmake pattern (zero allocation)
- NPS improved 100x+ with mutable SearchBoard

### 7.2 VCF Solver

Victory by Continuous Fours - tactical solver for forcing win sequences.

**Purpose:**
- Detect forced wins before main search
- Search specifically for four-in-a-row sequences
- Prune positions with known outcomes

**Result Types:**
- `VCFWin` - forced win sequence found
- `VCFNoWin` - proven no forced win exists (exhaustive search)
- `VCFTimeout` - search timed out before proving win or no-win

**Integration:**
- Runs before alpha-beta search
- Own-side solve is skipped when the opponent has an immediate win (flex4 or double block4) - the pre-solve would waste time on a lost race (this guard is part of the VCF pre-solve, not null-move pruning)
- Block hint: when the opponent has a proven VCF, the re-solved block square is passed to alpha-beta as the preferred first root move (full search still runs; the hint is only trusted on `VCFNoWin` verification, not `VCFTimeout`)
- Overline validation in `findFourBlocks`: checks cells beyond block squares for overline
- Candidate radius reduced to 2 (fours are always adjacent)
- Depth-limited for practical use (per-level caps in 9.3)

### 7.3 Exactly-5 Validation

Caro rule requires exactly 5 stones (not 6+) to win.

**Detection:**
- Win detector checks for 5-in-a-row
- 6+ in row doesn't count
- Both ends blocked = not a win

**Evaluation Impact:**
- Overlines scored as neutral
- Blocked fours can still win
- Double threats prioritized

### 7.4 Threat Detection Philosophy

The engine uses search-based threat evaluation rather than reflexive blocking.

**Key Principle:**
- Threat blocks are added to candidate list, not returned immediately
- Search evaluates offensive options alongside defensive blocks
- AI maintains strategic initiative instead of reactive blocking

**Threat Categories:**
- **Open Four** (4-in-a-row, both ends open): 2 winning squares - inherently unblockable
- **Open Three** (3-in-a-row, both ends open): Becomes open four next move
- **Broken Four** (4 with gap): Single winning square - can be blocked
- **Double Threat**: Two independent threats - usually winning

**Blocking Strategy:**
- Open three blocks: Added to candidates, search chooses best move
- Open four blocks: Mandatory (but often too late - prevention required)
- Multiple threats: Search evaluates which to address

**Design Rationale:**
- Immediate blocking forces reactive play
- Search-based evaluation maintains strategic flexibility
- Prevents "strength inversion" where weaker AI exploits predictable blocking

### 7.5 Open Rule

Red's second move must be at Chebyshev distance >= 3 from first (outside 5x5 zone centered on first stone).

**Implementation:**
- Enforced at game logic level via Chebyshev distance: `max(|dx|, |dy|) >= 3`
- Move generation filters invalid moves
- Frontend highlights invalid cells during Red's 2nd move

---

## 8. Concurrency Model

### 8.1 Thread Safety

All shared data structures designed for concurrent access.

**Immutable State:**
- Game state is immutable
- Operations return new instances
- No shared mutable state in game logic

**Thread-Safe Structures:**
- TT with 16 shards, each protected by a monitor lock
- Search workers as long-running tasks with a concurrent result bag
- Independent history tables per worker

### 8.2 Cancellation

Coordinated search cancellation via CancellationToken.

**Mechanism:**
- HTTP request context propagated through GetBestMove to search dispatch
- Derived context combines external cancellation with internal time-monitor
- Long-running task workers respect context cancellation
- Clean termination on timeout, stop command, or client disconnect
- Ponder tasks cancel the same way; `StopPonder` cancels and joins,
  and `GetBestMove`/`Dispose` drain any running ponder first (see 6.3)

### 8.3 Statistics Collection

Atomic counters collect search telemetry without blocking hot paths.

**Counters:**
- `TimeMonitor.Nodes`: `Interlocked.Increment` at entry of search nodes
- `TranspositionTable.probes` / `hits`: `Interlocked` TT lookup statistics
- Elapsed time from `TimeMonitor.startTime`

**Aggregation:**
- Sequential search: direct counter reads
- Parallel search: shared `Nodes` counter and shared TT stats across all workers

---

## 9. UCI Protocol

### 9.1 Command Support

Standard UCI commands for engine control:

| Command | Description |
|---------|-------------|
| uci | Initialize engine |
| isready | Check engine ready |
| ucinewgame | Reset for new game |
| position | Set board position |
| go | Start search |
| stop | Stop search |
| quit | Exit engine |
| setoption | Set engine option |

### 9.2 Engine Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| Threads | spin | 4 | Search workers (1-Max); L4/L5 auto-scale to Pow2((N-2)/2) via difficulty profile |
| Hash | spin | 256 | TT size (MB) |
| Skill Level | spin | 5 | Difficulty 1-5 (1=Novice, 5=Grandmaster) |

### 9.3 Difficulty Levels (Skill Levels)

Strength-based difficulty via `DifficultyProfile`: depth caps make level differences hold on any machine, with the time fraction as a secondary cap.

| Level | Name | Depth Cap | Time Fraction | Threads | Parallel | VCF | VCF Depth | Ponder | TT Size |
|-------|------|-----------|---------------|------------|----------|-----|-----------|--------|---------|
| 1 | Novice | 2 | 5% | 1 | No | No | 0 | No | 64MB |
| 2 | Beginner | 4 | 15% | 1 | No | No | 0 | No | 64MB |
| 3 | Intermediate | 4 | 40% | 2 | Yes | Yes | 2 | No | 256MB |
| 4 | Advanced | 5 | 70% | Pow2((N-2)/2)/2 | Yes | Yes | 4 | No | 1GB |
| 5 | Grandmaster | 50 | 100% | Pow2((N-2)/2) | Yes | Yes | 12 | Yes | 1GB |

**How it works:**
- `MaxDepth`: The primary strength knob up to the measured plateau: past depth ~6
  at bullet time controls, extra ID depth stops buying strength in self-play, so
  L3/L4 caps stay at 4/5 and the ladder scales `VCFDepth` (solver sight) instead.
- `TimeFraction` (0.0-1.0): Secondary cap on the allocated search time.
- `UseVCF` / `VCFDepth`: Disabling the pre-search VCF solver removes tactical
  precision at low levels; mid levels see the opponent's forced-four threats
  only to a shallow solver depth.
- Thread count scales with difficulty: level 1-2 single-threaded, level 3 dual-threaded, level 4-5 adaptive to hardware.
- Level 4 uses half of L5's thread count (next power of 2 down).
- `Ponder`: L5 only; background search on the predicted reply during the
  opponent's turn (see 6.3).
- Level 5 = full-strength engine with all optimizations.

**Per-player difficulty:** The HTTP API accepts `redDifficulty` and `blueDifficulty` independently, allowing asymmetric matches (e.g., L5 vs L1).

### 9.4 Move Notation

Two-character algebraic notation for Caro:
- First character: row (y), a-p for 0-15
- Second character: column (x), a-p for 0-15
- Example: bd = row 1, column 3; pp = row 15, column 15

---

## 10. Source Code Organization

### 10.1 Project Layout (`backend/`)

| Project | Files | Responsibility |
|---------|-------|---------------|
| `Caro.Domain` | Board.cs, GameState.cs, Player.cs, Position.cs, Zobrist.cs, Win.cs, Constants.cs, CaroException.cs, GameMode.cs, OpenRule.cs | Domain entities, game rules, no dependencies |
| `Caro.Engine` | MinimaxAI.cs, Search.cs, AlphaBeta.cs, Quiescence.cs, ParallelSearch.cs, Evaluation.cs, PatternWindow.cs, Pattern4.cs, Vcf.cs, TranspositionTable.cs, MovePicker.cs, Candidates.cs, SearchHeuristics.cs, TimeManager.cs, TimeMonitor.cs, Difficulty.cs, SearchBoard.cs, BitBoard.cs, Ponder.cs, SearchTypes.cs, IterationBudget.cs | AI engine, search algorithms |
| `Caro.Uci` | UciHandler.cs, Notation.cs | UCI protocol handling |
| `Caro.Api` | GameHandlers.cs, MovePersistence.cs, UciWebSocket.cs, GameSession.cs, GameSession.Ponder.cs, GameStore.cs, Contracts.cs, Middleware.cs, Statline.cs, EndpointRoutes.cs, ApiApp.cs, Log.cs, ResponseJson.cs | HTTP/WebSocket API |
| `Caro.Persistence` | MatchStore.cs | Structured match persistence (SQLite) |

### 10.2 Centralized Constants

| File | Constants |
|------|-----------|
| `Caro.Domain/Constants.cs` | BoardSize, WinLength, Infinity, WinScore, MaxEval, search thresholds (LMR, null-move, aspiration, quiescence), TT shard count, VCF search depth, time management (phase divisors, soft/hard bounds, buffer), cell counts |
| `Caro.Engine/Search.cs` | Search orchestration logic (constants moved to domain) |
| `Caro.Engine/MovePicker.cs` | Staged picker score thresholds |
| `Caro.Engine/Evaluation.cs` | Pattern scores, center bonus weights |
| `Caro.Engine/Difficulty.cs` | L1-L5 difficulty profiles, thread counts |

### 10.3 Main Engine Files

**Caro.Engine/** (all files <= 400 SLOC):

| File | Role |
|------|------|
| `MinimaxAI.cs` | MinimaxAI class definition, constructor, public API, Dispose |
| `Search.cs` | Iterative deepening, aspiration windows, VCF preferred move hint |
| `AlphaBeta.cs` | PVS alpha-beta, LMR (with tactical guard), null-move pruning (depth>=4, reduction=2), root search |
| `Quiescence.cs` | Four-forcing quiescence, mate-score adjustment helpers |
| `ParallelSearch.cs` | Lazy SMP worker dispatch, result aggregation |
| `Evaluation.cs` | Zero-sum Pattern4-based evaluation with combination bonuses and center bonus |
| `PatternWindow.cs` | 11-cell line-window primitives over Span<sbyte>: ExtractLine, SpanThrough, LineCompletions, placement analysis |
| `Pattern4.cs` | 4-direction threat classification on window primitives (Flex/Block patterns, combined counts) |
| `Vcf.cs` | Victory by Continuous Fours pre-search solver |
| `TranspositionTable.cs` | Sharded TT (per-shard monitor) with depth-age replacement |
| `MovePicker.cs` | Staged move ordering (6 stages: TT -> Win -> Block -> Threat -> Killer/Counter -> Quiet) |
| `Candidates.cs` | Radius-2 neighborhood candidate generation (3x3 center seed on empty board), four-forcing tactical filter, open-rule filter |
| `SearchHeuristics.cs` | Killer moves, continuation/butterfly/counter-move history |
| `TimeManager.cs` | Phase-aware time allocation with clock safety floors |
| `IterationBudget.cs` | Predicted iteration-cost gating against the soft budget |
| `TimeMonitor.cs` | CancellationToken-aware search time monitoring with a 10ms watchdog |
| `Difficulty.cs` | Hardware-agnostic L1-L5 difficulty profiles |
| `SearchBoard.cs` | Mutable board for search hot path (make/unmake, zero allocation) |
| `BitBoard.cs` | BitBoard struct with ulong operations (hardware PopCount) |
| `Ponder.cs` | Background ponder lifecycle on MinimaxAI (predict, start, stop-and-consume) |

**UCI Project** (`Caro.Uci/`):
| File | Role |
|------|------|
| `UciHandler.cs` | UCI command dispatcher, search controller, engine options (Threads, Hash, Skill Level) |
| `Notation.cs` | Double-letter coordinate encoding/decoding |
## 11. References

### Source Repositories
- **Rapfi:** https://github.com/dhbloo/rapfi
- **Stockfish:** https://github.com/official-stockfish/Stockfish

### Documentation
- **Chess Programming Wiki:** https://www.chessprogramming.org/
- **Stockfish Wiki:** https://www.chessprogramming.org/Stockfish

### Key Topics
- [Transposition Table](https://www.chessprogramming.org/Transposition_Table)
- [History Heuristic](https://www.chessprogramming.org/History_Heuristic)
- [Late Move Reductions](https://www.chessprogramming.org/Late_Move_Reductions)
- [Lazy SMP](https://www.chessprogramming.org/Lazy_SMP)
- [Continuation History](https://www.chessprogramming.org/Continuation_History)

### Key Papers
1. "History Heuristic" - J. Schaeffer
2. "Late Move Reductions" - E.A. Heinz
3. "NNUE: Efficiently Updatable Neural Networks" - Y. Nasu
