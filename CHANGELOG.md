# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- Editing guideline: Keep entries concise. One-line summaries per change. No test counts, no performance tables, no documentation-only sub-sections. -->

## [2.4.1] - 2026-04-01

### Changed
- Test magic numbers replaced with named constants referencing centralized config (`GameConstants`, `EvaluationConstants`, `MoveOrderingConstants`, `SearchConstants`, `GameConfig`) across 17 backend + 3 frontend test files
- Board boundary assertions: `< 15` replaced with `< GameConstants.BoardSize` across 5 files (39 occurrences)
- Evaluation score thresholds, ELO defaults, TT size defaults now derive from single source of truth

### Fixed
- Boundary bug: position 15 is valid on 16x16 board but was excluded by `< 15` checks

## [2.4.0] - 2026-03-31

### Fixed
- Frontend board size mismatch: config (32x32), grid (15x15), and UCI coordinates now match backend 16x16
- Landing page rules updated with correct time controls (Bullet/Blitz/Rapid/Classical)

### Added
- LockFreeTranspositionTable tests: concurrent read/write, shard distribution, depth-age replacement, ABA prevention
- SIMDBitBoardEvaluator tests: pattern evaluation, defense multiplier, symmetry, batch evaluation, hardware detection
- TacticalEvaluator tests: tactical patterns, emergency defense, critical moves, null-move safety, futility pruning
- SearchHeuristics tests: killer moves, history tables, butterfly tables
- AdaptiveTimeManager tests: phase detection, sudden death, time scramble, PID multiplier adjustment
- UCI protocol test expansion: case-insensitive commands, whitespace handling, sequential flow, setoption edge cases
- Frontend haptics unit tests: valid/invalid vibration patterns
- Frontend UCI coordinate round-trip tests: all 256 positions, boundary values

### Changed
- README: documented game modes, time controls, ELO rating system, UX features table

## [2.3.1] - 2026-03-31

### Added
- UCI protocol tests: engine options, go parameters, position converter, protocol commands
- VCFSolver full test coverage
- Pattern4Evaluator classification tests (52 tests, all 13 CaroPattern4 enum values)
- Lazy SMP parallel search integration tests

### Fixed
- SignalR dependency removed from frontend; README/ENGINE_FEATURES updated for WebSocket UCI bridge
- Stale ZobristTables reference removed from ZeroAllocationTests

### Changed
- CHANGELOG condensed to concise one-line format

## [2.3.0] - 2026-03-31

### Removed
- Dead code: FastThreatDetector, LineExtractor, DirectionalThreatLUT (hardcoded board size 15, incompatible with 16x16)
- Dead Zobrist hash: BoardTechnicalState.Hash computed but never read; ZobristTables duplicated Domain-layer Zobrist
- BoardTechnicalState wrapper: BoardExtensions now uses Domain-layer Zobrist directly

### Fixed
- MoveOrderer center distance used (16,16) for 32x32 instead of GameConstants.CenterPosition (8) for 16x16
- MoveOrderer magic numbers replaced with named constants
- Async blocking in DI: `.Wait()`/`.Result` replaced with `GetAwaiter().GetResult()`
- Random seeding in parallel search: correlated DateTime ticks replaced with golden-ratio hash
- SPSAOptimizer fallback Random replaced with Random.Shared

## [2.2.2] - 2026-03-31

### Removed
- Matchup test project, tournament test snapshots, tournament nav link, dead NetworkConfig export

### Fixed
- Stale comments referencing TournamentManager, opening book, tournament play
- Documentation sync: removed matchup/tournament/opening-book traces across README, ENGINE_FEATURES, CSHARP_ONBOARDING, test docs

## [2.2.1] - 2026-03-31

### Fixed
- GameService.UndoMoveAsync discarded return value (saved old unchanged state)
- GameState.UndoMove player-switching: kept same player instead of switching to Opponent()
- ParallelThreatAnalyzer missing BrokenFour in GetCriticalThreatMoves
- ENGINE_FEATURES/README stale TT write policy docs
- BoardEvaluator dead constants duplicating centralized EvaluationConstants
- MinimaxAI stale TimeCheckInterval comment

## [2.2.0] - 2026-03-30

### Removed
- Variable strength engine config: AdaptiveDepthCalculator, ContestManager, BinaryBookFormat, difficulty-dependent code paths. Engine is singular, all optimizations always on.

