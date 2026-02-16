# Checkpoint: v1.60.0 Development

## Summary

Fixed AI strength inversion between Easy and Braindead at blitz time controls (3+2).

## Issues Resolved

### 1. Parallel Search Overhead at Blitz Time Controls

**Problem**: At blitz time controls (< 2s per move), parallel search overhead made Easy perform worse than single-threaded.

**Solution**: Added time-based threshold for parallel search. If `HardBoundMs < 2000ms`, use single-threaded search.

```csharp
const long ParallelTimeThresholdMs = 2000;
bool shouldUseParallel = threadCount > 1 && timeAlloc.HardBoundMs >= ParallelTimeThresholdMs;
```

### 2. Time Bound Enforcement

**Problem**: D1-D2 iterations could take 2-3x longer than the time budget because time checks were skipped for shallow depths.

**Solution**: Added pre-iteration time estimate check:
```csharp
if (lastIterationElapsedMs > 0 && remainingTimeMs < lastIterationElapsedMs * 2)
{
    if (bestDepth >= 1) break;
}
```

### 3. Easy vs Braindead Strength Inversion

**Problem**: Easy was losing 100% to Braindead at blitz time controls.

**Root Cause**: Both AIs reached similar depths (D1-D2) at blitz, and 10% error rate wasn't enough to make Braindead clearly weaker.

**Solution**: Increased Braindead error rate from 10% to 40%.

| Setting | Before | After |
|---------|--------|-------|
| Braindead error rate | 10% | 40% |

## Test Results

After fixes, Easy now wins consistently against Braindead:
- Game 1: Easy (Blue) wins
- Game 2: Easy (Red) wins

## Files Modified

| File | Change |
|------|--------|
| `ParallelMinimaxSearch.cs` | Time-based parallel threshold, pre-iteration time estimate |
| `AIDifficultyConfig.cs` | Braindead error rate 10% → 40% |

## Remaining Known Issues

1. **Time overruns still occur**: D1 can take 2-3x longer than allocated. Root cause: SearchRoot doesn't check time frequently enough during search. Fix requires adding time checks inside the minimax search itself.

2. **Diagnostic Th: value shows configured threads, not actual**: The ThreadCount in stats comes from difficulty config, not from actual search. This is cosmetic but could be improved.

## Next Steps

1. Consider adding fine-grained time checks inside SearchRoot/Minimax
2. Update README.md to reflect new Braindead error rate
3. Run full matchup test suite to verify all difficulty orderings

## Version

- Target: v1.60.0
- Previous: v1.59.0
