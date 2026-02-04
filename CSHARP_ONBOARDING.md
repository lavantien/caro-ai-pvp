# Modern C\# Survival Guide (C\# 14 / .NET 10 Era)

Welcome to the **Caro AI PvP** codebase! This is a grandmaster-level Caro (Gomoku variant) AI built with .NET 10 and Clean Architecture principles.

## Quick Project Overview

### Solution Structure
```
backend/
├── src/
│   ├── Caro.Core.Domain/         # Pure C# entities (no dependencies)
│   ├── Caro.Core.Application/    # DTOs, mappers, interfaces
│   ├── Caro.Core.Infrastructure/ # AI algorithms, persistence
│   ├── Caro.Api/               # Web API + SignalR hub
│   ├── Caro.BookBuilder/        # Opening book generation CLI
│   └── Caro.TournamentRunner/   # AI strength validation tests
└── tests/
    ├── Caro.Core.Domain.Tests/     # 85 unit tests
    ├── Caro.Core.Application.Tests/ # 8 unit tests
    ├── Caro.Core.Infrastructure.Tests/ # 48 unit/integration tests
    └── Caro.Core.Tests/         # 484 mixed tests (AI/tournament/concurrency)
```

### Key Technologies
- .NET 10, C# 14
- ASP.NET Core 10 + SignalR
- xUnit 2.9.2, Moq 4.20.72, FluentAssertions 7.0.0-8.8.0
- 660+ total tests (583 backend unit + 44 integration + 30 concurrency + 17 statistical + 19 frontend unit + 17 frontend E2E)

### Architecture Principles
This codebase follows **Clean Architecture** with a "Functional Core, Imperative Shell" approach:

- **Immutability**: Data cannot change once created (use `record` types)
- **Statelessness**: AI logic has no mutable instance variables
- **Dependency Injection**: All dependencies injected via constructor parameters
- **Layered Design**: Domain → Application → Infrastructure clear separation



## Part 1: Immutability (The "Data" Layer)

In this codebase, you rarely see standard properties with get; set;. Instead, we model data as snapshots that cannot change once created.

### 1. The record Type

The record keyword is the cornerstone of immutable C\#. It provides value-based equality (two different objects with the same data are considered "equal") and concise syntax.

**Old Way (Class with setters):**

```csharp
public class User
{
public int Id { get; set; } // Mutable! Dangerous in distributed systems.
public string Username { get; set; }
}
```

**The Modern Way (Positional Records):**

This uses **Primary Constructors**. The compiler automatically generates properties that are init-only (immutable after initialization).

```csharp
// Definition
public record User(int Id, string Username, string Email);
// Usage
var user1 = new User(1, "jdoe", "jdoe@example.com");
// user1.Username = "admin"; // COMPILER ERROR: Property is init-only.
```

### 2. Non-Destructive Mutation (with)

Since you can't change user1, how do you update the email? You don't. You create a _new_ copy with the specific change using the with expression.

```csharp
var updatedUser = user1 with { Email = "new.email@example.com" };

// Result:

// user1.Email is STILL "jdoe@example.com" (Safety!)
// updatedUser.Email is "new.email@example.com"
```

### 3. Collection Expressions

In modern C\#, we avoid new List<int> { 1, 2, 3 }. We use the cleaner collection expression syntax [].

```csharp
// Immutable Array initialization
ImmutableArray<int> numbers = [1, 2, 3, 4, 5];
```

## Part 2: Statelessness & Dependency Injection (The "Logic" Layer)

Logic lives in "Services" or "Handlers". These classes hold no state (no private variables that change value). They only hold _dependencies_.

### 1. Primary Constructors for DI

In older C\#, injecting dependencies required a lot of boilerplate code. Modern C\# reduces this drastically.

**Old Way:**

```csharp
public class GameStateService
{
 private readonly ILogger _logger;
 private readonly IGameRepository _repo;
 public GameStateService(ILogger<GameStateService> logger, IGameRepository repo)
 {
 _logger = logger;
 _repo = repo;
 }
}
```

**The Modern Way (Primary Constructors):**

We declare dependencies directly in the class signature.

```csharp
// The parameters 'logger' and 'store' are available throughout the class
public class StatelessSearchEngine(ILogger<StatelessSearchEngine> logger) :
 IAIEngine
{
 private const int WinLength = 5;

 public (int x, int y, int score) FindBestMove(
 GameState state,
 AIDifficulty difficulty,
 CancellationToken ct = default)
 {
 logger.LogDebug("Searching with difficulty {Difficulty}", difficulty);
 // Pure logic: State -> Options -> Best Move

 // Stateless design - no mutable instance variables
 var emptyCells = GetOrderedMoves(state, difficulty);
 // ... search logic

 return (bestX, bestY, bestScore);
 }
}
```

### 2. Service Lifetimes

