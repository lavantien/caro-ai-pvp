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

- **Speedup:** 100-500x over naive minimax
- **Parallelism:** Lazy SMP with power-of-2 goroutines (largest power of 2 <= (GOMAXPROCS-2)/2)
- **Runtime:** Go 1.26 with Green Tea GC, context.Context cancellation, channel-based concurrency

---

## 2. Search Architecture

### 2.1 Lazy SMP Parallel Search

Lazy SMP with all-equal goroutines sharing a single sharded TT — no master/slave split. Each worker runs its own iterative deepening loop independently.

**Core Principle:**
- All goroutines are identical: fresh local search board, shared TT, fresh heuristics
- Each worker runs iterative deepening from depth (1 + workerID % 2) to maxDepth
- Workers cooperate via shared TT — standard Lazy SMP pattern
- Best move selected by deepest completed depth; ties broken by score

**Goroutine Distribution:**
- Goroutine count: Largest power of 2 <= (GOMAXPROCS-2)/2 (e.g., 20 cores -> 8 goroutines)
- Workers dispatched per-search via goroutine pool with result channel
- Each goroutine maintains independent heuristics (killers, history)
- Shared TT provides inter-worker cooperation via hash move hints

**Result Selection:**
- Deepest completed depth wins
- Score breaks ties at same depth
- Workers that complete more depths naturally contribute more results

**Advantages:**
- Shared TT eliminates redundant search across workers
- No master/slave coordination overhead
- context.Context provides clean cancellation
- Sharded RWMutex TT avoids false sharing between goroutines

### 2.2 Principal Variation Search (PVS)

PVS is an enhancement to alpha-beta search that uses null-window searches to prove moves are suboptimal quickly.

**Implementation Scope:**
- Sequential path (MinimaxAI): Full PVS with null-window searches and re-searches
- Parallel path: Alpha-beta with Move-Dependent Adaptive Pruning (MDAP/LMR) and aspiration windows; traditional PVS not applied

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
- Filters to only tactical (forcing) moves via TacticalEvaluator and Pattern4Evaluator
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

Single sharded RWMutex transposition table shared across all search paths. In parallel search, all workers share the same TT instance via per-shard `sync.RWMutex`.

### 3.1 Sharded RWMutex Architecture

**Shard Distribution:**
- 16 independent segments, each protected by `sync.RWMutex`
- Hash-based index calculation: `shardIndex = (hash >> 32) & 0xF`
- Reads use `RLock` (concurrent), writes use `Lock` (exclusive)
- Race-detector compatible

**Depth-Age Replacement:**
- Priority formula: depth - 8 * age
- Higher priority entries kept; lower priority entries overwritten
- Age increments per search iteration via `IncrementAge()`

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
- **Static Eval** - Cached static evaluation

### 3.3 Shared TT Write Policy

All workers in Lazy SMP share a single TT instance. Writes are coordinated via
per-shard `sync.RWMutex` to prevent data races. Each worker writes at all depths
to the shared TT, providing cross-worker move hints via hash moves.
The depth-age replacement strategy handles entry quality naturally.

---

## 4. Move Ordering System

### 4.1 Ordering Priority

Move ordering is critical for alpha-beta efficiency. The engine uses staged generation with strict priority:

| Priority | Stage | Description |
|----------|-------|-------------|
| 1 | TT_MOVE | Transposition table move, searched unconditionally first |
| 2 | MUST_BLOCK | Mandatory defense against opponent's open four or five threat |
| 3 | WINNING_MOVE | Creates winning position (open four, double threat) |
| 4 | THREAT_CREATE | Creates threats (open three, broken four) |
| 5 | KILLER_COUNTER | Killer moves and counter-move responses combined |
| 6 | GOOD_QUIET | Quiet moves with high history scores (>500) |
| 7 | BAD_QUIET | Remaining quiet moves |

### 4.2 Staged Move Picker

Moves are generated and scored in stages, allowing early termination on cutoffs.

