# Caro Gomoku AI Improvement Research

**Date:** 2026-02-12
**Project:** Caro AI-PVP (19x19 Gomoku Variant)
**Purpose:** Technical reference for potential engine improvements based on Rapfi, Stockfish 18, Chess Programming Wiki, and minimax.dev research.

---

## 1. Overview

This document consolidates research on game engine optimization techniques applicable to the Caro Gomoku variant. It serves as a technical reference for understanding implemented features and evaluating potential improvements.

Key sources:
- **Rapfi** - State-of-the-art Gomoku/Renju engine with BitKey pattern system
- **Stockfish 18** - Chess engine with proven alpha-beta optimizations
- **Chess Programming Wiki** - Comprehensive algorithm documentation
- **minimax.dev** - Ultimate Tic Tac Toe solving techniques

---

## 2. Currently Implemented Techniques

The following techniques are already implemented in the codebase:

### 2.1 Search Optimizations

| Technique | Description | File |
|-----------|-------------|------|
| Lazy SMP | Parallel search with shared TT, thread sharding | `ParallelMinimaxSearch.cs`, `LockFreeTranspositionTable.cs` |
| Principal Variation Search | Alpha-beta with null-window searches | `ParallelMinimaxSearch.cs` |
| Late Move Reduction (LMR/MDAP) | Depth reduction for later moves | `ParallelMinimaxSearch.cs:53-55` |
| Aspiration Windows | Narrowed alpha-beta bounds near root | `ParallelMinimaxSearch.cs` |
| Killer Moves | Per-thread killer move storage | `ParallelMinimaxSearch.cs:66` |
| History Heuristic | Move ordering based on past performance | `ParallelMinimaxSearch.cs:68-69` |

### 2.2 Transposition Table

| Feature | Description | File |
|---------|-------------|------|
| Multi-Entry Clusters | Multiple entries per hash bucket | `TranspositionTable.cs` |
| Lock-Free Parallel Access | Sharded design with atomic operations | `LockFreeTranspositionTable.cs` |
| Depth-Age Replacement | `depth - 8 * age` formula for entry replacement | `TranspositionTable.cs:99` |
| Static Evaluation Storage | Caches static eval with TT entries | `TranspositionTable.cs:48` (Eval16) |
| 10-byte Packed Entries | Cache-efficient entry structure | `TranspositionTable.cs:29-93` |

### 2.3 Move Ordering

| Feature | Description | File |
|---------|-------------|------|
| Continuation History | 6-ply move pair statistics | `ContinuationHistory.cs` |
| Bounded History Updates | Overflow prevention formula | `ContinuationHistory.cs:11` |
| Hash Move Prioritization | TT move searched first | `ParallelMinimaxSearch.cs` |

### 2.4 Domain-Specific

| Feature | Description | File |
|---------|-------------|------|
| VCF Solver | Victory by Continuous Fours detection | `VCFSolver.cs` |
| Opening Book | SQLite-based opening database | `OpeningBook.cs` |
| Exactly-5 Validation | Win detection for Caro rules | `WinDetector.cs` |

### 2.5 Automated Tuning

| Feature | Description | File |
|---------|-------------|------|
| SPSA Optimizer | Gradient-free parameter tuning | `SPSAOptimizer.cs` |
| PID Time Management | Control theory for time allocation | `PIDTimeManager.cs` |

---

## 3. Research Candidates

The following techniques are not yet implemented and could be evaluated for potential improvement:

### 3.1 Rapfi BitKey Pattern System

**Description:** O(1) pattern lookup using bitkey rotation. Rapfi encodes board positions as 64-bit keys with 2 bits per cell, then uses rotation to align patterns around the position being evaluated.

**Rapfi Implementation:**
```cpp
// Four directional bitkeys (64-bit each)
uint64_t bitKey0[FULL_BOARD_SIZE];          // Horizontal
uint64_t bitKey1[FULL_BOARD_SIZE];          // Vertical
uint64_t bitKey2[FULL_BOARD_SIZE * 2 - 1];  // Diagonal
uint64_t bitKey3[FULL_BOARD_SIZE * 2 - 1];  // Anti-diagonal

// Key extraction with rotation
template <Rule R>
inline uint64_t Board::getKeyAt(Pos pos, int dir) const {
    constexpr int L = PatternConfig::HalfLineLen<R>;
    int x = pos.x() + BOARD_BOUNDARY;
    int y = pos.y() + BOARD_BOUNDARY;

    switch (dir) {
    case 0: return rotr(bitKey0[y], 2 * (x - L));
    case 1: return rotr(bitKey1[x], 2 * (y - L));
    case 2: return rotr(bitKey2[x + y], 2 * (x - L));
    case 3: return rotr(bitKey3[FULL_BOARD_SIZE - 1 - x + y], 2 * (x - L));
    }
}
```

