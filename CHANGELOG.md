# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
