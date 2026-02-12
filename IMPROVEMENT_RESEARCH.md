# Caro Gomoku AI Improvement Research Report

**Date:** 2026-02-12 (Updated)
**Project:** Caro AI-PVP (19x19 Gomoku Variant)
**Goal:** Extract insights from Rapfi, Stockfish 18, Chess Programming Wiki, minimax.dev, and advanced optimization techniques to improve the Caro engine.

---

## Executive Summary

This comprehensive research report analyzes multiple sources of game engine optimization techniques and synthesizes them into actionable improvements for the Caro Gomoku variant. The current Caro implementation already incorporates several advanced techniques (Lazy SMP, PVS, LMR, Transposition Tables), but significant improvements are possible through:

1. **Automated weight tuning** using CLOP/SPSA (100-200 ELO potential)
2. **Enhanced move ordering** with continuation history (30-50 ELO)
3. **Evaluation caching** with Zobrist hashing (50-100 ELO)
4. **Adaptive LMR** based on position type (40-80 ELO)
5. **TD Learning** for evaluation improvement (100-300 ELO potential)
6. **PID Time Management** (20-50 ELO in time controls)
7. **Rapfi BitKey system** for O(1) pattern lookup (50-100 ELO)
8. **VCF solver** for winning sequence detection (30-50 ELO)
9. **NNUE-style incremental evaluation** (100-200 ELO long-term)

**Total Potential Gain:** 600-1000+ ELO points with focused implementation over time.

---

## Part 1: Current Caro Implementation Baseline

### Architecture Overview

- **Tech Stack:** .NET 10 backend, SvelteKit 2.49 frontend
- **Search Algorithm:** Minimax with Alpha-Beta pruning
- **Parallel Search:** Lazy SMP for D7+ difficulties
- **Board Representation:** Dual system (immutable Board + performance BitBoard)
- **Opening Book:** SQLite-based with parallel generation

### Custom Rules Implementation

| Rule | Implementation |
|------|----------------|
| 19x19 board | 6 ulongs (384 bits) for 361 cells |
| Exactly-5 | `WinDetector.cs` validates exact 5-in-row |
| No close blocked | Built into threat evaluation |
| Opening rule | `OpenRuleValidator.cs` validates >=3 intersections |

### Current Optimizations Already Implemented

- Hash Move prioritization (2-5x speedup)
- Principal Variation Search (20-40% improvement)
- Late Move Reduction (30-50% improvement)
- 256MB Transposition Table (2-5x speedup)
- History Heuristic (10-20% improvement)
- Aspiration Windows (10-30% improvement)

### Current Limitations

- Maximum depth effectively limited to 5-6 for full search
- Branching factor ~2.5 (still high for Caro complexity)
- Basic pattern recognition (no advanced double-threat detection)
- Single-node search only
- No automated weight tuning

---

## Part 2: Rapfi Gomoku/Renju Implementation Insights

### 2.1 Board Representation

**Key Innovation: 32x32 Board with Boundary**

```cpp
// Rapfi's position encoding
struct Pos {
    int16_t _pos;
    constexpr Pos(int x, int y) : _pos(((y + BOARD_BOUNDARY) << 5) | (x + BOARD_BOUNDARY)) {}
    // 5-bit x, 5-bit y packed into 16 bits
    constexpr int x() const { return (_pos & 31) - BOARD_BOUNDARY; }
    constexpr int y() const { return (_pos >> 5) - BOARD_BOUNDARY; }
};
```

**Constants:**
- `FULL_BOARD_SIZE = 32`
- `BOARD_BOUNDARY = 5` (eliminates bounds checking in hot paths)
- `MAX_BOARD_SIZE = 22` (32 - 2*5)

**Applicability to Caro:** Direct adoption possible. The boundary system eliminates bounds checking in pattern evaluation loops.

### 2.2 Pattern System

**Pattern Hierarchy (Rapfi):**

Rapfi uses a precomputed pattern lookup system with 16-bit keys:

```cpp
// Pattern lookup from 64bit bit key
template <Rule R>
inline Pattern2x lookupPattern(uint64_t key) {
    key = fuseKey<R>(key);  // Remove center, compress
    if constexpr (R == Rule::FREESTYLE)
        return PATTERN2x[key];
    else if constexpr (R == Rule::STANDARD)
        return PATTERN2xStandard[key];
    else if constexpr (R == Rule::RENJU)
        return PATTERN2xRenju[key];
}
```

**Pattern Types:**
- `DEAD` → X_.__X (can never make five)
- `OL` → OO.OOO (one step before overline)
- `B1-B3` → Blocked patterns (one, two, three stones)
- `F1-F5` → Flex patterns (one to five stones)
- Combined `Pattern4` evaluates all 4 directions together

**Precomputed Tables:**
- `PATTERN2x[65536]` - Freestyle pattern lookup
- `PCODE[PATTERN_NB][4]` - Combined 4-direction patterns
- `DEFENSE[65536][2]` - Defense recommendations

### 2.3 Move Ordering (Rapfi)

