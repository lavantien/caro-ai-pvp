# C# 14 / .NET 10 Onboarding Guide

A survival guide for contributing to the Caro AI PvP backend on the modern
C# / .NET 10 stack. The engine was ported line-for-line from the Go
implementation that preceded it; `GO_ONBOARDING.md` stays in the repo as the
archived guide for that era.

## Quick Project Overview

### Solution Structure

```
backend/
├── Caro.sln
├── global.json                 (SDK 10.0.400, rollForward latestFeature)
├── Directory.Build.props       (net10.0, nullable, warnings as errors)
├── Makefile                    (build / test / fmt / lint / clean via dotnet)
├── src/
│   ├── Caro.Domain/            pure game rules, zero dependencies
│   ├── Caro.Engine/            the AI: search, VCF, TT, pondering
│   ├── Caro.Uci/               UCI protocol state machine
│   ├── Caro.Persistence/       SQLite match archive (Microsoft.Data.Sqlite)
│   ├── Caro.Api/               HTTP handlers, sessions, middleware, WebSocket
│   ├── Caro.Server/            ASP.NET Core host on :5207
│   └── Caro.UciEngine/         standalone UCI console engine
└── tests/                      one xUnit project per src project
```

### Key Technologies

| Concern | Choice |
|---------|--------|
| Language and runtime | C# 14 on .NET 10 (`net10.0`) |
| HTTP | ASP.NET Core minimal APIs, Kestrel |
| WebSocket | ASP.NET Core WebSockets (`/ws/uci`) |
| Persistence | SQLite via `Microsoft.Data.Sqlite` (managed, no CGO) |
| Tests | xUnit + coverlet |
| Formatting and lint | `dotnet format` + repo `.editorconfig` |

### Architecture Principles

1. **Immutability at the data layer.** `Board` and `GameState` never mutate;
   every transition returns a new instance.
2. **One-directional dependencies.** `Domain` references nothing.
   `Engine` references Domain. `Uci` and `Api` reference Engine.
   `Persistence` stands alone. The project references enforce it.
3. **Exceptions as the error channel.** The Go port's sentinel errors became
   a `CaroException` hierarchy; one middleware maps them to HTTP statuses.
4. **No DI container ceremony.** Six singletons in `CaroApp.AddCaroApi`; the
   handlers take what they need as constructor parameters.

## Part 1: C# 14 / .NET 10 Features Used Here

### 1.1 Primary constructors

```csharp
public sealed class GameHandlers(GameStore store, MatchStore? matches = null, ILogger? logger = null)
```

The parameters are in scope for the whole class. Used across sessions,
handlers, middleware, and the VCF solver.

### 1.2 Collection expressions

```csharp
List<Position> seed = [];
Position[] line = [new(3, 5), new(4, 5), new(5, 5), new(6, 5), new(7, 5)];
```

### 1.3 File-scoped namespaces and target-typed new

Every file starts `namespace Caro.Engine;` and `new()` appears wherever the
type is obvious from the left side.

### 1.4 `readonly record struct` for value types

```csharp
public readonly record struct Position(int X, int Y);
```

Gives structural equality (`==`, `GetHashCode`) for dictionary keys in the
VCF solver and tests, with no allocation.

### 1.5 `System.Threading.Lock` and `PeriodicTimer`

Session and ponder state guard with `lock` on a `Lock` instance (smaller,
faster than a monitor on `object`). The cleanup service and the search
watchdog use `PeriodicTimer` instead of raw threads.

### 1.6 Spans in the engine hot path

The 11-cell pattern windows live in `Span<sbyte>` buffers, `stackalloc`-ed
once per call site (see `PatternWindow.cs`). No LINQ, no closures, no
per-node allocations inside the search.

## Part 2: Immutable Domain Patterns

### 2.1 Immutable Board

`Board` is a sealed class with `readonly` arrays. `PlaceStone` clones the
cells and bitboards and returns a new board with the Zobrist hash updated
incrementally:

