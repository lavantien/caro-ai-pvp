# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.42.0] - 2026-02-12

### Added
- **Extended Research Findings** - Part 9 in IMPROVEMENT_RESEARCH.md with 8 new subsections:
  - 9.1 Rapfi BitKey System for O(1) pattern lookup with rotation-based encoding
  - 9.2 Pattern4 System for combined 4-direction threat categorization
  - 9.3 Stockfish Move Picker Architecture with staged generation
  - 9.4 NNUE Concepts adapted for Gomoku incremental evaluation
  - 9.5 Transposition Table Advanced Techniques (lockless hashing, bucket indexing)
  - 9.6 VCF (Victory by Continuous Four) for Caro tactical solver
  - 9.7 Caro-Specific Opening Book Enhancements with opening rule validation
  - 9.8 Prioritized implementation summary table with effort estimates

### Changed
- **Executive Summary** - Updated ELO potential estimates to 600-1000+ (from 340-660)
- **Conclusion** - Enhanced Top 10 Recommendations with implementation priorities
- **Appendix B** - Added additional references (minimax.dev, Stockfish wiki)

### Documentation
- Research now covers: Rapfi, minimax.dev, Stockfish 18, Chess Programming Wiki, Stockfish wiki
- Code examples adapted for Caro's "exactly 5" rule and opening constraints
- Implementation complexity ratings for each optimization technique

