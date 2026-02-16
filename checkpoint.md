# Checkpoint: v1.60.0 Development

## Summary

Investigating AI strength inversions between difficulty levels. Primary issue: Easy loses 100% to Braindead at blitz time controls (3+2).

## Current Status: ROOT CAUSE IDENTIFIED

### Root Cause: Lazy SMP Depth Offset at Blitz Time Controls

The Lazy SMP depth offset (`threadData.ThreadIndex % 2`) causes helper threads to start at depth 2 at blitz time controls. With limited time (~200-800ms per move), these threads often don't complete even one iteration at depth 2, returning bad results.

### Test Results Summary

| Configuration | Easy Win Rate | Notes |
|---------------|---------------|-------|
| Parallel + depth offset | 0% (0/4) | All threads start at different depths |
| Parallel + no depth offset | 25% (1/4) | All threads start at depth 1 |
| Single-threaded | 50% (2/4) | Best performance at blitz |

### Key Insight

At blitz time controls (3+2), Easy gets ~200-800ms per move. With the depth offset:
- Thread 0 (master): starts at depth 1, completes D1, maybe D2
- Thread 1 (helper): starts at depth 2, doesn't complete, returns bad result
- Thread 2 (helper): starts at depth 1, completes D1

Helper threads starting at depth 2 waste time on iterations they can't complete, and return bad results that pollute the result selection.

### Fixes Applied

1. **Lazy SMP depth offset**: Implemented per Chessprogramming Wiki
2. **Node count validation**: Only count iterations where nodes were searched
3. **Depth offset disabled for blitz**: All threads start at depth 1

### Remaining Gap

Parallel search (25% win rate) is still worse than single-threaded (50% win rate). Possible causes:
1. Thread synchronization overhead
2. Result selection still selecting some bad results
3. Time overhead from Task.WaitAll

## Files Modified

| File | Change |
|------|--------|
| `ParallelMinimaxSearch.cs` | Lazy SMP depth offset (now disabled for blitz), node count validation |
| `AIDifficultyConfig.cs` | No permanent changes |

## Recommendation

For v1.60.0, consider:
1. Keep parallel search for Medium+ difficulties (more time per move)
2. Use single-threaded search for Easy (blitz time controls)
3. Or improve parallel search efficiency for short time controls

## Next Steps

1. Consider conditional parallel: use single-threaded for short time allocations
2. Investigate why parallel is still 25% worse than single-threaded
3. Profile thread synchronization overhead

## Test Status

Backend tests passing. Matchup tests showing improvement from 0% to 25% win rate.
