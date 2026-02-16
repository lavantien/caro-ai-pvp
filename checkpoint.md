# Checkpoint: v1.60.0 Development

## Summary

Investigating AI strength inversions between difficulty levels at blitz time controls (3+2).

## Changes Made

### 1. Time Bound Enforcement (ParallelMinimaxSearch.cs)

**Problem**: D1-D2 iterations could take 2-3x longer than time budget because time checks were skipped for shallow depths.

**Solution**: Always check hard bound, even at D1-D2. Added pre-iteration time estimate check for D3+ only:

```csharp
// Only apply pre-iteration check for D3+ to ensure D1 and D2 are always attempted
if (currentDepth > 2 && lastIterationElapsedMs > 0 && remainingTimeMs < lastIterationElapsedMs * 2)
{
    break;
}
```

### 2. Braindead Error Rate (AIDifficultyConfig.cs)

**No change** - Kept at 10% per README.md specification.

## Test Results

After fixes:
- Easy now reaches D2 in some positions (when time allows)
- Easy vs Braindead matchup is more balanced (~50% win rate)
- Previous: Easy lost 100% to Braindead
- After: Easy wins approximately 50% of games

## Files Modified

| File | Change |
|------|--------|
| `ParallelMinimaxSearch.cs` | Hard bound check for all depths, pre-iteration time estimate for D3+ |

## Root Cause Analysis

The original issue was that the pre-iteration time check was applied at all depths, including D1. This caused:
1. If remaining time < lastIterationTime * 2, skip next depth
2. At blitz with ~900ms allocation, D1 takes ~300-800ms
3. D1 * 2 = 600-1600ms, but remaining time might be 100-600ms
4. D2 was skipped even when there was enough time to attempt it

**Fix**: Only apply the pre-iteration check for D3+. This ensures:
- D1 is always attempted
- D2 is always attempted
- D3+ uses time estimate to avoid starting iterations that won't complete

## Known Limitation (Per README.md)

> At blitz time controls (3+2), both Braindead and Easy reach only D1-D2 depth where the evaluation cannot reliably distinguish good from bad moves. Strength separation between these levels is more pronounced at longer time controls (Rapid 7+5, Classical 15+10) where depth separation increases.

## Version

- Target: v1.60.0
- Previous: v1.59.0