**Score Types:**
```cpp
enum ScoreType {
    ATTACK       = 0b01,
    DEFEND       = 0b10,
    BALANCED     = ATTACK | DEFEND,
    POLICY       = 0b100,       // NNUE-guided
    MAIN_HISTORY = 0b1000,
    COUNTER_MOVE = 0b10000,
};
```

**Move Stages:**
1. TT Move (unconditional #1 for thread work sharing)
2. Policy Score (NNUE-guided if available)
3. Captures/Threats
4. History Moves (killer + counter)
5. Quiets

**Key Insight:** The "unconditional #1" TT move is critical for Lazy SMP work sharing between threads.

### 2.4 History Tables

```cpp
// Main History Table
typedef HistTable<int16_t, 10692, SIDE_NB, FULL_BOARD_CELL_COUNT, MAIN_HIST_TYPE_NB> MainHistory;

// Counter Move History
typedef HistTable<std::pair<Pos, Pattern4>, 0, SIDE_NB, MAX_MOVES> CounterMoveHistory;
```

**Update Formula:**
```cpp
void operator<<(int bonus) {
    value += bonus - value * std::abs(bonus) / Range;
}
```

This ensures values stay bounded within `[-Range, Range]`.

### Recommendations for Caro

1. **Adopt boundary-based board representation** - Eliminates bounds checking
2. **Implement precomputed pattern tables** - O(1) pattern lookup
3. **Add counter-move history** - Natural response to moves
4. **Use bounded history updates** - Prevent overflow

---

## Part 3: Stockfish 18 and Chess Programming Wiki Techniques

### 3.1 Continuation History for Move Ordering

**Overview**
Continuation history captures move pair statistics across multiple plies. Stockfish maintains six levels of continuation history (plies -1 through -6).

**Stockfish Implementation:**
```cpp
using ContinuationHistory = MultiArray<PieceToHistory, PIECE_NB, SQUARE_NB>;

m.value = 2 * mainHistory[us][move];
m.value += continuationHistory[0][pc][to];  // Previous ply
m.value += continuationHistory[1][pc][to];  // Two plies ago
m.value += continuationHistory[2][pc][to];  // Three plies ago
```

**C#/Caro Adaptation:**
```csharp
public class ContinuationHistory
{
    private readonly short[,,] _history = new short[2, 15, 15]; // [player, prevPos, currentPos]
    private const int MaxScore = 30000;

    public int GetScore(Player player, int prevCell, int currentCell)
    {
        if (prevCell < 0 || prevCell >= 15 || currentCell < 0 || currentCell >= 15)
            return 0;
        return _history[(int)player, prevCell, currentCell];
    }

    public void Update(Player player, int prevCell, int currentCell, int bonus)
    {
        if (prevCell < 0 || prevCell >= 15 || currentCell < 0 || currentCell >= 15)
            return;

        int clampedBonus = Math.Clamp(bonus, -MaxScore, MaxScore);
        int current = _history[(int)player, prevCell, currentCell];
        _history[(int)player, prevCell, currentCell] =
            current + clampedBonus - Math.Abs(current * clampedBonus) / MaxScore;
    }
}
```

**Expected ELO Gain:** +15-25 ELO

### 3.2 Adaptive Late Move Reduction

**Overview**
Adaptive LMR reduces search depth for later moves, with dynamic adjustment based on move count, history scores, and node type.

**Stockfish Implementation:**
```cpp
Depth r = reduction(improving, depth, moveCount, delta);

if (ss->ttPv)      r += 946;
if (cutNode)       r += 3372 + 997 * !ttData.move;
if (move == ttData.move) r -= 2151;
if (capture)       r += 1119;

r -= ss->statScore * 850 / 8192;
```

**C#/Caro Adaptation:**
```csharp
public int GetAdaptiveReduction(SearchContext context, Move move, int depth, int moveCount)
{
    int r = CalculateReduction(context.Improving, depth, moveCount, context.Delta);

    if (context.IsPvNode)       r -= 1000;
    if (context.IsCutNode)      r += 1500;
    if (move == context.TTMove) r -= 2000;
    if (move.IsCapture)         r += 1000;

    int historyScore = GetMoveHistoryScore(move, context.Player);
    r -= historyScore * 850 / 8192;

    return Math.Clamp(r / 1024, 0, depth - 1);
}
```

**Expected ELO Gain:** +25-40 ELO

### 3.3 Multi-Entry Transposition Table

**Overview**
Stockfish uses 3 entries per cluster (32 bytes, cache-line aligned) with depth-age replacement.

**Stockfish Implementation:**
```cpp
constexpr int ClusterSize = 3;

struct Cluster {
    TTEntry entry[ClusterSize];
    char padding[2];  // Pad to 32 bytes
};

// Replacement: value = depth - 8 * age
TTEntry* replace = tte;
for (int i = 1; i < ClusterSize; ++i)
    if (replace->depth8 - 8 * replace->relative_age(generation8)
        > tte[i].depth8 - 8 * tte[i].relative_age(generation8))
        replace = &tte[i];
```

**C#/Caro Adaptation:**
```csharp
public class TranspositionTable
{
    private const int ClusterSize = 3;

    public struct TTEntry
    {
        public ulong Key;
        public short Value;
        public sbyte Depth;
        public byte BoundAndAge; // Bound (2 bits) + Age (6 bits)
        public short Move;
        public short Eval;
    }

    public struct Cluster
    {
        public fixed TTEntry Entries[ClusterSize];
        private fixed short _padding[1]; // Pad to 32 bytes
    }

    private Cluster[] _table;
    private byte _generation;

    public bool Probe(ulong key, out TTEntry entry, out TTEntry writer)
    {
        int clusterIndex = (int)((key * (ulong)_clusterCount) >> 32);
        ref Cluster cluster = ref _table[clusterIndex];
        ushort key16 = (ushort)key;

        // Search for matching key
        for (int i = 0; i < ClusterSize; i++)
        {
            if (cluster.Entries[i].Key == key16 && cluster.Entries[i].Depth != -128)
            {
                entry = cluster.Entries[i];
                writer = cluster.Entries[i];
                return true;
            }
        }

        // Find entry to replace based on depth - 8*age
        int replaceIdx = 0;
        int bestValue = int.MinValue;

        for (int i = 0; i < ClusterSize; i++)
        {
            TTEntry e = cluster.Entries[i];
            int age = GetRelativeAge(e.BoundAndAge);
            int value = e.Depth - 8 * age;
            if (value > bestValue)
            {
                bestValue = value;
                replaceIdx = i;
            }
        }

        entry = default;
        writer = cluster.Entries[replaceIdx];
        return false;
    }
}
```

**Expected ELO Gain:** +30-50 ELO

### 3.4 Evaluation Caching

**Overview**
Stockfish maintains correction history tables that record differences between static evaluation and search results.

**Stockfish Implementation:**
```cpp
Value to_corrected_static_eval(const Value v, const int cv) {
    return std::clamp(v + cv / 131072,
                      VALUE_TB_LOSS_IN_MAX_PLY + 1,
                      VALUE_TB_WIN_IN_MAX_PLY - 1);
}

// Update after search
int bonus = std::clamp(int(bestValue - ss->staticEval) * depth / (bestMove ? 10 : 8),
                      -CORRECTION_HISTORY_LIMIT / 4,
                      CORRECTION_HISTORY_LIMIT / 4);
update_correction_history(pos, ss, *this, bonus);
```

**C#/Caro Adaptation:**
```csharp
public class EvaluationCache
{
    private readonly Dictionary<ushort, short> _patternCache = new();
    private const int CacheLimit = 1024;

    public int GetCorrection(ulong positionKey, int staticEval)
    {
        ushort key = (ushort)(positionKey & 0xFFFF);
        if (_patternCache.TryGetValue(key, out short correction))
        {
            return Math.Clamp(staticEval + correction / 256, -1000, 1000);
        }
        return staticEval;
    }

    public void UpdateCorrection(ulong positionKey, int staticEval, int searchResult, int depth)
    {
        ushort key = (ushort)(positionKey & 0xFFFF);
        int error = searchResult - staticEval;
        int bonus = Math.Clamp(error * depth / 10, -CacheLimit, CacheLimit);

        if (_patternCache.TryGetValue(key, out short current))
        {
            int newCorrection = current + bonus - Math.Abs(current * bonus) / CacheLimit;
            _patternCache[key] = (short)newCorrection;
        }
        else
        {
            _patternCache[key] = (short)Math.Clamp(bonus, -CacheLimit, CacheLimit);
        }
    }
}
```

**Expected ELO Gain:** +10-20 ELO

### Summary: Stockfish Techniques

| Technique | ELO Gain | Complexity |
|-----------|----------|------------|
| Continuation History | +15-25 | Medium |
| Adaptive LMR | +25-40 | High |
| Multi-Entry TT | +30-50 | Medium |
| Evaluation Caching | +10-20 | Low |

**Total Estimated ELO Gain: 80-135 ELO**

---

## Part 4: Advanced Optimization Techniques

### 4.1 SPSA (Simultaneous Perturbation Stochastic Approximation)

**Core Concept**
Gradient-free optimization requiring only 2 objective function evaluations per iteration, regardless of parameter count.

**Algorithm:**
```
α = 0.602; γ = 0.101; A = 100; a = 1.0; c = 0.1;
for (k = 0; k < N; k++) {
    ak = a / pow(k + 1 + A, α);
    ck = c / pow(k + 1, γ);
    for each parameter p:
        Δp = 2 * round(rand() / (RAND_MAX + 1.0)) - 1.0;

    Θ_plus = Θ + ck * Δ;
    Θ_minus = Θ - ck * Δ;

    result = match(Θ_plus, Θ_minus);  // Play games, return [-2, +2]
    Θ += ak * result / (ck * Δ);
}
```

**C# Implementation:**
```csharp
public class SPSAOptimizer
{
    private double a = 1.0;
    private double c = 0.1;
    private double A = 100;
    private double alpha = 0.602;
    private double gamma = 0.101;
    private Random rng = new();

    public double[] Optimize(Func<double[], double> objective, int dimensions, int iterations)
    {
        double[] theta = InitializeTheta(dimensions);

        for (int k = 0; k < iterations; k++)
        {
            double ak = a / Math.Pow(k + 1 + A, alpha);
            double ck = c / Math.Pow(k + 1, gamma);

            // Generate perturbation vector
            double[] delta = new double[dimensions];
            for (int i = 0; i < dimensions; i++)
                delta[i] = rng.Next(2) == 0 ? 1.0 : -1.0;

            // Evaluate at perturbed points
            double[] thetaPlus = theta.Zip(delta, (t, d) => t + ck * d).ToArray();
            double[] thetaMinus = theta.Zip(delta, (t, d) => t - ck * d).ToArray();

            double fPlus = objective(thetaPlus);
            double fMinus = objective(thetaMinus);

            // Update parameters
            for (int i = 0; i < dimensions; i++)
            {
                theta[i] += ak * (fPlus - fMinus) / (2.0 * ck * delta[i]);
            }
        }

        return theta;
    }
}
```

**Expected ELO Gain:** +20-40 ELO

### 4.2 RSPSA (Resilient SPSA)

**Enhancement** - Combines SPSA with RPROP for improved convergence:

```csharp
public class RSPSAOptimizer : SPSAOptimizer
{
    private double[] deltaSizes; // Individual step sizes
    private double etaPlus = 1.2;
    private double etaMinus = 0.5;
    private double deltaMin = 1e-6;
    private double deltaMax = 1.0;

    protected override void UpdateStepSizes(double[] gradientSigns)
    {
        for (int i = 0; i < deltaSizes.Length; i++)
        {
            double product = gradientSigns[i] * previousGradientSigns[i];

            if (product > 0)
                deltaSizes[i] = Math.Min(deltaMax, etaPlus * deltaSizes[i]);
            else if (product < 0)
                deltaSizes[i] = Math.Max(deltaMin, etaMinus * deltaSizes[i]);
        }
    }
}
```

**Expected ELO Gain:** +30-60 ELO (with variance reduction)

### 4.3 CLOP (Confident Local Optimization)

**Core Concept**
CLOP by Rémi Coulom uses Bayesian inference to model uncertainty and maintain confidence intervals for each parameter.

**Application to Caro AI:**
```bash
# Define evaluation parameters in config file
caro-ai --mode=clop --config=parameters.clop

# CLOP runs games and automatically converges to optimal settings
```

**Parameters to Tune:**
- Pattern weights (open 3, open 4, blocked patterns)
- Positional bonuses (center control, proximity)
- Defense multiplier
- Threat scores by game phase

**Expected ELO Gain:** +30-50 ELO

### 4.4 Genetic Algorithms for Parameter Exploration

**Core Concept**
Use evolutionary algorithms for global exploration of parameter space before refinement with SPSA/CLOP.

**C# Implementation:**
```csharp
public class GeneticOptimizer
{
    private int populationSize = 50;
    private double mutationRate = 0.1;
    private double crossoverRate = 0.7;
    private int tournamentSize = 3;

    public double[] Optimize(Func<double[], double> fitness, int dimensions, int generations)
    {
        var population = InitializePopulation(populationSize, dimensions);

        for (int gen = 0; gen < generations; gen++)
        {
            var fitnesses = population.Select(ind => fitness(ind)).ToArray();

            // Elitism: keep best
            int bestIdx = Array.IndexOf(fitnesses, fitnesses.Max());
            var newPopulation = new List<double[]> { (double[])population[bestIdx].Clone() };

            while (newPopulation.Count < populationSize)
            {
                var parent1 = TournamentSelection(population, fitnesses);
                var parent2 = TournamentSelection(population, fitnesses);

                double[] offspring = rng.NextDouble() < crossoverRate
                    ? ArithmeticCrossover(parent1, parent2)
                    : (double[])parent1.Clone();

                Mutate(offspring);
                newPopulation.Add(offspring);
            }

            population = newPopulation.ToArray();
        }

        return population.OrderBy(ind => fitness(ind)).Last();
    }

    private double[] ArithmeticCrossover(double[] p1, double[] p2)
    {
        double alpha = rng.NextDouble();
        return p1.Zip(p2, (a, b) => alpha * a + (1 - alpha) * b).ToArray();
    }
}
```

**Expected ELO Gain:** +40-80 ELO (as exploration phase)

### 4.5 TD Learning for Evaluation Tuning

**Core Concept**
Temporal Difference learning learns from experience by bootstrapping from current estimates, ideal for continuous evaluation improvement.

**TD(λ) Algorithm:**
```
For each timestep t:
    δ_t = r_{t+1} + γ·V(s_{t+1}) - V(s_t)  // TD error
    For all states s visited:
        e(s) ← λ·γ·e(s) + 1(s == s_t)       // Eligibility trace
        θ(s) ← θ(s) + α·δ_t·e(s)            // Weight update
```

**TDLeaf for Minimax Search:**
```csharp
public class TDLeafLearner
{
    private double lambda = 0.7;
    private double learningRate = 0.1;

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
                double trace = Math.Pow(lambda, t - i);
                UpdateWeights(leafGradients[i], tdError * trace * learningRate);
            }
        }
    }
}
```

**Expected ELO Gain:** +100-200 ELO with sufficient training

### 4.6 PID Time Management

**Core Concept**
Use control theory for intelligent time allocation in time-controlled games.

**C# Implementation:**
```csharp
public class PIDTimeManager
{
    private double Kp = 1.0;   // Proportional gain
    private double Ki = 0.1;   // Integral gain
    private double Kd = 0.5;   // Derivative gain

    private double integralError = 0;
    private double previousError = 0;

    public TimeSpan AllocateTime(GameState state, TimeSpan totalTimeRemaining, int movesRemaining)
    {
        // Target time per move
        TimeSpan targetTime = TimeSpan.FromTicks(totalTimeRemaining.Ticks / movesRemaining);

        // Error signal
        double error = (totalTimeRemaining - targetTime * movesRemaining).TotalSeconds;

        // Integral term
        integralError += error;

        // Derivative term
        double derivative = error - previousError;
        previousError = error;

        // PID output
        double adjustment = Kp * error + Ki * integralError + Kd * derivative;

        double baseSeconds = targetTime.TotalSeconds + adjustment;
        baseSeconds = ApplyGameModifiers(baseSeconds, state);

        baseSeconds = Math.Max(0.1, Math.Min(baseSeconds, totalTimeRemaining.TotalSeconds * 0.5));

        return TimeSpan.FromSeconds(baseSeconds);
    }

    private double ApplyGameModifiers(double baseTime, GameState state)
    {
        if (state.Complexity > 0.7)
            baseTime *= 1.5;

        if (state.IsForcedMove)
            baseTime *= 0.3;

        if (state.GamePhase == GamePhase.Endgame)
            baseTime *= 1.2;

        return baseTime;
    }
}
```

**Expected ELO Gain:** +20-50 ELO in time-controlled games

### 4.7 Contempt Factor

**Core Concept**
Adjust evaluation based on opponent strength to avoid draws or seek complexity.

**C# Implementation:**
```csharp
public class ContemptManager
{
    private double baseContempt = 0;

    public double ComputeContempt(GameState state, OpponentModel opponent)
    {
        double strengthDifference = opponent.EstimatedStrength - EstimatedStrength;
        double contemptFromStrength = Math.Tanh(strengthDifference / 100);

        double positionContempt = AnalyzePositionForContempt(state);
        double combinedContempt = baseContempt + contemptFromStrength + positionContempt;

        return Math.Max(-200, Math.Min(200, combinedContempt));
    }

    private double AnalyzePositionForContempt(GameState state)
    {
        if (Math.Abs(state.StaticEvaluation) < 50)
            return 20;  // Increase contempt in equal positions
        if (state.StaticEvaluation > 100)
            return -30; // Decrease when winning
        if (state.StaticEvaluation < -100)
            return 50;  // Increase when losing

        return 0;
    }
}
```

**Expected ELO Gain:** +5-20 ELO against varied opposition

### Summary: Advanced Techniques

| Technique | ELO Gain | Complexity | Development Time |
|-----------|----------|------------|------------------|
| SPSA | +20-40 | Medium | 1-2 weeks |
| RSPSA | +30-60 | Medium | 2-3 weeks |
| CLOP | +30-50 | Low (external) | Minimal |
| Genetic Algorithms | +40-80 | Medium | 2-3 weeks |
| TD Learning | +100-200 | High | 4-8 weeks |
| PID Time Management | +20-50 | Low | 1 week |
| Contempt Factor | +5-20 | Low | 3 days |

**Total Potential: 250-470 ELO**

---

## Part 5: Prioritized Implementation Roadmap

### Phase 1: Quick Wins (2-4 weeks)

| Feature | ELO Gain | Complexity | Files to Modify |
|---------|----------|------------|-----------------|
| **Multi-Entry TT** | +30-50 | Medium | `TranspositionTable.cs` |
| **Continuation History** | +15-25 | Medium | `MinimaxAI.cs`, new `ContinuationHistory.cs` |
| **CLOP Integration** | +30-50 | Low | new tuning infrastructure |
| **Contempt Factor** | +5-20 | Low | `MinimaxAI.cs` |
| **PID Time Management** | +20-50 | Low | new `PIDTimeManager.cs` |

**Phase 1 Total:** +100-195 ELO

### Phase 2: Core Optimizations (4-8 weeks)

| Feature | ELO Gain | Complexity | Files to Modify |
|---------|----------|------------|-----------------|
| **SPSA Tuner** | +20-40 | Medium | new `SPSATuner.cs` |
| **Adaptive LMR** | +25-40 | High | `MinimaxAI.cs`, `ParallelMinimaxSearch.cs` |
| **Evaluation Cache** | +10-20 | Medium | `BitBoardEvaluator.cs`, new `EvaluationCache.cs` |
| **RSPSA Enhancement** | +10-20 | Low | enhance SPSA implementation |

**Phase 2 Total:** +65-120 ELO

### Phase 3: Advanced Features (8-16 weeks)

| Feature | ELO Gain | Complexity | Files to Modify |
|---------|----------|------------|-----------------|
| **TD Learning** | +100-200 | High | new `TDLeafLearner.cs` |
| **Pattern Precomputation** | +40-80 | High | `ThreatDetector.cs`, `BitBoardEvaluator.cs` |
| **Boundary-based Board** | +20-40 | High | `BitBoard.cs`, `Position.cs` |
| **Counter-Move History** | +15-25 | Medium | Move ordering infrastructure |

**Phase 3 Total:** +175-345 ELO

### Summary

| Phase | ELO Gain | Timeline |
|-------|----------|----------|
| Phase 1 | +100-195 | 2-4 weeks |
| Phase 2 | +65-120 | 4-8 weeks |
| Phase 3 | +175-345 | 8-16 weeks |
| **Total** | **+340-660** | **14-28 weeks** |

---

## Part 6: Specific Recommendations for Caro

### 6.1 Pattern Recognition Enhancement

**Adopt Rapfi's Pattern Taxonomy:**
- Replace current simple pattern system
- Implement precomputed pattern lookup tables
- Add threat mask evaluation

**Files:** `ThreatDetector.cs`, `BitBoardEvaluator.cs`

### 6.2 Board Representation Optimization

**Consider boundary-based system:**
- Eliminate bounds checking in hot paths
- Use 32x32 with 5-cell boundary

**Files:** `BitBoard.cs`, `Position.cs` (domain)

### 6.3 Move Ordering Enhancement

**Implement continuation history:**
- Track move pairs across 6 plies
- Use bounded update formula
- Combine with existing history heuristic

**Files:** `MinimaxAI.cs`, new `ContinuationHistory.cs`

### 6.4 Weight Tuning Infrastructure

**Build automated tuning:**
- Start with CLOP (fastest ROI)
- Add SPSA for continuous improvement
- Parameter definitions for all evaluation weights

**Files:** new `Caro.Tuner` project

### 6.5 Learning System

**Implement TD Learning:**
- TDLeaf for evaluation tuning from self-play
- Continuous improvement framework
- Save/load trained weights

**Files:** new `Caro.Learning` project

---

## Part 7: Testing and Validation Strategy

### Automated Testing Framework

1. **Tournament Testing:**
   - Round-robin between engine versions
   - Statistical significance testing (ELO calculator)
   - Time control: 1+0, 3+0, 5+0

2. **Position Testing:**
   - Tactical puzzle suite
   - Endgame test positions
   - Opening diversity check

3. **Regression Testing:**
   - Ensure new changes don't break existing strength
   - Performance benchmark (nodes/second)

### Performance Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| NPS (1 thread) | ~100k | ~150k | Benchmark suite |
| NPS (multi-thread) | ~500k | ~800k | Benchmark suite |
| Max depth | 5-6 | 7-8 | Position tests |
| TT hit rate | ~40% | ~60% | Search stats |
| Branching factor | ~2.5 | ~2.0 | Search stats |

---

## Part 8: Conclusion

The Caro AI implementation is already sophisticated with Lazy SMP, PVS, LMR, and transposition tables. The research from Rapfi, Stockfish 18, Chess Programming Wiki, minimax.dev, and advanced optimization techniques reveals clear paths for improvement.

### Top 10 Recommendations (Updated):

1. **TD Learning** (Highest Long-term ROI)
   - +100-200 ELO potential
   - Continuous self-improvement
   - Proven in games like backgammon, chess

2. **NNUE-Style Evaluation** (Game-changer)
   - +100-200 ELO potential
   - Incrementally updated neural network
   - Adapted for exactly-5 gomoku rules

3. **Rapfi BitKey Pattern System** (High Impact)
   - +50-100 ELO potential
   - O(1) pattern lookup via bitkey rotation
   - Eliminates pattern evaluation overhead

4. **Multi-Entry Transposition Table** (Immediate ROI)
   - +30-50 ELO potential
   - Cache-line aligned clusters
   - Depth-age replacement strategy

5. **VCF Solver** (Caro-Specific)
   - +30-50 ELO potential
   - Victory by Continuous Four detection
   - Winning sequence identification

6. **SPSA/RSPSA Automated Tuning**
   - +30-60 ELO potential
   - Proven in Stockfish
   - Efficient for high-dimensional optimization

7. **CLOP Integration**
   - +30-50 ELO potential
   - External tool, minimal integration
   - Excellent for parameter exploration

8. **Continuation History**
   - +15-25 ELO potential
   - Proven technique from Stockfish
   - Works synergistically with existing history

9. **Pattern4 Combined Evaluation**
   - +40-80 ELO potential
   - 4-direction pattern combination
   - Threat-aware scoring

10. **PID Time Management**
    - +20-50 ELO in time controls
    - Adaptive time allocation
    - Position complexity awareness

### Total Potential:

**Conservative estimate:** 450-600 ELO improvement
**Aggressive estimate:** 700-1000+ ELO improvement (with TD learning and NNUE)

This would elevate the Caro engine from "strong" to "world-class" in Gomoku competition.

---

## Appendix: References

1. **Rapfi:** https://github.com/dhbloo/rapfi
2. **Stockfish 18:** https://github.com/official-stockfish/Stockfish
3. **Chess Programming Wiki:** https://www.chessprogramming.org/
4. **CLOP:** https://www.remi-coulom.fr/CLOP/

### Key Papers

1. "CLOP: Confident Local Optimization for Noisy Tuning" - Rémi Coulom
2. "SPSA for Noisy Optimization" - Spall
3. "History Heuristic" - Jonathan Schaeffer
4. "Late Move Reductions" - Ernst A. Heinz
5. "TDLeaf(λ): Combining Temporal Difference Learning with Game-Tree Search" - Baxter et al.
6. "RSPSA: Enhanced Parameter Optimization in Games" - Kocsis et al.

---

## Part 9: Extended Research Findings (2026-02-12 Update)

This section contains additional insights from deeper analysis of Rapfi, Stockfish 18, Chess Programming Wiki, and minimax.dev resources.

### 9.1 Rapfi BitKey System for Pattern Recognition

Rapfi uses an innovative bitkey system to encode board positions for O(1) pattern lookups:

```cpp
// Four directional bitkeys (64-bit each)
uint64_t bitKey0[FULL_BOARD_SIZE];          // Horizontal (RIGHT-LEFT)
uint64_t bitKey1[FULL_BOARD_SIZE];          // Vertical (DOWN-UP)
uint64_t bitKey2[FULL_BOARD_SIZE * 2 - 1];  // Diagonal (UP_RIGHT-DOWN_LEFT)
uint64_t bitKey3[FULL_BOARD_SIZE * 2 - 1];  // Anti-diagonal (DOWN_RIGHT-UP_LEFT)

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

**Key Insights for Caro:**
- Uses 2 bits per cell (00=empty, 01=black, 10=white, 11=unused)
- Rotation aligns the pattern around the position being evaluated
- Pattern lookup uses fused key that removes center cell and compresses

**C#/Caro Adaptation:**
```csharp
public class BitKeyBoard
{
    private const int FullBoardSize = 32;
    private const int BoardBoundary = 5;
    private const int HalfLineLen = 6; // For exactly-5 rules

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

**Expected ELO Gain:** +50-100 ELO (significant pattern evaluation speedup)

### 9.2 Rapfi Pattern4 System for Threat Detection

Rapfi's Pattern4 is a combined 4-direction pattern evaluation that categorizes positions:

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
    PATTERN4_NB
};
```

**Caro-Specific Adaptation for Exactly-5 Rule:**
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
    FourWithThree, // Four in a row with three potential
    Exactly5,   // Win condition
    Overline    // Invalid in Caro (exactly-5 rule)
}
```

### 9.3 Stockfish Move Picker Architecture

Stockfish uses a sophisticated staged move picker:

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
    EVASION_INIT,     // Check evasion initialization
    EVASION,          // Check evasion moves
    PROBCUT           // Probation cut moves
};
```

**Move Scoring in Stockfish:**
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

**C#/Caro Move Picker:**
```csharp
public class CaroMovePicker
{
    private enum Stage
    {
        MainTT,
        ThreatInit,
        Threat,
        GoodCapture,    // In Caro: forcing moves
        Refutation,     // Killer + counter moves
        QuietInit,
        GoodQuiet,
        BadCapture,
        BadQuiet
    }

