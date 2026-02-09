# Caro AI PvP

A tournament-strength Caro (Gomoku variant) with grandmaster-level AI, built with .NET 10, SvelteKit 2.49+ with Svelte 5 Runes.

---

## Overview

- **Grandmaster-level AI** - Lazy SMP parallel search reaching depth 11+
- **UCI Protocol Support** - Standalone engine compatible with UCI chess GUIs
- **Clean Architecture** - Separated Domain, Application, and Infrastructure layers
- **Real-time multiplayer** - WebSocket support via SignalR
- **AI tournament mode** - Balanced round-robin with ELO tracking
- **Mobile-first UX** - Ghost stone positioning and haptic feedback
- **670+ automated tests** - Including adversarial concurrency tests

---

## Features

### AI Engine

State-of-the-art algorithms from computer chess achieving 100-500x speedup over naive minimax:

| Optimization | Speedup / ELO Gain |
|--------------|-------------------|
| Multi-Entry Transposition Table | 2-5x (+30-50 ELO) |
| Continuation History | +15-25 ELO |
| Evaluation Cache | +10-20 ELO |
| Adaptive Late Move Reduction | 30-50% (+25-40 ELO) |
| PID Time Management | +20-50 ELO |
| Contest Factor (Contempt) | +5-20 ELO |
| Principal Variation Search (PVS) | 20-40% |
| Quiescence Search | Prevents blunders |
| Hash Move First (Lazy SMP) | 2-5x (TT work sharing) |
| History Heuristic | 10-20% |
| Aspiration Windows | 10-30% |

**Move Ordering Priority (Optimized for Lazy SMP):**
1. Hash Move (TT Move) - UNCONDITIONAL #1 for thread work sharing
2. Emergency Defense - Blocks opponent's immediate threats (Open 4)
3. Winning Threats - Creates own threats (Open 4, Double 3)
4. Continuation History - Move pair statistics from recent plies
5. Killer Moves - Caused cutoffs at sibling nodes
6. History/Butterfly Heuristic - General statistical sorting
7. Positional Heuristics - Center proximity, nearby stones

**Advanced Features:**
- Lazy SMP parallel search - Helper threads share TT via hash move priority
- Multi-entry TT clusters - 3 entries per cluster (32-byte cache-line aligned)
- Continuation history - 6-ply move pair tracking with bounded updates
- VCF Solver - Runs BEFORE alpha-beta to detect forcing sequences
- Emergency Defense - Immediate threat blocking at priority #2
- Threat Space Search - Tactical move generation
- Evaluation cache - Correction values for position evaluations
- Adaptive LMR - Dynamic reduction based on improving, depth, delta, node types
- PID time management - Proportional-Integral-Derivative time allocation
- Contest factor - Position-aware contempt from -200 to +200 centipawns
- SPSA tuner - Gradient-free parameter optimization
- Structured search logging - Async file-based logging with rotation

### Difficulty Levels

| Level | Threads | Time Budget | Error | Book Depth | Features |
|-------|---------|-------------|-------|------------|----------|
| Braindead | 1 | 5% | 10% | 0 | Beginners |
| Easy | 2 | 20% | 0% | 0 | Parallel search |
| Medium | 3 | 50% | 0% | 0 | Parallel + pondering |
| Hard | 4 | 75% | 0% | 24 plies | Parallel + pondering + VCF + Opening book |
| Grandmaster | (N/2)-1 | 100% | 0% | 32 plies | Max parallel, VCF, pondering, Opening book |
| Experimental | (N/2)-1 | 100% | 0% | 40 plies | Full opening book, max features |

**Depth:** Dynamic calculation based on host machine NPS and time control. Formula: `depth = log(time * nps * timeMultiplier) / log(ebf)`

### UCI Protocol

Universal Chess Interface (UCI) protocol compatibility for standalone engine usage:

- **Standalone console engine** - Run as separate process like Stockfish
- **Standard UCI commands** - uci, isready, ucinewgame, position, go, stop, quit, setoption
- **Engine options** - Skill Level, Use Opening Book, Book Depth Limit, Threads, Hash, Ponder
- **WebSocket bridge** - Frontend can connect directly to UCI engine
- **Algebraic notation** - Coordinates a-s (columns), 1-19 (rows)

