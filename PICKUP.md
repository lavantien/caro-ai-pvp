# PICKUP.md - Session Continuation Guide

## Context Summary

**Session 2025-01-20**: Implemented comprehensive AI Strength Validation Test Suite with statistical analysis. This provides proper statistical validation for AI difficulty levels, replacing the basic QuickTest.cs with a robust framework.

**Previous Session**: Focused on implementing advanced AI features: Lazy SMP parallel search, constant pondering, MDAP, and adaptive time management. While implementations were successful, a critical non-determinism issue was discovered in parallel search that causes AI strength inversion in some test runs.

## Key Issues Addressed

### 1. Lazy SMP Parallel Search - IMPLEMENTED (HAS ISSUES)
- **Thread Count Formula**: `(processorCount / 2) - 1` for conservative multi-core usage
- **Files Modified**:
  - `ThreadPoolConfig.cs` - Added `GetLazySMPThreadCount()` and `GetPonderingThreadCount()`
  - `ParallelMinimaxSearch.cs` - Updated constructor to use new thread formula
  - `MinimaxAI.cs` - Enabled parallel search for D7+ (depth 4+)

**Status**: Enabled but **NON-DETERMINISTIC** - see Outstanding Issues below

### 2. Constant Pondering for D7+ - IMPLEMENTED
- **Files Modified**: `Ponderer.cs`
- D7+ (VeryHard and above) always ponders during opponent's turn
- D1-D6 only ponders when there are immediate threats (VCF pre-check enabled)

### 3. MDAP (Move-Dependent Adaptive Pruning) - IMPLEMENTED
- **Files Modified**: `ParallelMinimaxSearch.cs`
- First 4 moves searched at full depth
- Moves after index 4 get reduced depth (base -1, scaling with move index)
- High-priority moves (hash moves) skip reduction
- Verification re-search if reduced depth score beats alpha/beta

### 4. Adaptive Time Management - INTEGRATED
- **Files Modified**: `MinimaxAI.cs`
- Switched from `TimeManager` to `AdaptiveTimeManager`
- PID-like controller with feedback loop
- Added `ReportTimeUsed()` calls after each move
- Added reset in `ClearAllState()`

### 5. SIMD Evaluator Bug - DOCUMENTED (NOT FIXED)
- **Files Modified**:
  - `EvaluatorComparisonTests.cs` - Comprehensive scalar vs SIMD comparison tests
  - `SIMDDebugTest.cs` - Focused debugging tests
  - `SIMDPerspectiveTest.cs` - Perspective verification tests
  - `BoardEvaluator.cs` - Detailed bug documentation