**Two ordering systems:**
- `MovePicker` (parallel path): Large absolute scores from `MoveOrderingConstants` for strict priority separation
- `MoveOrderer` (sequential path): Compact scale (TT=10K, EmergencyDefense=5K, Killer=1K) for tiebreaking within stages

**Stage Sequence (MovePicker - parallel path):**
1. **TT_MOVE** - Single move from transposition table (1M score)
2. **MUST_BLOCK** - Mandatory blocks against opponent's winning threats (2M score)
3. **WINNING_MOVE** - Creates open four or double threat (1.5M score)
4. **THREAT_CREATE** - Creates open three or broken four (800K score)
5. **KILLER_COUNTER** - Killer moves (400K-500K) + counter-move responses (150K)
6. **GOOD_QUIET** - Quiet moves with continuation + butterfly history > 500
7. **BAD_QUIET** - Remaining quiet moves

**Score Constants (MoveOrderingConstants):**
| Category | Score |
|----------|-------|
| Must Block | 2,000,000 |
| Winning Move | 1,500,000 |
| TT Move | 1,000,000 |
| Threat Create | 800,000 |
| Killer 1 | 500,000 |
| Killer 2 | 400,000 |
| Counter Move | 150,000 |
| Continuation Max | 300,000 |
| History Max | 30,000 |
| Good Quiet Threshold | 500 |

### 4.3 Continuation History

Tracks move pairs across consecutive plies to identify good move sequences.

**Structure:**
- Dimensions: [player, previous_cell, current_cell]
- Score range: -30,000 to +30,000
- Update formula with overflow prevention

**Update Mechanism:**
- Bonus for moves causing cutoffs
- Penalty for moves that didn't cause cutoffs
- Bounded updates prevent overflow

**Ply Span:**
- Tracks 6 plies of history
- Recent plies weighted more heavily
- Contributes to quiet move scoring

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
- Dimensions: [player, from_cell, to_cell]
- Long-term statistics across game

**Update Policy:**
- Successful cutoffs: positive bonus
- Failed moves: negative penalty
- Gravity formula prevents extreme values

---

## 5. Evaluation System

### 5.1 BitKey Pattern System

O(1) pattern lookup using 64-bit keys with bit rotation for board alignment.

**Principle:**
- Board positions encoded as bit sequences
- 2 bits per cell (empty, red, blue)
- Rotation aligns patterns around position being evaluated

**Directional Keys:**
- Horizontal: Row-based bitkeys
- Vertical: Column-based bitkeys
- Diagonal: Index-sum based bitkeys
- Anti-diagonal: Index-difference based bitkeys

**Pattern Extraction:**
- Rotate bitkey to center evaluation position
- Extract relevant bits for pattern window
- Lookup pattern classification in table

### 5.2 Pattern4 Classification

Combined 4-direction threat classification for each position.

**Pattern Categories:**

| Category | Threat Level | Description |
|----------|--------------|-------------|
| None | 0 | No significant pattern |
| Flex1 | 1 | Single stone with potential |
| Block1 | 1 | Single blocked stone |
| Flex2 | 2 | Open two |
| Block2 | 2 | Blocked two |
| Flex3 | 4 | Open three (must defend) |
| Block3 | 3 | Blocked three |
| Flex4 | 8 | Open four (winning threat) |
| Block4 | 4 | Blocked four |
| DoubleFlex3 | 16 | Two open threes (winning) |
| Flex4Flex3 | 32 | Open four + open three (winning) |
| Exactly5 | 64 | Win condition |
| Overline | 0 | Invalid (exactly-5 rule) |