When registering these in Program.cs, you define how long they live.
- **Transient (AddTransient)** : Created _every time_ it is requested. (Lightweight, stateless).
- **Scoped (AddScoped)** : Created once per _HTTP Request_. (Standard for web APIs).
- **Singleton (AddSingleton)** : Created once per _Application lifetime_. (Be careful! Not thread-safe if you hold state).

```csharp
// Program.cs
var builder = WebApplication.CreateBuilder(args);

// Register AI services
builder.Services.AddScoped<IBookGenerator, OpeningBookGenerator>();
builder.Services.AddSingleton<TournamentManager>();
builder.Services.AddSignalR();

// Register repositories
builder.Services.AddScoped<IGameRepository, InMemoryGameRepository>();
```

## Part 3: Testing (xUnit)

We use **xUnit**. It is the industry standard for .NET.

### Test Stack

| Component | Version | Purpose |
|-----------|----------|---------|
| xUnit | 2.9.2 | Test framework |
| xUnit Runner Visual Studio | 3.1.4 | Test runner |
| FluentAssertions | 7.0.0-8.8.0 | Assertion library |
| Moq | 4.20.72 | Mocking library |
| Coverlet | 6.0.4 | Code coverage |

### 1. The Attributes

- [Fact]: A test that is always true. It takes no arguments. Used for invariant logic.
- [Theory]: A test that is true for a specific set of data. It takes arguments.
- [Trait("Category", "Integration")]: Marks integration tests that require external dependencies.

### 2. AAA Pattern (Arrange, Act, Assert)

Every test method should be visually split into these three sections.

### 3. Practical Examples

#### Example 1: Testing Immutable Records (Domain Layer)

**The Code to Test:**

```csharp
// Position.cs - Immutable record
public record Position(int X, int Y)
{
 public bool IsValid => X >= 0 && X < 19 && Y >= 0 && Y < 19;
}
```

**The Test Class:**

```csharp
using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;

public class PositionTests
{
 [Fact]
 public void Invalid_Position_IsInvalid()
 {
 // Arrange
 var position = new Position(-1, 0);

 // Act & Assert
 position.IsValid.Should().BeFalse();
 }

 [Fact]
 public void InRange_Position_IsValid()
 {
 // Arrange
 var positions = new[]
 {
 new Position(0, 0),
 new Position(9, 9),
 new Position(18, 18)
 };

 // Act & Assert
 foreach (var pos in positions)
 {
 pos.IsValid.Should().BeTrue();
 }
 }
}
```

#### Example 2: Testing with [Theory] and Data

```csharp
using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;

public class PlayerTests
{
 [Theory]
 [InlineData(Player.Red, true)]
 [InlineData(Player.Blue, true)]
 [InlineData(Player.None, false)]
 public void Player_IsValid_WhenHasValue(Player player, bool expected)
 {
 // Act
 var isValid = player != Player.None;

 // Assert
 isValid.Should().Be(expected);
 }
}
```

#### Example 3: Testing Non-Destructive Mutation

```csharp
using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;

public class GameStateTests
{
 [Fact]
 public void MakeMove_ReturnsNewState_WithoutMutatingOriginal()
 {
 // Arrange
 var originalState = GameState.CreateInitial();

 // Act
 var newState = originalState.MakeMove(9, 9);

 // Assert - original unchanged
 originalState.MoveNumber.Should().Be(0);
 originalState.Board.IsEmpty().Should().BeTrue();

 // Assert - new state updated
 newState.MoveNumber.Should().Be(1);
 newState.Board.GetCell(9, 9).Player.Should().Be(Player.Red);
 }
}
```

### 4. Mocking Dependencies

Since your codebase uses DI, your services depend on Interfaces. In unit tests, we "mock" these to isolate the class we are testing.

**We use Moq 4.20.72** (not NSubstitute).

```csharp
using Xunit;
using FluentAssertions;
using Moq;
using Caro.Core.Infrastructure.Persistence;

public class AIServiceTests
{
 [Fact]
 public async Task GetBestMove_ShouldCallStore_WhenMoveRequested()
 {
 // Arrange
 var mockStore = new Mock<IGameRepository>();
 var mockLogger = new Mock<ILogger<AIService>>();

 // Setup mock behavior
 mockStore
 .Setup(x => x.GetLastMoveAsync(It.IsAny<string>()))
 .ReturnsAsync((9, 9));

 var service = new AIService(mockStore.Object, mockLogger.Object);

 // Act
 var result = await service.GetBestMoveAsync("game-123");

 // Assert - verify interaction
 mockStore.Verify(x => x.GetLastMoveAsync("game-123"), Times.Once);
 result.X.Should().Be(9);
 result.Y.Should().Be(9);
 }
}
```

### 5. Project-Specific Testing Patterns

#### Testing Value Objects (BitBoard)