**Bug**: SIMD evaluator has sign inversion causing 22000 point score difference
- Scalar: -19865 (correctly penalizes Blue's threat)
- SIMD: +2135 (incorrectly ADDS Blue's weighted threat)
- **Status**: SIMD evaluator remains disabled

### 6. AI Strength Validation Test Suite - NEWLY IMPLEMENTED (2025-01-20)

**Purpose**: Provides comprehensive statistical validation for AI difficulty levels, replacing the basic QuickTest.cs with a robust, scientifically-grounded testing framework.

**Files Created**:
1. `StatisticalAnalyzer.cs` - Core statistical analysis functions
   - CalculateLOS() - Likelihood of Superiority (probability one AI is truly stronger)
   - CalculateEloWithCI() - Elo difference with 95% confidence intervals
   - BinomialTestPValue() - Statistical significance testing
   - DetectColorAdvantage() - Color bias detection using paired game analysis
   - SPRT() - Sequential Probability Ratio Test for early termination

2. `MatchupStatistics.cs` - Data model for matchup results
   - Tracks wins, draws, color-specific performance
   - Stores LOS, Elo difference, confidence intervals, p-values
   - Provides summary string formatting

3. `StatisticalAnalyzerTests.cs` - Unit tests (38 tests, all passing)
   - Known-value tests for mathematical functions
   - Property-based tests for range validation
   - Color advantage detection tests

4. `AIStrengthValidationSuite.cs` - XUnit test suite with 4 phases:
   - Phase 1: Adjacent difficulty testing (D11 vs D10, D10 vs D9, etc.)
   - Phase 2: Cross-level testing (D11 vs D9, D11 vs D8, etc.)
   - Phase 3: Color advantage detection (symmetric matchups)
   - Phase 4: Quick round-robin (Elo ranking)

5. `HtmlReportGenerator.cs` - HTML report generation
   - Beautiful, responsive HTML reports
   - Summary statistics with pass/fail tracking
   - Elo ranking tables
   - Color advantage visualization

6. `AIStrengthTestRunner.cs` - CLI runner
   - Configurable games per matchup, time control
   - Progress reporting during long runs
   - HTML report generation
   - Quick validation mode (10 games per matchup)

**Usage**:
```bash
# Run full validation suite (25 games per matchup)
cd backend/src/Caro.TournamentRunner
dotnet run -- --validate-strength

# Quick validation (10 games per matchup)
dotnet run -- --quick-validate

# Custom configuration
dotnet run -- --validate-strength --games 50 --verbose

# Run XUnit tests
cd backend/tests/Caro.Core.Tests
dotnet test --filter "FullyQualifiedName~AIStrengthValidationSuite"
```

**Statistical Methods Used**:
- **Elo Difference**: Standard chess rating system (400-point scale)
- **95% Confidence Intervals**: Delta method approximation
- **Likelihood of Superiority (LOS)**: Based on error function
- **Binomial Test**: For win rate significance (p < 0.05)
- **Paired Game Design**: Color swapping to neutralize first-move advantage

**Configuration**:
- Default: 25 games per matchup, 2+1 time control
- Sample size based on industry best practices (CCRL, CEGT)
- 40-100 games for preliminary results, 200+ for high confidence

## Test Results (2025-01-20) - 7+5 Time Control with Lazy SMP

| Test | Expected | Actual | Result | Notes |
|------|----------|--------|--------|-------|
| D11 vs D11 | Tie | D11 won | ⚠️ Expected tie | Parallel working but VCF decisive |
| D11 vs D10 | D11 wins | D11 won (22m) | ✅ PASS | VCF-only advantage working |
| D10 vs D8 | D10 wins | **INCONSISTENT** | ❌ **FAIL** | Non-deterministic! |
| D11 vs D6 | D11 wins | D11 won (18m) | ✅ PASS | VCF decisive |
| D4 vs D6 | D6 wins | D6 won (27m) | ✅ PASS | Depth advantage wins |

## Outstanding Issues

### 1. PARALLEL SEARCH NON-DETERMINISM - CRITICAL BUG 🔴

**Symptom**: D10 vs D8 test produces inconsistent results between runs
- **Run 1**: Grandmaster (D10) won in 8 moves ✅
- **Run 2**: Expert (D8) won in 19 moves ❌

**Root Cause**: Lazy SMP relies on "natural" diversity from thread timing differences, but this introduces non-determinism:
1. Different threads finish at different times due to OS scheduling
2. Result aggregation may pick suboptimal moves depending on which thread finishes first
3. Transposition table race conditions during concurrent writes
4. Time-based cancellation happening at different points in different threads

**Evidence**: The exact same matchup (D10 vs D8) with same code produces different winners, proving non-determinism.

**Impact**: AI strength ordering is NOT RELIABLE with parallel search enabled.

### 2. Why Parallel Search is Non-Deterministic

The current implementation in `ParallelMinimaxSearch.cs`:
- Multiple threads search the SAME tree independently
- Each thread has its own killer moves, history tables
- Shared transposition table for caching
- Master thread (ThreadIndex=0) is supposed to be deterministic
- BUT: Threads finish at different times, and TT entries depend on search order

**The fundamental issue**: When helper threads finish at different times, they write different entries to the TT. The master thread then reads these entries, which can vary based on thread timing.

## Files Modified This Session

### Previous Session Files
1. **ThreadPoolConfig.cs**
   - Added `GetLazySMPThreadCount()` - returns `(processorCount/2)-1`
   - Added `GetPonderingThreadCount()` - returns `processorCount/4` for pondering

2. **ParallelMinimaxSearch.cs**
   - Updated constructor to use `GetLazySMPThreadCount()` by default
   - Added MDAP constants: `LMRMinDepth`, `LMRFullDepthMoves`, `LMRBaseReduction`
   - Implemented LMR in `Minimax()` with verification re-search
   - Updated class documentation to reflect all optimizations

3. **MinimaxAI.cs**
   - Enabled Lazy SMP for D7+ (depth 4+)
   - Switched to `AdaptiveTimeManager` instead of `TimeManager`
   - Added time tracking and `ReportTimeUsed()` feedback loop
   - Added `_adaptiveTimeManager.Reset()` in `ClearAllState()`
   - Updated class documentation

4. **Ponderer.cs**
   - Modified `StartPondering()` to skip VCF pre-check for D7+
   - Updated class documentation

5. **Test Files Created (Previous)**
   - `EvaluatorComparisonTests.cs` - Scalar vs SIMD comparison
   - `SIMDDebugTest.cs` - Focused SIMD debugging tests
   - `SIMDPerspectiveTest.cs` - Perspective verification tests

### New Files (2025-01-20) - AI Strength Validation Suite

#### Core Library Files
1. **src/Caro.Core/Tournament/StatisticalAnalyzer.cs**
   - CalculateLOS() - Likelihood of Superiority
   - CalculateEloWithCI() - Elo with confidence intervals
   - BinomialTestPValue() - Statistical significance test
   - DetectColorAdvantage() - Color bias detection
   - SPRT() - Sequential Probability Ratio Test

2. **src/Caro.Core/Tournament/MatchupStatistics.cs**
   - Data model for matchup results
   - Color-specific win tracking (RedAsRed_Wins, BlueAsBlue_Wins, etc.)
   - Statistical metrics (LOS, Elo, CI, p-value)
   - ToSummaryString() for formatted output

#### Test Files
3. **tests/Caro.Core.Tests/Tournament/StatisticalAnalyzerTests.cs** (38 tests)
   - Known-value tests for mathematical functions
   - Property-based range validation tests
   - Color advantage detection tests
   - SPRT early termination tests

4. **tests/Caro.Core.Tests/Tournament/AIStrengthValidationSuite.cs**
   - Phase 1: Adjacent difficulty tests (10 matchups)
   - Phase 2: Cross-level tests (4 large gaps)
   - Phase 3: Color advantage detection (4 symmetric tests)
   - Phase 4: Quick round-robin (Elo ranking)

#### CLI and Reports
5. **src/Caro.TournamentRunner/ReportGenerators/HtmlReportGenerator.cs**
   - Responsive HTML report generation
   - Summary statistics with pass/fail tracking
   - Elo ranking tables
   - Color advantage visualization
   - CSS styling for beautiful output

6. **src/Caro.TournamentRunner/AIStrengthTestRunner.cs**
   - RunAllAsync() - Full validation suite
   - RunQuickAsync() - Quick validation (10 games)
   - Configurable games, time control, pondering
   - Progress reporting

7. **src/Caro.TournamentRunner/Program.cs** (Modified)
   - Added --validate-strength flag
   - Added --quick-validate flag
   - Added --games, --time, --inc options
   - Added ShowHelp() function

## Key Findings

### 1. Parallel Search Non-Determinism is a Fundamental Issue

Lazy SMP as implemented is inherently non-deterministic because:
- Thread scheduling is OS-dependent and non-deterministic
- Shared TT state depends on which thread writes first
- Master thread result selection depends on timing

**Potential Solutions** (for future work):
1. **Root Parallelization**: Different threads search different root moves, then compare
2. **YBWC (Young Brothers Wait Concept)**: Helper threads depend on master thread
3. **Deterministic TT partitioning**: Each thread has its own TT partition
4. **Full tree search with parallel evaluation**: Parallelize only the evaluation function

### 2. MDAP Implementation Appears Sound

The LMR implementation follows standard practices:
- Full depth for first 4 moves
- Reduced depth for late moves
- Verification re-search when reduced depth is promising
- High-priority moves skip reduction

No evidence that MDAP is causing issues.

### 3. SIMD Evaluator Bug is Well-Documented

The sign inversion bug (22000 point difference) is now:
- Thoroughly documented in `BoardEvaluator.cs`
- Reproducible via unit tests
- SIMD evaluator remains safely disabled

## Next Session Priorities

### Priority 1: DISABLE PARALLEL SEARCH UNTIL FIXED

The non-determinism is a critical bug that makes AI strength ordering unreliable. Options:

1. **Immediate**: Disable parallel search (revert to sequential only)
2. **Investigation**: Study the race conditions and timing dependencies
3. **Fix**: Implement one of the solutions listed above

### Priority 2: Verify Baseline with Sequential Search Only

Run the full test suite with parallel search disabled to confirm AI strength ordering is stable without it.

### Priority 3: Re-implement Parallel Search (if desired)

Once baseline is confirmed, consider:
- Root parallelization (safest, deterministic)
- YBWC (more complex but proven in chess engines)
- Or accept the current Lazy SMP with documented non-determinism

## Git Status

**Previously modified files (from earlier session):**
- `ThreadPoolConfig.cs`
- `ParallelMinimaxSearch.cs`
- `MinimaxAI.cs`
- `Ponderer.cs`
- Test files: `EvaluatorComparisonTests.cs`, `SIMDDebugTest.cs`, `SIMDPerspectiveTest.cs`

**New files (2025-01-20) - AI Strength Validation Suite:**
- `src/Caro.Core/Tournament/StatisticalAnalyzer.cs`
- `src/Caro.Core/Tournament/MatchupStatistics.cs`
- `tests/Caro.Core.Tests/Tournament/StatisticalAnalyzerTests.cs`
- `tests/Caro.Core.Tests/Tournament/AIStrengthValidationSuite.cs`
- `src/Caro.TournamentRunner/ReportGenerators/HtmlReportGenerator.cs`
- `src/Caro.TournamentRunner/AIStrengthTestRunner.cs`
- `src/Caro.TournamentRunner/Program.cs` (modified)

## Commands

### AI Strength Validation (NEW)
```bash
# Full validation suite (25 games per matchup, ~30-60 minutes)
cd backend/src/Caro.TournamentRunner
dotnet run -- --validate-strength

# Quick validation (10 games per matchup, ~10-15 minutes)
dotnet run -- --quick-validate

# Custom configuration
dotnet run -- --validate-strength --games 50 --verbose
dotnet run -- --validate-strength --games 100 --time 180 --inc 2

# Show help
dotnet run -- --help
```

### Run XUnit Tests
```bash
# Statistical analyzer tests (38 tests, fast)
cd backend/tests/Caro.Core.Tests
dotnet test --filter "FullyQualifiedName~StatisticalAnalyzerTests"

# Full AI strength validation suite (slow, runs actual AI games)
dotnet test --filter "FullyQualifiedName~AIStrengthValidationSuite"

# Specific phase
dotnet test --filter "FullyQualifiedName~Phase1"
dotnet test --filter "FullyQualifiedName~Phase3_1"
```

### Legacy Quick Tests
```bash
cd backend/src/Caro.TournamentRunner
dotnet run -- --test
```

### Build
```bash
cd backend
dotnet build
```

### Run Specific Tests
```bash
cd backend/tests/Caro.Core.Tests
dotnet test --filter "FullyQualifiedName~EvaluatorComparisonTests"
```

## Implementation Details

### Lazy SMP Thread Count Formula
```csharp
// Formula: (total threads/2) - 1
// Example: 20 cores -> (20/2)-1 = 9 helper threads
public static int GetLazySMPThreadCount()
{
    int processorCount = Environment.ProcessorCount;
    int halfCount = processorCount / 2;
    return Math.Max(1, halfCount - 1);
}
```

### MDAP (Late Move Reduction)
```csharp
// Apply LMR for late moves when depth is sufficient
if (depth >= LMRMinDepth && moveIndex >= LMRFullDepthMoves)
{
    bool isHighPriority = (cachedMove.HasValue && cachedMove.Value == (x, y));
    if (!isHighPriority)
    {
        int extraReduction = Math.Min(2, (moveIndex - LMRFullDepthMoves) / 4);
        reducedDepth = depth - LMRBaseReduction - extraReduction;
        if (reducedDepth < 1) reducedDepth = 1;
        doLMR = true;
    }
}
```

### Constant Pondering for D7+
```csharp
bool isHighDifficulty = difficulty >= AIDifficulty.VeryHard;
if (!isHighDifficulty)
{
    // VCF pre-check - skip pondering if no immediate threats
    // D1-D6 only ponder when there are threats
}
// D7+ always ponders regardless of position
```

## Known Workarounds

1. **Parallel Search Disabled**: Set `useParallelSearch = false` in MinimaxAI.cs line 303 to use sequential search only
2. **SIMD Disabled**: Already disabled in BoardEvaluator.cs line 74
3. **VCF Defense Disabled**: Already disabled in MinimaxAI.cs lines 221-235

## Performance Impact

With parallel search enabled (9 threads):
- Node throughput: 10K-100K nodes/sec depending on position
- NPS varies significantly due to thread scheduling
- Time per move: 5-30 seconds for D7-D11

With sequential search:
- Node throughput: More consistent but lower
- Time per move: More predictable

## Conclusion

All requested features were successfully implemented, but the parallel search has a critical non-determinism bug that causes AI strength inversion in some test runs. Until this is fixed, the parallel search should be disabled for reliable AI strength ordering.
