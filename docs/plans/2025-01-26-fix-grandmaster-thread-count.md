# Fix Grandmaster Thread Count Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix Grandmaster AI to use single-threaded search (as configured) instead of buggy parallel search

**Architecture:** The issue is a disconnect between AdaptiveDepthCalculator.GetThreadCount() (which returns 0 for Grandmaster) and ParallelMinimaxSearch (which ignores this setting). We'll pass the thread count through the call chain so Grandmaster actually uses single-threaded search.

**Tech Stack:** C# .NET 8, MinimaxAI, ParallelMinimaxSearch, AdaptiveDepthCalculator

---

## Background

Current state:
- `AdaptiveDepthCalculator.GetThreadCount(Grandmaster)` returns `0` (single-threaded)
- But `MinimaxAI.cs` line 310: `bool useParallelSearch = difficulty >= AIDifficulty.Grandmaster` enables parallel for Grandmaster
- `ParallelMinimaxSearch.SearchLazySMP()` calculates its own thread count: `Math.Min(_maxThreads, Math.Max(2, depth / 2))`
- For depth 9-11, this creates 4-5 threads
- Result: Grandmaster uses buggy parallel search and loses to Hard/Medium

Fix:
- Pass threadCount from AdaptiveDepthCalculator through the call chain
- Handle threadCount=0 as single-threaded fallback in SearchLazySMP

---

### Task 1: Modify GetBestMoveWithStats signature in ParallelMinimaxSearch

**Files:**
- Modify: `backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs:192-198`

**Step 1: Read the current method signature**

Run: Read the file to see current signature
Expected: Method has 6 parameters without `fixedThreadCount`

**Step 2: Add fixedThreadCount parameter**

Find this code (around line 192):
```csharp
public ParallelSearchResult GetBestMoveWithStats(
    Board board,
    Player player,
    AIDifficulty difficulty,
    long? timeRemainingMs = null,
    TimeAllocation? timeAlloc = null,
    int moveNumber = 0)
```

Replace with:
```csharp
public ParallelSearchResult GetBestMoveWithStats(
    Board board,
    Player player,
    AIDifficulty difficulty,
    long? timeRemainingMs = null,
    TimeAllocation? timeAlloc = null,
    int moveNumber = 0,
    int fixedThreadCount = -1)
```

**Step 3: Build to verify syntax**

Run: `cd backend && dotnet build`
Expected: No errors (parameter is optional with default -1)

**Step 4: Commit**

```bash
git add backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs
git commit -m "feat: add fixedThreadCount parameter to GetBestMoveWithStats"
```

---

### Task 2: Pass fixedThreadCount to SearchLazySMP

**Files:**
- Modify: `backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs:275`

**Step 1: Find the SearchLazySMP call**

Find this code (around line 275):
```csharp
// Multi-threaded Lazy SMP for deeper searches
return SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc);
```

**Step 2: Add fixedThreadCount parameter**

Replace with:
```csharp
// Multi-threaded Lazy SMP for deeper searches
return SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc, fixedThreadCount);
```

**Step 3: Build to verify**

Run: `cd backend && dotnet build`
Expected: No errors (SearchLazySMP already has fixedThreadCount parameter)

**Step 4: Commit**

```bash
git add backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs
git commit -m "feat: pass fixedThreadCount to SearchLazySMP"
```

---

### Task 3: Handle threadCount=0 as single-threaded in SearchLazySMP

**Files:**
- Modify: `backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs:334-340`

**Step 1: Read current threadCount calculation**

Find this code (around line 334):
```csharp
// Number of threads based on depth and available cores
// FIX: Use fixed thread count when provided to reduce non-determinism
int threadCount = fixedThreadCount > 0
    ? fixedThreadCount
    : Math.Min(_maxThreads, Math.Max(2, depth / 2));
```

**Step 2: Update to handle threadCount=0**