```csharp
Board b2 = b1.PlaceStone(7, 7, Player.Red);   // b1 unchanged
ulong h = b2.Hash;                            // b1.Hash ^ ZobristKey(7,7,Red)
```

The four-`ulong` bitboards (one per 64-cell stripe of the 16x16 board) are
shared with the engine through `BitBoardsFromDomain`.

### 2.2 GameState transitions

`GameState` is immutable; `WithMove` returns the next state (open-rule
checked, win detected upstream), `UndoMove` replays from the board history.
Error paths throw (`GameOverException`, `OpenRuleException`), matching the
Go port's `(GameState, error)` returns.

### 2.3 Domain errors

One exception hierarchy in `CaroException.cs`, message strings byte-equal to
the Go sentinels so persistence rows and error bodies stay stable.

## Part 3: Concurrency Patterns

### 3.1 Lazy SMP worker pool

`ParallelSearch.Run` starts `config.Threads` long-running tasks (dedicated
threads, not thread-pool work), each with its own `SearchBoard` and its own
heuristics snapshot:

```csharp
SearchHeuristics[] workerHeuristics = new SearchHeuristics[numWorkers];
workerHeuristics[0] = heuristics;              // worker 0 evolves the shared one
for (int w = 1; w < numWorkers; w++)
{
    workerHeuristics[w] = heuristics.Clone();  // snapshots BEFORE any worker runs
}
```

Results land in a `ConcurrentBag<ParallelResult>`; the coordinator folds by
(deepest completed depth, then best score). Joins use `Task.WaitAll` inside
the compute context. The async boundary exists only at the HTTP layer, which
`await`s.

### 3.2 The MakeAIMove snapshot-compute-revalidate flow

The expensive search must not hold the session lock for seconds:

1. `ExtractForAI()` snapshots board, side to move, clock, difficulty under
   the lock.
2. The search runs unlocked (`await Task.Run(() => ai.GetBestMove(...))`).
3. `ApplyAIMove(x, y, expectedPlayer)` re-validates the turn under the lock;
   a stale result is rejected with `NotPlayerTurnException`.

The thread budget is computed BEFORE taking the session lock
(`GetOrCreateAI`): the callback locks the store, and the store's
`ActiveGameCount` locks sessions, so the reverse order would deadlock.

### 3.3 Sharded transposition table

16 shards, each a `TtSlot[]` under a `ReaderWriterLockSlim`; writes use the
depth-age replacement priority stamped at `IncrementAge`. Counters are
`Interlocked`.

### 3.4 The L5 background ponderer

`MinimaxAI.StartPonder` runs a long-running task over the predicted reply
position, sharing the AI's TT but not its heuristics or stats. The outcome is
consumed exactly once: `StopPonder` cancels, joins, and takes the stored
outcome atomically; a second call returns false. The session derives the
ponder time cap from the opponent's live clock.

### 3.5 TimeMonitor

One long-running watchdog task sleeps ~10ms per tick and flips the stop flag
at the hard bound. `ShouldStop` checks the flag, the elapsed time, and the
linked cancellation token (external abort or stop) in that order.

## Part 4: Error Handling

### 4.1 Domain errors to HTTP

`ErrorMappingMiddleware` is the single mapping site:

| Exception | Status | Body `error` |
|-----------|--------|--------------|
| `GameNotFoundException` | 404 | `not_found` |
| `TooManyGamesException` | 429 | `too_many_games` |
| `CellOccupied` / `PositionBounds` / `GameOver` / `OpenRule` / `InvalidLevel` | 400 | `bad_request` |
| `NotPlayerTurnException` | 409 | `not_your_turn` |
| anything else | 500 | `internal` |

`NoMovesException` is deliberately unmapped (500), exactly like the Go
original. Malformed JSON bodies answer 400 with the parser's message.

## Part 5: Testing

### 5.1 Test stack

One xUnit project per src project (`backend/tests/Caro.*.Tests`), each with
`InternalsVisibleTo` access. Coverlet collects coverage for the badge.

