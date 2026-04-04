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
- **Parallelism:** Lazy SMP with power-of-2 threads (largest power of 2 <= logical cores)

---

## 2. Search Architecture

### 2.1 Lazy SMP Parallel Search

Lazy SMP is a parallel search paradigm where multiple threads explore the game tree independently, sharing work through the transposition table.

**Core Principle:**
- Master thread performs full search with all pruning
- Helper threads use parity-based depth offset (odd-indexed start at depth 2, even-indexed at depth 1) with TT sharing
- Hash move priority enables cross-thread work distribution

**Thread Distribution:**
- Thread count: Largest power of 2 <= logical_cores (e.g., 20 cores -> 16 threads)
- Each thread maintains independent killer moves and history tables
- TT writes from helpers filtered to depth >= 3 (depth-age replacement handles entry quality)

**Advantages:**
- Simple implementation with good scaling
- No complex work distribution logic
- TT naturally becomes shared knowledge base

### 2.2 Principal Variation Search (PVS)

PVS is an enhancement to alpha-beta search that uses null-window searches to prove moves are suboptimal quickly.

**Implementation Scope:**
- Sequential path (MinimaxAI): Full PVS with null-window searches and re-searches
- Parallel path (ParallelMinimaxSearch): Alpha-beta with Move-Dependent Adaptive Pruning (MDAP/LMR) and aspiration windows; traditional PVS not applied

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
- Window size typically ±25-50 centipawns
- Failed searches expand window and re-search

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
- Search only threat moves (captures/winning moves)
- Stand-pat score for quiet positions
- Depth limit to prevent explosion

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

The engine has two TT implementations, each serving a different search context:

### 3.1 Cluster-Based TranspositionTable (MinimaxAI)

Used by `MinimaxAI` for single-threaded (sequential) search path.

**Cluster Structure:**
- 3 entries per cluster
- 10-byte TTEntry structs (compact storage)
- Depth-age replacement scheme

**Replacement Policy:**
- Priority: depth - 8 * age
- Higher priority entries are kept
- Age increments per search iteration

### 3.2 LockFreeTranspositionTable (ParallelMinimaxSearch)

Primary TT used by `ParallelMinimaxSearch` for Lazy SMP multi-threaded search.

**Shard Distribution:**
- 16 independent segments
- Hash-based index calculation: `shardIndex = (hash >> 32) & shardMask`
- Reduces cache coherency traffic

**Thread Access:**
- SeqLock pattern with version counters for lockless reads
- Each thread can access any shard
- No locking required for read operations
- Atomic version counter updates for writes

### 3.3 Entry Structure

Each TT entry stores:
- **Hash Key** - Position identification (truncated)
- **Depth** - Search depth of stored result
- **Bound Type** - Exact, lower bound (beta cutoff), or upper bound (alpha cutoff)
- **Score** - Position evaluation
- **Best Move** - Principal variation move
- **Static Eval** - Cached static evaluation

### 3.4 Lockless Access (SeqLock)

SeqLock pattern with version counters enables parallel access without locks.

**Mechanism:**
- Version counter incremented before and after writes
- Readers verify version unchanged during read (no torn reads)
- Retry on version mismatch
- Detects concurrent modification without locks

### 3.5 TT Write Policy

Helper threads in Lazy SMP are filtered to only write TT entries at depth >= 3,
preventing shallow/noisy entries from polluting the table. The master thread
writes at all depths. The depth-age replacement strategy handles entry quality
naturally for entries that pass the filter.

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

**Pattern Scores:**

| Pattern | Score (centipawns) |
|---------|-------------------|
| Five in row | 100,000 (win) |
| Open four | 10,000 |
| Closed four | 1,000 |
| Open three | 1,000 |
| Closed three | 100 |
| Open two | 100 |
| Center bonus | 50 |

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

Background search during opponent's turn.

**Characteristics:**
- Enabled by default
- Searches predicted opponent move
- TT stored for potential reuse
- Interrupted on opponent move

**Ponder Hit Handling:**
- `HasPonderHitResult` property checks for valid ponder hit
- `GetPonderHitResult()` retrieves result immediately (no waiting)
- Ponder time is "free" precomputation - runs during opponent's turn
- GetBestMove checks for ponder hit before starting new search

---

## 7. Domain-Specific Features

### 7.1 Board Representation

Immutable board design with pre-computed AI optimization data.

**Architecture:**
- Board is immutable - operations return new instances
- Cell uses record struct for value semantics
- Pre-computed bitboards and hash updated incrementally

**Performance Optimization:**
- BitBoards: `ulong[4]` arrays (256 bits for 16x16 board)
- Hash: Zobrist-style XOR updated on each move
- O(1) access during AI search instead of O(n²) iteration

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

Red's second move must be at least 3 intersections from first.

**Implementation:**
- Enforced at game logic level
- Move generation filters invalid moves

---

## 8. Concurrency Model

### 8.1 Thread Safety

All shared data structures designed for concurrent access.

**Immutable State:**
- Game state is immutable
- Operations return new instances
- No shared mutable state in game logic

**Thread-Safe Structures:**
- TT with sharding and lockless access
- Channels for async communication
- Independent history tables per thread

### 8.2 Cancellation

