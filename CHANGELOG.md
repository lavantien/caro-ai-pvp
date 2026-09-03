# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- Editing guideline: Keep entries concise. One-line summaries per change. No test counts, no performance tables, no documentation-only sub-sections. -->

## [9.4.2] - 2026-09-03

### Added
- VCF solver observability: node count and forced-chain length surface in `SearchStats`, the `ai-move` response (`vcfDepth`/`vcfNodes`), and `matches.db` (`vcf_depth`/`vcf_nodes` columns)
- Ponder probe fields in the `ai-move` response: `ponderDepth`, `ponderNodes`, and `ponderHit` (null when no ponder ran, false when it ran and missed); statline bytes unchanged
- Round-robin benchmark runner `scripts/run-round-robin.mjs`: fixed 12-pairing schedule (1v1 fail-fast first, 5v5 calibration last), sequential games with seeded openings, per-run artifacts (`run.log`/`summary.json`/`report.md` committed under `docs/artifacts/tournaments/`, `matches.db` local), ladder with adjacent-level monotonicity verdict, incremental atomic summary rewrites, abort-on-error with partial evidence kept
- STATS.md round-robin section: commands, artifact layout, fail-fast contract, anomaly policy, determinism statement

### Fixed
- `move-statline` log lines reach backend stdout in the real host (`GameHandlers` now takes `ILogger<GameHandlers>`; the untyped `ILogger` never resolved from DI)
- `startBackend` kills the port before building so a running server cannot lock its dlls