Replace with:
```csharp
// Number of threads based on depth and available cores
// FIX: Use fixed thread count when provided to reduce non-determinism
// fixedThreadCount = 0 means single-threaded (skip parallel search)
int threadCount = fixedThreadCount >= 0
    ? fixedThreadCount  // 0 = single-threaded, >0 = use that many threads
    : Math.Min(_maxThreads, Math.Max(2, depth / 2));

// If threadCount is 0, fall back to single-threaded search
if (threadCount == 0)
{
    Interlocked.Exchange(ref _realNodesSearched, 0);
    _transpositionTable.IncrementAge();

    var (x, y) = SearchSingleThreaded(board, player, depth, candidates);
    long actualNodes = Interlocked.Read(ref _realNodesSearched);
    return new ParallelSearchResult(x, y, depth, actualNodes, 0);
}
```

**Step 3: Build to verify**

Run: `cd backend && dotnet build`
Expected: No errors

**Step 4: Commit**

```bash
git add backend/src/Caro.Core/GameLogic/ParallelMinimaxSearch.cs
git commit -m "feat: handle threadCount=0 as single-threaded fallback in SearchLazySMP"
```

---

### Task 4: Pass threadCount from MinimaxAI to GetBestMoveWithStats

**Files:**
- Modify: `backend/src/Caro.Core/GameLogic/MinimaxAI.cs:316-329`

**Step 1: Find the parallel search call**

Find this code (around line 316-329):
```csharp
if (useParallelSearch)
{
    // Track time for parallel search
    var parallelStopwatch = System.Diagnostics.Stopwatch.StartNew();

    // Use Lazy SMP parallel search
    var parallelResult = _parallelSearch.GetBestMoveWithStats(
        board,
        player,
        difficulty,
        timeRemainingMs,
        timeAlloc,
        moveNumber
    );
```

**Step 2: Add threadCount retrieval and pass it**

Replace with:
```csharp
if (useParallelSearch)
{
    // Get thread count from AdaptiveDepthCalculator
    int threadCount = AdaptiveDepthCalculator.GetThreadCount(difficulty);

    // Track time for parallel search
    var parallelStopwatch = System.Diagnostics.Stopwatch.StartNew();

    // Use Lazy SMP parallel search
    var parallelResult = _parallelSearch.GetBestMoveWithStats(
        board,
        player,
        difficulty,
        timeRemainingMs,
        timeAlloc,
        moveNumber,
        threadCount
    );
```

**Step 3: Build to verify**

Run: `cd backend && dotnet build`
Expected: No errors

**Step 4: Commit**

```bash
git add backend/src/Caro.Core/GameLogic/MinimaxAI.cs
git commit -m "feat: pass threadCount from AdaptiveDepthCalculator to parallel search"
```

---

### Task 5: Verify the fix works

**Files:**
- No file changes
- Test: Run diagnostic tool

**Step 1: Build the project**

Run: `cd backend && dotnet build`
Expected: Build succeeds with no warnings

**Step 2: Run baseline stats test**

Run: `dotnet run --project src/Caro.Diagnostic -- --matchups 6 --games 10 --time "7+5"`
Expected: Tests run (will take several minutes)

**Step 3: Verify results**

Expected outcomes:
- Grandmaster vs Hard: Grandmaster wins (8-2 or better)
- Hard vs Medium: Hard wins
- Medium vs Easy: Medium wins
- Easy vs Braindead: Easy wins
- Grandmaster vs Medium: Grandmaster dominates

**Step 4: If tests pass, commit verification**

```bash
git add docs/plans/2025-01-26-fix-grandmaster-thread-count.md
git commit -m "docs: add Grandmaster thread count fix plan"
```

---

## Notes

- The key insight is that `fixedThreadCount > 0` was excluding `0`, so thread count 0 (single-threaded) was falling through to the default calculation
- Changed to `fixedThreadCount >= 0` to explicitly handle the 0 case
- Added explicit fallback to `SearchSingleThreaded` when threadCount is 0
- This preserves the ability to use parallel search for other difficulties while keeping Grandmaster single-threaded

## Related Documentation

- `backend/src/Caro.Core/GameLogic/AdaptiveDepthCalculator.cs` - Defines thread count per difficulty
- `backend/src/Caro.Core/GameLogic/ThreadPoolConfig.cs` - Default Lazy SMP thread calculation
- Commit 1bda783 - Previous fix that disabled parallel search for Hard
