# Checkpoint: v1.60.0 Development

## Summary

Investigating AI strength inversions between difficulty levels. Primary issue: Easy loses 100% to Braindead at blitz time controls (3+2), and Hard beats GM in some games.

## Current Status: LAZY SMP FIX IMPLEMENTED

**Fix Applied**: Added helper thread depth offset per Chessprogramming Wiki's Lazy SMP algorithm.

### Results After Fix

| Metric | Before Fix | After Fix | Improvement |
|--------|------------|-----------|-------------|
| GM NPS | 300-700 | 6.9K-7.8K | **10-20x** |
| GM Depth | D1 (stuck) | D1-D2 | More variation |
| TT Hit Rate | 0% | 8.3%-42.8% | Now working! |
| Helper Depth | 0.0 | 1.0-2.0 | Helpers contributing |

### Matchup Results After Fix (QuickSmokeTest)

| Matchup | Result | Notes |
|---------|--------|-------|
| Braindead vs Easy | Braindead 2-0 | Games longer (25-34 moves) |
| Easy vs Braindead | Braindead 2-0 | Games longer (32-37 moves) |
| GM vs Hard | Timed out | NPS improved significantly |

**Observation**: Lazy SMP fix improved performance 10-20x, but Braindead still beats Easy 100%. This indicates a **separate issue** with shallow-depth evaluation or move ordering.

## Key Findings from QuickSmokeTest

| Matchup | Result | Critical Observation |
|---------|--------|---------------------|
| Braindead vs Easy | Braindead 4-0 | 100% Braindead win rate |
| GM vs Hard (Game 5) | **Hard wins** | Strength inversion! |
| GM vs Hard (Game 6) | 200+ moves, still running | GM (9 threads) reaches D1, Hard (5 threads) reaches D4 |

### NPS Collapse Analysis (GM vs Hard)

| Move | Player | Threads | Depth | Nodes | Time | NPS |
|------|--------|---------|-------|-------|------|-----|
| G5 M1 | GM | 9 | D1 | 2.0K | 5.5s | **356** |
| G5 M3 | GM | 9 | D1 | 5.8K | 8.5s | **688** |
| G5 M2 | Hard | 5 | D4* | 0 | 1.0s | - |

**Critical**: GM with MORE threads (9) achieves WORSE depth (D1) than Hard with FEWER threads (5, D4). NPS is catastrophically low (300-700 instead of 50K-500K).

*Hard's D4 came from ponder results (N:0 = no main search nodes).

## Root Cause: Lazy SMP Implementation Bug

### What the Correct Algorithm Should Do (per Chessprogramming Wiki)

According to [Chessprogramming.org Lazy SMP](https://www.chessprogramming.org/Lazy_SMP):

```
starting helper threads:
  for each helper thread:
    signal helper to start root search at current depth
    (add 1 for each even helper assuming 0-based indexing)
```

**Key**: Helper threads should search at DIFFERENT depths (D, D+1, D, D+1, ...) to exploit nondeterminism.

### What Our Implementation Does

Our `SearchWithIterationTimeAware` starts ALL threads at the SAME depth:
- All 9 threads search at depth 1
- All threads complete at similar times
- No exploitation of nondeterminism
- Threads compete for same hash table entries

### Consequences

1. **Redundant work**: All threads search the same tree, no diversity
2. **Thread contention**: 9 threads fighting for same TT entries
3. **Cache thrashing**: High coherence traffic between threads
4. **Performance collapse**: More threads = SLOWER, not faster

## Secondary Issue: Ponder Result Usage

When Hard shows "D4" with "N:0", it means:
- Main search didn't complete (0 nodes)
- Move was selected from **ponder results** (opponent's turn thinking)
- Ponder reached D4, but it's for a different position tree

This gives a false appearance of deeper search when the main search actually failed.

## Files Involved

| File | Issue |
|------|-------|
| `ParallelMinimaxSearch.cs` | Missing helper thread depth offset |
| `LockFreeTranspositionTable.cs` | Sharding works, but contention still high |
| `AIDifficultyConfig.cs` | Thread counts may need adjustment after fix |

## Fix Plan

### Phase 1: Fix Helper Thread Depth Offset

In `SearchWithIterationTimeAware`, modify the starting depth for helper threads:

```csharp
// Current: all threads start at depth 1
int currentDepth = 1;

// Should be: helper threads get depth offset
int currentDepth = (threadData.ThreadIndex % 2 == 0) ? 1 : 2;
// Or per Cheng: threadIndex + 1 for even-indexed helpers
```

### Phase 2: Test with Fix

Run QuickSmokeTest to verify:
1. GM achieves deeper search than Hard (not shallower)
2. NPS increases to expected range (50K+)
3. Strength inversions are resolved

### Phase 3: Consider Thread Count Reduction

If Lazy SMP doesn't scale well beyond 4-8 threads:
- Reduce GM thread count from 9 to 5-6
- Reduce Hard thread count from 5 to 3-4

## Sources

- [Lazy SMP - Chessprogramming Wiki](https://www.chessprogramming.org/Lazy_SMP)
- Cheng's Pseudo Code for correct implementation

## Test Status

All 575 backend tests passing. Matchup tests show systematic issues that need code fix.

## Important Note

The "Known Limitation" in README.md about blitz time controls is INCOMPLETE. The issue is not just depth parity - there's a **bug in the Lazy SMP implementation** that causes more threads to be SLOWER than fewer threads.
