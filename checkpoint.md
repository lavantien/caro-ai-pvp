# Checkpoint - 2026-02-07

## Current Goal
Release v1.31.0 with memory leak fixes, performance optimizations, and test reorganization.

## Recent Changes

### Performance Optimizations (Phase 1-3 from plan)
1. **OpeningBookGenerator: MinimaxAI Object Pooling**
   - Added `_aiPool` (ConcurrentBag<MinimaxAI>) field
   - Added `RentAI()` method - gets from pool or creates new 64MB TT instance
   - Added `ReturnAI(MinimaxAI ai)` - clears state and returns to pool
   - Fixed nested parallelism - removed Task.WhenAll for candidate evaluation
   - Now processes candidates sequentially per position (outer loop provides parallelism)

2. **LockFreeTranspositionTable: Zero-Allocation Struct**
   - Converted `TranspositionEntry` from class to 16-byte struct
   - [StructLayout(LayoutKind.Explicit, Size = 16)]
   - Hash verification in Lookup() validates integrity post-read
   - Eliminated heap allocations in hot path

3. **SQLite Batch Operations**
   - Added `StoreEntriesBatch(IEnumerable<OpeningBookEntry>)` to IOpeningBookStore
   - SqliteOpeningBookStore: transactions with 500-1000 entries
   - InMemoryOpeningBookStore: implemented for consistency

### Test Reorganization
1. **Created Caro.Core.IntegrationTests project**
   - Marked `<IsTestProject>false</IsTestProject>` to exclude from default runs
   - Tests only run with explicit project targeting

2. **Moved 13 slow integration test files:**
   - DefensivePlayTests.cs
   - MasterDifficultyTests.cs
   - QuickGrandmasterVsEasy.cs
   - TranspositionTablePerformanceTests.cs
   - NodeCountingTests.cs
   - DiagonalThreatTest.cs
   - ThreatDetectorDebugTest.cs
   - ZeroAllocationTests.cs
   - AspirationWindowTests.cs
   - HistoryHeuristicTests.cs
   - LateMoveReductionTests.cs
   - PrincipalVariationSearchTests.cs
   - QuiescenceSearchTests.cs

3. **Updated namespaces:** `Caro.Core.Tests.GameLogic` -> `Caro.Core.IntegrationTests.GameLogic`

## Next Step
- Commit changes atomically
- Push to GitHub
- Create v1.31.0 release

## Open Questions
- Unit tests still running slowly after moving integration tests - needs investigation in next checkpoint

## Files to Commit
- Modified: OpeningBookGenerator.cs, LockFreeTranspositionTable.cs, SqliteOpeningBookStore.cs
- Modified: IOpeningBookStore.cs, InMemoryOpeningBookStore.cs
- Modified: CHANGELOG.md, README.md
- Modified: MatchupTests (namespace updates)
- Deleted: 13 test files from Caro.Core.Tests
- Added: Caro.Core.IntegrationTests project with 13 moved test files
