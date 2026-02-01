# Modern C\# Survival Guide (C\# 14 / .NET 10 Era)

Welcome to the codebase! You are entering an environment that likely follows a "Functional Core, Imperative Shell" architecture. This is very different from "classic" Object-Oriented C\#. We prioritize data consistency (Immutability), side-effect-free logic (Statelessness), and robust modularity (DI).

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
public class OrderService
{
private readonly IRepository _repo;
public OrderService(IRepository repo) // Constructor
{
_repo = repo;
}
}
```

**The Modern Way (Primary Constructors):**

We declare dependencies directly in the class signature.

```csharp
// The parameters 'repo' and 'logger' are available throughout the class
public class OrderService(IOrderRepository repo, ILogger<OrderService> logger) :
IOrderService
{
public async Task ProcessOrderAsync(Order order)
{
logger.LogInformation("Processing order {Id}", order.Id);
// Pure logic: Inputs -> Processing -> Output


var processedOrder = order with { Status = OrderStatus.Processed };
await repo.SaveAsync(processedOrder);
}
}
```

### 2. Service Lifetimes

When registering these in Program.cs, you define how long they live.
- **Transient (AddTransient)** : Created _every time_ it is requested. (Lightweight, stateless).
- **Scoped (AddScoped)** : Created once per _HTTP Request_. (Standard for web APIs).
- **Singleton (AddSingleton)** : Created once per _Application lifetime_. (Be careful! Not thread-safe if you hold state).

```cshsarp
// Program.cs
var builder = WebApplication.CreateBuilder(args);
// Registering our service
builder.Services.AddScoped<IOrderService, OrderService>();
```

## Part 3: Testing (xUnit)

We use **xUnit**. It is the industry standard for .NET.

### 1. The Attributes

- [Fact]: A test that is always true. It takes no arguments. Used for invariant logic.
- [Theory]: A test that is true for a specific set of data. It takes arguments.

### 2. AAA Pattern (Arrange, Act, Assert)

Every test method should be visually split into these three sections.

### 3. Practical Example

Let's test a simple calculator logic.

**The Code to Test:**

```csharp
public record Calculator
{
public int Add(int a, int b) => a + b;
public double Divide(double a, double b)
{
if (b == 0) throw new DivideByZeroException();
return a / b;
}
}
```

**The Test Class:**

```csharp
using Xunit;
using FluentAssertions; // Highly recommended library for readable assertions
public class CalculatorTests
{
// [Fact]: Single scenario
[Fact]
public void Divide_ShouldThrowException_WhenDivisorIsZero()
{
// Arrange (Setup objects)
var calculator = new Calculator();
// Act (Execute the method)
// For exceptions, we record the action
Action act = () => calculator.Divide(10, 0);
// Assert (Verify results)
act.Should().Throw<DivideByZeroException>();
}
// [Theory]: Multiple data scenarios
[Theory]
[InlineData(1, 2, 3)] // Case 1
[InlineData(-1, -1, -2)] // Case 2
[InlineData(100, 50, 150)] // Case 3
public void Add_ShouldReturnSum_WhenNumbersAreValid(int a, int b, int expected)
{
// Arrange
var calculator = new Calculator();
// Act
var result = calculator.Add(a, b);
// Assert
result.Should().Be(expected);
}
}
```

### 4. Mocking Dependencies

Since your codebase uses DI, your services depend on Interfaces (IRepository, etc.). In unit tests, we "mock" these to isolate the class we are testing.

*Common library: NSubstitute or Moq.*

```csharp
[Fact]
public async Task ProcessOrder_ShouldCallSave_WhenOrderIsValid()
{
// Arrange
var mockRepo = Substitute.For<IOrderRepository>(); // Fake repository
var service = new OrderService(mockRepo); // Inject fake
var order = new Order(1, "Book");
// Act
await service.ProcessOrderAsync(order);
// Assert
// Verify that the repository's Save method was actually called
await mockRepo.Received(1).SaveAsync(Arg.Any<Order>());
}
```

## Summary Checklist for your Onboarding

1. **Look for record definitions** : Understand the data shapes.
2. **Identify the Aggregates** : Look for the root objects being mutated via with.
3. **Check Program.cs** : See how services are wired (Transient vs Scoped).
4. **Run the Tests** : Open the Test Explorer. If a test fails, look at the [Theory] data to see  *which* specific input caused the crash.


