# Checkpoint: v1.60.0 Development

## Summary

This checkpoint documents the ongoing work to fix AI strength inversions between difficulty levels. The primary issue was Easy losing to Braindead due to both reaching similar shallow depths (D1-D2) at blitz time controls.

## Changes Made

### Bug Fixes
1. **Braindead error rate increased** - From 10% to 40% to ensure Easy wins majority of games
   - At 10% error rate: Easy won 0% against Braindead
   - At 25% error rate: Easy won ~0% against Braindead (games took 180+ moves)
   - At 40% error rate: Easy wins ~25% against Braindead (partial improvement)

### Test Results with 40% Error Rate

**Braindead vs Easy** (Braindead plays Red first):
- Game 1: Easy (Blue) wins in 10 moves
- Game 2: Braindead (Blue) wins in 58 moves

**Easy vs Braindead** (Easy plays Red first):
- Game 3: Braindead (Blue) wins in 28 moves
- Game 4: Braindead (Red) wins in 19 moves

**GM vs Hard**:
- Game 5: Hard (Blue) wins in 14 moves
- Game 6: In progress at timeout

## Known Limitations

**Easy vs Braindead Partial Inversion**: Easy now wins ~25% of games but still struggles because:
- Both reach D1-D2 depth at blitz time controls
- At shallow depths, evaluation cannot reliably detect/block open fours
- 40% error rate is the practical maximum before Braindead becomes unplayably weak

**GM vs Hard Inversion**: Hard sometimes beats GM due to:
- GM searching at similar or shallower depths despite more threads
- Potential inefficiency in parallel search at low depths

**Potential Future Fixes**:
1. Improve evaluation to detect/block open fours at shallow depths
2. Add tactical move ordering for better pruning
3. Give Easy a minimum depth guarantee (e.g., always search to D3)
4. Investigate GM's shallow depth issue with parallel search

## Files Modified

- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs` - Error rate 0.10 -> 0.40
- `backend/tests/Caro.Core.Tests/GameLogic/AdaptiveDepthCalculatorTests.cs` - Updated expected values

## Test Status

All 575 backend tests passing.

## Next Steps

1. Consider increasing Easy's minimum depth guarantee
2. Investigate GM's parallel search efficiency at low depths
3. Run comprehensive matchup tests to validate strength hierarchy