### Changed
- All engine constants consolidated into `Caro.Core.Domain/Configuration/` (SearchConstants, EvaluationConstants, MoveOrderingConstants, PruningConstants)

## [2.1.1] - 2026-03-30

### Fixed
- API board serialization off-by-one: `Range(0, 15)` → `Range(0, 16)` for 16x16 board
- Position/Board stale comments referencing 32x32 instead of 16x16
- ThreadPoolConfig thread count minimum: `Math.Max(1,...)` → `Math.Max(5,...)`

## [2.1.0] - 2026-03-30

### Changed
- Engine source decomposition: extracted MinimaxAI/ParallelMinimaxSearch into 8 modules under `GameLogic/Search/`

### Added
- Search modules: TacticalEvaluator, CandidateGenerator, SearchHeuristics, MoveOrderer, QuickWinChecker, TimeBudgetCalculator, ParallelThreatAnalyzer, ParallelNodeEvaluator
- Configuration: SearchConstants.cs, PruningConstants.cs

## [2.0.0] - 2026-03-30

### Removed
- Multi-difficulty bot system: AIDifficulty enum, AIDifficultyConfig, difficulty-specific parameters. Engine runs at full strength only.
- Tournament mode: TournamentRunner, TournamentEngine, MatchScheduler, TournamentManager, TournamentHub (SignalR), tournament API endpoints
- UCI Skill Level option, difficulty-dependent search radius (now fixed at 7)

## [1.83.0] - 2026-03-29

### Fixed
- Board-full draw detection: game loop now checks `board.IsFull()` before requesting AI move, preventing illegal-move forfeit on full boards

## [1.82.0] - 2026-03-29

### Changed
- Difficulty-dependent search radius: scales Braindead=3 through Grandmaster=7

### Fixed
- TT depth inflation in pondering: added MaxSearchDepth=50 cap and zero-nodes guard
- ParallelMinimaxSearch entry points converted to zero-allocation SearchBoard

## [1.81.0] - 2026-03-28

### Added
- Three-tier matchup testing framework: Failsafe, Smoke, Integration

## [1.80.0] - 2026-03-28

### Fixed
- 10+ test fixes: tuple deconstruction, center coordinate assertions, Zobrist cross-compare, CheckWinner reflection ambiguity, DFPN solver assertions, LMR early-game radius, concurrency depth assertion, matchup snapshots

### Removed
- Dead skip attributes: SlowFact, SlowTheory, DebugFact, StressFact (zero usages)

## [1.79.0] - 2026-03-27

### Fixed
- README architecture diagram arrows, CSHARP_ONBOARDING test counts, ENGINE_FEATURES difficulty mapping

## [1.78.0] - 2026-03-27

### Added
- WinDetector.CheckWinFromMove: static method extracting Caro win logic into Core game logic layer
- AIService pondering wired through MinimaxAI subsystem

### Changed
- GameService replaced private CheckForWin/IsBlocked/BuildWinningLine (~140 lines) with WinDetector.CheckWinFromMove

### Removed
- StatelessSearchEngine and tests (redundant after AIService unified on MinimaxAI in v1.77.0)

## [1.77.0] - 2026-03-27

### Fixed
- UCI Open Rule moveNumber hardcoded to 0
- UCI increment parsed but never forwarded to MinimaxAI
- UCI search score hardcoded to 0
- UCI Hash option stored but never applied
- UCI version mismatch (1.61.0 vs 1.30.0)
- UCI Threads used as boolean instead of actual count
- UCI go depth/nodes/movetime parsed but never forwarded
- WebSocket handler brace mismatch

### Added
- MinimaxAI parameters: increment, threadCount, maxDepth, maxNodes, maxTimeMs
- ResizeTranspositionTable for runtime TT resize
- UCI info enrichment: NPS, TT hit rate, real eval score

### Changed
- AIService unified on MinimaxAI (replaced StatelessSearchEngine dependency)

### Removed
- Dead IUCIProtocolHandler interface and related types

## [1.76.0] - 2026-03-27

### Removed
- Opening book system removed across all layers: BookBuilder, OpeningBook entities, BookServices, SQLite persistence, MinimaxAI integration, UCI option, frontend method, ~130 test files, MoveType.Book/BookValidated

### Fixed
- PositionTests used invalid coordinates for 16x16 board; UCIMoveNotation comments referenced 32x32

## [1.75.0] - 2026-03-15

### Added
- `--temperature` CLI option for configurable self-play sampling with smooth decay

