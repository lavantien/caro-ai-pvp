# Checkpoint: v1.68.0 Development

## Summary

Opening Book Builder overhaul: memory management, variable depth search, VCF integration, self-play generation, and in-memory lookup.

## Progress

### Major Features Implemented

1. **Streaming Batch Processing**
   - Batch size: 65,536 (power of 2)
   - Memory-bounded generation (no more OOM at depth 10+)
   - Incremental SQLite writes with progress tracking

2. **Variable Depth Search**
   - Plies 0-8: VCF solving (20-30 ply depth, 8 moves/position)
   - Plies 8-16: Deep search (14-20 ply depth, 4 moves/position)
   - Plies 16+: Self-play only (high win-rate moves)

3. **Move Classification (MoveSource enum)**
   - `Solved` - Proven wins via VCF/VCT solver
   - `Learned` - Deep search evaluation
   - `SelfPlay` - Engine vs engine game results

4. **In-Memory Lookup**
   - `InMemoryOpeningBook` - ConcurrentDictionary-based fast lookup
   - `InMemoryBookStore` - Adapter implementing IOpeningBookStore
   - Performance: 40K+ lookups/sec (~24μs per lookup)

5. **Self-Play Generation**
   - `SelfPlayGenerator` - Engine vs engine games
   - CLI: `--self-play`, `--time-control`, `--max-moves`

6. **MinimaxAI Integration**
   - `LoadOpeningBook(IOpeningBookStore)` method
   - `CheckOpeningBook(Board, Player)` method

7. **CLI Enhancements**
   - `--resume` flag for continuation after interruption
   - `--depth`, `--moves` configurable parameters

### Files Modified

| File | Change |
|------|--------|
| `OpeningBookGenerator.cs` | Streaming batches, variable depth, VCF integration |
| `SqliteOpeningBookStore.cs` | Progress tracking, schema v3 |
| `IOpeningBookStore.cs` | New method signatures |
| `OpeningBookEntry.cs` | MoveSource enum, BookMove enhancements |
| `MinimaxAI.cs` | LoadOpeningBook, CheckOpeningBook methods |
| `Program.cs (BookBuilder)` | --resume, --self-play CLI flags |

### Files Created

| File | Purpose |
|------|---------|
| `InMemoryOpeningBook.cs` | Fast in-memory book lookup |
| `InMemoryBookStore.cs` | Adapter for IOpeningBookStore |
| `SelfPlayGenerator.cs` | Engine vs engine game generation |
| `InMemoryOpeningBookTests.cs` | Unit tests for in-memory book |
| `SelfPlayGeneratorTests.cs` | Unit tests for self-play |
| `InMemoryBookPerformanceTests.cs` | Performance verification |

### Test Results

- **Total tests**: 701 (637 + 64)
- **All passing**: Yes
- **Performance tests**: 3 new tests for lookup speed

### Verification Tests

| Test | Result | Details |
|------|--------|---------|
| Memory/Batch | PASS | Streaming batches, 90%+ prune rate |
| Continuation | PASS | Progress saved/resumed correctly |
| Self-Play | PASS | Engine vs engine games completed |
| Performance | PASS | 40K+ lookups/sec (~24μs) |

## Version

- Target: v1.68.0
- Previous: v1.67.0