### 5.2 Table tests become Theory

```csharp
public static TheoryData<int, string, double, double, int, bool, int, int> ProfileCases => new()
{
    { 1, "Novice", 0.04, 0.06, 1, false, 0, 2 },
    ...
};

[Theory]
[MemberData(nameof(ProfileCases))]
public void DifficultyProfileLevels(...)
```

Go's `assert.ErrorIs` maps to `Assert.Throws<T>`; polling with
`require.Eventually` becomes a small `Eventually(condition, timeout, poll)`
helper; concurrency tests use `Parallel.For`.

### 5.3 In-test server

`TestHostFactory.Create()` builds the exact same `WebApplication` the real
host runs (`AddCaroApi` + `UseCaroPipeline`) on a `TestServer`, so route,
middleware, CORS, and JSON-shape tests exercise production wiring. JSON is
asserted as parsed `Dictionary<string, object?>` trees with camelCase keys.

### 5.4 Engine regression suites

`VcfTests`, `SearchTests`, `SearchFixesTests`, `PonderTests` carry over the
Go suite verbatim: VCF chains on real game positions, LMR tactical guards,
mate-score round trips, zero-time fallback ordering, and the ponder
lifecycle. Run them before touching the search.

### 5.5 Running tests

```bash
cd backend
dotnet test Caro.sln                 # all projects
dotnet test tests/Caro.Engine.Tests # one project
dotnet test tests/Caro.Engine.Tests -c Release   # realistic NPS
make test                            # same, via the Makefile
```

## Part 6: Build and Run

```bash
# Build (Release for realistic engine speed)
dotnet build backend/Caro.sln -c Release

# Run the API server (:5207, data/matches.db)
dotnet run --project backend/src/Caro.Server

# Run the standalone UCI engine on stdio
dotnet run --project backend/src/Caro.UciEngine

# Frontend + backend together
node scripts/dev.mjs

# Tournament
node scripts/run-tournament.mjs --games 20 --red 1 --blue 5 --tc 3+2 --seed 20260821
```

Environment: `MATCH_DB_PATH` overrides the SQLite path, `CARO_DISABLE_PONDER=1`
kills pondering process-wide.

## Part 7: Project Dependency Rules

| Project | May reference |
|---------|---------------|
| `Caro.Domain` | nothing |
| `Caro.Engine` | Domain |
| `Caro.Uci` | Domain, Engine |
| `Caro.Persistence` | nothing (only Microsoft.Data.Sqlite) |
| `Caro.Api` | Domain, Engine, Uci, Persistence |
| `Caro.Server` / `Caro.UciEngine` | Api / Uci |

The csproj graph enforces this; a violation is a build error, not a review
comment.

## Sanctioned deviations from the Go engine

The port is a transliteration; these differences are known and accepted:

1. Sort tie order (unstable `List.Sort` vs Go's pdqsort) can reorder equal
   moves, so node counts and scores may drift slightly; fixed-depth parity
   probes show bestmoves matching.
2. Thread scheduling nondeterminism in Lazy SMP (true in Go as well).
3. Exceptions instead of sentinel error returns.
4. The soft time budget gates iteration starts on measured wall-clock, so a
   slow build (Debug) may stop one depth earlier than a fast one (Release).
5. JIT tier-up: first-move NPS is lower until the search loop is hot; warm
   up before speed probes.

## Summary Checklist

- [ ] Ran `dotnet test backend/Caro.sln` and it is green
- [ ] Touched the engine? Read `ENGINE_FEATURES.md` and ran the engine suite
- [ ] Touched serialization? Kept `[JsonPropertyName]` names byte-stable
      (the frontend and Playwright tests depend on them)
- [ ] Touched the statline? `Statline.BuildMoveDetail` output must stay
      byte-identical (tournament tooling parses it)
- [ ] New file under 400 SLOC, `dotnet format` clean
- [ ] Commit follows Conventional Commits with tests included