    public Move? NextMove()
    {
        switch (_stage)
        {
            case Stage.MainTT:
                _stage = Stage.ThreatInit;
                return _ttMove;

            case Stage.ThreatInit:
                GenerateThreatMoves();
                _stage = Stage.Threat;
                goto case Stage.Threat;

            case Stage.Threat:
                if (SelectThreat())
                    return _currentMove;
                _stage = Stage.Refutation;
                goto case Stage.Refutation;

            // ... continue stages
        }
        return null;
    }
}
```

### 9.4 NNUE Concepts for Gomoku

From Stockfish's NNUE implementation, key concepts applicable to gomoku:

**Network Architecture:**
- 4-layer network (W1 through W4)
- W1 is overparameterized but incrementally updated
- Only changed neurons recalculated on move/unmove
- Remaining layers computed with SIMD (AVX2/AVX-512)

**Incremental Update Pattern:**
```csharp
// For gomoku, W1 input could be:
// - Position occupancy (19x19 = 361 inputs)
// - Pattern codes for each direction at each position
// - Threat masks

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

**For Caro's Exactly-5 Rule:**
- Network should penalize overlines
- Output should be trained on self-play with exactly-5 validation
- Pattern encoding should distinguish between 4, 5, and 6+ in a row

### 9.5 Transposition Table Advanced Techniques