Coordinated search cancellation via CancellationTokenSource.

**Mechanism:**
- Single token for all search threads
- Checked at regular intervals
- Clean termination on timeout or stop command

### 8.3 Statistics Publishing

Publisher-subscriber pattern for AI telemetry.

**Components:**
- Channel-based event queue
- Async subscriber tasks
- Non-blocking to search threads

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
| Threads | spin | 4 | Search threads (1-32; internal parallel search auto-detects from CPU count) |
| Hash | spin | 256 | TT size (MB) |
| Ponder | check | false | Enable pondering |

### 9.3 Move Notation

Algebraic notation for Caro:
- Columns: aa-dd (double-letter encoding: column = firstIndex * 4 + secondIndex, each letter a-d)
- Rows: 1-16
- Example: bd9 = column 7, row 9; dd16 = column 15, row 16

---

## 10. Source Code Organization

### 10.1 Search Module (`GameLogic/Search/`)

Extracted from the main search classes for cohesion and maintainability:

| File | Responsibility |
|------|---------------|
| `TacticalEvaluator.cs` | Tactical pattern detection (threats, futility, null-move safety) |
| `CandidateGenerator.cs` | Candidate move generation with center-of-mass ordering |
| `SearchHeuristics.cs` | Killer moves, history tables, butterfly tables |
| `MoveOrderer.cs` | Staged move ordering with TT/killer/continuation scoring |
| `QuickWinChecker.cs` | Pre-search tactical shortcuts (winning moves, forced blocks) |
| `TimeBudgetCalculator.cs` | VCF time limits, ponder time, default allocation |
| `ParallelThreatAnalyzer.cs` | Opponent threat detection for parallel search path |
| `ParallelNodeEvaluator.cs` | Per-node tactical eval, adaptive LMR, winner checks |

### 10.2 Centralized Constants (`Caro.Core.Domain/Configuration/`)

| File | Constants |
|------|-----------|
| `SearchConstants.cs` | MaxSearchRadius, TT size, null-move thresholds, aspiration window, killer/history limits |
| `PruningConstants.cs` | Futility margins, LMR parameters, PVS depth threshold |
| `MoveOrderingConstants.cs` | Staged picker score thresholds |
| `EvaluationConstants.cs` | Pattern scores, defense multipliers |
| `SearchHeuristicConstants.cs` | Threat scoring weights, alpha-beta/aspiration bounds, depth controls, time allocation ratios, VCF time thresholds |
| `TimeConstants.cs` | TimeMonitor intervals, AsyncQueue capacity, UCI timeouts, SearchLogger rotation, Ponderer limits, DFPN/TSS defaults, HardBound buffers |
| `TimeManagementConstants.cs` | Default time controls, PID controller weights, phase thresholds, adaptive scaling, emergency thresholds, multiplier adjustments |

### 10.3 Main Search Classes

Decomposed into partial class files (all ≤ 400 lines):

**MinimaxAI** (sequential search):
| File | Role |
|------|------|
| `MinimaxAI.cs` | Class definition, constructor, public API |
| `MinimaxAI.Helpers.cs` | Shared utilities and helper methods |
| `MinimaxAI.MoveSelection.cs` | Top-level move selection orchestration |
| `MinimaxAI.MoveSelection.Attack.cs` | Winning/threat-creation attack moves |
| `MinimaxAI.MoveSelection.Defense.cs` | Blocking and defensive moves |
| `MinimaxAI.MoveSelection.PonderHit.cs` | Ponder hit detection and reuse |
| `MinimaxAI.MoveSelection.SearchDispatch.cs` | Search invocation and result handling |
| `MinimaxAI.MoveSelection.ThreatBlocking.cs` | Threat-based forced blocking |
| `MinimaxAI.Search.cs` | Search orchestration, iterative deepening |
| `MinimaxAI.Search.Core.cs` | Core PVS alpha-beta search |
| `MinimaxAI.Search.Minimax.cs` | Full-width minimax with LMR |
| `MinimaxAI.Stats.cs` | Statistics and telemetry |

**ParallelMinimaxSearch** (Lazy SMP):
| File | Role |
|------|------|
| `ParallelMinimaxSearch.cs` | Class definition, thread data, WorkerPool |
| `ParallelMinimaxSearch.Orchestration.cs` | Entry points: GetBestMove, GetBestMoveWithStats |
| `ParallelMinimaxSearch.Orchestration.SearchLazySMP.cs` | Lazy SMP thread coordination |
| `ParallelMinimaxSearch.Orchestration.IterativeDeepening.cs` | Time-aware iterative deepening |
| `ParallelMinimaxSearch.Search.cs` | Parallel alpha-beta with adaptive LMR |
| `ParallelMinimaxSearch.Search.Quiesce.cs` | Quiescence search |
| `ParallelMinimaxSearch.MoveOrdering.cs` | Move ordering with killer/history scoring |
| `ParallelMinimaxSearch.MoveOrdering.Helpers.cs` | Candidate generation, legacy sort helpers |
| `ParallelMinimaxSearch.Pondering.cs` | Ponder search and hit detection |

**Other:**
| File | Role |
|------|------|
| `IterativeDeepeningSearch.cs` | Iterative deepening driver for sequential path |

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