**Run standalone UCI engine:**
```bash
dotnet run --project backend/src/Caro.UCI
```

**Run UCI Mock Client (engine vs engine testing):**
```bash
dotnet run --project backend/src/Caro.UCIMockClient -- --games 4 --time 180 --inc 2
```

**Example UCI session:**
```
> uci
< id name Caro AI 1.31.0
< id author Caro AI Project
< option name Skill Level type spin default 3 min 1 max 6
< option name Use Opening Book type check default true
< uciok
> position startpos moves j10
> go movetime 2000
< info depth 2 nodes 13524 time 1590 pv j11
< bestmove j11
```

### Opening Book

Precomputed opening positions with SQLite storage, symmetry reduction, and parallel generation:

- **Hard+ only** - Easy/Medium do NOT use opening book, AI calculates first move naturally
- **Configurable depth** - Hard: 24 plies, Grandmaster: 32 plies, Experimental: 40 plies
- **Tiered continuation** - Response counts decrease with depth (4→3→2→1) ensuring coverage
- **Symmetry reduction** - 8-way transformations reduce storage by ~8x
- **Worker pool** - Parallel position/candidate evaluation for 30x throughput
- **Resume capability** - Incremental deepening of existing books

**Generate book:**
```bash
dotnet run --project backend/src/Caro.BookBuilder
```

**Custom output path:**
```bash
dotnet run --project backend/src/Caro.BookBuilder -- --output=custom_book.db
```

**Verify existing book:**
```bash
dotnet run --project backend/src/Caro.BookBuilder -- --verify-only --output=opening_book.db
```

**Book Structure (hardcoded 4-3-2-1 tapered beam):**
- Plies 0-14: 4 moves per position (early game + survival zone)
- Plies 15-24: 3 moves per position (Hard difficulty)
- Plies 25-32: 2 moves per position (Grandmaster)
- Plies 33-40: 1 move per position (Experimental)

### Tournament Mode

- 5 AI levels in round-robin format
- ELO tracking with standard rating calculation
- Balanced scheduling (one game per bot per round)
- SQLite logging with FTS5 full-text search
- SignalR broadcasts via async queues

### Test Projects

Separate test projects for focused testing:

```bash
# Unit tests (fast, no integration/matchup tests)
cd backend/tests/Caro.Core.Tests && dotnet test

# Integration tests (opt-in, full AI searches - slower)
cd backend/tests/Caro.Core.IntegrationTests && dotnet test

# Matchup/integration tests (slower, AI vs AI matchups)
cd backend/tests/Caro.Core.MatchupTests && dotnet test
```

| Project | Tests | Duration |
|---------|-------|----------|
| Caro.Core.Tests | 414 unit tests | ~2 sec |
| Caro.Core.IntegrationTests | 153 | Opt-in, AI searches |
| Caro.Core.MatchupTests | ~57 | Variable |
| Caro.Core.Domain.Tests | 67 entity tests | ~1 sec |
| Caro.Core.Application.Tests | 8 service tests | ~1 sec |
| Caro.Core.Infrastructure.Tests | 72 tests | ~42 sec |

**Note:** Run `dotnet test` in Caro.Core.Tests for fast unit test feedback. IntegrationTests are excluded from default test runs (marked as `<IsTestProject>false</IsTestProject>`).

---

## Architecture

Clean Architecture with three core layers:

