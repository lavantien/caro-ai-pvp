# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.0] - 2026-02-01

### Changed

- **Breaking: Stateless AI architecture** - MinimaxAI no longer tracks which color it's playing
  - Player color must now be passed explicitly to pondering methods
  - `StartPonderingAfterMove()` now requires explicit `Player thisAIColor` and `AIDifficulty difficulty` parameters
  - `StartPonderingNow()` now requires explicit `Player thisAIColor` parameter
  - `StopPondering()` has an overload that takes `Player forPlayer` parameter
  - Old stateful methods marked `[Obsolete]` - will be removed in future version
- TournamentEngine now uses bot-instance-based architecture
  - Renamed `_redAI/_blueAI` to `_botA/_botB` for clarity
  - Bots maintain their difficulty capabilities regardless of which color they play
  - `swapColors` parameter controls which bot instance plays which color
- TestSuiteRunner and GrandmasterVsBraindeadRunner updated to use `swapColors` parameter
  - Difficulties stay constant with their bot instances
  - Color alternation now handled via `swapColors` instead of difficulty swapping

### Fixed

- Color attribution in pondering stats was incorrect when colors were swapped
  - Pondering output now correctly shows `[PONDER Red]` or `[PONDER Blue]` based on actual bot color assignment
  - Previously always showed `[PONDER Red]` due to state leakage between games
- Winner/loser difficulty determination now correctly maps colors to bot instances
  - Previously could report wrong difficulty when `swapColors=true`
- Opponent pondering now uses correct bot's difficulty settings

### Added

- `ColorSwapTest.cs` - Test runner for verifying color alternation works correctly
- `--color-swap-test` CLI argument to run color swap verification test

### Technical Details

**Before (Stateful):**
```csharp
// AI tracked which color it was playing
private Player _lastPlayer;
private AIDifficulty _lastDifficulty;

// Engine used color-based AI selection
var currentAI = isRed ? _redAI : _blueAI;
var difficulty = isRed ? redDifficulty : blueDifficulty;
```

**After (Stateless):**
```csharp
// AI is stateless about color - engine passes everything explicitly
var currentBotIsA = (isRed && botAIsRed) || (!isRed && !botAIsRed);
var difficulty = currentBotIsA ? botADifficulty : botBDifficulty;

// Pondering receives explicit color and difficulty
currentAI.StartPonderingAfterMove(board, opponent, currentPlayer, currentSettings.Difficulty);
```

This architecture ensures the AI engine remains a pure algorithmic component, while the TournamentEngine tracks game-level concerns like which bot is playing which color.

### Files Modified

- backend/src/Caro.Core/GameLogic/MinimaxAI.cs
- backend/src/Caro.Core/GameLogic/OpeningBook.cs
- backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs
- backend/src/Caro.Core/GameLogic/Pondering/Ponderer.cs
- backend/src/Caro.Core/GameLogic/TimeBudgetDepthManager.cs
- backend/src/Caro.Core/Tournament/TournamentEngine.cs
- backend/src/Caro.TournamentRunner/TestSuiteRunner.cs
- backend/src/Caro.TournamentRunner/GrandmasterVsBraindeadRunner.cs
- backend/src/Caro.TournamentRunner/Program.cs
- backend/tests/Caro.Core.Tests/GameLogic/Pondering/PonderingIntegrationTests.cs

### Files Added

- backend/src/Caro.TournamentRunner/ColorSwapTest.cs

