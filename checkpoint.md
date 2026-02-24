# Checkpoint: v1.65.0 Development

## Summary

Improved Grandmaster vs Braindead win rate from 40% to 80% through counter-attack optimization.

## Progress

### Win Rate Progression (Grandmaster vs Braindead, 3+2 Blitz)

| Version | Win Rate | Notes |
|---------|----------|-------|
| v1.62.0 | 95% | Baseline with old blocking |
| v1.64.0 | 40% | Regression from time formula change |
| Current | 80% | Counter-attack improvements |

### Changes Made

1. **Reverted three-threat counter-attack scoring**
   - Adding `+ ourThreeThreats * 1000` made win rate worse (60%)
   - Reverted to only `+ ourFourThreats * 8000`

2. **Added desperate counter-attack logic**
   - When best blocking score is < -5000 (losing position)
   - Search all squares for a move creating verified winning four-threat
   - Take counter-attack instead of futile block

### Files Modified

| File | Change |
|------|--------|
| `MinimaxAI.cs` | Added counter-attack when blocking is futile |
| `MinimaxAI.cs` | Reverted three-threat scoring addition |

### Test Results

**Tournament 1 (with three-threat scoring):**
- 12 wins, 8 losses (60% win rate) - WORSE

**Tournament 2 (reverted, with desperate counter-attack):**
- 16 wins, 4 losses (80% win rate) - BETTER

## Remaining Work

- Target: 100% win rate (currently at 80%)
- Losses occur when Braindead creates multiple threats early
- Grandmaster needs more proactive defense

## Version

- Target: v1.65.0
- Previous: v1.64.0
