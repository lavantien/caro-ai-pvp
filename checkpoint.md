# Checkpoint: v1.64.0 Development

## Summary

Bug fixes for time management and threat detection, plus new matchup data with updated time formula.

## Changes

### Time Management Fix

Changed time allotment formula from `3x increment` to `(initial_time / 20) + (increment * 2)`:

| Time Control | Old Max | New Max | Formula |
|--------------|---------|---------|---------|
| 180+2 | 6s | 13s | 180/20 + 2*2 = 9 + 4 |
| 300+3 | 9s | 21s | 300/20 + 3*2 = 15 + 6 |
| 420+5 | 15s | 31s | 420/20 + 5*2 = 21 + 10 |

This allows more time per move in longer games while preventing clock burn.

### Threat Detection Fix

Fixed crash when board is nearly full:
- `ThreatDetector.IsWinningMove()` now checks if cell is empty before placing stone
- Prevents `InvalidOperationException: Cell is already occupied` on full boards

### Files Modified

| File | Change |
|------|--------|
| `AdaptiveTimeManager.cs` | Updated time allotment formula |
| `ThreatDetector.cs` | Added empty cell check in IsWinningMove |

### Matchup Results (180+2, 20 games)

Braindead vs Grandmaster with new time formula:
- **Braindead: 12 wins (60%)**
- **Grandmaster: 8 wins (40%)**
- Average moves: 50.1
- Average time: 170.4s/game

Note: This is significantly worse than the v1.62.0 result (95% Grandmaster win rate). The new time formula may require further investigation.

## Version

- Target: v1.64.0
- Previous: v1.63.0