### Changed
- SelfPlayGenerator: fixed-tier temperature → smooth decay from ply 8 to 0 at ply 26
- MoveVerifier: added play count histogram logging

## [1.74.5] - 2026-03-11

### Fixed
- MoveVerifier `--min-play-count` ignored at move level (hardcoded 512); now applies correctly

## [1.74.4] - 2026-03-09
### Documentation
- Clarified `--resume` requires re-specifying all CLI options

## [1.74.3] - 2026-03-09
### Documentation
- Moved benchmark results from README to STATS.md

## [1.74.2] - 2026-03-09
### Documentation
- Removed stale test counts from README and backend/tests/README

## [1.74.1] - 2026-03-09

### Added
- `--max-moves` CLI option for configurable moves per position

## [1.74.0] - 2026-03-09

### Added
- Parallel verification with shared TT (Phase 2 uses all CPU cores)
- Zobrist hashing (SplitMix64) replacing simple XOR
- `--min-play-count` CLI option

## [1.73.0] - 2026-03-03

### Changed
- Book builder verification time doubled: 2048ms→4096ms default, 4096ms→8192ms survival zone

## [1.72.2] - 2026-03-03

### Added
- `--resume` flag for interrupted self-play generation

## [1.72.1] - 2026-03-03

### Fixed
- Self-play progress reporting: now prints per-game instead of per-percent
- Self-play move evaluation now respects time budget
- Self-play time allocation: adaptive (5% remaining, 100ms-2000ms) instead of hardcoded 500ms

## [1.72.0] - 2026-03-03

### Added
- Self-play sampling tests for expert report compliance

### Fixed
- SelfPlayGenerator.SampleMove temperature=0 division by zero

## [1.71.0] - 2026-03-02

### Added
- SPSA parameter tuning: IEvaluationParameterProvider, TunableParameters (8 params), `--tune` CLI command
- Parameterized evaluation: BitBoardEvaluator supports custom parameters
- BookBuilder CLI documentation restructure

### Fixed
- FileStagingBookStore thread-safety for concurrent game recording

## [1.70.0] - 2026-03-02

### Changed
- SelfPlayGenerator: game-level SGF storage instead of position-level; TT cleared between games
- MoveVerifier: board reconstruction from games by replaying SGF moves

## [1.69.0] - 2026-03-01

### Added
- Streaming batch processing, variable depth search, InMemoryOpeningBook (40K+ lookups/sec), SelfPlayGenerator

## [1.68.0] - 2026-02-26

### Added
- Baseline benchmark runner (12 matchups, EBF/FMC% metrics), BaselineSummaryRegenerator

## [1.67.0] - 2026-02-25

### Added
- OpeningBookPathResolver: centralized path resolution searching upward to repo root

## [1.66.0] - 2026-02-24

### Fixed
- Grandmaster vs Braindead win rate 40%→80%: added desperate counter-attack logic for losing positions

## [1.65.0] - 2026-02-20

### Fixed
- Time allotment formula: `3x increment` → `(initial_time/20) + (increment*2)`
- ThreatDetector crash on nearly full boards (cell occupancy check before placing stone)

## [1.64.0] - 2026-02-20

### Changed
- Board size reduced from 32x32 to 16x16; BitBoard 16 ulongs→4 ulongs; center (16,16)→(8,8)

### Added
- SearchBoard class: mutable board with make/unmake (115x speedup over immutable Board.PlaceStone)
- MinimaxCore/QuiesceCore SearchBoard-based search methods

## [1.63.0] - 2026-02-20

### Fixed
- Critical open-three blocking bypassed search entirely, causing Grandmaster strength inversion (~40% win vs Braindead). Open three blocks now added to candidates for proper evaluation. Win rate recovered to ~97%.

## [1.62.0] - 2026-02-17

### Added
- Move type tracking: Normal, Book, BookValidated, ImmediateWin, ImmediateBlock, ErrorRate, CenterMove, Emergency
- Improved tournament output format with move type codes

## [1.61.0] - 2026-02-17

### Changed
- Removed artificial depth/speed handicaps (MinDepth, TargetNps); depth purely from time budget and machine capability

## [1.60.0] - 2026-02-16

### Fixed
- Parallel search fallback: broken fallback when no results returned
- Parallel search time management: 2x time usage from Task.WaitAll + fallback
- Immediate win detection and blocking for all difficulty levels

