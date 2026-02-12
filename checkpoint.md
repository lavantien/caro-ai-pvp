# Checkpoint - 2026-02-13

## Current Goal
Release v1.47.0 with UCI notation, opening book access, and time control changes.

## Recent Changes

### v1.47.0 Release

1. **UCI Notation System Overhaul**
   - Changed from base-26 (a-z, aa-af) to grid-based double-letter format (aa-hd)
   - Encoding: `column = firstLetterIndex * 4 + secondLetterIndex`
   - Updated backend `UCIMoveNotation.cs` and frontend `uciEngine.ts`

2. **Opening Book Access Extended**
   - Easy: 8 plies (4 moves per side)
   - Medium: 16 plies (8 moves per side)
   - Previously only Hard+ had book access

3. **Time Control Change**
   - ComprehensiveMatchupRunner: 7+5 → 3+2
   - Initial time: 420s → 180s
   - Increment: 5s → 2s

4. **Documentation Updates**
   - README.md: Updated UCI notation, difficulty table, time control
   - ENGINE_FEATURES.md: Version bump to 1.47.0
   - CHANGELOG.md: Added v1.47.0 entry

## Files Modified
- `backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs`
- `backend/src/Caro.Core/GameLogic/UCI/UCIMoveNotation.cs`
- `backend/src/Caro.TournamentRunner/ComprehensiveMatchupRunner.cs`
- `backend/src/Caro.UCI/UCIProtocol.cs`
- `frontend/src/lib/uciEngine.ts`
- `README.md`
- `ENGINE_FEATURES.md`
- `CHANGELOG.md`

## Test Status
- Backend: 579/579 tests passing (515 Core + 64 Infrastructure)
- Frontend: 19/19 tests passing

## Next Step
- Push to GitHub
- Create v1.47.0 release
