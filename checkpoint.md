# Checkpoint - 2026-02-13

## Current Goal
Release v1.48.0 with opening book hash collision fix and enhanced progress reporting.

## Recent Changes

### v1.48.0 Release

1. **Opening Book Hash Collision Fix (Critical)**
   - Root cause: Different boards can share the same canonical hash
   - Fix: Added `DirectHash` field to `OpeningBookEntry` for unique identification
   - Storage now uses compound key `(CanonicalHash, DirectHash, Player)`
   - SQLite schema updated with new primary key
   - Automatic migration drops old unreliable entries

2. **Book Builder Progress Enhancement**
   - Progress interval: 60s → 15s
   - Added throughput metrics (positions/minute, nodes/sec)
   - Added candidate statistics (evaluated, pruned, early exits)
   - Added write buffer utilization tracking

3. **Documentation Updates**
   - README.md: Updated book builder performance metrics (80-100 pos/min)
   - ENGINE_FEATURES.md: Version bump to 1.48.0
   - CHANGELOG.md: Added v1.48.0 entry

## Files Modified
- `backend/src/Caro.Core.Domain/Entities/OpeningBookEntry.cs`
- `backend/src/Caro.Core/GameLogic/OpeningBook/IOpeningBookStore.cs`
- `backend/src/Caro.Core/GameLogic/OpeningBook/OpeningBookGenerator.cs`
- `backend/src/Caro.Core.Infrastructure/Persistence/SqliteOpeningBookStore.cs`
- `backend/src/Caro.BookBuilder/Program.cs`
- `backend/src/Caro.UCI/UCIProtocol.cs`
- `backend/tests/Caro.Core.Tests/Helpers/MockOpeningBookStore.cs`
- `backend/tests/Caro.Core.Tests/Helpers/OpeningBookEntryBuilder.cs`
- `backend/tests/Caro.Core.Tests/GameLogic/OpeningBook/OpeningBookSymmetryTests.cs`
- `backend/tests/Caro.Core.Infrastructure.Tests/Persistence/SqliteOpeningBookStoreTests.cs`
- `README.md`
- `ENGINE_FEATURES.md`
- `CHANGELOG.md`

## Test Status
- Backend: 579/579 tests passing (515 Core + 64 Infrastructure)
- Book builder: 10-minute verification run with zero errors

## Next Step
- Commit changes atomically
- Push to GitHub
- Create v1.48.0 release
