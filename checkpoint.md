# Checkpoint: v1.60.0 Development

## Summary

Fixed critical bug in parallel search where search results were being discarded due to premature cancellation check.

## Root Cause

In `SearchWithIterationTimeAware`, the cancellation check was placed BEFORE updating `bestScore`/`bestMove` from the SearchRoot result. This meant:

1. SearchRoot completes and returns a valid result
2. Cancellation token is checked
3. If cancelled, loop breaks WITHOUT saving the result
4. `bestScore` stays at `int.MinValue`
5. All threads return `int.MinValue` score
6. Result selection picks a bad move

## Changes Made

### 1. Move Result Update Before Cancellation Check (ParallelMinimaxSearch.cs)

**Before:**
```csharp
if (cancellationToken.IsCancellationRequested)
    break;

// Update bestScore/bestMove...
```

**After:**
```csharp
// Update bestScore/bestMove BEFORE checking cancellation
if (result.score > bestScore || bestMove == (-1, -1))
    bestScore = result.score;
bestMove = (result.x, result.y);
bestDepth = currentDepth;

// NOW check cancellation - after saving the result
if (cancellationToken.IsCancellationRequested)
    break;
```

### 2. Lazy SMP Depth Offset (ParallelMinimaxSearch.cs)

Per Chessprogramming Wiki, helper threads should search at different depths:
- Master (ThreadIndex=0): Start at depth 1
- Helper odd (ThreadIndex=1,3,...): Start at depth 2
- Helper even (ThreadIndex=2,4,...): Start at depth 1

```csharp
int depthOffset = threadData.ThreadIndex % 2 == 1 ? 1 : 0;
```

### 3. Null Check for Stopwatch (ParallelMinimaxSearch.cs)

Fixed null reference warnings:
```csharp
_searchStopwatch?.Restart();
```

## Test Results

After fixes:
- Easy now consistently reaches D2 at blitz time controls
- Games last longer (30-50+ moves instead of 20-27)
- Parallel search results are now properly aggregated

## Known Limitation (Per README.md)

> At blitz time controls (3+2), both Braindead and Easy reach only D1-D2 depth where the evaluation cannot reliably distinguish good from bad moves. Strength separation between these levels is more pronounced at longer time controls (Rapid 7+5, Classical 15+10) where depth separation increases.

## Files Modified

| File | Change |
|------|--------|
| `ParallelMinimaxSearch.cs` | Move result update before cancellation check, add Lazy SMP depth offset, null check for stopwatch |

## Version

- Target: v1.60.0
- Previous: v1.59.0
