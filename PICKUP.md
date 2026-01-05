# PICKUP.md - Session Continuation Guide

## Context Summary

This session focused on fixing time management and AI strength inversion issues in the Caro AI PvP tournament system.

## Key Issues Addressed

### 1. Clock Bug (Frontend) - FIXED
- Clock turn logic was inverted
- Clock was skipping numbers (not real-time)
- Fixed in `frontend/src/lib/stores/tournamentStore.svelte.ts`

### 2. Time Control Updates - DONE
- Changed from 3+2 (Blitz) to 7+5 (Rapid) time control
- Updated `QuickTest.cs` to use 420s initial + 5s increment
- `backend/src/Caro.TournamentRunner/QuickTest.cs` lines 35-36

### 3. Time Management - PARTIALLY DONE
- **Problem**: D10-D11 target depths are NOT reachable in 7+5 time control
- **Root Cause**: Depth 9 takes 60-90 seconds; Depth 10+ would take 120-180s
- With 420s total, only 3-4 moves possible before timeout

**Changes Made:**
- `CalculateDepthForTime` now uses percentage-based thresholds (not fixed seconds)
- `TimeManager.cs` has difficulty-based time multipliers:
  - Legend (D11): 3.5x (~20s per move in opening)
  - Grandmaster (D10): 2.5x (~14s per move in opening)
  - Master (D9): 1.8x time allocation
  - Expert (D8): 1.3x time allocation

**Current Behavior:**
- D11 shows `adjustedDepth=11` at start (420s)
- Actual search reaches depth 9 with ~10M nodes
- Depth 9 is the practical maximum for 7+5 time control

### 4. Parallel Search - DISABLED
- Lazy SMP had architectural issues
- Sequential search is now used for all difficulties

## Outstanding Issues

### AI Strength Inversion - NOT RESOLVED
From previous test outputs:
- Legend (D11) vs Harder (D6): D11 lost 0-3 (should win)
- Grandmaster (D10) vs Expert (D8): D10 lost 0-3 (should win)

**Root Cause (from plan file):**
Symmetric scoring in `SIMDBitBoardEvaluator.cs` and `BitBoardEvaluator.cs`:
```csharp
score += Evaluate(playerBoard, ...);      // My threats: positive
score -= Evaluate(opponentBoard, ...);     // Opponent threats: negative, same magnitude
```

This causes AI to value offense and defense equally. In Caro, **Defense > Offense**.

**Required Fix:** Asymmetric Scoring with Defense Multiplier (2.2x)
- Files: `SIMDBitBoardEvaluator.cs:56-68`, `BitBoardEvaluator.cs:50-70`
- See plan: `~/.claude/plans/nested-kindling-willow.md`

## Files Modified This Session

1. `backend/src/Caro.Core/GameLogic/TimeManagement/TimeManager.cs`
   - Added difficulty-based time multipliers
   - Updated CalculateMoveTime signature to accept difficulty parameter

2. `backend/src/Caro.Core/GameLogic/MinimaxAI.cs`
   - Updated CalculateDepthForTime to use percentage-based thresholds
   - Added debug output for high difficulties
   - Updated CalculateMoveTime call to pass difficulty

3. `backend/src/Caro.TournamentRunner/QuickTest.cs`
   - Updated time control to 7+5 (420s + 5s increment)
   - Added D11 vs D11, D11 vs D10 matchups

4. `frontend/src/lib/stores/tournamentStore.svelte.ts`
   - Clock turn logic and desync fixes

## Test Status

QuickTest is running with matchups:
1. D11 vs D11 - Maximum depth verification
2. D11 vs D10 - Full depth verification
3. D10 vs D8 - High difficulty
4. D11 vs D6 - Very high vs mid
5. D4 vs D6 - Sequential search

Command to run tests:
```bash
cd backend/src/Caro.TournamentRunner
dotnet run -- --test
```

## Next Session Priorities

### Option A: Accept Realistic Depths
Update difficulty labels to reflect achievable depths in 7+5:
- D11 "Legend" → Actually reaches D9
- D10 "Grandmaster" → Actually reaches D8
- Verify AI strength ordering is correct

### Option B: Fix AI Strength Inversion (Recommended)
Implement asymmetric scoring with defense multiplier:
1. Add `DefenseMultiplier = 2.2f` constant
2. Apply to opponent score evaluation
3. Run tests to verify higher difficulties beat lower ones

### Option C: Longer Time Control for High Difficulties
Implement 15+10 or 20+15 for D10-D11 matches to allow reaching full depth.

## Plan File

See `~/.claude/plans/nested-kindling-willow.md` for full implementation details on asymmetric scoring fix.

## Git Status

Many files are staged but not committed. Run:
```bash
git status
git add -A
git commit -m "feat: time management and test updates"
```
