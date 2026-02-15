# Caro AI Engine Features

**Version:** 1.55.0
**Board:** 32x32 (1024 intersections)
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

- **Depth:** 9-12+ ply on modern hardware
- **Speedup:** 100-500x over naive minimax
- **Parallelism:** Lazy SMP with N/2-1 helper threads

---

## 2. Search Architecture

### 2.1 Lazy SMP Parallel Search

Lazy SMP is a parallel search paradigm where multiple threads explore the game tree independently, sharing work through the transposition table.

**Core Principle:**
- Master thread performs full search with all pruning
- Helper threads search at reduced depth with TT sharing
- Hash move priority enables cross-thread work distribution

**Thread Distribution:**
- Thread count: `(logical_cores / 2) - 1` for Grandmaster difficulty
- Each thread maintains independent killer moves and history tables
- TT writes from helpers are filtered (shallow depths, exact bounds only)

**Advantages:**
- Simple implementation with good scaling
- No complex work distribution logic
- TT naturally becomes shared knowledge base

### 2.2 Principal Variation Search (PVS)

PVS is an enhancement to alpha-beta search that uses null-window searches to prove moves are suboptimal quickly.

**Algorithm Structure:**
1. Search first move with full alpha-beta window
2. For remaining moves, search with null-window (minimal bounds)
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

### 3.1 Multi-Entry Clusters

The TT uses cluster-based storage with multiple entries per hash bucket.

**Cluster Structure:**
- 3 entries per cluster
- 32-byte cache-line aligned
- Depth-age replacement scheme

**Replacement Policy:**
- Priority: depth - 8 * age
- Higher priority entries are kept
- Age increments per search iteration

### 3.2 Entry Structure

Each TT entry stores:
- **Hash Key** - Position identification (truncated)
- **Depth** - Search depth of stored result
- **Bound Type** - Exact, lower bound (beta cutoff), or upper bound (alpha cutoff)
- **Score** - Position evaluation
- **Best Move** - Principal variation move
- **Static Eval** - Cached static evaluation

### 3.3 TT Sharding

For parallel access efficiency, the TT is divided into independent segments.

**Shard Distribution:**
- 16 independent segments
- Hash-based index calculation
- Reduces cache coherency traffic

**Thread Access:**
- Each thread can access any shard
- No locking required for read operations
- Atomic operations for writes

### 3.4 Lockless Hashing

XOR-based key verification enables parallel access without locks.

**Mechanism:**
- Entry key XORed with stored data
- Verification through reverse XOR
- Detects torn reads/writes

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

**Stage Sequence:**
1. **TT_MOVE** - Single move from transposition table (2M score)
2. **MUST_BLOCK** - Mandatory blocks against opponent's winning threats (2M score)
3. **WINNING_MOVE** - Creates open four or double threat (1.5M score)
4. **THREAT_CREATE** - Creates open three or broken four (800K score)
5. **KILLER_COUNTER** - Killer moves (400K-500K) + counter-move responses (150K)
6. **GOOD_QUIET** - Quiet moves with continuation + butterfly history > 500
7. **BAD_QUIET** - Remaining quiet moves

**Score Constants:**
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
| History Max | 20,000 |
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

### 5.5 Contest Factor (Contempt)

Dynamic adjustment for draw-ish vs. sharp positions.

**Range:** -200 to +200 centipawns

**Position Awareness:**
- Positive: Avoid draws, play aggressively
- Negative: Accept draws, play solidly
- Adjusts based on game phase and score

---

## 6. Time Management

### 6.1 PID Time Manager

Uses control theory principles for time allocation.

**Components:**
- **Proportional (Kp=1.0):** React to current error
- **Integral (Ki=0.1):** Account for accumulated error
- **Derivative (Kd=0.5):** Predict future error

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
| Classical | 15 min | 10 sec | Tournament games |

### 6.3 Pondering

Background search during opponent's turn.

**Characteristics:**
- Enabled for Medium+ difficulty
- Searches predicted opponent move
- TT stored for potential reuse
- Interrupted on opponent move

---

## 7. Domain-Specific Features

### 7.1 VCF Solver

Victory by Continuous Fours - tactical solver for forcing win sequences.

**Purpose:**
- Detect forced wins before main search
- Search specifically for four-in-a-row sequences
- Prune positions with known outcomes

**Integration:**
- Runs before alpha-beta search
- Results cached for reuse
- Depth-limited for practical use

### 7.2 Opening Book

Precomputed opening positions for early game guidance.

**Structure:**
- SQLite database with symmetry reduction
- 8-way transformations reduce storage ~8x
- Tapered beam: 4→3→2→1 moves per depth tier

**Book Depth by Difficulty:**

| Difficulty | Book Depth |
|------------|------------|
| Braindead | None |
| Easy | 4 plies |
| Medium | 6 plies |
| Hard | 10 plies |
| Grandmaster | 14 plies |
| Experimental | Unlimited |

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

### 7.4 Open Rule

Red's second move must be at least 3 intersections from first.

**Implementation:**
- Enforced at game logic level
- Move generation filters invalid moves
- Opening book respects rule

---

## 8. Automated Tuning

### 8.1 SPSA Optimizer

Simultaneous Perturbation Stochastic Approximation for parameter optimization.

**Algorithm:**
- Gradient-free optimization
- Simultaneous parameter perturbation
- Works with noisy evaluations

**Parameters:**
- Alpha: 0.602 (default)
- Gamma: 0.101 (default)
- Multiple presets available

**Tuning Targets:**
- Evaluation weights
- Move ordering thresholds
- LMR reduction factors

### 8.2 Self-Play Infrastructure

Engine supports automated self-play for tuning and testing.

**Features:**
- Engine vs engine matches
- Multiple time controls
- Statistical significance testing
- ELO calculation

---

## 9. Concurrency Model

### 9.1 Thread Safety

All shared data structures designed for concurrent access.

**Immutable State:**
- Game state is immutable
- Operations return new instances
- No shared mutable state in game logic

**Thread-Safe Structures:**
- TT with sharding and lockless access
- Channels for async communication
- Independent history tables per thread

### 9.2 Cancellation

Coordinated search cancellation via CancellationTokenSource.

**Mechanism:**
- Single token for all search threads
- Checked at regular intervals
- Clean termination on timeout or stop command

### 9.3 Statistics Publishing

Publisher-subscriber pattern for AI telemetry.

**Components:**
- Channel-based event queue
- Async subscriber tasks
- Non-blocking to search threads

---

## 10. UCI Protocol

### 10.1 Command Support

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

### 10.2 Engine Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| Skill Level | spin | 3 | 1-6 difficulty |
| Use Opening Book | check | true | Enable book |
| Book Depth Limit | spin | 14 | Max book ply (Grandmaster default) |
| Threads | spin | auto | Search threads |
| Hash | spin | 128 | TT size (MB) |
| Ponder | check | true | Enable pondering |

### 10.3 Move Notation

Algebraic notation for Caro:
- Columns: a-af (1-32)
- Rows: 1-32
- Example: j10 = column 10, row 10

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
3. "SPSA for Noisy Optimization" - J. Spall
4. "NNUE: Efficiently Updatable Neural Networks" - Y. Nasu
