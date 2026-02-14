# Checkpoint - 2026-02-14

## Current Goal
Release v1.49.0 with TT memoization optimization for book generator.

## Recent Changes

### v1.49.0 Release

1. **TT Memoization for Book Generator (Performance)**
   - Added `MinimaxAI.ClearSearchState()` method
   - Clears history and killer moves but preserves TT
   - Book generator uses `ClearSearchState()` instead of `ClearAllState()`
   - TT entries preserved across candidates/positions for subtree reuse
   - Expected 2-5x speedup at deep depths

2. **Version Updates**
   - UCIProtocol.cs: 1.48.0 -> 1.49.0
   - ENGINE_FEATURES.md: Version bump to 1.49.0
   - CHANGELOG.md: Added v1.49.0 entry

## Files Modified
- `backend/src/Caro.Core/GameLogic/MinimaxAI.cs` - Added ClearSearchState()
- `backend/src/Caro.Core/GameLogic/OpeningBook/OpeningBookGenerator.cs` - Use ClearSearchState()
- `backend/src/Caro.UCI/UCIProtocol.cs` - Version bump
- `ENGINE_FEATURES.md` - Version bump
- `CHANGELOG.md` - Added v1.49.0 entry

## Test Status
- Backend: 579/579 tests passing (515 Core + 64 Infrastructure)

## Next Step
- Commit changes
- Push to GitHub
- Create v1.49.0 release
- User to verify performance improvement during book generation