## [1.59.0] - 2026-02-16

### Fixed
- Parallel search iteration time tracking: cumulative→per-iteration, causing premature termination
- Thread allocation updated to scaled formulas per difficulty

## [1.58.0] - 2026-02-15

### Added
- Opening book database: 7,986 positions, 13,035 move recommendations

## [1.57.0] - 2026-02-15

### Removed
- Dead code: ISearchEngine, ITimeManager, BitBoard (Domain duplicate), corresponding tests
- Hardcoded board size 19→GameConstants.BoardSize across all test files
- GameMapper placeholder hash → board.GetHash()

## [1.56.0] - 2026-02-15

### Fixed
- Documentation/code alignment across ENGINE_FEATURES, MinimaxAI, OpeningBook, CSHARP_ONBOARDING, README
- Unified configuration: TimeBudgetDepthManager delegates to AIDifficultyConfig (single source of truth)

### Removed
- Dead AdaptiveDepthCalculator.GetThreadCount() and obsolete tests

## [1.55.0] - 2026-02-15

### Added
- Test infrastructure: BoardBuilder, TestPositions, AdaptiveDepthCalculatorTests, IterativeDeepeningSearchTests

### Removed
- Debug test files: SIMDDebugTest, SimdDebugTest2, ThreatDetectorDebugTest, AdaptiveLMRTests

## [1.54.0] - 2026-02-15

### Added
- Frontend test coverage: gameStore tests, boardUtils tests

## [1.53.0] - 2026-02-15

### Fixed
- Win detection: GameService used Gomoku rules (5+) instead of Caro rules (exactly 5, overline=no win, both-ends-blocked=no win)

## [1.52.0] - 2026-02-15

### Changed
- Book builder performance: 8 workers, more candidates, relaxed static eval pruning

## [1.51.0] - 2026-02-15

### Changed
- Opening book depth: generation stops at depth 14; tiered difficulty access (Easy=4, Medium=6, Hard=10, GM=14)

## [1.50.0] - 2026-02-15

### Changed
- Book generator TT memoization: ClearSearchState preserves TT between searches for subtree reuse

## [1.49.0] - 2026-02-14

### Fixed
- Opening book hash collision: added DirectHash field for unique board identification with compound key

## [1.48.0] - 2026-02-13

### Changed
- UCI notation: base-26 → grid-based double-letter format (aa through hd)
- Opening book access extended to Easy/Medium; tournament time control 7+5→3+2

## [1.47.0] - 2026-02-13

### Changed
- OpeningBookGenerator code quality: consolidated duplicated overloads (~170 lines eliminated)

### Removed
- Dead code in OpeningBookGenerator: unused constants, methods, records, AtomicBoolean

## [1.46.0] - 2026-02-13

### Added
- Centralized configuration: GameConstants, EvaluationConstants, MoveOrderingConstants; frontend mirror gameConfig.ts
- Counter-Move History, Staged Move Picker, Pattern4 Evaluation, BitKey Pattern System

### Changed
- Board size 19x19→32x32; BitBoard ShiftUp/ShiftDown fixed for 32-bit row boundaries

## [1.44.0] - 2026-02-13

### Changed
- IMPROVEMENT_RESEARCH.md: 1400→450 lines, removed speculative ELO estimates, restructured as technical reference

## [1.43.0] - 2026-02-12

### Fixed
- Flaky SearchLoggerTests: proper async flush before reading log files

## [1.42.0] - 2026-02-12

### Added
- Extended research findings in IMPROVEMENT_RESEARCH.md (Part 9: BitKey, Pattern4, Stockfish Move Picker, NNUE, TT techniques, VCF, contempt)

## [1.41.0] - 2026-02-12

### Fixed
- Opening book IsNearEdge transformation bug causing "position occupied" crash at depth 20
- Added 12 symmetry integrity tests

## [1.40.0] - 2026-02-11

### Fixed
- Opening book symmetry transformation: used wrong symmetry on retrieval, causing "Cell already occupied"

## [1.39.0] - 2026-02-10

### Changed
- Opening book generation: 12-15x speedup (15000ms→1000ms per position, smart candidate pruning)

## [1.38.0] - 2026-02-09

### Added
- Multi-Entry TT (cluster-based, 3 entries/cluster), Continuation History, Evaluation Cache, Adaptive LMR, PID Time Manager, Contest Manager (contempt), SPSA Tuner, Structured Search Logger

## [1.37.0] - 2026-02-09