```csharp
[Fact]
public void BitBoard_SetBit_UpdatesCorrectly()
{
 // Arrange
 var board = new BitBoard();

 // Act
 board.SetBit(9, 9);

 // Assert
 board.GetBit(9, 9).Should().BeTrue();
 board.GetBit(0, 0).Should().BeFalse();
 board.CountBits().Should().Be(1);
}
```

#### Testing AI Algorithms

```csharp
[Theory]
[InlineData(AIDifficulty.Easy, 3)]
[InlineData(AIDifficulty.Medium, 5)]
[InlineData(AIDifficulty.Hard, 7)]
public void SearchDepth_CorrelatesWithDifficulty(AIDifficulty difficulty, int expectedDepth)
{
 // Arrange
 var state = GameState.CreateInitial();
 var ai = new MinimaxAI();

 // Act
 var (x, y, depth) = ai.GetBestMove(state, difficulty);

 // Assert
 depth.Should().BeGreaterOrEqualTo(expectedDepth);
}
```

#### Testing Concurrency

```csharp
[Fact]
public async Task ConcurrentMoves_DoNotCauseRaceConditions()
{
 // Arrange
 var state = GameState.CreateInitial();
 var tasks = new List<Task>();

 // Act - simulate concurrent access
 for (int i = 0; i < 100; i++)
 {
 tasks.Add(Task.Run(() =>
 {
 // Each thread gets its own state copy
 var threadState = state.MakeMove(i % 19, i % 19);
 return threadState;
 }));
 }

 await Task.WhenAll(tasks);

 // Assert - original state unchanged (immutable)
 state.MoveNumber.Should().Be(0);
 state.Board.IsEmpty().Should().BeTrue();
}
```

## Part 4: Clean Architecture Context

This codebase follows Clean Architecture with three core layers:

### 1. Domain Layer (`Caro.Core.Domain`)
- **Purpose**: Core business logic, entities, value objects
- **Dependencies**: None (pure C#)
- **Key Types**:
  - Records: `GameState`, `Position`, `Player`
  - Value Objects: `BitBoard`, `ZobristHash`
  - No framework dependencies

**Testing**: Unit tests only (no mocking needed)
- Project: `Caro.Core.Domain.Tests`
- Example: Test record immutability, value object operations

### 2. Application Layer (`Caro.Core.Application`)
- **Purpose**: Application services, DTOs, mappers
- **Dependencies**: Domain layer
- **Key Types**:
  - Interfaces: `IStatsPublisher`, `ITimeManager`
  - DTOs: `GameDTO`, `MoveDTO`
  - Mappers: `GameMapper`

**Testing**: Unit tests with mocking
- Project: `Caro.Core.Application.Tests`
- Mock: Domain entities (using FluentAssertions assertions)

### 3. Infrastructure Layer (`Caro.Core.Infrastructure`)
- **Purpose**: External concerns, AI algorithms, persistence
- **Dependencies**: Domain, Application layers
- **Key Types**:
  - AI Engine: `StatelessSearchEngine`, `MinimaxAI`
  - Services: `AIService`, `TimeManagementService`
  - Persistence: `InMemoryGameRepository`

**Testing**: Unit and integration tests
- Project: `Caro.Core.Infrastructure.Tests`
- Mock: External dependencies (file system, database, timers)

### Test Project Organization

| Test Project | Source Project | Test Type | Count |
|-------------|----------------|------------|--------|
| Caro.Core.Domain.Tests | Caro.Core.Domain | Unit | 85 |
| Caro.Core.Application.Tests | Caro.Core.Application | Unit | 8 |
| Caro.Core.Infrastructure.Tests | Caro.Core.Infrastructure | Unit/Integration | 48 |
| Caro.Core.Tests | Caro.Core (GameLogic/Tournament/Concurrency) | Mixed | 484 |

## Summary Checklist for your Onboarding

1. **Understand Clean Architecture layers** : Domain → Application → Infrastructure
2. **Locate test projects** : Find tests in Caro.Core.Domain.Tests, Caro.Core.Application.Tests, etc.
3. **Review record definitions** : Understand immutable data shapes (GameState, Position, Player)
4. **Identify with-expression mutations** : Look for non-destructive state updates
5. **Check Program.cs** : See how services are wired (Transient/Scoped/Singleton)
6. **Run Tests** : Use `dotnet test` in backend/ or Test Explorer
   - Caro.Core.Domain.Tests: 85 unit tests (no mocking needed)
   - Caro.Core.Application.Tests: 8 unit tests (mock DTOs)
   - Caro.Core.Infrastructure.Tests: 48 unit/integration tests
   - Caro.Core.Tests: 484 mixed tests (AI, tournament, concurrency)
7. **When tests fail** : Check [Theory] inline data to see which input caused crash
8. **Mocking** : Use Moq 4.20.72, FluentAssertions 7.0.0-8.8.0