From Chess Programming Wiki research:

**Depth-Age Replacement Formula:**
```
replace_score = depth - 8 * age
```

**Lockless Hashing (for parallel search):**
```csharp
// XOR key with stored data to detect corruption
public struct TTEntry
{
    public ulong Key;           // XOR of position key with data
    public short Value;
    public sbyte Depth;
    public byte BoundAndAge;
    public short Move;
    public short StaticEval;
}

public void Store(ulong key, short value, sbyte depth, byte bound, short move, short eval)
{
    // XOR key with data for lockless parallel access
    entry.Key = key ^ (ulong)(ushort)value ^ ((ulong)(byte)depth << 16)
                     ^ ((ulong)bound << 24) ^ ((ulong)(ushort)move << 32);
    entry.Value = value;
    entry.Depth = depth;
    // ...
}
```

**Multiple Probes Strategy:**
```csharp
// Probe multiple entries in cluster
public bool ProbeMultiple(ulong key, out TTEntry bestEntry)
{
    int clusterIdx = GetClusterIndex(key);
    var cluster = _clusters[clusterIdx];

    bestEntry = default;
    int bestDepth = -1;

    for (int i = 0; i < ClusterSize; i++)
    {
        var entry = cluster.Entries[i];
        if (KeyMatches(entry, key) && entry.Depth > bestDepth)
        {
            bestEntry = entry;
            bestDepth = entry.Depth;
        }
    }

    return bestDepth >= 0;
}
```