### Added
- AI improvement research report (IMPROVEMENT_RESEARCH.md): Rapfi, Stockfish 18, Chess Programming Wiki analysis

## [1.36.0] - 2026-02-09
### Changed
- Trailing whitespace cleanup

## [1.35.0] - 2026-02-09

### Added
- Test helpers: MockOpeningBookStore, MockPositionCanonicalizer, OpeningBookEntryBuilder, BookMoveBuilder
- Performance: opening book 70%+ CPU utilization, Server GC enabled, 8GB→1GB memory

## [1.34.0] - 2026-02-08

### Fixed
- SQLite transaction error in batch storage (missing Transaction assignment)
- NullReferenceException during cancellation disposal

### Changed
- OpeningBookGenerator: dedicated Thread worker swarm replacing Parallel.ForEachAsync

## [1.33.0] - 2026-02-07

### Fixed
- Domain/Application test projects updated for current simplified API

## [1.32.0] - 2026-02-07

### Changed
- Slow AI tests moved to IntegrationTests project; default `dotnet test` ~30s

## [1.31.0] - 2026-02-07

### Performance
- MinimaxAI object pooling (8GB→reusable instances), TT struct-based zero-allocation hot path, SQLite batch writes

### Fixed
- Nested parallelism in opening book generation (96 threads→sequential per worker)

### Changed
- Created Caro.Core.IntegrationTests project for slow AI search tests

## [1.30.0] - 2026-02-07

### Changed
- Book builder: hardcoded 4-3-2-1 tapered beam structure; removed misleading --max-depth/--target-depth CLI args

## [1.29.0] - 2026-02-07

### Fixed
- NU1510 warning (built-in Immutable package), Svelte 5 reactivity, flaky AsyncQueue test

## [1.28.0] - 2026-02-07

### Added
- UCI Mock Client for engine vs engine testing

### Fixed
- Time scramble timeout: increment-based allocation when remaining < 3x increment
- Infinite recursion in OpeningBookLookupService.NextRandomInt()

## [1.27.0] - 2026-02-07

### Added
- UCI protocol support: Caro.UCI console app, UCI protocol library, WebSocket bridge, frontend client
- Engine options: Skill Level, Opening Book, Threads, Hash, Ponder

## [1.26.0] - 2026-02-07

### Added
- Tiered opening book continuation system (4-3-2-1 by depth range)

## [1.25.0] - 2026-02-06

### Added
- Four time controls (Bullet/Blitz/Rapid/Classical), unified game creation API, AIvAI frontend mode

## [1.24.0] - 2026-02-06

### Changed
- Fully immutable domain model: Cell readonly record struct, GameState sealed record

## [1.23.0] - 2026-02-06

### Fixed
- AI hardcoded board size 15→board.BoardSize; Board.Clone shallow copy bug

## [1.22.0] - 2026-02-06
### Changed
- Opening book: removed singleton, moved types to Domain.Entities namespace

## [1.21.0] - 2026-02-06
### Added
- Opponent response generation for opening book

## [1.20.0] - 2026-02-05
### Added
- SQLite opening book DI integration, depth-based filtering by difficulty

## [1.19.0] - 2026-02-05
### Fixed
- Opening book depth off-by-one; progress stalled at 99%

## [1.18.0] - 2026-02-05
### Fixed
- Diagnostic logging for corrupted book data

## [1.17.0] - 2026-02-05
### Fixed
- "Cell occupied" during book generation: CloneBoard→Board.Clone()

## [1.16.0] - 2026-02-05
### Changed
- Created Caro.Core.MatchupTests; moved failing/flaky tests from unit tests

## [1.15.0] - 2026-02-05
### Changed
- Increased default book generation depths; enhanced survival zone (plies 6-13)

## [1.14.0] - 2026-02-05
### Added
- `--debug` and `--help` flags for book builder

## [1.13.0] - 2026-02-05
### Fixed
- Book generation stopping at depth 1: candidate selection before Open Rule validation

## [1.12.0] - 2026-02-05
### Changed
- README and CSHARP_ONBOARDING accuracy refresh

## [1.11.0] - 2026-02-04
### Fixed
- Thread oversubscription in book generation; thread safety with shared MinimaxAI

## [1.10.0] - 2026-02-04
### Fixed
- Canonical coordinate storage bug; progress display stuck at 95%

## [1.9.0] - 2026-02-04
### Added
- AsyncQueue progress tracking, resume functionality, configurable max-depth

