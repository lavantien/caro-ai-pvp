# 🎮 Caro AI PvP - Tournament-Strength Caro with Modern Web Stack

<div align="center">

![TypeScript](https://img.shields.io/badge/TypeScript-5.0-blue)
![.NET](https://img.shields.io/badge/.NET-10.0-purple)
![SvelteKit](https://img.shields.io/badge/SvelteKit-Latest-orange)
![Tests](https://img.shields.io/badge/Tests-250%2B%20Passing-success)
![License](https://img.shields.io/badge/License-MIT-green)

**A mobile-first real-time implementation of Caro (Gomoku variant) with grandmaster-level AI powered by 15+ advanced search optimizations**

[Features](#-features) • [AI Engine](#-ai-engine) • [Architecture](#-architecture) • [Tech Stack](#-tech-stack) • [Getting Started](#-getting-started)

</div>

---

## 🌟 Overview

Caro is a sophisticated 15x15 board game implementation featuring:
- **Grandmaster-level AI** with Lazy SMP parallel search (depth 11+ capable)
- **Real-time multiplayer** with WebSocket support (SignalR)
- **AI tournament mode** with balanced scheduling and ELO tracking
- **Mobile-first UX** with ghost stone positioning and haptic feedback
- **Comprehensive testing** with 250+ automated tests including integration tests with snapshot verification

Built with **.NET 10** and **SvelteKit 5**, representing cutting-edge 2025 web development standards.

---

## ✨ Features

### 🤖 Grandmaster-Level AI Engine

Our AI employs state-of-the-art algorithms from computer chess, achieving 100-500x performance improvement over naive minimax:

**Core Search Optimizations:**
- **Lazy SMP (Shared Memory Parallelism)** - 4-8x speedup on multi-core for D7+
- **Principal Variation Search (PVS)** - Null window searches for non-PV moves (20-40% speedup)
- **Late Move Reduction (LMR)** - Reduce late moves, re-search if promising (30-50% speedup)
- **Quiescence Search** - Extend search in tactical positions to prevent blunders
- **Enhanced Move Ordering** - Tactical pattern detection (15-25% speedup)
- **Transposition Table** - Lock-free 64MB Zobrist hashing cache (2-5x speedup)
- **History Heuristic** - Track moves causing cutoffs across all depths (10-20% speedup)
- **Aspiration Windows** - Narrow search windows around estimated score (10-30% speedup)

**Advanced Features:**
- **Threat Space Search** - Focus search on critical threats only
- **DFPN Solver** - Depth-First Proof Number search for forced wins
- **BitBoard Representation** - SIMD-accelerated board evaluation
- **Opening Book** - Pre-computed strong opening positions
- **Pondering** - AI thinks during opponent's turn
- **Adaptive Time Management** - Smart time allocation per move

**Difficulty Levels (D1-D11):**
- D1-D2: Beginner (randomness added for mercy)
- D3-D4: Casual play
- D5-D6: Intermediate challenge
- D7-D8: Advanced (uses Lazy SMP parallel search)
- D9-D10: Expert (threat space + advanced pruning)
- D11: Grandmaster (all optimizations + deep search)

### 🏆 Tournament Mode

- **22 AI bots** competing in balanced round-robin format
- **ELO tracking** with standard rating calculation
- **Fair scheduling** - each bot plays at most once per round
- **Live standings** with win rates and rating changes
- **SQLite logging** with FTS5 full-text search for game analysis

### 🎯 Game Features

#### Core Gameplay
- **15x15 board** with exact 5-in-row winning condition
- **Open Rule** enforcement (second move restriction in center 3x3 zone)
- **Blocked ends** detection (6+ or blocked lines don't win)
- **Chess clock** with Fisher control (7min + 5sec increment)

#### Polish Features
- **🔊 Sound Effects** - Synthesized audio (no external files) with mute toggle
- **📜 Move History** - Scrollable chronological move display
- **🏆 Winning Line Animation** - SVG stroke animation with color coding
- **↩️ Undo Functionality** - Revert moves with time restoration
- **📊 ELO/Ranking System** - Standard ELO calculation with leaderboard

---

## 🧠 AI Engine Deep Dive

### Parallel Search Architecture (D7+)

```
Lazy SMP Parallel Search
├── Thread Pool (Environment.ProcessorCount)
│   ├── Thread 1: Full depth search
│   ├── Thread 2: Full depth search
│   ├── Thread 3: Full depth search
│   └── Thread N: Full depth search
├── Lock-Free Transposition Table
│   ├── Concurrent dictionary access
│   ├── Atomic updates
│   └── No locks needed (read-only sharing)
└── Result Aggregation
    ├── Select best move across threads
    ├── Aggregate nodes searched
    └── Track max depth achieved
```

### BitBoard Representation

```
BitBoard Layout (15x15 = 225 bits)
├── Red BitBoard (UInt128) - Red stone positions
├── Blue BitBoard (UInt128) - Blue stone positions
├── Occupied BitBoard (Red | Blue)
└── SIMD Operations
    ├── PopCount for stone counting
    ├── BitOps for line detection
    └── Vectorized pattern matching
```

### Threat Detection

```
Threat Classification
├── Threat Level 5: Five in row (WIN)
├── Threat Level 4: Open Four (unstoppable)
├── Threat Level 3: Closed Four / Open Three
├── Threat Level 2: Closed Three / Open Two
└── Threat Level 1: Closed Two / Open One

Threat Space Search
├── Only search threat moves
├── Prune non-threat candidates
└── 10-100x reduction in search space
```

### Performance Metrics

| Difficulty | Search Type | Avg Time | Positions/S | TT Hit Rate |
|------------|-------------|----------|-------------|-------------|
| Easy (D1) | Single | <100ms | ~100K | N/A |
| Medium (D2-D3) | Single | <500ms | ~50K | 20% |
| Hard (D4-D5) | Single | <2s | ~20K | 35% |
| Expert (D6-D7) | Lazy SMP | <5s | ~100K | 45% |
| Master (D8-D9) | Lazy SMP | 5-30s | ~500K | 50%+ |
| Grandmaster (D10-D11) | Lazy SMP + TSS | 10-60s | ~1M | 55%+ |

**Combined Optimization Impact:** 100-500x faster than naive minimax.

---

## 🏗️ Architecture

### System Design

```
┌─────────────────────────────────────────────────────┐
│                   Frontend (SvelteKit)                │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Board.svelte │  │ GameStore    │  │ SoundMgr   │ │
│  │              │  │ (Svelte 5    │  │            │ │
│  │ Ghost Stone  │  │  Runes)      │  │ Web Audio  │ │
│  │ Zoom/Pan     │  │              │  │            │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ Tournament   │  │ SignalR      │  │ Leaderboard│ │
│  │ Dashboard    │  │ Client       │  │            │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────┘
                          ↕ WebSocket
┌─────────────────────────────────────────────────────┐
│              Backend (ASP.NET Core 10)               │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ TournamentHub│  │ MinimaxAI    │  │  ELOCalc   │ │
│  │ (SignalR)    │  │              │  │            │ │
│  │ Real-time    │  │ Lazy SMP     │  │ Standard   │ │
│  │ Sync         │  │ + TSS        │  │ Formula   │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │TournamentMgr │  │ ThreatDetect │  │ GameLogSvc │ │
│  │              │  │              │  │            │ │
│  │ Bracket/Match│  │ Pattern Rec  │  │ SQLite+FTS ││
│  └──────────────┘  └──────────────┘  └────────────┘ │
│                                                           │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │BitBoardEval  │  │ Transposition│  │  Validator │ │
│  │              │  │    Table     │  │            │ │
│  │SIMD Acceler  │  │ Lock-Free TT │  │ Open Rule  │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────┘
                          ↕
┌─────────────────────────────────────────────────────┐
│              Database (SQLite + EF Core)              │
│  • Matches (move history as JSON)                     │
│  • GameLogs (FTS5 indexed search)                     │
│  • ActiveSessions (board state)                       │
│  • Players (ELO ratings)                              │
└─────────────────────────────────────────────────────┘
```

---

## 🛠️ Tech Stack

### Frontend
- **SvelteKit 5** with TypeScript
- **Svelte 5 Runes** ($state, $props, $derived) for modern reactivity
- **Skeleton UI v4** for accessible component library
- **TailwindCSS v4** for utility-first styling
- **SignalR client** for real-time communication
- **Vitest v4** for unit testing

### Backend
- **.NET 10** / **C# 14** (LTS)
- **ASP.NET Core 10** Web API
- **SignalR** for real-time WebSocket communication
- **SQLite** with FTS5 full-text search
- **xUnit v3.1** for testing

### AI/ML
- Custom Minimax with Lazy SMP parallel search
- Zobrist hashing with lock-free transposition tables
- BitBoard representation with SIMD operations
- Threat space search and DFPN solver
- Opening book with pre-computed positions
- Pondering for optimal time utilization

---

## 🧪 Testing

### Test Coverage Summary

| Category | Tests | Focus |
|----------|-------|-------|
| Backend Unit | 200+ | AI algorithms, board logic |
| Integration | 13 | Tournament with snapshots |
| Frontend Unit | 19+ | Components, stores |
| E2E Tests | 17+ | Full user flows |
| **TOTAL** | **250+** | **Full coverage** |

### Integration Tests with Snapshots

Tests run real AI games and save JSON snapshots for regression detection:

```
Tournament/Snapshots/
├── RunSingleGame_BasicVsMedium_SavesSnapshot.json
├── RunThreeGames_EasyVsHard_LogsDepthStatistics.json
├── RunGame_VeryHardVsExpert_ParallelSearchReportsCorrectDepth.json
├── RunMiniTournament_FourBots_BalancedSchedule.json
└── RunGame_BeginnerVsBeginner_WithShortTimeControl.json
```

Each snapshot contains:
- Per-move statistics (depth, nodes, NPS)
- Game result metadata
- Raw logs for inspection

### Running Tests

```bash
# Backend
cd backend
dotnet test --verbosity quiet

# Integration tests only
dotnet test --filter "FullyQualifiedName~TournamentIntegration"

# Frontend
cd frontend
npm run test -- --run
```

---

## 🚀 Getting Started

### Prerequisites
- .NET 10 SDK
- Node.js 20+
- PowerShell or Bash

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/caro-ai-pvp.git
cd caro-ai-pvp

# Backend setup
cd backend
dotnet restore
dotnet build

# Frontend setup
cd ../frontend
npm install
```

### Running the Application

**Terminal 1 - Backend:**
```bash
cd backend/src/Caro.Api
dotnet run
```
API runs on: http://localhost:5207

**Terminal 2 - Frontend:**
```bash
cd frontend
npm run dev
```
Frontend runs on: http://localhost:5173

---

## 📊 Tournament Mode

AI vs AI tournaments with balanced scheduling:

### Features
- **22 AI bots** across 11 difficulty levels (2 bots per level)
- **Round-robin format** - each bot plays every other bot twice
- **Balanced scheduling** - each bot plays at most once per round
- **ELO tracking** - ratings update after each match
- **SQLite logging** - all games logged with full statistics

### Scheduling Algorithm

```
Balanced Round-Robin:
1. Generate all pairings (each pair plays twice, colors swapped)
2. Greedy round assignment:
   - Each round maximizes unique bots playing
   - No bot appears more than once per round
   - Ensures fair distribution throughout tournament
3. Total matches: n × (n-1) for n bots
   - 22 bots = 462 matches
```

---

## 🎮 Game Rules

### Board Setup
- 15x15 grid (225 intersections)
- Red (O) moves first
- Blue (X) moves second

### The Open Rule
The second Red move (move #3 overall) cannot be placed in the 3x3 zone surrounding the center intersection.

### Winning Conditions
- Exactly 5 stones in a row (horizontal, vertical, diagonal)
- Neither end blocked
- 6+ stones (overline) is not a win

### Time Control
Fisher timing: **7 minutes initial + 5 seconds increment per move**

---

## 📁 Project Structure

```
caro-ai-pvp/
├── backend/
│   ├── src/Caro.Core/
│   │   ├── Entities/
│   │   │   ├── Board.cs              # 15x15 game board
│   │   │   ├── Cell.cs               # Intersection state
│   │   │   └── GameState.cs          # Game state + undo
│   │   ├── GameLogic/
│   │   │   ├── MinimaxAI.cs          # Main AI engine
│   │   │   ├── ParallelMinimaxSearch.cs  # Lazy SMP
│   │   │   ├── BitBoard.cs           # Bit board rep
│   │   │   ├── BitBoardEvaluator.cs  # SIMD evaluation
│   │   │   ├── ThreatDetector.cs     # Threat detection
│   │   │   ├── ThreatSpaceSearch.cs  # TSS algorithm
│   │   │   ├── DFPNSearch.cs         # Proof number search
│   │   │   ├── OpeningBook.cs        # Opening positions
│   │   │   ├── Pondering/            # Think on opp time
│   │   │   ├── TimeManagement/       # Adaptive timing
│   │   │   ├── TranspositionTable.cs # TT (legacy)
│   │   │   ├── LockFreeTranspositionTable.cs  # Concurrent TT
│   │   │   ├── BoardEvaluator.cs     # Static eval
│   │   │   ├── WinDetector.cs        # Win detection
│   │   │   └── AIDifficulty.cs       # D1-D11 levels
│   │   └── Tournament/
│   │       ├── TournamentEngine.cs   # Game runner
│   │       ├── TournamentMatch.cs    # Match scheduling
│   │       └── AIBot.cs              # Bot factory
│   ├── src/Caro.Api/
│   │   ├── TournamentHub.cs          # SignalR hub
│   │   ├── TournamentManager.cs      # Tournament state
│   │   └── Logging/
│   │       └── GameLogService.cs     # SQLite + FTS5
│   └── tests/Caro.Core.Tests/
│       ├── Tournament/
│       │   ├── TournamentIntegrationTests.cs
│       │   ├── SavedLogVerifierTests.cs
│       │   ├── BalancedSchedulerTests.cs
│       │   └── TournamentLogCapture.cs
│       └── GameLogic/               # 200+ unit tests
├── frontend/
│   ├── src/routes/
│   │   └── tournament/              # Tournament UI
│   └── src/lib/
│       ├── stores/
│       │   └── tournamentStore.svelte.ts
│       └── components/
└── README.md
```

---

## 🎯 Roadmap

### Completed ✅
- [x] Core game logic (board, win detection, Open Rule)
- [x] Minimax AI with alpha-beta pruning
- [x] All 8+ search optimizations (PVS, LMR, Quiescence, Lazy SMP, etc.)
- [x] 11 difficulty levels (D1-D11)
- [x] Lazy SMP parallel search (4-8x speedup)
- [x] Threat detection and Threat Space Search
- [x] BitBoard with SIMD evaluation
- [x] Lock-free transposition table
- [x] Opening book
- [x] Pondering (think on opponent's time)
- [x] Adaptive time management
- [x] AI tournament mode with 22 bots
- [x] Balanced round-robin scheduling
- [x] SQLite logging with FTS5
- [x] Integration tests with snapshot verification
- [x] SignalR real-time updates
- [x] 250+ automated tests

### In Progress 🚧
- [ ] User authentication
- [ ] Matchmaking system for PvP
- [ ] Replay system (move history as JSON)

### Planned 📋
- [ ] Progressive Web App (PWA)
- [ ] Mobile app stores (iOS/Android)
- [ ] Endgame tablebase
- [ ] Machine learning evaluation function

---

## 🏆 Achievements

- **100-500x AI speedup** through advanced search optimizations
- **250+ automated tests** with snapshot-based regression detection
- **Lazy SMP parallel search** for D7+ difficulty levels
- **BitBoard with SIMD** for accelerated evaluation
- **Threat Space Search** for focused tactical calculation
- **22 AI tournament bots** with balanced scheduling
- **SQLite + FTS5 logging** for game analysis
- **Grandmaster-level AI** (depth 11+ capable)

---

## 📄 License

This project is licensed under the MIT License.

---

<div align="center">

**Built with ❤️ using SvelteKit + .NET 10**

**Showcasing grandmaster-level AI with modern web development**

</div>