```
┌─────────────────────────────────────────────────────────────────┐
│                         Presentation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   SvelteKit │  │  SignalR    │  │  ASP.NET Core API       │  │
│  │   Frontend  │  │   Hub       │  │  Controllers            │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────────┘  │
└─────────┼────────────────┼────────────────────┼─────────────────┘
          │                │                    │
          ▼                ▼                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Application Layer                           │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  TournamentEngine  │  MatchScheduler  │  IStatsPublisher    │ │
│  └─────────────────────────────────────────────────────────────┘ │
└───────────────────────────┬──────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Domain Layer (Core)                         │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌─────────────────┐  │
│  │  Board   │  │  Player  │  │ GameState │  │  AIDifficulty   │  │
│  │  (19x19) │  │  Enum    │  │           │  │  (Braindead-GM) │  │
│  └──────────┘  └──────────┘  └───────────┘  └─────────────────┘  │
└───────────────────────────┬──────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                             │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  MinimaxAI   │  VCFSolver            │  ParallelMinimaxSearch  │ │
│  │  Hash Move   │  VCF Pre-Search       │  Lazy SMP │ TT Sharding │ │
│  │  Priority #1 │  Emergency Defense    │  BitBoardEvaluator      │ │
│  │  OpeningBook │ PositionCanonicalizer │  BookGeneration         │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

**Clean Architecture Projects:**

| Project | Purpose | Dependencies |
|---------|---------|--------------|
| `Caro.Core.Domain` | Core entities, value objects | None |
| `Caro.Core.Application` | Interfaces, application services | Domain |
| `Caro.Core.Infrastructure` | AI algorithms, external concerns | Domain, Application |

**Immutable Domain Model:**

All domain entities are fully immutable for thread safety:
- `Cell` - `readonly record struct` with `Player` property
- `GameState` - `sealed record` with `ImmutableStack<Board>` for undo history
- `Board` - Immutable via `PlaceStone()` returning new instances
- Operations return new state: `WithMove()`, `WithGameOver()`, `UndoMove()`

### AI Improvements (Phase 1 & 2)

Recent implementation of advanced AI optimization techniques targeting +125-245 ELO gain:

| Feature | Description | ELO Gain |
|---------|-------------|----------|
| **Multi-Entry TT** | 3-entry clusters (32-byte aligned), depth-age replacement | +30-50 |
| **Continuation History** | 6-ply move pair statistics with bounded updates | +15-25 |
| **Evaluation Cache** | Position evaluation correction caching | +10-20 |
| **Adaptive LMR** | Dynamic reduction based on position factors | +25-40 |
| **PID Time Manager** | Proportional-Integral-Derivative time allocation | +20-50 |
| **Contest Manager** | Dynamic contempt (-200 to +200 centipawns) | +5-20 |
| **SPSA Tuner** | Gradient-free parameter optimization | +20-40 |
| **Structured Logging** | Async search logging with file rotation | - |

**Key Implementation Details:**
- Cluster-based TT with 10-byte TTEntry struct, 30-byte cluster (padded to 32 bytes)
- ContinuationHistory: `short[,,]` for `[player, prevCell, currentCell]` with MaxScore=30000
- SPSA: Default (α=0.602, γ=0.101), Aggressive, Conservative presets
- PID: Kp=1.0, Ki=0.1, Kd=0.5 with integral windup clamping
- SearchLogger: Channel-based async logging, 100MB/24h rotation
| `Caro.Api` | Web API, SignalR hub, WebSocket UCI bridge | All layers |
| `Caro.BookBuilder` | CLI tool for offline book generation | Infrastructure |
| `Caro.UCI` | Standalone UCI console engine | Infrastructure |
| `Caro.UCIMockClient` | UCI protocol testing tool (engine vs engine) | Infrastructure |

### Component Flow

**Move Request Flow:**
1. Frontend sends move via SignalR → TournamentHub
2. TournamentEngine calls `MinimaxAI.GetBestMove()`
3. Parallel search spawns N threads (based on difficulty)
4. Master thread selects best result, helpers explore with TT sharing

**Stats Pub-Sub Flow:**
1. MinimaxAI implements `IStatsPublisher` with `Channel<MoveStatsEvent>`
2. After each move, stats published to channel (MainSearch, Pondering, VCFSearch)
3. TournamentEngine runs async subscriber tasks for both AIs
4. Ponder stats cached separately for post-move reporting

**Transposition Table Sharding:**
- 16 segments with independent hash-based distribution
- `shardIndex = (hash >> 32) & shardMask`
- Reduces cache coherency traffic for parallel threads

**Helper Thread TT Write Policy:**
```
if (threadIndex > 0) {
    if (depth < rootDepth / 2) return;  // Too shallow
    if (flag != Exact) return;           // Misleading bounds
}
// Master threads (threadIndex=0) can store any entry
```

---

## Concurrency

Production-grade concurrency following .NET 10 best practices:

| Pattern | Purpose |
|---------|---------|
| Channel-based queues | No fire-and-forget exceptions |
| Per-game locks | 100+ concurrent games |
| CancellationTokenSource | Coordinated search cancellation |
| TT sharding (16 segments) | Reduced cache contention |
| Publisher-Subscriber | AI telemetry without callbacks |

**Testing:** 32 adversarial concurrency tests validate thread-safety under high contention.

---

## Performance

| Difficulty | Threads | Time Budget | Depth (7+5 TC) |
|------------|---------|-------------|----------------|
| Braindead | 1 | 5% | ~1-3 |
| Easy | 2 | 20% | ~3-5 |
| Medium | 3 | 50% | ~5-7 |
| Hard | 4 | 75% | ~7-9 |
| Grandmaster | (N/2)-1 | 100% | ~9-12+ |

**Depth varies by host machine** - calculated dynamically from NPS and time budget. Higher-spec machines achieve greater depth naturally.

---

## Tech Stack

**Frontend:** SvelteKit 2.49+ with Svelte 5 Runes, TypeScript 5.9, TailwindCSS 4.1, SignalR (@microsoft/signalr 8.0), Vitest 4.0, Playwright 1.57

**Backend:** .NET 10, ASP.NET Core 10, SignalR, System.Threading.Channels, SQLite + FTS5, xUnit 2.9.2 with xUnit Runner 3.1.4, Moq 4.20.72, FluentAssertions 7.0.0-8.8.0

**AI:** Custom Minimax, alpha-beta pruning, Zobrist hashing, BitBoard, VCF pre-search solver, Lazy SMP, Hash Move-first ordering, Opening book with symmetry reduction

---

## Testing

| Project | Tests | Focus |
|---------|-------|-------|
| Caro.Core.Tests | 414 | Unit tests (algorithms, evaluators, concurrency, immutable state, test helpers, AI improvements) |
| Caro.Core.IntegrationTests | 153 | AI search integration (full depth searches, performance benchmarks, opening book edge cases + performance tests) |
| Caro.Core.MatchupTests | ~57 | AI matchups, integration, tournament, opening book verification |
| Caro.Core.Domain.Tests | 67 | Entities (Board, Cell, Player, GameState, Position) |
| Caro.Core.Application.Tests | 8 | Services, interfaces, DTOs, Mappers |
| Caro.Core.Infrastructure.Tests | 72 | AI algorithms, external concerns |
| Frontend Unit (Vitest) | 19 | Component tests |
| Frontend E2E (Playwright) | 17 | End-to-end gameplay |
| **TOTAL** | **750+** | |

### Frontend E2E Tests

Playwright end-to-end tests covering core gameplay mechanics:

| Feature | Tests |
|---------|-------|
| Basic Mechanics (move placement, open rule) | 4 |
| Sound Effects (valid/invalid moves) | 3 |
| Move History (tracking, display) | 3 |
| Winning Line Animation | 2 |
| Timer Functionality (Fisher time control) | 3 |
| Regression Tests (edge cases) | 2 |

Run E2E tests:
```bash
cd frontend && npm run test:e2e
```

---

## Getting Started

```bash
# Clone
git clone https://github.com/lavantien/caro-ai-pvp.git
cd caro-ai-pvp

# Backend
cd backend && dotnet restore && dotnet build
cd src/Caro.Api && dotnet run

# Frontend (new terminal)
cd frontend && npm install
npm run dev
```

Backend: http://localhost:5207 | Frontend: http://localhost:5173

---

## Game Rules

- **19x19 board** (361 intersections)
- **Open Rule:** Red's second move must be at least 3 intersections away from first
- **Win:** Exactly 5 in a row (6+ or blocked ends don't count)
- **Time Control:** Selectable - 1+0 (Bullet), 3+2 (Blitz), 7+5 (Rapid), 15+10 (Classical)

---

## Documentation

**Improvement Research:** `IMPROVEMENT_RESEARCH.md` - Comprehensive research report on AI optimization techniques from Rapfi, Stockfish 18, Chess Programming Wiki, and advanced optimization methods.

---

## License

MIT

---

Built with SvelteKit + .NET 10
