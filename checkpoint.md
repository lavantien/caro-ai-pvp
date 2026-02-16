# Checkpoint: Development Loop Progress

## Current Goal

Fix strength inversion where Braindead beats Easy ~100% of the time.
Target: Easy should beat Braindead ~95% of the time.

## Summary of Findings

### Depth Analysis
Both bots reach similar depths:
- **Braindead** (5% time, 10% error): D1-D2, 1-300 nodes
- **Easy** (20% time, 0% error): D1-D2, 1-5K nodes

Easy searches 10x more nodes but reaches similar depths because:
- Both use same search algorithm
- At shallow depths, parallelism doesn't help much
- The evaluation function is the same for both

### Root Cause of Strength Inversion

At D1-D2 depth, the evaluation function doesn't differentiate well between good and bad positions:
- D1 sees only your own moves
- D2 sees one exchange
- Neither is enough to see winning tactical combinations

Braindead's 10% random moves sometimes help by being unpredictable, while Easy's "optimal" play at shallow depth is predictable.

### What We Tried
1. ✅ Added immediate win check (AI takes winning move instantly)
2. ❌ Added full-board immediate win block (too expensive O(n²))
3. ❌ Increased Easy's time budget (still reaches D1-D2)
4. ❌ Removed expensive full-board scan

### What Could Help
1. **Smarter evaluation at shallow depths**: Add threat detection that works at D1-D2
2. **Tactical pruning**: Prioritize moves that create/neutralize threats
3. **Better move ordering**: Search forcing moves first to see deeper tactically
4. **Reduce Braindead's strength**: Lower its search depth or increase error rate

## Current Results

```
=== Braindead vs Easy ===
Game 1: Braindead wins on move 31
Game 2: Braindead wins on move 36

=== Easy vs Braindead ===
Game 3: Braindead wins on move 40
Game 4: Braindead wins on move 17

Total: Braindead beats Easy 4-0
```

## Commits Made

1. `35515ee` - Add immediate win/loss detection
2. `14dab39` - Update test expectations  
3. `1a1176a` - Remove expensive full-board scan

## Files Modified

- `backend/src/Caro.Core/GameLogic/MinimaxAI.cs` - Immediate win detection
- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs` - Settings (reverted to README values)
- `backend/tests/Caro.Core.Tests/GameLogic/AdaptiveDepthCalculatorTests.cs` - Test expectations

## Next Steps

1. Investigate evaluation function for shallow depth tactical awareness
2. Consider if Braindead's 10% error rate should be higher to make Easy win
3. Look at move ordering to prioritize tactical moves at shallow depth
4. Consider if this is a fundamental limitation of depth-limited search

## Open Questions

- Is the README's expected win rate achievable with D1-D2 depth?
- Should the evaluation function be tuned differently for different depths?
- Is Braindead's unpredictability actually a strength at shallow depths?