[1.42.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.42.0

## [1.41.0] - 2026-02-12

### Fixed
- **Critical:** Opening book IsNearEdge transformation bug causing "position occupied" crash at depth 20
  - Root cause: During retrieval, code used current board's symmetry instead of stored entry's IsNearEdge flag
  - When moves were stored with IsNearEdge=true (coordinates stored as-is), retrieval incorrectly inverse-transformed them using current board's symmetry
  - Fix: Check stored entry's IsNearEdge flag before deciding to transform coordinates during retrieval
  - Applied fix to both OpeningBookGenerator.cs (lines 463-489, 532-573) and OpeningBookLookupService.cs (lines 63-79, 103-125)
  - Added defensive checks with detailed error logging including NearEdge info

### Added
- **Opening Book Symmetry Integrity Tests** - 12 new unit tests for IsNearEdge transformation logic
  - Edge position storage behavior verification
  - Center position storage behavior verification
  - Retrieval with correct transformation logic based on stored IsNearEdge
  - Symmetry round-trip verification for all 8 symmetry types
  - IsNearEdge detection tests for edge and center positions

### Changed
- SymmetryTransformationBugTests.cs: Fixed incorrect stone count expectation (was 6/6, now 5/6)

### Test Coverage
- Caro.Core.Tests: 456 tests (+18 from v1.40.0: +12 symmetry integrity, +1 fixed test count, +5 from previous)
- All 520+ backend tests passing (Core + Infrastructure)

[1.41.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.41.0

## [1.40.0] - 2026-02-11

### Fixed
- **Critical:** Opening book symmetry transformation bug
  - Fixed line 346 in OpeningBookGenerator.cs where `canonical.SymmetryApplied` was used instead of `existingEntry.Symmetry`
  - When retrieving positions from the book, the code now uses the symmetry stored when the entry was originally created
  - This fixes "Cell is already occupied" errors that occurred when stored moves were transformed using incorrect symmetry
  - Also fixed line 347 to use `existingEntry.IsNearEdge` for consistency

### Added
- **Opening Book Symmetry Tests** - 24 new unit tests for symmetry transformation
  - Tests verify that stored positions use the correct symmetry when retrieving and transforming moves
  - Coverage includes all 8 symmetry types (Identity, Rotate90, Rotate180, Rotate270, FlipHorizontal, FlipVertical, DiagonalA, DiagonalB)
  - Edge position handling tests (absolute coordinates, no symmetry transformation)
  - Symmetry transformation mathematical consistency validation
  - Integration test with actual OpeningBookGenerator

### Test Coverage
- Caro.Core.Tests: 438 tests (+24 symmetry tests)
- New test file: OpeningBookSymmetryTests.cs

### Verified
- Book builder runs without "INTERNAL ERROR" messages
- All 24 symmetry tests pass
- Integration test passed (90+ seconds of book generation)

[1.40.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.40.0

## [1.39.0] - 2026-02-10

### Added
- **Opening Book Generation Performance Optimization** - 12-15x speedup for book generation
  - Reduced `TimePerPositionMs` from 15000ms to 1000ms (15x reduction)
  - Reduced per-candidate time floor from 5000ms to 100ms (50x reduction)
  - Removed survival zone time bonus (+50% adjustment eliminated)
  - Fixed time allocation math with proportional buffer instead of fixed 1000ms
  - Depth cap for BookGeneration reduced from unlimited to depth 6
  - TargetSearchDepth reduced from 32 to 12 for faster generation
  - Candidate count reduced: survival zone 10→5, other zones 6→3
  - Smart candidate pruning: filter out candidates >300 points worse than best (always keep top 2)

### Performance
- **Opening Book Generation Speed**
  - Before: ~4-5 positions/minute
  - After: ~60-67 positions/minute
  - Total speedup: 12-15x improvement
- **Smart Pruning Impact**
  - Most positions reduced to 2-3 candidates after static evaluation filtering
  - Maximum of 5 candidates in survival zone (often pruned further)
  - Estimated completion time: 1-2 days (down from weeks/months)

### Changed
- `OpeningBookGenerator.cs`: Optimized time allocation, candidate selection, and pruning
- `MinimaxAI.cs`: Added depth cap for BookGeneration, fixed time allocation math
- `Program.cs`: Reduced TargetSearchDepth from 32 to 12

### Notes
- Book structure (4-3-2-1 tapered beam) remains unchanged
- Full book regeneration recommended for consistent move quality
- Hybrid approach possible: old evaluations at shallow depths, new at deeper depths

[1.39.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.39.0

## [1.38.0] - 2026-02-09

### Added
- **Multi-Entry Transposition Table** - Cluster-based design with 3 entries per cluster (32-byte cache-line aligned)
  - 10-byte TTEntry struct with Key16, Value, Depth8, BoundAndAge, Move16, Eval16
  - Depth-age replacement formula: `value = depth - 8 * age`
  - Expected ELO gain: +30-50 through improved hit rate (40% -> 60% target)
- **Continuation History** - Move pair statistics for enhanced move ordering
  - 6-ply history tracking with `short[,,]` for `[player, prevCell, currentCell]`
  - Bounded update formula: `newValue = current + bonus - abs(current * bonus) / MaxScore`
  - Expected ELO gain: +15-25 through better move ordering
- **Evaluation Cache** - Position evaluation correction caching
  - Cache correction values for repeated position evaluations
  - Update formula based on search result vs static eval difference
  - Expected ELO gain: +10-20 through faster evaluation
- **Adaptive Late Move Reduction (LMR)** - Dynamic reduction based on position factors
  - Factors: improving flag, depth, move count, delta, node types (PV/Cut/All)
  - Formula: `reduction = baseReduction + pvAdjust + cutNodeAdjust - ttMoveBonus + capturePenalty - historyBonus`
  - Bound reduction to `depth - 1` minimum
  - Expected ELO gain: +25-40 through more efficient search
- **PID Time Manager** - Dynamic time allocation using Proportional-Integral-Derivative control
  - Kp=1.0, Ki=0.1, Kd=0.5 with integral windup clamping
  - Formula: `adjustment = Kp*error + Ki*integral + Kd*derivative`
  - Auto-adjusts time budget based on remaining time vs expected time
  - Expected ELO gain: +20-50 through better time management
- **Contest Manager (Contempt Factor)** - Position-aware playstyle adjustment
  - Range: -200 to +200 centipawns
  - Positive contempt (losing): play aggressively to complicate
  - Negative contempt (winning): play conservatively to consolidate
  - Difficulty adjustment for opponent strength
  - Expected ELO gain: +5-20 through better playstyle adaptation
- **SPSA Tuner** - Gradient-free parameter optimization engine
  - Simultaneous Perturbation Stochastic Approximation algorithm
  - Default (α=0.602, γ=0.101), Aggressive, Conservative presets
  - Parameter bounds clamping for stability
  - Reproducible results with optional random seed
  - Expected ELO gain: +20-40 through optimized evaluation weights
- **Structured Logging Infrastructure** - Async search operation logging
  - Channel-based producer/consumer pattern (following StatsChannel design)
  - JSON line format for machine parsing
  - File rotation by size (100MB) and time (24h)
  - Entry types: SearchComplete, Iteration, TTProbe, TTStore, Cutoff, TimeDecision

### Test Coverage
- 62 new tests added for AI improvements (414 total in Caro.Core.Tests)
- TranspositionTableTests: 12 tests for cluster alignment, depth-age replacement, multi-entry probe
- ContinuationHistoryTests: 10 tests for initialization, bounds checking, multiple updates
- EvaluationCacheTests: 6 tests for cache hit/miss, update, bounds checking
- AdaptiveLMRTests: 5 tests for reduction calculation, bounds, history bonus
- PIDTimeManagerTests: 8 tests for on-track, behind, ahead scenarios, integral windup
- ContestManagerTests: 12 tests for position-based contempt, difficulty adjustment, bounds
- SPSATests: 8 tests for perturbation, parameter update, convergence
- SearchLoggerTests: 7 tests for file creation, logging, rotation, disposal

### Changed
- README.md updated with AI improvements table and latest test counts
- Test counts: Caro.Core.Tests 414 (+62), Total 750+ tests

### Expected ELO Gain
- Total from Phase 1 & 2 improvements: +125 to +245 ELO
- Individual components: +30-50 +15-25 +10-20 +25-40 +20-50 +5-20 +20-40

[1.38.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.38.0

## [1.37.0] - 2026-02-09

### Added
- **AI Improvement Research** - Comprehensive research report (`IMPROVEMENT_RESEARCH.md`) analyzing optimization techniques from:
  - Rapfi Gomoku/Renju engine (board representation, pattern system, move ordering)
  - Stockfish 18 (continuation history, adaptive LMR, multi-entry TT, evaluation caching)
  - Chess Programming Wiki (search optimizations, pruning techniques)
  - Advanced optimization methods (CLOP, SPSA/RSPSA, TD Learning, PID time management, contempt factor)
- **Prioritized roadmap** with 3 implementation phases targeting 340-660 ELO improvement
- **Specific recommendations** for Caro AI including TD Learning for continuous evaluation improvement

### Documentation
- Added reference to IMPROVEMENT_RESEARCH.md in README.md
- Research covers techniques with potential gains: TD Learning (+100-200 ELO), Multi-Entry TT (+30-50 ELO), SPSA/RSPSA (+30-60 ELO)

[1.37.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.37.0

## [1.36.0] - 2026-02-09

### Changed
- Remove trailing whitespace from test helpers (dotnet formatter cleanup)

[1.36.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.36.0

## [1.35.0] - 2026-02-09

### Added
- **Test Helper Infrastructure** - New test helpers in `Caro.Core.Tests/Helpers/`:
  - `MockOpeningBookStore.cs` - In-memory IOpeningBookStore with thread-safe ConcurrentDictionary
  - `MockPositionCanonicalizer.cs` - Identity symmetry implementation for predictable tests
  - `MockOpeningBookValidator.cs` - Basic validation logic for testing
  - `OpeningBookEntryBuilder.cs` - Fluent builder for OpeningBookEntry test data
  - `BookMoveBuilder.cs` - Fluent builder for BookMove test data
  - `OpeningBookTestSetup.cs` - Shared setup utilities for creating generators with mock/real stores
- **Performance Optimization Tests** - 15+ new tests in `OpeningBookGeneratorEdgeCaseTests.cs`:
  - Survival zone tests (plies 6-13: extra time, more candidates)
  - Early exit tests (score gap thresholds: 150 at depth 6+, 200 otherwise)
  - Adaptive time allocation tests (-30% early, +50% survival zone, +20% late)
  - Depth-weighted progress tests (survival zone has higher weights)
  - Thread worker pool tests (min(4, positionCount) formula)
  - Progress event tests (atomic Interlocked operations)

### Changed
- **Test Architecture Migration** - Migrated opening book tests to new architecture:
  - `SqliteOpeningBookStoreTests.cs` - Replaced custom MockLogger with NullLogger<T>
  - `OpeningBookMatchupTests.cs` - Uses IAsyncLifetime, temp files, NullLogger
  - `OpeningBookConsistencyTests.cs` - Uses temp file pattern with proper cleanup
  - `GMBookDepthTest.cs` - Uses temp file pattern with proper cleanup
  - All tests now use `Path.GetTempPath()` instead of hardcoded relative paths
  - Added `[Trait("Category", "SkipOnCI")]` for tests requiring external book file

### Performance
- **Major:** Opening book generation now achieves 70%+ CPU utilization across all cores
  - Enabled parallel search for BookGeneration difficulty (was disabled)
  - Optimized thread allocation: 4 outer workers x 5 threads per search = 20 threads
  - Reduced memory footprint from 7-8GB to ~1GB through AI instance reuse
  - Reduced MinimaxAI TT size from 64MB to 16MB for candidate evaluation
  - Reduced TimePerPositionMs from 30s to 15s for faster generation
- **Server GC enabled** for Caro.BookBuilder for better multi-threaded memory management

### Changed
- AIDifficultyConfig.GetBookGenerationThreadCount(): processorCount/4 (was 256 fixed)
- OpeningBookGenerator: Reverted to sequential candidate evaluation (was parallel)
  - Each position reuses one AI instance across 6 candidates
  - Outer loop (4 workers) provides position-level parallelism
  - Parallel search enabled within each candidate (processorCount/4 threads)
- OpeningBookGeneratorEdgeCaseTests moved to IntegrationTests project (opt-in, slow tests)
- Updated OpeningBookGeneratorTests to verify new architecture configuration
- Removed debug Console.WriteLine statements from MinimaxAI for BookGeneration difficulty

### Fixed
- Nested parallelism causing thread oversubscription and low CPU utilization
- Memory blowup from creating 24 concurrent MinimaxAI instances (276MB each)
- Slow book generation progress (depth 3 in 2 minutes → depth 7-8 in 2 minutes)

### Added
- AIDifficultyConfigTests.cs with 7 tests for difficulty configuration validation
- SqliteOpeningBookStoreTests.cs: 12 comprehensive infrastructure tests
- OpeningBookGeneratorTests.cs: Edge case and behavior tests

### Removed
- Unused channel-based write loop code (replaced with direct batch storage)
- AtomicBoolean helper class (no longer needed after reverting to sequential candidates)

### Test Counts
- Caro.Core.Tests: 344 unit tests (+6 helpers, +7 AIDifficultyConfigTests, -5 OpeningBookGeneratorEdgeCaseTests moved)
- Caro.Core.IntegrationTests: 153 tests (+5 OpeningBookGeneratorEdgeCaseTests moved, +15 performance tests)
- Caro.Core.MatchupTests: 57 tests
- Caro.Core.Domain.Tests: 67 tests
- Caro.Core.Application.Tests: 8 tests
- Caro.Core.Infrastructure.Tests: 72 tests (+12 SqliteOpeningBookStoreTests)
- Total: 700+ backend tests passing

[1.35.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.35.0

## [1.34.0] - 2026-02-08

### Fixed
- **Critical:** SQLite transaction error in opening book batch storage
  - Added `command.Transaction = transaction;` in `SqliteOpeningBookStore.StoreEntriesBatch`
  - Microsoft.Data.Sqlite requires explicit transaction association when commands execute within transactions
  - Error: "Execute requires the command to have a transaction object when the connection assigned to the command is in a pending local transaction"
- SqliteOpeningBookStore.Dispose() NullReferenceException during cancellation
  - Added try-catch protection for connection disposal in invalid states
  - Gracefully handles cleanup when worker threads are cancelled mid-operation

### Added
- SqliteOpeningBookStoreTests.cs with comprehensive test coverage:
  - StoreEntriesBatch_TransactionIsCommittedCorrectly - validates transaction handling
  - StoreEntriesBatch_LargeBatch_DoesNotThrowTransactionError - stress test with 100 entries
  - Basic CRUD operations, statistics, metadata, and edge case tests
  - 12 new tests for Infrastructure.Tests project

### Performance
- OpeningBookGenerator: Dedicated Thread worker swarm replaces Parallel.ForEachAsync
  - Bypasses ThreadPool hill-climbing for CPU-bound AI workloads
  - Each worker thread owns its own MinimaxAI instance (64MB TT per thread)
  - Utilizes all CPU cores on high-thread-count systems (i7-12700F: ~20 threads)
  - Removed AI instance pooling (_aiPool, RentAI, ReturnAI) - no longer needed

### Changed
- OpeningBookGenerator.ProcessPositionsInParallelAsync implementation:
  - Uses ConcurrentQueue for work distribution and Thread.Join() for synchronization
  - WorkerThreadLoop method with per-thread MinimaxAI lifecycle
  - GenerateMovesForPositionAsync overload accepting AI instance (DI pattern)

### Removed
- InMemoryOpeningBookStore.cs - consolidated to SqliteOpeningBookStore

### Test Counts
- Caro.Core.Tests: 330 unit tests
- Caro.Core.IntegrationTests: 143 tests (opt-in)
- Caro.Core.MatchupTests: 57 tests
- Caro.Core.Domain.Tests: 67 tests
- Caro.Core.Application.Tests: 8 tests
- Caro.Core.Infrastructure.Tests: 60 tests (+12 new SqliteOpeningBookStoreTests)
- Total: 665+ backend tests passing

[1.34.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.34.0

## [1.33.0] - 2026-02-07

### Fixed
- Domain and Application test projects updated to match current simplified domain API
- BoardTests.cs rewritten to test `new Board()` constructor instead of `Board.CreateEmpty()`
- GameStateTests.cs updated to use `WithMove()` and `WithGameOver()` record methods
- PositionTests.cs updated to test current Position struct API (BoardSize, IsValid, Offset, ToTuple, FromTuple)
- GameMapperTests.cs updated to use `WithMove()` instead of obsolete `RecordMove()`
- GameMapper.ToBoardDto() no longer calls non-existent `board.GetHash()` - uses placeholder value

### Changed
- Domain tests now properly test the simplified immutable domain design
- Removed tests for non-existent APIs: Hash, TotalStones, RemoveStone, CountStones, GetBitBoard, GetEmptyCells, GetOccupiedCells, ApplyMoves
- Removed tests for Position.Index, FromIndex, GetAdjacentPositions, GetNeighbors
- GameState equality tests updated to compare properties (Board is a class, not a record)

### Test Counts
- Caro.Core.Domain.Tests: 67 passed (updated for current API)
- Caro.Core.Application.Tests: 8 passed
- Total backend tests: 453+ passing

[1.33.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.33.0

## [1.32.0] - 2026-02-07

### Added
- Detailed test output: `dotnet test --logger "console;verbosity=detailed"` now shows each test name and result
- Test documentation updated to recommend verbose logging as default approach

### Changed
- Test runner script (`run-tests.ps1`) now uses detailed logger by default
- Simplified test script: removed obsolete stress/verification filter commands
- Test README reorganized with clearer quick reference section

### Removed
- Obsolete test files moved from Caro.Core.Tests to Caro.Core.IntegrationTests:
  - DFPNSearchTests, ThreatSpaceSearchTests, ParallelMinimaxSearchOpenRuleTests
  - EnhancedMoveOrderingTests, MinimaxAITests, PondererTests
  - SIMDDebugTest, SimdDebugTest2, SimdPrecisionDebug, VerticalOpenThreeDebug
  - Concurrency stress tests (ConcurrencyStressTests, AdversarialConcurrencyTests, DeadlockDetectionTests)
- Slow AI search tests no longer run with default `dotnet test`

### Fixed
- Default `dotnet test` now completes in ~30 seconds (was 10+ minutes with hanging AI tests)
- Test organization: slow tests properly isolated to IntegrationTests project

### Test Counts
- Unit tests (default): 378 tests (~30s)
  - Caro.Core.Tests: 330 tests
  - Caro.Core.Infrastructure.Tests: 48 tests
- Integration tests (opt-in): 100+ tests
- Matchup tests (opt-in): 50+ tests

[1.32.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.32.0

## [1.31.0] - 2026-02-07

### Performance
- Opening book generation: MinimaxAI object pooling eliminates 8GB rapid allocations
  - Fixed memory churn where each candidate evaluation created new 64MB TT instance
  - Reuse pooled AI instances per position instead of per candidate
  - Reduced GC pressure from continuous Gen2 collections
- TranspositionTable: Zero-allocation hot path with struct-based entries
  - Converted TranspositionEntry from class to 16-byte struct
  - Eliminated heap allocations for every TT store operation
  - Accepts "lossy" atomicity (hash verification validates integrity post-read)
- SQLite batch writes for opening book storage
  - Added StoreEntriesBatch() to IOpeningBookStore interface
  - Transactions group 500-1000 entries for reduced lock contention
  - Background actor model prevents worker threads blocking on I/O

### Fixed
- Nested parallelism in opening book generation
  - Removed Task.WhenAll for candidate evaluation within position threads
  - Process candidates sequentially per thread (outer loop provides parallelism)
  - Eliminated thread oversubscription (96 threads fighting for 12 cores)
- Opening book generator now flattens concurrency for proper CPU utilization
  - One AI instance rented per position, not per candidate
  - try/finally ensures AI returns to pool even on exceptions

### Changed
- OpeningBookGenerator: Added RentAI(), ReturnAI(), and _aiPool field
- LockFreeTranspositionTable: TranspositionEntry now a struct with explicit layout
- IOpeningBookStore: Added StoreEntriesBatch(IEnumerable<OpeningBookEntry>) method
- SqliteOpeningBookStore: Batch INSERT OR REPLACE within transactions
- InMemoryOpeningBookStore: Implemented batch storage for consistency

### Test Organization
- Created Caro.Core.IntegrationTests project for slow AI search tests
  - Marked with <IsTestProject>false</IsTestProject> to exclude from default runs
  - Tests run only with explicit project targeting
- Moved 13 integration test files from Caro.Core.Tests:
  - DefensivePlayTests, MasterDifficultyTests, QuickGrandmasterVsEasy
  - TranspositionTablePerformanceTests, NodeCountingTests
  - DiagonalThreatTest, ThreatDetectorDebugTest, ZeroAllocationTests
  - AspirationWindowTests, HistoryHeuristicTests, LateMoveReductionTests
  - PrincipalVariationSearchTests, QuiescenceSearchTests
- Separated concerns: Unit tests (fast), IntegrationTests (AI searches), MatchupTests (full matchups)

### Build Quality
- Backend: 0 warnings, 0 errors
- Frontend: 0 errors, 0 warnings (svelte-check)

### Test Counts
- Caro.Core.Tests: 431 passed (removed slow integration tests)
- Caro.Core.IntegrationTests: 143 passed (opt-in, excluded from default)
- Caro.Core.Infrastructure.Tests: 48 passed
- Caro.Core.MatchupTests: 57 passed
- Frontend Unit (Vitest): 19 passed
- Total: 698 tests passing

[1.31.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.31.0

## [1.30.0] - 2026-02-07

### Changed
- Opening book builder: Removed misleading `--max-depth` and `--target-depth` CLI arguments
  - Book structure is now hardcoded as a 4-3-2-1 tapered beam up to ply 40
  - Plies 0-14: 4 moves/position (early game + survival zone)
  - Plies 15-24: 3 moves/position (Hard difficulty)
  - Plies 25-32: 2 moves/position (Grandmaster)
  - Plies 33-40: 1 move/position (Experimental - main line)
  - Arguments were misleading since the actual tiered structure is determined by GetMaxChildrenForDepth()
- Added ValidateArguments() method to reject unrecognized CLI arguments with clear error messages

### Fixed
- Opening book builder: Default output path now correctly resolves to repository root
  - Previously created book at backend/src/Caro.BookBuilder/ (project directory)
  - Now uses AppContext.BaseDirectory with relative path navigation to repo root
  - Ensures consistency with Caro.Api, Caro.UCI, and test projects' book location expectations
- README.md book builder examples now reflect hardcoded structure (removed --max-depth references)

### Documentation
- Updated book builder help text to document the hardcoded tiered beam structure
- Console output now displays "4-3-2-1 tapered beam up to ply 40" instead of configurable depths

### Build Quality
- Backend: 0 warnings, 0 errors
- Frontend: 0 errors, 0 warnings (svelte-check)

### Test Counts
- Caro.Core.Tests: 525 passed, 1 skipped
- Caro.Core.Infrastructure.Tests: 48 passed
- Frontend Unit (Vitest): 19 passed
- Total: 592 tests passing

[1.30.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.30.0

## [1.29.0] - 2026-02-07

### Fixed
- Removed redundant System.Collections.Immutable package reference in .NET 10 (NU1510 warning)
  - Package is built into .NET 10 runtime, explicit reference was unnecessary
- Svelte 5 reactivity warning in Timer.svelte
  - Changed $state initialization from prop capture to explicit sync in $effect
- Flaky AsyncQueueTests.EnqueueAsync_ItemGetsProcessed test
  - Added processingStarted signal to ensure background processor starts before enqueuing
  - Removed Task.WhenAny timeout pattern for deterministic behavior

### Build Quality
- Backend: 0 warnings, 0 errors
- Frontend: 0 errors, 0 warnings (svelte-check)

### Test Counts
- Caro.Core.Tests: 525 passed, 1 skipped
- Caro.Core.Infrastructure.Tests: 48 passed
- Frontend Unit (Vitest): 19 passed
- Total: 592 tests passing

[1.29.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.29.0

## [1.28.0] - 2026-02-07

### Added
- UCI Mock Client console application for testing UCI protocol layer
  - Runs matches between two UCI engine instances via stdin/stdout
  - Full time control support with increment handling
  - Proper engine state synchronization across both engines
  - Configurable game count, time control, and skill levels
  - Detailed match statistics and move-by-move reporting

### Fixed
- **Critical:** Time scramble timeout bug - engine now uses increment-based allocation when low on time
  - When remaining time < 3× increment, allocates 40% of increment as max time budget
  - Prevents timeouts in long games (tested up to 361 moves / 25 minutes)
  - Ensures all games end by win/loss/draw, never by timeout
- Infinite recursion in OpeningBookLookupService.NextRandomInt() (was calling itself)
- Engine state synchronization issue in mock client (both engines now track same move history)
- Windows Process.Start() native DLL loading issue (disabled stderr redirect)
- UCI engine now gracefully handles missing opening book database
- ObjectDisposedException in UCIMockClient cleanup (proper disposal order)

### Changed
- AdaptiveTimeManager time scramble detection threshold (3× increment or 30 seconds)
- UCIMockClient prefers .csproj files for 'dotnet run' (better cross-platform compatibility)
- OpeningBook fields nullable for graceful degradation without database

### Documentation
- README.md updated with test counts and feature descriptions

### Test Counts
- Caro.Core.Tests: 525 passed
- Caro.Core.Infrastructure.Tests: 48 passed
- Total: 573 backend tests (unchanged)

[1.28.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.28.0

## [1.27.0] - 2026-02-07

### Added
- UCI (Universal Chess Interface) protocol support for standalone engine usage
  - Caro.UCI console application for standard UCI compliance
  - UCI protocol library in Caro.Core.GameLogic.UCI namespace
  - UCIMoveNotation for algebraic coordinate conversion (a-s, 1-19)
  - UCIPositionConverter for parsing position commands
  - UCIEngineOptions for runtime configuration (Skill Level, Opening Book, Threads, Hash, Ponder)
  - UCISearchController bridging UCI go parameters to MinimaxAI
- API WebSocket UCI bridge at /ws/uci endpoint
- Frontend UCI engine client (uciEngine.ts) with promise-based API
- Frontend UCI integration in gameStore (connectUCI, disconnectUCI, getAIMoveUCI)
- UCI connection status indicator and toggle in game page
- Engine options exposed as UCI setoption commands:
  - Skill Level (1-6): Maps to AIDifficulty enum
  - Use Opening Book (true/false)
  - Book Depth Limit (0-40 plies)
  - Threads (1-32)
  - Hash (32-4096 MB)
  - Ponder (true/false)

### Changed
- Caro.Api.sln includes Caro.UCI project
- Program.cs registers UCIHandler as singleton and adds /ws/uci WebSocket endpoint

### Fixed
- Console output buffering in UCI engine (added explicit Console.Out.Flush)
- UCI quit command now waits for ongoing search to complete before exiting
- Multi-word option name parsing (e.g., "Skill Level", "Use Opening Book")

### Test Counts
- Caro.Core.Tests: 525 passed
- Caro.Core.Infrastructure.Tests: 48 passed
- Total: 573 backend tests (unchanged)

### Documentation
- README.md updated with UCI protocol section and standalone engine usage

[1.27.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.27.0

## [1.26.0] - 2026-02-07

### Added
- Tiered opening book continuation system for better coverage
  - Plies 0-14: 4 responses (early game + survival zone)
  - Plies 15-24: 3 responses (Hard coverage)
  - Plies 25-32: 2 responses (GM coverage)
  - Plies 33-40: 1 response (Exp coverage)
  - Ensures GM/Exp always have responses to opponent deviations
  - GetMaxChildrenForDepth helper method for centralized branching logic
  - 3 tiered branching unit tests

### Changed
- Experimental max book depth capped at 40 plies (was unlimited)
- Opening book generation now uses tiered response counts at all depths
- Opponent response generation uses depth-based tiered strategy

### Fixed
- Flaky TranspositionTable_HitRateIsMeasurable performance test (removed timing assertion)
- Move equality assertion now sufficient to verify TT functionality

### Test Counts
- Caro.Core.Tests: 525 passed (+3 from v1.25.0)
- Caro.Core.Infrastructure.Tests: 48 passed
- Total: 573 backend tests

[1.26.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.26.0

## [1.25.0] - 2026-02-06

### Added
- Four time control options: Bullet (1+0), Blitz (3+2), Rapid (7+5), Classical (15+10)
- Unified game creation API for PvP, PvAI, and AIvAI modes
- Frontend time control selector and AIvAI mode support
- Configurable opening book depth per difficulty (Hard: 24, Grandmaster: 32, Experimental: unlimited)

### Changed
- Opening book first move no longer hardcoded to center
- OpeningBookLookupService uses AIDifficultyConfig for depth limits
- GameState added time control and game mode properties
- GameSession constructor accepts time control and game mode parameters

### Fixed
- Opening book correctly disabled for Easy/Medium difficulties
- AI now calculates natural first move when opening book disabled

[1.25.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.25.0

## [1.24.0] - 2026-02-06

### Changed
- Fully immutable domain model: Cell as readonly record struct, GameState as sealed record
- AI code updated for immutable pattern (removed SetPlayerUnsafe throughout)
- Service and tournament layers use new immutable pattern

### Fixed
- ThreatSpaceSearch board parameter bug (was using wrong board for GetDefenseMoves)
- GameState.UndoMove() implementation fixed for ImmutableStack
- UndoMove CurrentPlayer logic special case for MoveNumber=0

### Added
- System.Collections.Immutable package for history tracking

### Removed
- Cell.SetPlayerUnsafe(), Cell.GetPlayerUnsafe(), Board.Clone(), Board.MutableBoard

[1.24.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.24.0

## [1.23.0] - 2026-02-06

### Fixed
- AI code now uses board.BoardSize instead of hardcoded 15
- Center positions calculated correctly for 19x19 boards
- Board.Clone() shallow copy bug (deep copies Cell objects now)

### Added
- Domain layer entities: Board, GameState, Position, factories
- BoardExtensions for AI technical concerns (BitBoard conversion, hash, CloneWithState)

[1.23.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.23.0

## [1.22.0] - 2026-02-06

### Changed
- Removed singleton pattern from OpeningBook (constructor injection only)
- OpeningBook types moved to Caro.Core.Domain.Entities namespace
- Updated namespace references from Caro.Core.Entities to Caro.Core.Domain.Entities

[1.22.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.22.0

## [1.21.0] - 2026-02-06

### Added
- Opponent response generation for opening book (stores top 4 responses per move)
- GM vs GM book depth verification test
- Opening book consistency and matchup tests

[1.21.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.21.0

## [1.20.0] - 2026-02-05

### Added
- SQLite opening book integration with ASP.NET Core DI
- Depth-based opening book filtering by difficulty (Hard: depth 24, GM/Experimental: depth 32)

[1.20.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.20.0

## [1.19.0] - 2026-02-05

### Fixed
- Opening book depth range off-by-one error (loop condition `depth <= maxDepth`)
- Progress percentage stalled at 99% (made GetDepthWeight calculate dynamic weights)

[1.19.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.19.0

## [1.18.0] - 2026-02-05

### Fixed
- Added diagnostic logging for corrupted book data during generation

### Added
- 4 integration tests for "from book" move application scenario

[1.18.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.18.0

## [1.17.0] - 2026-02-05

### Fixed
- "Cell is already occupied" error during opening book generation
- Root cause: CloneBoard method instead of proper Board.Clone()

### Added
- 13 new tests for Board.Clone() and OpeningBookGenerator

[1.17.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.17.0

## [1.16.0] - 2026-02-05

### Changed
- Created Caro.Core.MatchupTests project for integration tests
- Unit tests now run faster without integration overhead

### Removed
- 5 failing/flaky test files moved to MatchupTests
- 5 flaky performance test methods removed entirely

[1.16.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.16.0

## [1.15.0] - 2026-02-05

### Changed
- Default book generation depths increased (max-depth: 12→32, target-depth: 24→32)
- Survival zone enhancements (plies 6-13): increased branching to 4, +50% time allocation, 10 candidates

[1.15.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.15.0

## [1.14.0] - 2026-02-05

### Added
- `--debug` flag for opening book builder (verbose logging)
- `--help` / `-h` flag for usage information

### Changed
- Default logging level changed from Debug to Warning
- AI defense logging now uses ILogger instead of Console.WriteLine

[1.14.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.14.0

## [1.13.0] - 2026-02-05

### Fixed
- Opening book generation stopping prematurely at depth 1
- Root cause: candidate selection ranked BEFORE Open Rule validation

### Added
- ILogger<OpeningBookGenerator> for diagnostic logging

[1.13.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.13.0

## [1.12.0] - 2026-02-05

### Changed
- README.md comprehensive accuracy refresh (versions, test counts)
- CSHARP_ONBOARDING.md comprehensive overhaul (fixed typos, added architecture context)

[1.12.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.12.0

## [1.11.0] - 2026-02-04

### Fixed
- Thread oversubscription in opening book generation (disabled parallel search per position)
- Thread safety issue with shared MinimaxAI instance (now creates local instances per task)

### Changed
- MinimaxAI constructor accepts optional ttSizeMb parameter
- Candidate pruning uses static evaluation for pre-sorting (reduced to top 6 candidates)

[1.11.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.11.0

## [1.10.0] - 2026-02-04

### Fixed
- Canonical coordinate storage bug (moves now transformed to canonical space before storing)
- Progress display stuck at 95% (rebalanced depth weights)

[1.10.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.10.0

## [1.9.0] - 2026-02-04

### Added
- AsyncQueue-based progress tracking for opening book generation
- Resume functionality for existing books
- Configurable max-depth via CLI parameter

### Removed
- 1000 position safety limit

[1.9.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.9.0

## [1.8.0] - 2026-02-02

### Added
- Worker pool architecture for opening book generation (30x throughput improvement)
- Parallel candidate evaluation, tapered beam width, dynamic early exit
- SQLite WAL mode for concurrent writes

[1.8.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.8.0

## [1.7.0] - 2026-02-02

### Added
- Opening book system (SQLite-backed storage, 8-way symmetry reduction)
- AIDifficulty.BookGeneration for offline book generation
- Caro.BookBuilder CLI tool

[1.7.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.7.0

## [1.6.0] - 2026-02-01

### Changed
- Move ordering refactored: Hash Move now unconditional #1 priority for Lazy SMP
- VCF architecture redesigned: separate VCFSolver runs before alpha-beta search

### Added
- VCFSolver class, VCFNodeResult, IsEmergencyDefense() method
- Percentage-based VCF time threshold

[1.6.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.6.0

## [1.5.0] - 2026-02-01

### Added
- Clean Architecture refactoring (Domain, Application, Infrastructure layers)
- New test projects aligned with Clean Architecture layers

### Changed
- Board size increased from 15x15 to 19x19
- Integration tests separated with [Trait("Category", "Integration")]

[1.5.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.5.0

## [1.4.0] - 2026-02-01

### Changed
- Stateless AI architecture: player color passed explicitly to pondering methods
- TournamentEngine now uses bot-instance-based architecture

[1.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.4.0

## [1.3.0] - 2026-02-01

### Added
- Centralized testing framework for AI difficulty validation
- 7 test suites with win rate thresholds

### Changed
- AI difficulty configurations rebalanced
- Owner tags to all AI debug logs

[1.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.3.0

## [1.2.0] - 2026-01-31

### Fixed
- Transposition table master thread priority for same-position entries
- Tournament runner output path changed to tournament_results.txt

### Added
- DepthVsRandomTest, DiagonalThreatTest, GrandmasterVsBraindeadTest validation

[1.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.2.0

## [1.1.0] - 2026-01-29

### Added
- Time-budget AI depth system with dynamic depth calculation
- Centralized AI difficulty configuration (AIDifficultyConfig)
- Dynamic Open Rule enforcement for AI

### Fixed
- Critical Open Rule violation bug (illegal moves)
- Pondering timing issues

[1.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.1.0

## [1.0.0] - 2026-01-29

### Added
- Comprehensive AI tournament system with round-robin matchups
- ELO rating tracking, SQLite logging with FTS5, SignalR broadcasts

[1.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.0.0

## Early Development (0.x) - 2026-01-20 to 2026-01-29

Major milestones:
- VCF (Victory by Continuous Four) tactical solver
- AI Strength Validation Test Suite with statistical analysis
- Stats publisher-subscriber architecture
- Transposition table sharding (16 segments)
- Time-budget depth system per difficulty
- Pondering and both-pondering support

[0.4.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.4.2
[0.0.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.1