**C#/Caro Adaptation:**
```csharp
public class BitKeyBoard
{
    private const int FullBoardSize = 32;
    private const int BoardBoundary = 5;
    private const int HalfLineLen = 6;

    private ulong[] _bitKey0 = new ulong[FullBoardSize];
    private ulong[] _bitKey1 = new ulong[FullBoardSize];
    private ulong[] _bitKey2 = new ulong[FullBoardSize * 2 - 1];
    private ulong[] _bitKey3 = new ulong[FullBoardSize * 2 - 1];

    public ulong GetKeyAt(int x, int y, int direction)
    {
        int bx = x + BoardBoundary;
        int by = y + BoardBoundary;

        return direction switch
        {
            0 => BitOperations.RotateRight(_bitKey0[by], 2 * (bx - HalfLineLen)),
            1 => BitOperations.RotateRight(_bitKey1[bx], 2 * (by - HalfLineLen)),
            2 => BitOperations.RotateRight(_bitKey2[bx + by], 2 * (bx - HalfLineLen)),
            3 => BitOperations.RotateRight(_bitKey3[FullBoardSize - 1 - bx + by], 2 * (bx - HalfLineLen)),
            _ => 0
        };
    }
}
```

**Considerations:**
- Requires significant refactoring of pattern evaluation
- Would replace current `ThreatDetector` and `BitBoardEvaluator` pattern logic
- 32x32 board representation with 5-cell boundary eliminates bounds checking

**Files to Modify:** `BitBoard.cs`, `ThreatDetector.cs`, `BitBoardEvaluator.cs`

---

### 3.2 Counter-Move History

**Description:** Track which moves are good responses to specific opponent moves. Unlike continuation history (which tracks our own previous moves), counter-move history captures move-response patterns.

**Stockfish Implementation:**
```cpp
typedef HistTable<std::pair<Pos, Pattern4>, 0, SIDE_NB, MAX_MOVES> CounterMoveHistory;
```

**C#/Caro Adaptation:**
```csharp
public class CounterMoveHistory
{
    // [player, opponentMove, ourMove] -> score
    private readonly short[,,] _counterHistory = new short[2, 361, 361];
    private const int MaxScore = 30000;

    public int GetCounterScore(Player player, int opponentCell, int ourCell)
    {
        if (opponentCell < 0 || opponentCell >= 361 || ourCell < 0 || ourCell >= 361)
            return 0;
        return _counterHistory[(int)player - 1, opponentCell, ourCell];
    }

    public void UpdateCounterScore(Player player, int opponentCell, int ourCell, int bonus)
    {
        if (opponentCell < 0 || opponentCell >= 361 || ourCell < 0 || ourCell >= 361)
            return;

        int current = _counterHistory[(int)player - 1, opponentCell, ourCell];
        int clampedBonus = Math.Clamp(bonus, -MaxScore, MaxScore);
        _counterHistory[(int)player - 1, opponentCell, ourCell] =
            (short)(current + clampedBonus - Math.Abs(current * clampedBonus) / MaxScore);
    }
}
```

**Considerations:**
- Adds another dimension to move ordering
- Memory overhead: 2 * 361 * 361 * 2 bytes = ~500KB
- Integrates with existing continuation history

**Files to Modify:** `ParallelMinimaxSearch.cs`, new `CounterMoveHistory.cs`

---

### 3.3 TD Learning for Evaluation Tuning

**Description:** Temporal Difference learning improves evaluation function by learning from self-play. TD(λ) bootstraps from current position estimates to update evaluation weights.

**TD(λ) Algorithm:**
```
For each timestep t:
    δ_t = r_{t+1} + γ·V(s_{t+1}) - V(s_t)  // TD error
    For all states s visited:
        e(s) ← λ·γ·e(s) + 1(s == s_t)       // Eligibility trace
        θ(s) ← θ(s) + α·δ_t·e(s)            // Weight update
```

