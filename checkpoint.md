# Checkpoint: v1.59.0 Release

## Summary

This checkpoint concludes the development loop for AI strength improvements. While Easy still loses to Braindead due to shallow depth limitations (D1-D2), several critical bugs were fixed and important features were added.

## Changes Made

### Bug Fixes
1. **Parallel search fallback** - Fixed broken fallback when parallel results empty
2. **Parallel search time management** - Fixed 2x time usage from timeout handling
3. **Search depth for Easy** - Improved depth calculation for D2 minimum

### Features Added
1. **Immediate win detection** - AI takes winning moves instantly
2. **Immediate win blocking** - AI blocks opponent's single winning threats

### Documentation Updates
- UCI version updated to 1.59.0
- Test counts updated to 575
- CHANGELOG.md prepared for v1.59.0

## Known Limitations

**Easy vs Braindead Strength Inversion**: Easy still loses ~100% to Braindead because:
- Both reach D1-D2 depth
- At shallow depths, evaluation cannot prevent open fours
- Open four (4-in-a-row with both ends open) is unblockable

**Potential Future Fixes**:
1. Increase Braindead's error rate from 10% to 20-30%
2. Improve evaluation to prevent open fours at shallow depths
3. Add tactical move ordering for better pruning

## Files Modified

- `backend/src/Caro.Core/GameLogic/MinimaxAI.cs`
- `backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs`
- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs`
- `backend/src/Caro.UCI/UCIProtocol.cs`
- `backend/tests/Caro.Core.Tests/GameLogic/AdaptiveDepthCalculatorTests.cs`
- `README.md`
- `CHANGELOG.md`

## Test Status

All 575 backend tests passing.

## Commits in This Release

1. `44a16f4` - fix: parallel search fallback and diagnostics collection
2. `cf012f8` - fix: parallel search time management
3. `35515ee` - fix: improve search depth for Easy difficulty
4. `14dab39` - test: update Easy difficulty test expectations
5. `1a1176a` - fix: remove expensive full-board immediate win scan
6. `4017b45` - fix: add immediate win block check for opponent threats