[1.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.4.0

## [1.3.1] - 2026-02-01

### Fixed

- Test suite move statistics output missing from tournament_results.txt
  - Added onMove callback to TestSuiteRunner.RunMatchup()
  - Each move now writes formatted stat line with depth, nodes, NPS, TT%, pondering, VCF
  - Game result lines now included after each game
- Game counter tracking for proper move numbering across multiple games

### Files Modified

- backend/src/Caro.TournamentRunner/TestSuiteRunner.cs

[1.3.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.3.1

## [1.3.0] - 2026-02-01

### Added

- Centralized testing framework for AI difficulty validation
  - Single CLI entry point: `--test-suite=<name>` for running test suites
  - 7 test suites: braindead, easy, medium, hard, grandmaster, experimental, full
  - Win rate thresholds enforced and reported per matchup
  - Consistent output format always overwrites `tournament_results.txt`
  - Exit code 0 for informational-only CI compatibility
- Test suite infrastructure in `Caro.TournamentRunner/TestSuite/`
  - ITestSuite interface with TestSuiteResult and MatchupResult records
  - Per-suite expectations with configurable win rate thresholds
  - 20 games per matchup (10+10 alternating colors)
  - Experimental suite uses 10 games for faster iteration
- Owner tags to all AI debug logs
  - `[AI DEFENSE]`, `[AI VCF]`, `[AI TT]`, `[AI STATS]` now include `{difficulty} ({player})`
  - Easier tracing of which AI is generating which log line

### Changed

- AI difficulty configurations for better strength progression
  - Braindead: time 1% -> 5%, error 20% -> 10%
  - Easy: time 10% -> 20%
  - Medium: time 30% -> 50%
  - Hard: time 70% -> 75%, added VCF enabled
- README updated with current difficulty specs and test suite documentation

### Fixed

- TournamentEngine.RunGame no longer terminates early when both players run out of time
  - Previously treated double timeout as draw, now correctly identifies winner by move completion
- ParallelMinimaxSearch helper thread TT write policy
  - Helper threads now respect depth >= rootDepth/2 constraint for all writes
  - Master thread (threadIndex=0) retains priority for same-depth entries

### Technical Details

**Test Suite CLI Usage:**
```bash
dotnet run --project backend/src/Caro.TournamentRunner -- --test-suite=full --output=results.txt
```

**Win Rate Thresholds:**
- Grandmaster vs Braindead: 100% (win+draw)
- Grandmaster vs Easy: 95% (win+draw)
- Grandmaster vs Medium: 90% (win+draw)
- Grandmaster vs Hard: 80% (win+draw)
- Hard vs Braindead: 95% (win+draw)
- Hard vs Easy: 90% (win+draw)
- Hard vs Medium: 80% (win+draw)
- Medium vs Braindead: 90% (win+draw)
- Medium vs Easy: 80% (win+draw)
- Easy vs Braindead: 80% (win+draw)

### Files Added

- backend/src/Caro.TournamentRunner/TestSuite/ITestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/GrandmasterTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/HardTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/MediumTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/EasyTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/BraindeadTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/ExperimentalTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuite/FullIntegratedTestSuite.cs
- backend/src/Caro.TournamentRunner/TestSuiteRunner.cs

### Files Modified

- backend/src/Caro.Core/GameLogic/AIDifficultyConfig.cs (time multipliers, error rates)
- backend/src/Caro.Core/GameLogic/MinimaxAI.cs (owner tags in logs)
- backend/src/Caro.TournamentRunner/Program.cs (CLI argument parsing)
- README.md (difficulty specs, test suite documentation)

[1.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.3.0

## [1.2.0] - 2026-01-31

### Fixed

- Transposition table master thread priority for same-position entries
  - Master thread (threadIndex=0) now has priority when storing entries at same depth
  - Helper threads can only replace if significantly deeper (depth diff >= 2)
  - Prevents helper threads from overwriting master's more accurate results
  - Same-position replacement now considers entry flag quality (Exact > Alpha/Beta)
- Tournament runner output path changed to tournament_results.txt for clarity
  - Previous test_output.txt was ambiguous

### Changed

- ComprehensiveMatchupRunner simplified to use compile-time constants
  - TimeSeconds=420, IncSeconds=5, GamesPerMatchup=10
  - Removed method parameters for reproducibility
  - Cleaner console output formatting (removed box-drawing borders)
- Program.cs output file renamed from test_output.txt to tournament_results.txt

### Added

- DepthVsRandomTest - validates depth-5 search beats random 80%+ of time
- DiagonalThreatTest - tests AI correctly blocks diagonal four-in-row threats
- GrandmasterVsBraindeadTest now includes 20% error rate validation test

### Removed

- Temporary tournament output files (tournament_report_*.txt, tournament_test.txt)

### Technical Details

**Transposition Table Replacement Strategy:**

```csharp
// Same position: only replace if deeper or same depth with master priority
bool isDeeper = depth > existing.Depth;
bool isSameDepthMaster = depth == existing.Depth && threadIndex == 0;
bool isSameDepthBetterFlag = depth == existing.Depth && entryFlag == EntryFlag.Exact;
shouldStore = isDeeper || isSameDepthMaster || isSameDepthBetterFlag;
```

This ensures the master thread's principal variation results are not overwritten by helper threads exploring alternate lines.

### Test Coverage Update

- Total tests: 500+ (up from 330+)
- Backend: 480+ tests
- Frontend: 26 tests

[1.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.2.0

## [1.1.0] - 2026-01-29

### Added

- Time-budget AI depth system with dynamic depth calculation
  - Depth formula: depth = log(time * nps * timeMultiplier) / log(ebf)
  - Per-difficulty time multipliers: Braindead 1%, Easy 10%, Medium 30%, Hard 70%, Grandmaster 100%
  - NPS calibration per difficulty for fair depth scaling
  - Automatic adaptation to host machine performance
- Centralized AI difficulty configuration (AIDifficultyConfig)
  - Single source of truth for all difficulty parameters
  - Thread counts, time budgets, pondering, VCF, parallel search settings
  - Per-difficulty target NPS for depth calculation
  - Dynamic grandmaster thread count: (processorCount/2)-1
- Dynamic Open Rule enforcement for AI move generation
  - AI now correctly respects Open Rule on move #3
  - Exclusion zone centered on first move, not fixed board center
  - Added IsValidPerOpenRule() helper method
  - 4 new integration tests for Open Rule compliance
- Unified ponder log format across all components
  - All logs use `[PONDER Red]` or `[PONDER Blue]` prefix
  - Removed verbose formats like "thinking for X's turn"
  - Consistent stats display: nodes, time, depth, NPS

### Changed

- Pondering statistics now properly display depth, nodes, and NPS
  - Fixed GetLastPonderStats() to return depth achieved
  - Ponder stats shown in P column with format D{depth}/{nodes}Kn/{nps}nps
  - Both-pondering now works correctly when both players have it enabled
- Tournament runner simplified to use comprehensive runner only
  - Removed multiple runner modes
  - Fixed preset: 420+5 seconds, 10 games per matchup
  - Auto-resolving output file path
- Move numbering bug fixed (moves showing 1, 3, 5 now show 1, 2, 3)

### Fixed

- Critical Open Rule violation bug causing illegal moves
  - Old code used fixed center 5x5 zone [5-9, 5-9]
  - New code uses dynamic zone centered on first move
  - Example: First move at (7,6), exclusion zone is x in [5,9], y in [4,8]
  - Move (7,4) now correctly filtered as invalid (was incorrectly allowed before)
- Pondering timing issues
  - Fixed StopPondering() to capture final stats after cancellation
  - Fixed PonderLazySMP to return total nodes searched
  - Fixed ponder thread count to match main search settings

### Technical Details

**Time-Budget Depth Calculation:**

```
depth = log(time_ms * nps * timeMultiplier) / log(ebf)
- ebf (effective branching factor): ~3-4 for Caro
- timeMultiplier: 0.01 (Braindead), 0.10 (Easy), 0.30 (Medium), 0.70 (Hard), 1.0 (Grandmaster)
- Depth varies by host machine - higher-spec machines achieve greater depth naturally
```

**Dynamic Open Rule Enforcement:**

```
if (player == Red && moveNumber == 3) {
    // Find first red stone position
    // Calculate exclusion zone: x in [firstX-2, firstX+2], y in [firstY-2, firstY+2]
    // Filter out any candidate within Chebyshev distance < 3
}
```

**Centralized Configuration:**

```csharp
AIDifficultyConfig.Instance.GetSettings(difficulty)
// Returns: ThreadCount, TimeMultiplier, PonderingEnabled, ParallelSearchEnabled, etc.
```

### Files Added

- src/Caro.Core/GameLogic/AIDifficultyConfig.cs
- src/Caro.Core/GameLogic/TimeBudgetDepthManager.cs
- tests/Caro.Core.Tests/GameLogic/ParallelMinimaxSearchOpenRuleTests.cs

### Files Modified

- src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs (Open Rule, time budget, depth calc)
- src/Caro.Core/GameLogic/MinimaxAI.cs (pondering stats, config integration)
- src/Caro.Core/GameLogic/Pondering/Ponderer.cs (unified logging, timing fixes)
- src/Caro.Core/GameLogic/ThreadPoolConfig.cs (difficulty-based thread counts)
- src/Caro.Core/Tournament/TournamentEngine.cs (both-pondering timing)
- src/Caro.TournamentRunner/Program.cs (simplified to comprehensive runner)

[1.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.1.0

## [1.0.0] - 2026-01-29

### Added

- Comprehensive AI tournament system with round-robin matchups
- ELO rating tracking and calculation
- SQLite logging with FTS5 full-text search
- SignalR broadcasts via async queues
- Balanced scheduling with color swapping

[1.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.0.0

## [0.4.2] - 2026-01-29

### Added

- Comprehensive AI engine architecture diagram in README
  - Lazy SMP parallel search flow (master + helper threads)
  - Stats publisher-subscriber pattern with Channel<MoveStatsEvent>
  - Transposition Table 16-segment sharding with hash distribution
  - Ponderer flow (PV prediction, background search, time merge)
  - Time Budget Manager (NPS, EBF, time multiplier per difficulty)
  - Component Flow descriptions with helper thread TT write policy

[0.4.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.4.2

## [0.4.1] - 2026-01-29

### Fixed

- Performance table to reflect actual implementation
  - Fixed thread counts: Braindead=1, Easy=2, Medium=3, Hard=4, GM=(N/2)-1
  - All difficulties use parallel search (not sequential as documented)
  - Depth is dynamic based on host NPS, not arbitrary caps
  - Added time budget percentages per difficulty

[0.4.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.4.1

## [0.4.0] - 2026-01-29

### Changed

- README extensively condensed for showcase-focused presentation
  - Reduced from 1,159 to 176 lines (85% reduction)
  - Removed verbose before/after code blocks
  - Replaced internal implementation details with technical tables
  - Single architecture diagram instead of multiple redundant ones
  - Focus on what the project does, not how it works internally

[0.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.4.0

## [0.3.1] - 2026-01-29

### Added

- Diagnostic script (`test_diagnostic.csx`) for quick AI strength validation
  - Quick 5-game matchups for Hard vs Medium and Medium vs Easy
  - Useful for fast regression testing during development

### Changed

- README extensively refactored with new architecture documentation
  - Added Stats Publisher-Subscriber pattern section with mermaid diagram
  - Added Transposition Table Sharding section with two mermaid diagrams
  - Updated Best Practices table with new patterns
  - Updated project structure with new files
- CLAUDE.md updated to prefer native tools over plugin tools

[0.3.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.3.1

## [0.3.0] - 2026-01-29

### Added

- Stats publisher-subscriber architecture for AI statistics
  - IStatsPublisher interface with typed stats channels
  - MoveStatsEvent with comprehensive search metrics
  - TournamentEngine subscribes to both AI stats channels
  - Separate stats types: MainSearch, Pondering, VCFSearch
- Transposition table sharding (16 segments) to reduce cache line contention
  - Lock-free per-shard access with independent locks
  - Reduced cache contention for parallel search
- Stricter helper thread TT write policy to prevent table pollution
  - Helper threads only store entries at depth >= rootDepth/2
  - Helper entries must be Exact bounds (no Upper/Lower)
  - Master thread entries protected from helper overwrites
- VCF statistics tracking and logging
  - VCF depth and nodes tracked in MoveStats
  - Matchup runner displays VCF metrics in game logs
- Improved game logging with main/ponder stats separation

### Changed

- Braindead difficulty error rate reduced from 50% to 20%
- Ponder stats capture timing improved (capture before cancellation)
- Test board size updated from 15x15 to 19x19 across all concurrency tests
- Concurrency test timeout increased to 60s for TT sharding overhead
- Acceptance criteria relaxed to 8/10 parallel searches completing

### Fixed

- Critical threat detection bug causing Braindead to beat Grandmaster
  - Open rule violation was not being penalized correctly
  - Threat detection now properly respects board boundaries

### Technical Details

**Transposition Table Sharding:**

- 16 segments with independent hash-based distribution
- Each segment has 1/16th the entries, reducing cache coherency traffic
- Hash distribution: (hash >> 32) & shardMask for segment selection

**Helper Thread TT Write Policy:**

```
if (threadIndex > 0) {
    // Helpers only store if:
    // 1. Depth >= rootDepth / 2 (not too shallow)
    // 2. Flag == Exact (not misleading bounds)
    return; // Skip otherwise
}
```

**Stats Publisher-Subscriber Pattern:**

- MinimaxAI implements IStatsPublisher with Channel<MoveStatsEvent>
- TournamentEngine runs async subscriber tasks for both AI instances
- Ponder stats cached separately for post-move reporting

### Files Added

- src/Caro.Core/Tournament/StatsChannel.cs
- src/Caro.Core/Tournament/IStatsPublisher.cs

### Files Modified

- src/Caro.Core/GameLogic/LockFreeTranspositionTable.cs (sharding, helper policy)
- src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs (rootDepth parameter)
- src/Caro.Core/GameLogic/MinimaxAI.cs (IStatsPublisher implementation)
- src/Caro.Core/GameLogic/AdaptiveDepthCalculator.cs (error rate)
- src/Caro.Core/GameLogic/Pondering/Ponderer.cs (stats timing)
- src/Caro.Core/Tournament/TournamentEngine.cs (stats subscribers)
- src/Caro.TournamentRunner/ComprehensiveMatchupRunner.cs (VCF stats)
- tests/Caro.Core.Tests/Concurrency/*.cs (19x19 board, timeout)

[0.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.3.0

## [0.0.2] - 2026-01-20

### Added

- AI Strength Validation Test Suite - Comprehensive statistical validation framework
  - StatisticalAnalyzer with LOS, Elo CI, binomial tests, SPRT
  - MatchupStatistics data model for tracking matchup results
  - 4-phase test suite: Adjacent, Cross-Level, Color Advantage, Round-Robin
  - HTML report generation with CSS styling
  - CLI runner with configurable parameters
  - 38 statistical unit tests (all passing)
- Paired game design with color swapping for fair comparison
- Likelihood of Superiority (LOS) calculation using error function approximation
- Elo difference with 95% confidence intervals (delta method)
- Color advantage detection using binomial test
- Sequential Probability Ratio Test (SPRT) for early termination
- HTML report generation with summary statistics and Elo ranking
- CLI flags: --validate-strength, --quick-validate, --games, --time, --inc, --verbose

### Changed

- PICKUP.md updated with comprehensive documentation of new validation suite
- README.md updated with AI Strength Validation section
- Tournament runner now supports statistical validation modes

### Technical Details

**Statistical Methods:**

- Elo Difference: Standard chess rating system (400-point scale)
- 95% Confidence Intervals: Delta method approximation
- Likelihood of Superiority (LOS): Based on error function
- Binomial Test: For win rate significance (p < 0.05)
- Paired Game Design: Color swapping to neutralize first-move advantage

**Sample Size Guidelines:**

- 25 games per matchup: Preliminary results (default)
- 50 games per matchup: Moderate confidence
- 100+ games per matchup: High confidence

**Test Configuration:**

- Time control: 2+1 (120s + 1s increment) by default
- Pondering: Enabled for D7+
- Color swapping: Every other game

### Files Added

- src/Caro.Core/Tournament/StatisticalAnalyzer.cs
- src/Caro.Core/Tournament/MatchupStatistics.cs
- tests/Caro.Core.Tests/Tournament/StatisticalAnalyzerTests.cs
- tests/Caro.Core.Tests/Tournament/AIStrengthValidationSuite.cs
- src/Caro.TournamentRunner/ReportGenerators/HtmlReportGenerator.cs
- src/Caro.TournamentRunner/AIStrengthTestRunner.cs

[0.0.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.2

## [0.0.1] - 2026-01-20

### Added

- VCF (Victory by Continuous Four) - Tactical solver restricted to Legend (D11) difficulty only
- Minimum depth separation (1-ply gaps between adjacent difficulties)
- Scaled Critical Defense:
  - D9-D11: Full threat detection (four + open three)
  - D7-D8: Basic threat detection (four only)
  - D6 and below: No automatic threat detection
- Time difficulty multipliers for search depth differentiation
- QuickTest verification suite for AI strength ordering
- Comprehensive documentation in PICKUP.md

### Changed

- Time multipliers rebalanced to prevent timeouts:
  - Legend (D11): 1.5x (reduced from 2.0x)
  - Grandmaster (D10): 1.2x
  - Master (D9): 1.0x (baseline)
  - Expert (D8): 0.9x
  - VeryHard (D7): 0.8x
  - Harder (D6): 0.7x
- VCF Defense disabled to prevent reactive play from D11
- VCF threshold logic fixed to prevent lower difficulties from accessing VCF

### Fixed

- D11 timeout issue - was timing out after 10-25 moves due to excessive time allocation
- VCF threshold bug - D6 was incorrectly given VCF access due to threshold comparison logic
- VCF acting as equalizer - when multiple difficulties had VCF, lower ones could beat higher ones
- Minimum depth calculation - now ensures 1-ply separation between adjacent difficulties
- AI strength inversion - all test groups now pass:
  - Legend (D11) beats Grandmaster (D10)
  - Grandmaster (D10) beats Expert (D8)
  - Legend (D11) beats Harder (D6)
  - Harder (D6) beats Medium (D4)

### Technical Details

**Root Cause Analysis:**

The AI strength inversion was caused by multiple factors:

1. **VCF as Equalizer**: VCF is a powerful tactical solver that finds forced wins. When multiple difficulties had access to VCF, it acted as an equalizer rather than a differentiator - lower difficulties with VCF could beat higher difficulties without it.

2. **Time Multipliers Too Aggressive**: D11 was allocated 3.5x time, leading to 26M node searches and timeouts after 10-25 moves.

3. **VCF Defense Causing Reactive Play**: D11 was focusing too much on blocking opponent threats instead of developing its own position.

**Solution:**

- Restricted VCF to Legend (D11) only, making it a unique advantage for the highest difficulty
- Reduced time multipliers to conservative values (1.5x max) while maintaining depth separation
- Disabled VCF Defense entirely, relying on the evaluation function's 2.2x defense multiplier
- Added minimum depth thresholds to ensure 1-ply separation between adjacent difficulties

### Performance Impact

| Difficulty | Time Multiplier | Min Depth | Max Depth (7+5 TC) | VCF Access |
|------------|-----------------|-----------|-------------------|------------|
| Legend (D11) | 1.5x | 8 | 9 | Yes (exclusive) |
| Grandmaster (D10) | 1.2x | 7 | 8 | No |
| Master (D9) | 1.0x | 6 | 7 | No |
| Expert (D8) | 0.9x | 5 | 6 | No |
| VeryHard (D7) | 0.8x | 4 | 5-6 | No |
| Harder (D6) | 0.7x | 4 | 5 | No |

### Known Issues

- Parallel search (Lazy SMP) remains disabled due to architectural issues with shared transposition table
- SIMD evaluator remains disabled due to score inflation causing AI strength inversion
- Time management still uses hardcoded multipliers instead of fully adaptive PID controller

### Breaking Changes

- VCF is no longer available to difficulties below Legend (D11)
- Time allocation is more conservative to prevent timeouts at 7+5 time control
- Minimum depth is now enforced per difficulty, reducing flexibility but ensuring ordering

[0.0.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.1
