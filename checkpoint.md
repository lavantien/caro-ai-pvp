# Checkpoint: v1.60.0 Development

## Summary

Major refactoring of difficulty configuration to remove all artificial depth/speed handicaps. All depth is now determined purely by machine capability and time budget. Added pondering support for Easy difficulty.

## Baseline Test Results (Blitz 3+2)

| Matchup | Win Rate | Games | Avg Moves | Avg Time |
|---------|----------|-------|-----------|----------|
| Easy vs Braindead | 66% | 100 | 40.6 | 34.0s |
| Medium vs Braindead | 58% | 100 | 42.1 | 91.0s |

Lower win rates at blitz time control are expected - both sides reach only D1-D2 depth where evaluation cannot reliably distinguish positions. Separation increases at longer time controls (Rapid 7+5, Classical 15+10).

## Configuration Changes

### PonderingThreadCount = ThreadCount

All difficulties now use the same thread count for pondering as main search. Previous hardcoded values (1-3) wasted compute resources during opponent's turn.

| Difficulty | Before | After |
|------------|--------|-------|
| Easy | 0 (disabled) | = ThreadCount |
| Medium | 2 | = ThreadCount |
| Hard | 3 | = ThreadCount |
| Grandmaster | ThreadCount/2 | = ThreadCount |

### Easy Now Has Pondering

Since Easy uses multiple threads (max(2, N/5-1)), it now has pondering enabled for consistency.

### Removed MinDepth

Depth is no longer artificially capped per difficulty. The search naturally reaches whatever depth it can within the time budget based on machine NPS.

| Difficulty | Before | After |
|------------|--------|-------|
| Braindead | 1 | Removed |
| Easy | 2 | Removed |
| Medium | 3 | Removed |
| Hard | 4 | Removed |
| Grandmaster | 5 | Removed |
| Experimental | 5 | Removed |
| BookGeneration | 12 | Removed |

### Removed TargetNps

NPS is no longer calibrated from hardcoded targets. Instead, it's learned from actual search performance using exponential moving average.

| Difficulty | Before | After |
|------------|--------|-------|
| Braindead | 10K | Removed |
| Easy | 50K | Removed |
| Medium | 100K | Removed |
| Hard | 200K | Removed |
| Grandmaster | 500K | Removed |
| Experimental | 500K | Removed |
| BookGeneration | 1M | Removed |

## Design Principle

All depth/speed is determined by machine capability and time allotted:

1. **Thread count** - More threads = faster search = deeper results
2. **Time budget** - Higher difficulties get more time (5% to 100%)
3. **Feature flags** - VCF, opening book depth vary by difficulty
4. **Error rate** - Only Braindead has intentional errors (10%)

No hardcoded minimum depths or NPS targets.

## Files Modified

| File | Change |
|------|--------|
| `AIDifficultyConfig.cs` | Removed MinDepth, TargetNps; PonderingThreadCount = ThreadCount; Easy pondering enabled |
| `AdaptiveDepthCalculator.cs` | Removed GetMinimumDepth() |
| `TimeBudgetDepthManager.cs` | Removed GetMinimumDepth(), CalibrateNpsForDifficulty(); Added CalibrateFromSearch() |
| `MinimaxAI.cs` | Removed CalibrateNpsForDifficulty calls |
| `ParallelMinimaxSearch.cs` | Removed CalibrateNpsForDifficulty calls |
| `AdaptiveDepthCalculatorTests.cs` | Removed GetMinimumDepth test |
| `TimeBudgetDepthManagerTests.cs` | Removed CalibrateNpsForDifficulty tests; Added CalibrateFromSearch test |
| `README.md` | Updated difficulty table (Easy has pondering), added Medium vs Braindead baseline |

## Version

- Target: v1.60.0
- Previous: v1.59.0
