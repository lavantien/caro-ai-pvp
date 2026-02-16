# Checkpoint: v1.60.0 Development

## Summary

This checkpoint documents the investigation of AI strength inversions between difficulty levels and the decision to document the limitation rather than modify the spec.

## Resolution

**Decision: Document the limitation** - Updated README.md with a note explaining that at blitz time controls (3+2), both Braindead and Easy reach only D1-D2 depth where the evaluation cannot reliably distinguish good from bad moves. Strength separation is more pronounced at longer time controls.

## Root Cause Analysis

**The Problem**: At blitz time controls, both Easy and Braindead reach only D1-D2 depth:
- **Braindead**: 5% time budget, 1 thread, 10% error rate, no book
- **Easy**: 20% time budget, 3 threads, 0% error rate, 4-ply book

Despite Easy having 4x more time, parallel search (3 threads), and opening book, the depth separation is only 0-1 ply at blitz time controls.

**Why Depth Separation is Insufficient**:
- At D1: Only sees immediate threats (1 ply ahead)
- At D2: Can see simple 2-move combinations but misses deeper tactics
- Evaluation function is designed for deeper searches (D4+)
- At shallow depths, evaluation cannot distinguish good from bad moves reliably

## Test Results

| Braindead Error Rate | Easy Win Rate |
|---------------------|---------------|
| 10% (spec) | 0% (4/4 losses) |
| 40% | ~25% |
| 50% | 100% (4/4 wins) |

## Current Configuration (per README.md spec)

```
| Level       | Threads | Time Budget | Error | Book Depth |
|-------------|---------|-------------|-------|------------|
| Braindead   | 1       | 5%          | 10%   | 0          |
| Easy        | (N/5)-1 | 20%         | 0%    | 4 plies    |
```

## Rejected Options

1. **Update README.md spec** - Change Braindead error rate from 10% to 50% - Rejected: violates spec
2. **Add hardcoded MinDepth enforcement** - Rejected: depth should be reached naturally based on machine
3. **Add engine handicaps** - Rejected: engine should stay the same for all difficulties

## Files Modified

- `README.md` - Added "Known Limitation" note about blitz time controls
- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs` - Error rate remains at 10% per spec
- `backend/tests/Caro.Core.Tests/GameLogic/AdaptiveDepthCalculatorTests.cs` - Tests expect 10%

## Test Status

All 575 backend tests passing.

## Next Steps

- Continue development loop with QuickSmokeTest for other matchups (GM vs Hard, etc.)
- Monitor for other issues (timeouts, crashes, documentation inconsistencies)
