# Checkpoint: v1.63.0 Development

## Summary

Reduced board size from 32x32 to 16x16 and implemented SearchBoard with make/unmake pattern for 115x performance improvement in AI search.

## Changes

### Board Size Reduction (32x32 → 16x16)

- Board.cs, BitBoard.cs, GameConstants.cs updated for 16x16 dimensions
- BitBoard now uses 4 ulongs (256 bits) instead of 16 ulongs (1024 bits)
- Center position changed from (16, 16) to (8, 8)
- Search radius adjusted for smaller board

### SearchBoard Implementation

Mutable board representation with make/unmake pattern:

| Method | Description | Allocation |
|--------|-------------|------------|
| `MakeMove(x, y, player)` | Place stone, return undo info | Zero |
| `UnmakeMove(undo)` | Restore board state | Zero |
| `GetHash()` | Incrementally maintained hash | Zero |
| `GetBitBoard(player)` | Get bitboard copy | BitBoard struct |

### Performance Results

```
=== SearchBoard Performance Benchmark ===
Iterations: 1000
Depth: 4, Moves per depth: 10
Immutable Board: 115ms (115.00μs/iter)
Mutable SearchBoard: 1ms (1.00μs/iter)
Speedup: 115.00x
```

### Files Created

| File | Purpose |
|------|---------|
| `SearchBoard.cs` | Mutable board with make/unmake pattern |
| `SearchBoardExtensions.cs` | Helper extension methods |
| `SearchBoardTests.cs` | 19 unit tests |
| `SearchBoardExtensionsTests.cs` | 12 unit tests |
| `SearchBoardBenchmarks.cs` | Performance benchmark |

### Files Modified

| File | Change |
|------|--------|
| `MinimaxAI.cs` | Added MinimaxCore/QuiesceCore with SearchBoard; helper method overloads |
| `BitBoardEvaluator.cs` | Added SearchBoard overload |
| `BoardEvaluator.cs` | Added SearchBoard overload |
| `Board.cs` | Updated for 16x16 board |
| `BitBoard.cs` | Updated for 4-ulong layout |
| `GameConstants.cs` | BoardSize=16, CenterPosition=8 |

## Design Insights

### Why Make/Unmake is Faster

- Immutable Board.PlaceStone copies 256 cells on every move
- SearchBoard.MakeMove modifies in-place, returns 16-byte undo struct
- No heap allocations during search
- Hash incrementally maintained (XOR is its own inverse)

### Hash Compatibility

SearchBoard uses same hash formula as Board for TT compatibility:
```
pieceKey = (x << 8) | y) ^ (player-specific mask)
hash ^= pieceKey  // on make and unmake
```

## Version

- Target: v1.63.0
- Previous: v1.62.0
