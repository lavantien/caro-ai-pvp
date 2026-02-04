# Caro AI PvP

A tournament-strength Caro (Gomoku variant) with grandmaster-level AI, built with .NET 10 and SvelteKit 5.

---

## Overview

- **Grandmaster-level AI** - Lazy SMP parallel search reaching depth 11+
- **Clean Architecture** - Separated Domain, Application, and Infrastructure layers
- **Real-time multiplayer** - WebSocket support via SignalR
- **AI tournament mode** - Balanced round-robin with ELO tracking
- **Mobile-first UX** - Ghost stone positioning and haptic feedback
- **500+ automated tests** - Including adversarial concurrency tests

---

## Features

### AI Engine

State-of-the-art algorithms from computer chess achieving 100-500x speedup over naive minimax:

| Optimization | Speedup |
|--------------|---------|
| Hash Move First (Lazy SMP) | 2-5x (TT work sharing) |
| Principal Variation Search (PVS) | 20-40% |
| Late Move Reduction (LMR) | 30-50% |
| Quiescence Search | Prevents blunders |
| Transposition Table (256MB) | 2-5x |
| History Heuristic | 10-20% |
| Aspiration Windows | 10-30% |

**Move Ordering Priority (Optimized for Lazy SMP):**
1. Hash Move (TT Move) - UNCONDITIONAL #1 for thread work sharing
2. Emergency Defense - Blocks opponent's immediate threats (Open 4)
3. Winning Threats - Creates own threats (Open 4, Double 3)
4. Killer Moves - Caused cutoffs at sibling nodes
5. History/Butterfly Heuristic - General statistical sorting
6. Positional Heuristics - Center proximity, nearby stones

**Advanced Features:**
- Lazy SMP parallel search - Helper threads share TT via hash move priority
- VCF Solver - Runs BEFORE alpha-beta to detect forcing sequences
- Emergency Defense - Immediate threat blocking at priority #2
- Threat Space Search - Tactical move generation
- BitBoard representation (6x ulong for 19x19)
- Pondering - Think on opponent's time

### Difficulty Levels

| Level | Threads | Time Budget | Error | Features |
|-------|---------|-------------|-------|----------|
| Braindead | 1 | 5% | 10% | Beginners |
| Easy | 2 | 20% | 0% | Parallel search |
| Medium | 3 | 50% | 0% | Parallel + pondering |
| Hard | 4 | 75% | 0% | Parallel + pondering + VCF + Opening book |
| Grandmaster | (N/2)-1 | 100% | 0% | Max parallel, VCF, pondering, Opening book |
| Experimental | (N/2)-1 | 100% | 0% | Full opening book, max features |

**Depth:** Dynamic calculation based on host machine NPS and time control. Formula: `depth = log(time * nps * timeMultiplier) / log(ebf)`

### Opening Book

Precomputed opening positions for instant move retrieval and deeper analysis:

- **Symmetry reduction** - 8-way transformations (4 rotations × mirror) reduce storage by ~8x
- Moves stored in canonical coordinate space for symmetry-aware lookups
- **SQLite storage** - Persistent `opening_book.db` with indexed position lookup + WAL mode
- **Translation invariant** - Canonical positions work regardless of board location
- **Per-move metadata** - Win rate, depth achieved, nodes searched, forcing move flag
- **Worker pool generation** - Parallel position + candidate evaluation for 30x throughput
- **Tapered beam width** - Converts exponential growth to linear (4→2→1 children by depth)
- **Early exit optimization** - Skips remaining candidates when best move dominates
- **Resume capability** - Incremental deepening of existing books
- **Real-time progress** - Thread-safe position tracking with AsyncQueue

**Generate opening book** (~8 hours for ply 14):
```bash
dotnet run --project backend/src/Caro.BookBuilder -- \
  --output=opening_book.db \
  --max-depth=14 \
  --target-depth=24
```

**Resume and extend existing book:**
```bash
# Re-run with same output file - generation resumes from last depth
dotnet run --project backend/src/Caro.BookBuilder -- \
  --output=opening_book.db \
  --max-depth=20  # Extend beyond previous depth
```

**Verify existing book:**
```bash
dotnet run --project backend/src/Caro.BookBuilder -- --verify-only --output=opening_book.db
```

### Tournament Mode

- 5 AI levels in round-robin format
- ELO tracking with standard rating calculation
- Balanced scheduling (one game per bot per round)
- SQLite logging with FTS5 full-text search
- SignalR broadcasts via async queues

### Test Suites

Centralized AI testing framework for validating difficulty strength:

```bash
dotnet run --project backend/src/Caro.TournamentRunner -- --test-suite=<name>
```

| Suite | Description |
|-------|-------------|
| `braindead` | Self-play baseline (20 games) |
| `easy` | vs Braindead, self (20 games each) |
| `medium` | vs Braindead, Easy, self (20 games each) |
| `hard` | vs Braindead, Easy, Medium, self (20 games each) |
| `grandmaster` | vs All + self (20 games each) |
| `experimental` | Custom AI testing (10 games each) |
| `full` | Run all suites in series |

Results written to `backend/tournament_results.txt` with pass/fail thresholds.

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
┌──────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                          │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  MinimaxAI  │  VCFSolver  │  ParallelMinimaxSearch          │ │
│  │  Hash Move  │  VCF Pre-Search │  Lazy SMP  │  TT Sharding    │ │
│  │  Priority #1│  Emergency Defense │  BitBoardEvaluator       │ │
│  │  OpeningBook │ PositionCanonicalizer │ BookGeneration       │ │
│  └─────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

**Clean Architecture Projects:**

| Project | Purpose | Dependencies |
|---------|---------|--------------|
| `Caro.Core.Domain` | Core entities, value objects | None |
| `Caro.Core.Application` | Interfaces, application services | Domain |
| `Caro.Core.Infrastructure` | AI algorithms, external concerns | Domain, Application |
| `Caro.Api` | Web API, SignalR hub | All layers |
| `Caro.BookBuilder` | CLI tool for offline book generation | Infrastructure |

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

**Frontend:** SvelteKit 5, TypeScript, Svelte 5 Runes, Skeleton UI v4, TailwindCSS v4, SignalR

**Backend:** .NET 10, ASP.NET Core 10, SignalR, System.Threading.Channels, SQLite + FTS5, xUnit v3.1

**AI:** Custom Minimax, alpha-beta pruning, Zobrist hashing, BitBoard, VCF pre-search solver, Lazy SMP, Hash Move-first ordering, Opening book with symmetry reduction

---

## Testing

| Category | Tests |
|----------|-------|
| Backend Unit | 550+ |
| Statistical | 38 |
| AI Strength Validation | 19 |
| Concurrency | 32 |
| Integration | 13 |
| Frontend Unit | 26 |
| **TOTAL** | **550+** |

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
- **Time Control:** 7min + 5sec increment (Fisher)

---

## License

MIT

---

Built with SvelteKit + .NET 10
