# Checkpoint: v1.60.0 Development

## Summary

Fixed critical bug in SearchRoot where valid scores were discarded due to cancellation check placement. After fix, Easy beats Braindead 63.6% at blitz time controls (vs 0% before).

## Root Cause: SearchRoot Cancellation Check Order

The critical bug was in `SearchRoot` - when Minimax completed successfully and returned a valid score, the cancellation check was placed BEFORE updating `bestScore`/`bestMove`:

```csharp
// BUGGY CODE:
var score = Minimax(...);

// Check cancellation FIRST - WRONG!
if (cancellationToken.IsCancellationRequested)
    break;

// This is never reached if cancelled!
if (score > bestScore)
{
    bestScore = score;
    bestMove = (x, y);
}
```

When the search timed out:
1. Minimax returns valid score (e.g., -2147482648 = alpha value)
2. Cancellation check triggers
3. Loop breaks WITHOUT updating bestScore/bestMove
4. bestScore stays at int.MinValue (initial value)
5. SearchRoot returns garbage result

**The Fix:** Update bestScore/bestMove BEFORE checking cancellation, but only if the score is valid (not int.MinValue from cancelled Minimax):

```csharp
var score = Minimax(...);

// FIX: Update BEFORE checking cancellation
if (score != int.MinValue && score > bestScore)
{
    bestScore = score;
    bestMove = (x, y);
}

// NOW check cancellation
if (cancellationToken.IsCancellationRequested)
    break;
```

## All Bugs Fixed

### Bug 1: SearchRoot Cancellation Order (CRITICAL)

In `SearchRoot`, the cancellation check was BEFORE updating bestScore/bestMove, causing valid scores to be discarded.

**Fix:** Update bestScore/bestMove before checking cancellation.

### Bug 2: SearchWithIterationTimeAware Cancellation Order

Same issue in `SearchWithIterationTimeAware` - bestMove was updated unconditionally even when score was int.MinValue.

**Fix:** Only update bestMove/bestDepth when score is not int.MinValue.

### Bug 3: Result Selection Logic

The result selection used `OrderBy(r => (-r.score, ...))` which causes integer overflow when negating int.MinValue.

**Fix:** Use `OrderByDescending(r => r.score)` instead.

## Test Results

### Before Fix
- Easy vs Braindead: 0-11 (0% win rate)
- Games: 20-27 moves (quick losses)

### After Fix
- Easy vs Braindead: 7-4 (63.6% win rate)
- Games: 37.2 moves average
- Easy correctly finds winning positions (score=2147483647)

## Files Modified

| File | Change |
|------|--------|
| `ParallelMinimaxSearch.cs` | Fix SearchRoot cancellation order, fix SearchWithIterationTimeAware score validation, fix result selection logic |

## Version

- Target: v1.60.0
- Previous: v1.59.0