### 9.6 VCF (Victory by Continuous Four) for Caro

Specific to gomoku variants, VCF is a winning strategy using consecutive four-threats:

```csharp
public class CaroVCFSolver
{
    // VCF: Create open fours until opponent cannot block all
    public (bool CanWin, List<Move> Sequence) FindVCFSequence(Board board, int maxDepth)
    {
        for (int depth = 1; depth <= maxDepth; depth++)
        {
            var result = SearchVCF(board, depth, true);
            if (result.CanWin)
                return result;
        }
        return (false, null);
    }

    private (bool CanWin, List<Move> Sequence) SearchVCF(Board board, int depth, bool isAttacker)
    {
        if (depth == 0)
            return (false, null);

        if (isAttacker)
        {
            // Find all four-threat moves
            var fourThreats = FindFourThreats(board);
            foreach (var threat in fourThreats)
            {
                board.MakeMove(threat.Move);
                var result = SearchVCF(board, depth - 1, false);
                board.UnmakeMove();

                if (result.CanWin)
                {
                    result.Sequence.Insert(0, threat.Move);
                    return result;
                }
            }
            return (false, null);
        }
        else
        {
            // Defender must block all threats
            var blocks = FindRequiredBlocks(board);
            if (blocks.Count == 0)
                return (true, new List<Move>()); // Already won

            if (blocks.Count > 1)
                return (false, null); // Cannot block multiple

            board.MakeMove(blocks[0]);
            var result = SearchVCF(board, depth - 1, true);
            board.UnmakeMove();

            if (result.CanWin)
                result.Sequence.Insert(0, blocks[0]);
            return result;
        }
    }
}
```

