# Checkpoint: Development Loop Progress

## Current Goal

Fix strength inversion where Braindead (easiest) beats Easy (second easiest) ~100% of the time.
Target: Easy should beat Braindead ~95% of the time per documentation.

## Recent Changes (Commit 35515ee)

1. **Immediate Win Detection**: Added check for AI's own winning moves before search
2. **Immediate Win Block**: Added full-board scan for opponent's immediate winning threats  
3. **Easy Difficulty Boost**: Increased time multiplier from 20% to 35%, MinDepth to 3

## Current Results

- Game 1: Braindead won in 31 moves (37.3s)
- Game 2: Braindead won in 30 moves (33.8s)
- Games are longer than before (was 15-25 moves), but Easy still loses

## Root Cause Analysis

The immediate win block IS working (Easy blocks threats), but when Braindead has an **open four**
(4 in a row with both ends open), Easy can only block ONE square and Braindead wins on the other.

This is a fundamental game mechanic - you can only make one move per turn. If opponent creates
an unstoppable threat (open four), you lose regardless of AI strength.

## Key Question

Why is Braindead (10% error rate, D1-D2) able to create open fours while Easy (0% error, D1-D2) can't?

Possible explanations:
1. Braindead's random moves accidentally avoid predictable patterns
2. Easy's "optimal" play at D1-D2 is actually defensive, not aggressive enough
3. The evaluation function doesn't reward offensive threat creation enough
4. Easy's opening book leads to positions where Braindead gets initiative

## Next Steps to Try

1. **Increase aggressiveness**: Make Easy prioritize creating threats over blocking
2. **Improve threat detection**: Add "pre-threat" detection (3 in a row that can become open four)
3. **Counter-attack**: When facing open four, check if Easy can create own winning threat
4. **Deeper search**: Increase Easy's MinDepth to 4 to see more tactics
5. **Evaluation tuning**: Increase offensive threat scores relative to defensive scores

## Files Modified

- `backend/src/Caro.Core/GameLogic/MinimaxAI.cs` - Immediate win/loss detection
- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs` - Easy difficulty settings

## Open Questions

- Is Braindead's 10% error rate actually helping by creating unpredictability?
- Should Easy's opening book be disabled to avoid predictable patterns?
- Is the evaluation function too defensive (DefenseMultiplier = 1.5x)?