[9.4.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.4.2

## [9.4.1] - 2026-09-03

### Fixed
- Frontend move notation displays the documented algebraic format (column letter + row number, e.g. `1.h8`) via a new `toAlgebraic` helper; double-letter stays the UCI wire format only
- E2E per-test game cleanup derives its URL from `ApiConfig` instead of a hardcoded `http://localhost:5207`

### Changed
- Docs audited against the implementation: parity and strength metrics removed from prose (go-baseline README is a historical record, onboarding no longer asserts probe outcomes) and DEVELOPMENT.md gains a "measurements live in artifacts, not docs" rule
- README realigned: frontend config is the single `src/lib/config/index.ts`, the e2e section states the backend prerequisite, STATS.md joined the documentation guide, the env table documents `CARO_WEBSERVER_TIMEOUT_MS`, GO_ONBOARDING carries an archive banner, DEVELOPMENT's mirror example names the Playwright config
- `uci-probe`'s header describes its modes correctly (depth-bound searches, no movetime run) and drops the stale score-cp gate; the `ghostStoneScale` comment states its real use as the ghost-stone offset fraction
- Housekeeping: `frontend/test-results.json` untracked again (its 6.3.0 removal had regressed), frontend package version realigned to the release, dead .gitignore entries for removed backend tooling pruned, orphan `.claude/worktrees/difficulty-levels` skeleton deleted

[9.4.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.4.1

## [9.4.0] - 2026-09-03

### Fixed
- The canonical time-control table contains the default `7+5` entry (seeded from the `TimeControl` defaults), so overriding `Caro:TimeControl:Default` no longer decouples the frontend's `7+5` selection from a 7+5 clock
- Playwright's base URL honors `FRONTEND_PORT` as its config comment promised; the default port and the web-server timeout come from `scripts/lib.mjs` (`CARO_WEBSERVER_TIMEOUT_MS`)

### Changed
- Residual hardcoded values centralized: `Constants.Time.MsPerSecond` replaces eleven bare ms/second conversions, a new `Caro.Domain` `EndReasons` contract class covers the end-reason strings, the difficulty error message and the no-difficulty search defaults derive from central tables
- Scripts share an `ENDPOINTS` block instead of re-inlining API paths; the e2e spec reads the game-id hook key and winning-line color from config; `verify-screenshot` reuses `SCREENSHOT` timeouts; port-5207 definitions cross-reference each other like the heap-limit mirror
- Dead config exports removed (`UIConfig.timerSyncIntervalMs`, `E2EConfig.regressionMoveWaitMs`); the UCI default clock derives from the 3+2 table entry; the result banner animation duration moved to `UIConfig`; the landing page interpolates the configured win length; dangling `app.html` favicon reference removed
- New DEVELOPMENT.md codifying value discipline: every introduced number or string goes through the central hub and is wired to consumers, derivations over restatements, string contracts get constants classes, logic scales by clock/host/board ratios instead of absolute values, mirrors carry cross-reference comments; README documentation guide links it

[9.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.4.0

## [9.3.0] - 2026-09-03

### Added
- Startup configuration: `CaroConfig` binds the appsettings `Caro` section (or `Caro__*` environment variables), validates once at boot with the offending key named, and overrides the compiled defaults for game limits, abandoned-game window, TT sizes, opening spread, the time-control table, per-level difficulty profiles, UCI option bounds, and the eleven time-management knobs
- README section documenting runtime configuration, the overridable-vs-compile-time boundary, and the three-way mirror of the time-control and difficulty tables (backend tables, frontend config index, `scripts/lib.mjs`)

### Changed
- All business-logic constants centralized in `Caro.Domain`: `Constants.cs` holds nineteen nested scalar groups, `Constants.Tables.cs` holds the shared direction table, the L1-L5 difficulty ladder, and the canonical time-control table; nothing but sanctioned infra lives elsewhere
- Previously unnamed inline values now carry names: aspiration widen factor, LMR base and deep reductions, worker start-depth stagger, quiet-ordering center scale, and the twos and tactical-move thresholds
- `Difficulty.GetDifficultyProfile` is table-driven with a `ProfileThreads` host-adaptation enum replacing per-case thread math; UCI handshake and `setoption` validation share the configured bounds
- Frontend's nine config modules merged into a single `src/lib/config/index.ts` (value-domain unions defined there); scripts share `scripts/lib.mjs` with environment overrides for ports, URLs, and probe settings
- Three pre-existing tautological alias assertions removed (their subjects were already const aliases); the surviving cross-constant invariants are unchanged

[9.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.3.0

## [9.2.0] - 2026-09-02

### Fixed
- Evaluation combination cascade ranks block4+flex3 (13,000) above double flex3 (12,000) per the documented highest-matching-category rule; the Go engine carried the same inverted order, so this is a deliberate post-port conformance fix

### Changed
- Dead `vcf-block` statline tag removed; no producer ever existed in the Go engine or the port
- Unused constants `MaxVCFCacheEntries` and `TimeCheckInterval` removed; `HeapHardLimitBytes` annotated with its runtimeconfig mirror
- Engine, API, and onboarding docs realigned with the C# code: per-shard monitor TT locks, actual MovePicker additive scoring, L3/L4 depth caps with the VCF depth ladder, Go-era residue removed, singular API routes, UCI claim scoped to the implemented subset
- Frontend `EvaluationConfig` (unused, mirrored nothing) removed

[9.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.2.0

## [9.1.0] - 2026-09-01

### Fixed
- WebSocket UCI bridge: a non-final frame fragment that exactly fills the 4096-byte read buffer no longer leaves a zero-capacity receive that hangs the connection
- WebSocket reply writer survives transports that raise a bare `IOException` (instead of `WebSocketException`) when the peer vanishes mid-send
- Local-origin check accepts bracketed IPv6 loopback origins (`http://[::1]`) again, matching the Go server

### Changed
- Backend coverage badge union-merges per-line hits across test projects (the previous per-report average double-counted uncovered lines) and excludes generated sources

[9.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.1.0

## [9.0.0] - 2026-09-01

### Changed
- Backend rewritten from Go 1.26 to C# 14 / .NET 10 (solution layout `Caro.Domain` through `Caro.Server`); the engine is a line-for-line transliteration of the Go code, all 131 Go-era engine changes included
- SQLite access moved from `mattn/go-sqlite3` (CGO) to `Microsoft.Data.Sqlite` (managed); schema, WAL mode, and idempotent startup migration unchanged, existing `matches.db` files keep working
- Lazy SMP workers, ponder search, and the UCI search task run as long-running tasks with `CancellationToken` cancellation instead of goroutines with `context.Context`
- Scripts (`dev.mjs`, `run-tournament.mjs`, `capture-screenshot.mjs`) and the Makefiles build and run the backend through `dotnet`
- 2GB memory cap via runtimeconfig GC hard limit (replaces `debug.SetMemoryLimit`)

### Added
- `CSHARP_ONBOARDING.md` for the .NET 10 codebase; `GO_ONBOARDING.md` kept as an archive of the Go era
- `scripts/uci-probe.mjs` for deterministic fixed-position engine probes and speed measurement

### Removed
- Go toolchain, CGO requirement, gorilla/websocket, testify

[9.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v9.0.0

## [8.1.0] - 2026-08-21

### Added
- Grandmaster (L5) pondering: background search on the predicted opponent reply during the opponent's turn, capped by the opponent's remaining clock (scales with the time control)
- Ponder outcomes recorded for stats: `[PONDER]` statline tag on hit positions, `ponder_depth`/`ponder_nodes` persisted whenever a ponder preceded the move; the move itself always comes from the full search over the warmed TT
- Quiescence treats double threats (four-or-open-three in two directions) as forcing moves
- `CARO_DISABLE_PONDER=1` kill switch disables pondering process-wide

### Changed
- `moves.move_type` records the real search move type (`vcf`, `timeout-fallback`, `ponder-hit`) instead of a hardcoded `exact`

[8.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v8.1.0

## [8.0.0] - 2026-05-13

### Changed
- VCF solver returns tri-state result: `VCFWin`, `VCFNoWin`, `VCFTimeout` (was boolean)
- VCF-BLOCK converted from short-circuit to preferred move hint: block square passed to alpha-beta as first move to search, full search still runs
- VCF-BLOCK only trusted on `VCFNoWin` verification; `VCFTimeout` falls through to normal alpha-beta
- Engine constants centralized in `domain/constants.go`: time management (phase divisors, soft/hard bounds, buffer), TT shard count, VCF search depth
- TT depth protection: same-hash entries always keep the deeper result; shallow entries cannot overwrite deep ones
- Null move uses dedicated Zobrist key (`zobristNullMoveKey`) to prevent TT hash collision with parent position
- Tournament results: L5 vs L1 at 3+0 achieved 87.5% win rate (35/40 across two tournaments)

### Fixed
- Null-move TT poisoning: child node after null move no longer shares hash with parent, preventing opponent score corruption
- TT depth trampling: depth-2 leaf can no longer destroy depth-6 entry at same hash
- VCF-BLOCK fail-open on timeout: no longer short-circuits when block verification times out
- ENGINE_FEATURES.md: VCF integration section, centralized constants table, TT depth protection, null-move Zobrist key

[8.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v8.0.0

## [7.0.0] - 2026-05-12

### Changed
- Evaluation rewritten as strictly zero-sum: `playerScore - opponentScore` + center bonus delta
- Removed asymmetric defense multiplier (was 1.5x) that corrupted alpha-beta bounds
- Move ordering: WINNING_MOVE now precedes MUST_BLOCK (win-first before defense)
- Null-move pruning tightened: depth>=4 (was >=3), reduction=2 (was 3)
- VCF solver candidate radius reduced from 7 to 2 (fours are always adjacent)

### Fixed
- VCF solver skips search when opponent has immediate win (flex4 or double block4)
- VCF `findFourBlocks` validates overline beyond block squares
- Futility pruning removed (dropped tactical winning moves at shallow depths)
- ENGINE_FEATURES.md: move ordering priorities, defense multiplier references, SeqLock→RWMutex
- README.md: move ordering stage sequence

## [6.9.0] - 2026-05-12

### Changed
- Parallel search rewritten: shared TT Lazy SMP with per-shard `sync.RWMutex` (replaced per-worker isolated TTs)
- Aspiration window widened from 50 to 1500 with proper multi-attempt loop in parallel search
- TT storage replaced SeqLock with `sync.RWMutex` for race-detector compatibility
- Statline extended: added `thr` (thread count), `alloc` (allocated time), and `[VCF]` tag per move

### Fixed
- Score hierarchy harmonized: `Infinity (100k) > WinScore (30k) > MaxEval (25k) > fiveScore (WinScore)`
- Mate-distance scoring: `WinScore - plyFromRoot` with TT ply adjustment for correct mate ordering
- TT poisoning on search abort: TT stores skipped when `monitor.ShouldStop()` is true
- `createsOpenFour` corrected from `||` (one-end) to `&&` (both-ends) for true open fours
- VCF solver: opponent counter-win check added after opponent block move
- Open Rule enforced inside alpha-beta search tree
- `bestScore` initialization changed from `-WinScore*2` to `-Infinity` to prevent ghost scores
- Go vet warnings: unused error returns in test files

## [6.8.0] - 2026-05-11

### Changed
- Default TT size increased from 64MB to 1GB for deeper search in bot vs bot matches
- UCI Hash option default updated to 1024 MB (was 64 MB)

### Fixed
- ENGINE_FEATURES.md: Threads option default corrected from dynamic formula to actual UCI default (4); clarified L4/L5 auto-scaling
- GO_ONBOARDING.md: engine constructor example updated to match actual `NewMinimaxAI(logger, threads)` signature
- README.md: UCI example session Hash default updated to match implementation
- .gitignore: added `backend/coverage/` to exclude coverage badge artifacts

## [6.7.0] - 2026-05-11

### Added
- Pre-formatted statline in AI move API response (`lastMove.statline`): depth, nodes, NPS, TT hit rate, score, think time per move
- `lastMove` object in `/api/game/{id}/ai-move` response with move detail (position, player, engine stats, timing)
- Structured statline logging to backend stdout via `log/slog`
- SQLite migration for 5 future engine stat columns (master_pct, slave_depth, slave_nodes, ponder_depth, ponder_nodes)
- Per-move statline output in `simulate-match.mjs` and `run-tournament.mjs` scripts
- `e2e.txt` and `tournament.txt` tracked in version control for reproducible match logs
- Handler unit tests: formatStatlineNodes, formatStatlineNPS, buildMoveDetail, MakeAIMove returns lastMove
- Persistence unit tests: migration adds columns, record future stats, migration from old schema

### Changed
- `NewHandler` accepts logger parameter for structured statline logging
- STATS.md populated with actual L1 vs L5 benchmark results and per-level stat ranges

## [6.6.0] - 2026-05-10

### Added
- Frontend UCIEngine unit tests: MockWebSocket with response queue, 23 test cases covering connect/disconnect, command lifecycle, best-move resolution, timeout rejection, error handling
- Frontend game type tests: switchPlayer and difficultyName pure function validation (8 cases)
- Frontend API config tests: endpoint function coverage and wsBaseUrl derivation (8 cases)
- Frontend GameStore UCI integration tests: connect/disconnect, AI move flow, out-of-bounds handling, auto-connect on enable (11 cases)

### Changed
- Frontend unit test coverage: 45.24% -> 96.83% lines (100/221 -> 214/221)
- Frontend coverage badge: red -> blue

## [6.5.0] - 2026-05-10

### Added
- Pattern4 classification system (pattern4.go): 4-direction combined threat detection with Flex/Block/Broken pattern categories
- VCF solver (vcf.go): Victory by Continuous Fours pre-search running before alpha-beta (20% of allocated time)
- Staged move picker: 7-stage generation (TT -> Block -> Win -> Threat -> Killer/Counter -> Quiet) with early cutoff
- Tactical candidate generation: quiescence search filters to forcing moves only (open fours, open threes, blocks)
- Aspiration windows: narrowed bounds near root with incremental widening (3 attempts, delta doubling)
- Counter-move history: opponent-response pattern tracking in heuristics
- Continuation history: 6-ply move pair scoring in heuristics
- Depth-age replacement in transposition table: priority formula depth - 8*age
- Static eval cache in TT entries for futility pruning and correction
- ~30 new tests across 9 files for coverage improvement (89.6% → 91.6% with -race)

### Changed
- Evaluation rewritten using Pattern4-based scoring (replaced simple scoreTable)
- Defense multiplier 1.5x applied to opponent threats via Pattern4 player evaluation
- Move picker upgraded from flat scoring to staged generation with must-block and threat-create stages

### Fixed
- Frontend: AI vs AI mode sends redDifficulty/blueDifficulty explicitly instead of relying on backend fallback
- Documentation: SIMD evaluation and pondering marked as planned; TT section corrected to match sharded SeqLock implementation

[6.9.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.9.0
[6.8.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.8.0
[6.7.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.7.0
[6.6.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.6.0
[6.5.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.5.0

## [6.4.0] - 2026-05-10

### Fixed
- Server exits immediately on port bind failure instead of blocking forever as zombie process
- Scripts build binary first (`go build -o`) and spawn directly with `shell: false`, eliminating unreliable `cmd.exe → go.exe → server.exe` process tree cleanup on Windows
- Pre-startup port cleanup (`killPort`) kills stale processes on port 5207 before spawning new server
- TT `Dispose()` releases 64 MB backing arrays immediately for GC; separate from `Clear()` which zeroes for reuse
- UCI `ucinewgame` disposes old AI before creating new one (was leaking 64 MB TT per call)
- UCI `stop` command cancels ongoing search via `context.CancelFunc`
- `simulate-match.mjs` deletes game from server after completion

### Changed
- All scripts (`dev.mjs`, `run-tournament.mjs`, `capture-screenshot.mjs`) unified on build-then-spawn pattern with `shell: false`

[6.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.4.0

## [6.3.0] - 2026-04-30

### Added
- Root Makefile with `backend-coverage`, `frontend-coverage`, and `coverage` targets
- Coverage badges in README (shields.io endpoint badges reading from `coverage/*.json`)
- `scripts/coverage-badge.sh` helper to generate shields.io endpoint JSON
- `@vitest/coverage-v8` dev dependency for frontend coverage reporting
- `json-summary` reporter in vitest config for total coverage extraction

### Changed
- Frontend package version aligned to project version (6.3.0)
- Removed unnecessary `-tags "sqlite_fts5"` from test command (FTS5 is default in go-sqlite3)

### Removed
- Stale `nul` and `test_tt.cs` temp files from root
- `frontend/test-results.json` from version control (added to .gitignore)

[6.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.3.0

## [6.2.0] - 2026-04-29

### Added
- Engine search stats: nodes searched, NPS, depth achieved, TT hit rate collected via atomic counters
- Structured match persistence: games and moves tables in SQLite with per-move engine stats
- TT hit rate for parallel search: (mean + median) / 2 of all workers' local TT hit rates

### Changed
- Parallel search: all-equal goroutines with local TTs (no master/helper split)
- Persistence: generic event log replaced with structured MatchStore (games + moves tables)

[6.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.2.0

## [6.1.0] - 2026-04-29

### Added
- Board coordinate labels: column headers (a-p) above and below grid, row numbers (1-16) left and right
- Open Rule visual highlight: dimmed overlay on invalid cells during Red's 2nd move
- Bot difficulty labels: AI level shown in timer strips (e.g. "AI (Grandmaster)")
- Display notation: move history uses simple algebraic format (column letter + row number, e.g. i9)
- DOM verification script (scripts/verify-screenshot.mjs) for automated screenshot testing

### Fixed
- Open Rule: corrected from Manhattan distance to Chebyshev distance (5x5 zone exclusion, max(|dx|,|dy|) >= 3)
- Frontend notation: replaced UCI double-letter format (aa-dd) with simple algebraic (a-p, 1-16)

[6.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.1.0

## [6.0.0] - 2026-04-29

### Changed
- Complete backend port from C# (.NET 9 / ASP.NET Core) to Go 1.26
- Domain layer: immutable Board/GameState/Player types with value semantics, [4]uint64 BitBoard, Zobrist hashing (SplitMix64)
- Engine: MinimaxAI with iterative deepening, PVS, alpha-beta pruning, quiescence search, adaptive LMR
- Engine: Lazy SMP parallel search with channel-based goroutine pool, sharded SeqLock transposition table (16 segments, atomic version counters)
- Engine: staged MovePicker (7 stages), killer moves, continuation/butterfly history, VCF solver
- Engine: hardware-agnostic L1-L5 difficulty via DifficultyProfile with time fraction scaling
- Engine: PID time management with aspiration windows and context.Context cancellation
- UCI: full protocol handler with double-letter notation (a-p for row and column), WebSocket bridge
- API: net/http ServeMux with method+pattern matching (Go 1.22+), CORS/logging/recovery middleware
- API: GameSession with sync.Mutex, InMemoryStore with sync.RWMutex, max 4 concurrent games
- API: per-player isolated MinimaxAI instances, REST endpoints (POST/GET/DELETE /api/games)
- Persistence: SQLite + FTS5 via mattn/go-sqlite3, WAL mode, game event logging with full-text search
- Server: graceful shutdown via os.Signal, 2GB heap limit (debug.SetMemoryLimit), 5-min cleanup ticker

### Removed
- C# backend (ASP.NET Core, Clean Architecture layers, SignalR, 140+ files, ~26.5K LOC)
- CSHARP_ONBOARDING.md (replaced by GO_ONBOARDING.md)
- Persistent worker pool (replaced by per-search channel-based goroutine dispatch)
- ThreadPoolConfig / MaxEngineThreads singleton (replaced by goroutine count from GOMAXPROCS)

### Added
- GO_ONBOARDING.md: Go 1.26 idioms, project conventions, testing patterns
- 80+ tests across 5 packages (domain 39, engine, uci 6, api 29, persistence 6) with race detector
- FTS5 full-text search via mattn/go-sqlite3 (CGO_ENABLED=1)

[6.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v6.0.0

## [5.5.0] - 2026-04-29

### Changed
- Persistent worker pool replaces Task.Run for Lazy SMP search and pondering: zero thread-startup overhead per move
- Ponderer shares ParallelMinimaxSearch instance (and worker pool) with main search instead of creating its own
- Thread count formula changed from Pow2(N) to Pow2(N/2): leaves half the cores for OS, GC, SignalR
- New centralized `ThreadPoolConfig.MaxEngineThreads` property; all thread count decisions flow from it
- Dynamic thread scaling: concurrent games divide MaxEngineThreads so total usage stays bounded
- Default TT size reduced from 256MB to 64MB for multi-session web hosting (UCI can override via setoption)
- UCI Threads/Hash defaults now auto-detected from ThreadPoolConfig/SearchConstants instead of hardcoded
- CancellationToken from HTTP requests propagated through GetBestMove to search dispatch
- ParallelNodeEvaluator.Evaluate switched from scalar BitBoardEvaluator to SIMD-accelerated evaluator
- DifficultyProfile thread counts capped at MaxEngineThreads for all levels

### Fixed
- SIMDBitBoardEvaluator: `count >= 5` changed to `count == 5` (Caro exactly-5 rule, was counting overlines)
- SIMDBitBoardEvaluator: off-by-one in horizontal/vertical run boundary check
- ParallelMinimaxSearch now disposes worker pool on shutdown (was leaking persistent threads)
- UCIHandler now disposes AI engine on WebSocket close

### Added
- `PersistentWorkerPool`: dedicated threads that wait for search tasks, eliminating OS thread creation per move
- `ParallelMinimaxSearch.Dispose()` for proper resource cleanup
- GC heap hard limit (2GB) to prevent runaway TT allocations
- Max concurrent games limit (4) with 429 response when exceeded
- Abandoned game eviction (30-minute inactivity timeout)
- Graceful shutdown: disposes all remaining game sessions and compacts LOH
- `GameConstants.CardinalDirections` constant replaces inline direction arrays across all evaluators
- `InMemoryGameStore.CleanupAll()` for batch disposal during shutdown
- `InMemoryGameStore.ActiveGameCount` property for load-aware thread scaling

## [5.4.0] - 2026-04-22

### Fixed
- Memory leak: MinimaxAI now implements IDisposable; Ponderer background tasks and TT memory properly released on game end
- Stats bug: sequential search (L1-L2) reported system thread count instead of configured ThreadCount
- Stats bug: score sentinel values (int.MinValue/int.MaxValue) leaked to diagnostics output

### Added
- DELETE /api/game/{id} endpoint for explicit game cleanup
- Periodic cleanup timer (5-min interval) evicts completed games from memory
- InMemoryGameStore.CleanupCompleted() for batch eviction of finished sessions
- Tournament script deletes games after each match to free AI engine memory

## [5.3.0] - 2026-04-22

### Changed
- Per-player isolated MinimaxAI instances: each player in a game gets its own engine with separate transposition table, heuristics, killer moves, and pondering state (no shared singleton)
- AI instances released on game end to reclaim TT memory

## [5.2.0] - 2026-04-20

### Added
- run-tournament.mjs: self-contained tournament script that builds/starts backend, runs N matches with automatic color swapping, and reports per-game and aggregate statistics

## [5.1.0] - 2026-04-12

### Added
- Hardware-agnostic difficulty levels (L1 Novice through L5 Grandmaster) via DifficultyProfile static helper
- Per-player difficulty: `redDifficulty` and `blueDifficulty` parameters in game creation API
- TimeFraction (0.0-1.0 validated backing field) and UseVCF (bool) on SearchOptions
- UCI Skill Level option: `setoption name Skill Level value N` (1-5)
- simulate-match.mjs script for AI vs AI matches with per-player difficulty via HTTP API
- capture-screenshot.mjs updated to set difficulty slider before starting match

### Changed
- DifficultyProfile maps L1-L5 to search parameters: time fraction, thread count, VCF toggle, pondering, parallel search
- UCISearchController applies DifficultyProfile when Skill Level < 5
- MinimaxAI applies TimeFraction post-PID and respects UseVCF guard on VCF pre-search
- Frontend game types: DifficultyLevel union type, difficultyName helper, per-player difficulty fields

## [5.0.0] - 2026-04-06

### Added
- IGameStore abstraction and InMemoryGameStore (ConcurrentDictionary-backed) for swappable game session storage
- Aspiration windows with incremental widening in both sequential (MinimaxAI) and parallel (Lazy SMP) search paths
- Depth-advantage override in Lazy SMP: helper threads reaching depth+2 or deeper preferred over master thread result
- Span-based zero-allocation candidate generation in MinimaxCore and QuiesceCore (stackalloc 128 entries)
- Quiescence forced-response handling: stand-pat pruning skipped when opponent has forcing threats; non-tactical moves filtered
- Separate qsPly tracking in quiescence search (was conflated with rootDepth)

### Fixed
- Open Rule off-by-one: moveNumber == 3 changed to moveNumber == 2 (MoveNumber counts stones on board, Red's 2nd move = MoveNumber 2)
- UCIHandler DI: changed from Singleton to Transient (each WebSocket connection gets its own AI instance with 64MB TT)
- WebSocket error handling: catch WebSocketException and OperationCanceledException on send/receive paths
- CancellationToken propagation: WebSocket ReceiveAsync/SendAsync now respect context.RequestAborted
- TT entry padding: 20 bytes -> 32 bytes (cache-line fraction alignment, eliminates false sharing)
- Duplicate currentPlayer variable in QuiesceCore (CS0128 compiler error)

### Changed
- SearchWithDepth rewritten: aspiration uses previous iteration score (no pre-search), single-direction widening on fail-high/fail-low
- MinimaxCore candidate generation: List -> Span with stackalloc, converted to List only for move ordering
- QuiesceCore tactical filtering: uses TacticalEvaluator and Pattern4Evaluator to remove non-forcing moves before search
- Lazy SMP result selection: depth-advantage override evaluated before max-depth selection
- Parallel iterative deepening: 3 widening attempts with exponential delta doubling, full-window fallback

## [4.5.0] - 2026-04-05

### Added
- Dev bootstrap script (scripts/dev.mjs): builds backend, starts backend+frontend, opens browser

### Changed
- README: added Scripts section documenting dev.mjs and capture-screenshot.mjs
- .gitignore: added e2e.txt (temp artifact from capture-screenshot.mjs)

### Removed
- Stale checkpoint.md (v2.9.0 snapshot, superseded by CHANGELOG)

## [4.4.0] - 2026-04-05

### Fixed
- Critical: CheckFiveInRow detected 4-in-a-row as wins (4 shift terms instead of 5), causing false forced-win scores from move 4 onward and nonsensical play
- Evaluation scores unclamped: BitBoardEvaluator could return values exceeding WinScore, now clamped to ±20,000 (MaxCorrectedEval)
- Overline scoring: count >= 5 changed to count == 5 in both evaluation paths (Caro exactly-5 rule)
- FiveInRowScore (100,000) collided with WinScore; reduced to 50,000 with MaxCorrectedEval boundary at 20,000
- WinScore reduced from 100,000 to 30,000 to fit TT's 16-bit short storage without overflow
- Mate-distance scoring: terminal wins now scored as WinScore - plyFromRoot instead of flat WinScore
- TT mate-distance adjustment: ScoreToTT/ScoreFromTT convert between root-relative and position-relative mate scores
- Cancellation leak: minimizing nodes returned int.MaxValue on cancellation instead of alpha/beta bounds
- Quiescence search searched all moves (branching explosion); now filters to tactical moves only via IsTacticalMoveInQuiesce
- Forced-win break threshold: WinScore replaced with MaxCorrectedEval in iterative deepening and pondering
- Result validation: SearchLazySMP rejects scores exceeding 2x WinScore (int.MaxValue leak protection)

## [4.3.0] - 2026-04-04

### Changed
- Decompose all GameLogic files exceeding 400 lines into sub-400-line partial class files (zero-risk, compiler-verified refactoring)
- MinimaxAI split into 12 partial files (from ~3,100 lines)
- ParallelMinimaxSearch split into 9 partial files (from ~2,600 lines)
- TranspositionTable and LockFreeTranspositionTable each split into main + IO partials
- Updated ENGINE_FEATURES.md source layout to reflect new file structure

## [4.2.0] - 2026-04-03

### Added
- AI move endpoint passes time remaining and increment to SearchOptions (was using Default with 5s fallback)
- Backend console logging for game creation and AI move decisions
- Screenshot script: daemon stdout/stderr piped to parent console for visibility
- Screenshot script: browser console error/warning logging
- Screenshot script: Blitz (3+2) time control selection for more interesting games

### Fixed
- AI endpoint time management: engine now uses full time control budget instead of hardcoded 5s soft bound
- Open Rule validator removed from AI endpoint: engine doesn't know the constraint, was rejecting valid AI moves (400 errors)
- Screenshot script: spawnSync + shell:false for Windows process cleanup (async spawn won't complete before exit)
- Screenshot script: fullPage:true for complete board capture

## [4.1.0] - 2026-04-03

### Added
- Backend time tracking: GameSession tracks per-player time with Fisher increment in ExecuteMove
- AIvAI auto-chaining: makeAiMove() chains next move automatically, enabling full autonomous games
- GameResultBanner: top slide-in banner replacing full-screen modal (board stays visible)

### Changed
- Backend: GameSession.ExecuteMove replaces MutateUnderLock with integrated time deduction and increment
- Backend: BuildResponse now instance method with time remaining fields (previously hardcoded 0.0)
- Screenshot script: simplified to UI-driven approach (no API mocking, real browser gameplay)
- GameSettings: "New Game" button always visible (previously hidden until first move)

### Fixed
- Timer display: Math.round() on inactive branch prevents floating-point artifacts (e.g. 8:1.0459999)
- Screenshot script: selector updated to match new GameResultBanner component

### Removed
- GameOverOverlay component (replaced by GameResultBanner)

## [4.0.0] - 2026-04-02

### Changed
- Mobile-first responsive board: dynamic cell sizing via ResizeObserver, board fills viewport width (caps at 1024px)
- Vertical stack layout: opponent timer > board > player timer > move notation (chess-app style)
- Move notation: horizontal scrolling UCI coordinate codes replacing vertical move history
- Timer: compact PlayerTimerStrip replacing full-height Timer component
- Game settings: collapsible panel that auto-collapses after first move
- Game over: modal overlay replacing inline message
- Landing page: mobile-first responsive layout with full-width button
- Nav bar: compact with integrated SoundToggle
- Screenshot capture script: data-testid-based move injection (fixes Svelte style scoping issue)

### Removed
- Leaderboard component and ELO rating UI (rating config/stores preserved)
- MoveHistory component (replaced by MoveNotation)
- Timer component (replaced by PlayerTimerStrip)

## [3.0.0] - 2026-04-02

### Fixed
- AI move endpoint: pass SearchOptions.Default instead of null to GetBestMove (NullReferenceException on every AI turn)

### Added
- Screenshot capture script (scripts/capture-screenshot.mjs): E2E pipeline that builds backend+frontend, plays AI vs AI match, captures screenshot, inserts into README
- Screenshot of AI vs AI match in README header

## [2.9.0] - 2026-04-02

### Fixed
- Frontend board indexing: x-major/y-major mismatch between frontend and backend causing misplaced stones
- Frontend grid lines missing on game board cells
- Rating update logic: use previousPlayer (captured before API call) instead of currentPlayer (already switched by syncGameState)
- AI side selection labels: value/label mismatch for PvAI mode
- Timer server sync: removed broken periodic sync (backend returns hardcoded 0s, could reset timer)
- Open Rule description on landing page: corrected from "center 3x3 zone" to actual >=3 intersection rule
- Landing page CSS: restored app.html and app.pcss that were clobbered to empty

### Added
- Last-move highlighting on game board cells (colored ring around most recent stone)
- "New Game" button after game over
- Inline error banner replacing alert() dialogs

### Changed
- Landing page: added "Start Playing" call-to-action linking to /game
- Game page: moveInProgress guard prevents double-click race conditions
- E2E tests: updated winning line tests and Open Rule test data for 16x16 board
- API concurrency: ExecuteUnderLock renamed to MutateUnderLock for clarity

## [2.8.1] - 2026-04-02

### Fixed
- Handle full-board draw (256 moves, no winner) across all layers with sentinel value (-1,-1)
- Resolve test host crash in Caro.Core.Tests from MinimaxAI disposal race

### Changed
- Sync ENGINE_FEATURES.md with actual implementation (TT write policy, move ordering scores, history bounds)
- Sync README.md UCI example session with actual engine option defaults (Threads=4, Ponder=false)
- Sync README.md TT write policy description (helpers filtered to depth >= 3)

## [2.8.0] - 2026-04-01

### Changed
- Frontend: extract syncGameState, handleGameEnd, findNewMove helpers in game page (eliminates 4x/2x/2x duplication)
- Frontend: replace O(n) board.find() with O(1) index lookup in Board, gameStore, game page
- Frontend: Timer.svelte force-reactivity hack replaced with clean tick-based $derived approach
- Frontend: move history uses .push() instead of spread for Svelte 5 reactivity
- Frontend: avoid unnecessary 256-cell shallow copy per AI move by reordering diff before syncGameState

### Added
- UCIConnectionStatus type in shared game types (replaces inline literal)

### Removed
- Redundant uciToMove/moveToUCI wrapper functions from uciEngine.ts (use fromUCI/toUCI directly)
- Unused set export from ratingStore
- Empty stylesheet link from root layout
- Duplicate GameMode/TimeControl type declarations from game page (now imported from shared types)
- Narrating comments from game page

## [2.7.0] - 2026-04-01

### Added
- GameMode enum replacing stringly-typed "pvp"/"pvai"/"aivai" literals across Domain and API layers
- SearchOptions record encapsulating AI search parameters (replaces 11-parameter method overload)
- GameConstants.CardinalDirections: single source of truth for direction vectors (eliminates 4 duplicates)
- GameModeExtensions.ToLowerString() for backward-compatible API serialization

### Changed
- BitBoardEvaluator/SIMDBitBoardEvaluator: `new bool[Size,Size]` → `stackalloc` (256 bytes heap→stack per call)
- ParallelMinimaxSearch: candidate scanning `new bool[,]` → `stackalloc` with 1D indexing
- ParallelMinimaxSearch: `.Where().ToList()` → `RemoveAll()` in-place filtering (8 hot-path sites)
- Defense multiplier constants deduplicated: local consts → EvaluationConstants fields
- MinimaxAI: 4-param convenience overload creates SearchOptions and delegates
- UCI search controller constructs SearchOptions object instead of positional args

## [2.6.0] - 2026-04-01

### Changed
- MovePicker: GetWinningMoves and GetThreatCreateMoves scan only candidate moves instead of all 256 cells (hot-path)
- MovePicker: SortByScore uses in-place insertion sort, eliminating 3 array + 1 list allocation per call (hot-path)
- ParallelMinimaxSearch: OrderMovesStaged uses temp-variable swaps instead of tuple deconstruction (hot-path)
- ThreatDetector: deduplication uses int-hash HashSet instead of string interpolation (hot-path)
- WinDetector/SearchBoard/ThreatDetector: boundary checks consolidated through PositionExtensions.InBounds()
- Player enum: added ToLowerString() extension, used in Program.cs SSE/API serialization
- Frontend: +page.svelte uses ApiConfig.endpoints for all API paths
- Frontend: centralized switchPlayer() helper in game.ts, used by gameStore and +page.svelte

### Removed
- Dead code: unused playSound() from SoundManager
- Dead code: unused getTopPlayers() from ratingStore

## [2.5.1] - 2026-04-01

### Changed
- Engine thread count now uses largest power of 2 <= logical core count (e.g., 20 cores -> 16 threads), replacing the previous `max(5, (N/2)-1)` formula
- Thread count calculation centralized in `ThreadPoolConfig.GetLazySMPThreadCount()`, removing inline duplications from `MinimaxAI.cs`
- Removed unused `MinThreadCount` constant from `SearchHeuristicConstants`

## [2.5.0] - 2026-04-01

### Changed
- Production magic numbers replaced with named constants referencing centralized config hubs across 25 files (13 backend, 12 frontend)
- Backend: 3 new config classes (`SearchHeuristicConstants`, `TimeConstants`, `TimeManagementConstants`) serving MinimaxAI, ParallelMinimaxSearch, TimeManager, AdaptiveTimeManager, TimeBudgetDepthManager, TimeMonitor, AsyncQueue, Ponderer, DFPNSearch, ThreatSpaceSearch, SearchLogger, UCIProtocol, UCIMockClient
- Frontend: 7 new config modules (`apiConfig`, `audioConfig`, `e2eConfig`, `hapticConfig`, `ratingConfig`, `uciConfig`, `uiConfig`) serving +page, Board, Cell, Timer, WinningLine, gameStore, ratingStore, uciEngine, boardUtils, sound, haptics, e2e tests
- WinningLine.svelte: fixed wrong default props (boardSize=15, cellSize=40) now use config values

### Fixed
- Frontend WinningLine.svelte hardcoded wrong defaults (15x15 board, 40px cells) instead of actual 16x16 board with 64px cells

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

[5.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v5.0.0
[4.5.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.5.0
[4.4.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.4.0
[4.3.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.3.0
[4.2.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.2.0
[4.1.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.1.0
[4.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v4.0.0
[3.0.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v3.0.0
[2.9.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.9.0
[2.8.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.8.1
[2.8.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.8.0
[2.7.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.7.0
[2.6.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.6.0
[2.5.0]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v2.5.0
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