## [1.8.0] - 2026-02-02
### Added
- Worker pool architecture for book generation (30x throughput); SQLite WAL mode

## [1.7.0] - 2026-02-02
### Added
- Opening book system (SQLite, 8-way symmetry), Caro.BookBuilder CLI tool

## [1.6.0] - 2026-02-01
### Changed
- Move ordering: Hash Move #1 priority for Lazy SMP; VCF architecture redesigned as separate pre-search

## [1.5.0] - 2026-02-01
### Added
- Clean Architecture (Domain, Application, Infrastructure layers); board 15x15→19x19

## [1.4.0] - 2026-02-01
### Changed
- Stateless AI architecture; bot-instance-based TournamentEngine

## [1.3.0] - 2026-02-01
### Added
- Centralized testing framework for AI difficulty validation

## [1.2.0] - 2026-01-31
### Fixed
- TT master thread priority; tournament output path

## [1.1.0] - 2026-01-29
### Added
- Time-budget AI depth system, AIDifficultyConfig, dynamic Open Rule enforcement

## [1.0.0] - 2026-01-29
### Added
- AI tournament system (round-robin, ELO tracking, SQLite + FTS5, SignalR broadcasts)

## Early Development (0.x) - 2026-01-20 to 2026-01-29

- VCF (Victory by Continuous Four) tactical solver
- AI Strength Validation Test Suite with statistical analysis
- Stats publisher-subscriber architecture
- Transposition table sharding (16 segments)
- Time-budget depth system per difficulty
- Pondering and both-pondering support

[2.4.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.4.1
[2.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.4.0
[2.3.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.3.1
[2.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.3.0
[2.2.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.2.2
[2.2.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.2.1
[2.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.2.0
[2.1.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.1.1
[2.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.1.0
[2.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.0.0
[1.83.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.83.0
[1.82.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.82.0
[1.81.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.81.0
[1.80.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.80.0
[1.79.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.79.0
[1.78.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.78.0
[1.77.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.77.0
[1.76.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.76.0
[1.75.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.75.0
[1.74.5]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.5
[1.74.4]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.4
[1.74.3]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.3
[1.74.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.2
[1.74.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.1
[1.74.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.74.0
[1.73.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.73.0
[1.72.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.72.2
[1.72.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.72.1
[1.72.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.72.0
[1.71.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.71.0
[1.70.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.70.0
[1.69.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.69.0
[1.68.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.68.0
[1.67.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.67.0
[1.66.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.66.0
[1.65.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.65.0
[1.64.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.64.0
[1.63.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.63.0
[1.62.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.62.0
[1.61.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.61.0
[1.60.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.60.0
[1.59.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.59.0
[1.58.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.58.0
[1.57.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.57.0
[1.56.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.56.0
[1.55.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.55.0
[1.54.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.54.0
[1.53.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.53.0
[1.52.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.52.0
[1.51.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.51.0
[1.50.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.50.0
[1.49.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.49.0
[1.48.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.48.0
[1.47.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.47.0
[1.46.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.46.0
[1.45.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.45.0
[1.44.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.44.0
[1.43.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.43.0
[1.42.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.42.1
[1.42.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.42.0
[1.41.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.41.0
[1.40.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.40.0
[1.39.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.39.0
[1.38.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.38.0
[1.37.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.37.0
[1.36.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.36.0
[1.35.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.35.0
[1.34.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.34.0
[1.33.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.33.0
[1.32.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.32.0
[1.31.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.31.0
[1.30.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.30.0
[1.29.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.29.0
[1.28.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.28.0
[1.27.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.27.0
[1.26.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.26.0
[1.25.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.25.0
[1.24.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.24.0
[1.23.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.23.0
[1.22.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.22.0
[1.21.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.21.0
[1.20.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.20.0
[1.19.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.19.0
[1.18.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.18.0
[1.17.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.17.0
[1.16.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.16.0
[1.15.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.15.0
[1.14.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.14.0
[1.13.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.13.0
[1.12.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.12.0
[1.11.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.11.0
[1.10.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.10.0
[1.9.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.9.0
[1.8.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.8.0
[1.7.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.7.0
[1.6.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.6.0
[1.5.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.5.0
[1.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.4.0
[1.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.3.0
[1.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.2.0
[1.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.1.0
[1.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v1.0.0
[0.4.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.4.2
[0.0.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.1