**TDLeaf for Minimax (C# Adaptation):**
```csharp
public class TDLeafLearner
{
    private double _lambda = 0.7;
    private double _learningRate = 0.1;

    public void LearnFromGame(List<GameState> game, double result)
    {
        var leafEvaluations = new List<double>();
        var leafGradients = new List<double[]>();

        foreach (var state in game)
        {
            var (pv, leafEval) = SearchWithLeafTracking(state);
            leafEvaluations.Add(leafEval);
            leafGradients.Add(ComputeGradient(state, pv));
        }

        // Apply TD(λ) updates
        for (int t = 0; t < game.Count; t++)
        {
            double tdError = ComputeTDError(leafEvaluations, t, result);

            // Eligibility trace
            for (int i = 0; i <= t; i++)
            {
                double trace = Math.Pow(_lambda, t - i);
                UpdateWeights(leafGradients[i], tdError * trace * _learningRate);
            }
        }
    }
}
```

**Considerations:**
- Requires self-play infrastructure for training games
- Needs evaluation function to be parameterized with gradients
- Training quality depends on game diversity
- Long-term investment with continuous improvement potential

**Files to Create:** `Caro.Learning/TDLeafLearner.cs`, `Caro.Learning/SelfPlayEngine.cs`

---

### 3.4 NNUE-Style Evaluation

**Description:** Efficiently Updatable Neural Network evaluation. Uses incrementally maintained first layer that only updates changed positions, with remaining layers computed via SIMD.

**Architecture for Gomoku:**
```csharp
public class CaroNNUE
{
    private const int InputSize = 361 * 4; // 4 directions per cell
    private const int HiddenSize = 256;

    private int[] _accumulated = new int[HiddenSize];
    private int[] _weights = new int[InputSize * HiddenSize];

    // Incremental update on move
    public void UpdateOnMove(int position, int direction, int oldPattern, int newPattern)
    {
        int oldIdx = position * 4 + direction;
        int newIdx = position * 4 + direction;

        // Subtract old contribution
        for (int h = 0; h < HiddenSize; h++)
            _accumulated[h] -= _weights[oldIdx * HiddenSize + h];

        // Add new contribution
        for (int h = 0; h < HiddenSize; h++)
            _accumulated[h] += _weights[newIdx * HiddenSize + h];
    }
}
```

**Considerations:**
- Major architectural change to evaluation
- Requires training infrastructure and large dataset
- SIMD implementation needed for performance (AVX2/AVX-512)
- For Caro: network should penalize overlines (exactly-5 rule)

**Complexity:** Very High - would be a separate project

---

### 3.5 Staged Move Picker

**Description:** Sophisticated move generation stages similar to Stockfish's MovePicker. Current implementation uses simpler move ordering.

**Stockfish Stages:**
```cpp
enum Stages {
    MAIN_TT,          // Transposition table move
    CAPTURE_INIT,     // Initialize capture generation
    GOOD_CAPTURE,     // Search good captures
    REFUTATION,       // Search refutation moves (killer, counter)
    QUIET_INIT,       // Initialize quiet generation
    GOOD_QUIET,       // Search good quiets (high history)
    BAD_CAPTURE,      // Search remaining captures
    BAD_QUIET,        // Search remaining quiets
};
```

**Stockfish Move Scoring:**
```cpp
// For quiet moves
m.value = 2 * mainHistory[us][move]
        + continuationHistory[0][pc][to]   // Previous ply
        + continuationHistory[1][pc][to]   // Two plies ago
        + continuationHistory[2][pc][to]   // Three plies ago
        + continuationHistory[3][pc][to]   // Four plies ago
        - 4772 + 2 * ss->killers[move];    // Killer bonus

// Partial insertion sort threshold
partial_insertion_sort(cur, endCur, -3560 * depth);
```

**Considerations:**
- More granular move ordering control
- Better cutoff rates in alpha-beta
- Requires profiling to find optimal thresholds

**Files to Modify:** `ParallelMinimaxSearch.cs`

---

### 3.6 Rapfi Pattern4 Combined Evaluation

**Description:** 4-direction combined pattern evaluation that categorizes positions by threat level.

**Rapfi Pattern4 Types:**
```cpp
enum Pattern4 {
    NONE,           // No significant pattern
    FLEX1,          // Single stone with potential
    BLOCK1,         // Single blocked stone
    FLEX2,          // Open two
    BLOCK2,         // Blocked two
    FLEX3,          // Open three (threat)
    BLOCK3,         // Blocked three
    FLEX4,          // Open four (strong threat)
    BLOCK4_FLEX3,   // Blocked four with open three
    BLOCK4,         // Blocked four
    B_FLEX4,        // Double open four or open four + threat
    C_BLOCK4_FLEX3, // Critical: blocked four + open three
    A_FIVE,         // Win
};
```

**Caro-Specific Adaptation:**
```csharp
public enum CaroPattern4
{
    None,
    Flex1,      // Single stone
    Block1,     // Single blocked
    Flex2,      // Open two
    Block2,     // Blocked two
    Flex3,      // Open three - defensive priority
    Block3,     // Blocked three
    Flex4,      // Open four - winning threat
    Block4,     // Blocked four (still threatening in Caro)
    DoubleFlex3,// Two open threes - winning
    Flex4Flex3, // Open four + open three - winning
    Exactly5,   // Win condition
    Overline    // Invalid in Caro (exactly-5 rule)
}
```

**Considerations:**
- More nuanced threat classification
- Better double-threat detection
- Requires precomputed lookup tables

**Files to Modify:** `ThreatDetector.cs`, `BitBoardEvaluator.cs`

---

### 3.7 Lockless Hashing

**Description:** XOR-based key verification for parallel transposition table access without locks. Already partially implemented in `LockFreeTranspositionTable.cs`.

**Enhancement Pattern:**
```csharp
public void Store(ulong key, short value, sbyte depth, byte bound, short move, short eval)
{
    // XOR key with data for lockless parallel access
    entry.Key = key ^ (ulong)(ushort)value ^ ((ulong)(byte)depth << 16)
                     ^ ((ulong)bound << 24) ^ ((ulong)(ushort)move << 32);
}
```

**Current Status:** Basic lock-free access implemented; full XOR verification not yet applied.

---

## 4. Testing and Validation

### Recommended Testing Approach

1. **Tournament Testing:**
   - Round-robin between engine versions
   - Multiple time controls (1+0, 3+0, 5+0)
   - Statistical significance testing

2. **Position Testing:**
   - Tactical puzzle suite
   - Endgame test positions
   - Opening diversity check

3. **Performance Benchmarking:**
   - Nodes per second (NPS) measurement
   - TT hit rate tracking
   - Branching factor analysis

---

## 5. References

### Source Repositories

- **Rapfi:** https://github.com/dhbloo/rapfi (Gomoku/Renju engine)
- **Stockfish 18:** https://github.com/official-stockfish/Stockfish
- **YaneuraOu:** https://github.com/yaneurao/YaneuraOu (Shogi NNUE reference)

### Documentation

- **Chess Programming Wiki:** https://www.chessprogramming.org/
- **Stockfish Wiki:** https://www.chessprogramming.org/Stockfish
- **NNUE Paper:** Yu Nasu (2018) "Efficiently Updatable Neural-Network based Evaluation Functions"
- **minimax.dev:** https://minimax.dev/

### Key Topics

- [Transposition Table](https://www.chessprogramming.org/Transposition_Table)
- [History Heuristic](https://www.chessprogramming.org/History_Heuristic)
- [Late Move Reductions](https://www.chessprogramming.org/Late_Move_Reductions)
- [Lazy SMP](https://www.chessprogramming.org/Lazy_SMP)
- [NNUE](https://www.chessprogramming.org/NNUE)
- [Continuation History](https://www.chessprogramming.org/Continuation_History)

### Key Papers

1. "CLOP: Confident Local Optimization for Noisy Tuning" - R. Coulom
2. "SPSA for Noisy Optimization" - J. Spall
3. "History Heuristic" - J. Schaeffer
4. "Late Move Reductions" - E.A. Heinz
5. "TDLeaf(λ): Combining Temporal Difference Learning with Game-Tree Search" - Baxter et al.
6. "RSPSA: Enhanced Parameter Optimization in Games" - Kocsis et al.