**Caro-Specific Rules:**
- Overlines (6+) don't count as wins
- Blocked fours can still win (opponent can't block both ends)
- Double threats are winning

### 5.3 Evaluation Cache

Stores static evaluation corrections for position reuse.

**Purpose:**
- Avoid redundant evaluation computation
- Correction values improve accuracy
- Integrated with TT storage

**Mechanism:**
- Static eval cached in TT entry
- Correction applied on TT hit
- Reduces evaluation calls

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

**Defense Multiplier:**
- Defense valued at 3/2 of offense
- Prevents opponent threats prioritized

---

## 6. Time Management

### 6.1 PID Time Manager

Uses control theory principles for time allocation.

**Components:**
- **Proportional (weight=0.6):** React to current error
- **Integral (weight=0.3, gain=0.1):** Account for accumulated error, clamped at 0.5
- **Derivative (weight=0.1):** Predict future error

**Mechanism:**
- Target: optimal time per move
- Feedback: actual time used vs. remaining
- Output: time allocation for next move

**Safety Features:**
- Integral windup clamping
- Minimum time reserve
- Emergency stop for low time

### 6.2 Time Control Support

| Control | Initial | Increment | Use Case |
|---------|---------|-----------|----------|
| Bullet | 1 min | 0 sec | Speed games |
| Blitz | 3 min | 2 sec | Quick games |
| Rapid | 7 min | 5 sec | Standard games |
| Classical | 15 min | 10 sec | Long games |

### 6.3 Pondering

Background search during opponent's turn (planned).

**Planned Characteristics:**
- Enabled for L5 (Grandmaster) via DifficultyProfile
- Shares TT between ponder and main search (single MinimaxAI instance)
- Context cancellation terminates ponder cleanly
- Ponder hit reuses pre-computed result

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
- Hash: Zobrist-style XOR updated on each move
- O(1) access during AI search instead of O(n^2) iteration
- SIMD evaluation path via experimental simd/archsimd (build tag: goexperiment.simd; planned)

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

**Integration:**
- Runs before alpha-beta search
- Results cached for reuse
- Depth-limited for practical use

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
- TT with 16 shards, each protected by `sync.RWMutex` (concurrent reads, exclusive writes)
- Go channels for async communication
- Independent history tables per goroutine
- sync.Pool for SearchBoard reuse

### 8.2 Cancellation

Coordinated search cancellation via context.Context.

**Mechanism:**
- HTTP request context propagated through GetBestMove to search dispatch
- Derived context combines external cancellation with internal time-monitor
- Channel-based worker pool respects context cancellation
- Clean termination on timeout, stop command, or client disconnect

### 8.3 Statistics Collection

Atomic counters collect search telemetry without blocking hot paths.

**Counters:**
- `TimeMonitor.Nodes` (`atomic.Int64`): incremented at entry of search nodes
- `TranspositionTable.probes` / `hits` (`atomic.Int64`): TT lookup statistics
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
| Threads | spin | 4 | Search goroutines (1-Max); L4/L5 auto-scale to Pow2((N-2)/2) via difficulty profile |
| Hash | spin | 1024 | TT size (MB) |
| Ponder | check | false | Enable pondering |
| Skill Level | spin | 5 | Difficulty 1-5 (1=Novice, 5=Grandmaster) |

### 9.3 Difficulty Levels (Skill Levels)

Hardware-agnostic difficulty via `DifficultyProfile` -- search parameters scale independently of machine speed.

| Level | Name | Time Fraction | Goroutines | Pondering | Parallel | VCF |
|-------|------|---------------|------------|-----------|----------|-----|
| 1 | Novice | 5% | 1 | No | No | No |
| 2 | Beginner | 15% | 1 | No | No | No |
| 3 | Intermediate | 40% | 2 | No | Yes | Yes |
| 4 | Advanced | 70% | Pow2((N-2)/2)/2 | No | Yes | Yes |
| 5 | Grandmaster | 100% | Pow2((N-2)/2) | Planned | Yes | Yes |

**How it works:**
- `TimeFraction` (0.0-1.0): Post-PID multiplier on allocated search time. Level 1 uses 5% of allocated time; level 5 uses full budget.
- `UseVCF`: Disabling the pre-search VCF solver removes tactical precision at low levels.
- Goroutine count scales with difficulty: level 1-2 single-goroutine, level 3 dual-goroutine, level 4-5 adaptive to hardware.
- Level 4 uses half of L5's goroutine count (next power of 2 down).
- Level 5 = full-strength engine with all optimizations.

**Per-player difficulty:** The HTTP API accepts `redDifficulty` and `blueDifficulty` independently, allowing asymmetric matches (e.g., L5 vs L1).

### 9.4 Move Notation

Two-character algebraic notation for Caro:
- First character: row (y), a-p for 0-15
- Second character: column (x), a-p for 0-15
- Example: bd = row 1, column 3; pp = row 15, column 15

---

## 10. Source Code Organization

### 10.1 Package Layout (`internal/`)

| Package | Files | Responsibility |
|---------|-------|---------------|
| `internal/domain` | board.go, game.go, player.go, position.go, zobrist.go, win.go, constants.go, errors.go | Domain entities, game rules, no dependencies |
| `internal/engine` | minimax.go, search.go, parallel.go, evaluation.go, pattern4.go, vcf.go, transposition.go, movepicker.go, candidate.go, heuristics.go, timemanager.go, timemonitor.go, difficulty.go, searchboard.go, bitboard.go | AI engine, search algorithms |
| `internal/uci` | handler.go, notation.go, position.go, options.go | UCI protocol handling |
| `internal/api` | server.go, handlers.go, websocket.go, session.go, store.go, requests.go, middleware.go, errors.go | HTTP/WebSocket API |
| `internal/persistence` | matchstore.go | Structured match persistence (SQLite) |

### 10.2 Centralized Constants

| File | Constants |
|------|-----------|
| `internal/domain/constants.go` | BoardSize, WinLength, directions, cell counts |
| `internal/engine/search.go` | MaxSearchRadius, TT size, null-move thresholds, aspiration window, killer/history limits |
| `internal/engine/movepicker.go` | Staged picker score thresholds |
| `internal/engine/evaluation.go` | Pattern scores, defense multipliers |
| `internal/engine/timemanager.go` | Default time controls, PID controller weights, phase thresholds |
| `internal/engine/difficulty.go` | L1-L5 difficulty profiles, goroutine counts |

### 10.3 Main Engine Files

**internal/engine/** (all files <= 400 SLOC):

| File | Role |
|------|------|
| `minimax.go` | MinimaxAI struct definition, constructor, public API, Dispose |
| `search.go` | Iterative deepening, PVS alpha-beta, LMR, null-move pruning, aspiration windows |
| `parallel.go` | Lazy SMP goroutine pool dispatch, result aggregation |
| `evaluation.go` | Pattern4-based evaluation with defense multiplier and center bonus |
| `pattern4.go` | 4-direction threat classification (Flex/Block/Broken patterns, combined threat detection) |
| `vcf.go` | Victory by Continuous Fours pre-search solver |
| `transposition.go` | Sharded SeqLock TT with atomic.Uint32 version counters |
| `movepicker.go` | Staged move ordering (7 stages: TT -> Block -> Win -> Threat -> Killer/Counter -> Quiet) |
| `candidate.go` | Candidate generation with center-of-mass ordering, tactical filtering |
| `heuristics.go` | Killer moves, continuation/butterfly/counter-move history |
| `timemanager.go` | PID time management, phase-aware allocation |
| `timemonitor.go` | context.Context-based search time monitoring |
| `difficulty.go` | Hardware-agnostic L1-L5 difficulty profiles |
| `searchboard.go` | Mutable board for search hot path (make/unmake, zero allocation) |
| `bitboard.go` | BitBoard type with uint64 operations (math/bits) |

**UCI Package** (`internal/uci/`):
| File | Role |
|------|------|
| `handler.go` | UCI command dispatcher, search controller |
| `notation.go` | Double-letter coordinate encoding/decoding |
| `position.go` | Position string parsing |
| `options.go` | Engine options (Threads, Hash, Ponder, Skill Level) |

---

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