### 9.7 Caro-Specific Opening Book Enhancements

For Caro's long opening rule (second stone >=3 intersections from first):

```csharp
public class CaroOpeningValidator
{
    public bool IsValidSecondMove(Move firstMove, Move secondMove)
    {
        int dx = Math.Abs(secondMove.X - firstMove.X);
        int dy = Math.Abs(secondMove.Y - firstMove.Y);
        int chebyshev = Math.Max(dx, dy);

        return chebyshev >= 3;
    }

    public List<Move> GetValidSecondMoves(Move firstMove)
    {
        var validMoves = new List<Move>();
        int cx = firstMove.X;
        int cy = firstMove.Y;

        // Generate moves at distance >= 3 from first stone
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                int dist = Math.Max(Math.Abs(x - cx), Math.Abs(y - cy));
                if (dist >= 3 && dist <= 6) // Reasonable range
                    validMoves.Add(new Move(x, y));
            }
        }

        return validMoves;
    }
}
```

### 9.8 Summary: Additional Research Findings

| Source | Key Finding | ELO Impact | Implementation Priority |
|--------|-------------|------------|------------------------|
| Rapfi BitKey | O(1) pattern lookup via bitkey rotation | +50-100 | High |
| Rapfi Pattern4 | 4-direction combined pattern evaluation | +40-80 | High |
| Stockfish MovePicker | Staged move generation with partial insertion sort | +20-40 | Medium |
| Stockfish NNUE | Incrementally updated neural network | +100-200 | Long-term |
| TT Lockless | XOR-based key verification for parallel search | +10-20 | Medium |
| VCF Solver | Victory by continuous four-threat detection | +30-50 | High |
| Opening Validator | Caro-specific long opening rule support | +10-20 | Low |

**Additional ELO Potential from New Research:** 260-530 ELO

**Combined Total Potential:** 600-1200 ELO improvement possible with full implementation.

---

## Appendix B: Additional References

### Repositories
- **Rapfi:** https://github.com/dhbloo/rapfi (Gomoku/Renju engine)
- **Stockfish 18:** https://github.com/official-stockfish/Stockfish
- **YaneuraOu:** https://github.com/yaneurao/YaneuraOu (Shogi NNUE)

### Documentation
- **Chess Programming Wiki:** https://www.chessprogramming.org/
- **Stockfish Wiki:** https://www.chessprogramming.org/Stockfish
- **NNUE Paper:** Yu Nasu (2018) "Efficiently Updatable Neural-Network based Evaluation Functions"
- **minimax.dev:** https://minimax.dev/ (Ultimate Tic Tac Toe solving)

### Key Topics
- Transposition Table: https://www.chessprogramming.org/Transposition_Table
- History Heuristic: https://www.chessprogramming.org/History_Heuristic
- Late Move Reductions: https://www.chessprogramming.org/Late_Move_Reductions
- Lazy SMP: https://www.chessprogramming.org/Lazy_SMP
- NNUE: https://www.chessprogramming.org/NNUE

---

**End of Report**
