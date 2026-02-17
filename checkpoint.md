# Checkpoint: v1.61.0 Development

## Summary

Added comprehensive move type tracking to AI engine. Each move now reports how it was determined (search, book, immediate win/block, error rate). Improved tournament output format with compact stat lines.

## Changes

### Move Type Tracking

Added `MoveType` enum to track move determination:

| Type | Code | Description |
|------|------|-------------|
| Normal | `-` | Full search performed |
| Book | `Bk` | Opening book move (unvalidated) |
| BookValidated | `Bv` | Book move validated by search |
| ImmediateWin | `Wn` | Instant winning move (no search) |
| ImmediateBlock | `Bl` | Forced block of opponent threat |
| ErrorRate | `Er` | Random move (Braindead 10% error) |
| CenterMove | `Ct` | Center opening move |
| Emergency | `Em` | Emergency mode (low time) |

### Output Format Improvements

Before:
```
G1 M1 | R(16,16) by Easy | T: 69ms/0ms | B | Th: 3 | D1        | N:                    4 | NPS:                 71 | ...
```

After:
```
G1 M1 | R(16,16) by Easy | T: 69ms/0ms | Bk | Th: 3 | D1 | N: 4        | NPS: 71       | ...
```

- Replaced book-only column with move type column
- Reduced depth column: 9 → 3 chars
- Reduced nodes column: 20 → 8 chars
- Reduced NPS column: 20 → 8 chars

### Files Modified

| File | Change |
|------|--------|
| `StatsChannel.cs` | Added `MoveType` enum |
| `TournamentState.cs` | Added `MoveType` to `MoveStats` record |
| `MinimaxAI.cs` | Added `_moveType` tracking; set in all early exit paths |
| `TournamentEngine.cs` | Pass move type when creating `MoveStats` |
| `GameStatsFormatter.cs` | Display move type with short codes; compact columns |
| `UCISearchController.cs` | Updated tuple deconstruction |
| `OpeningBookGenerator.cs` | Updated tuple deconstruction |

## Design Insights

### Why Braindead Has Low Node Counts

Braindead frequently shows 1-5 nodes because:
1. **Immediate block detection** - If opponent has winning threat, block instantly (no search)
2. **Error rate (10%)** - Random moves skip search entirely
3. **Late game positions** - More likely to have immediate threats

### 0ms Allocated Time

Early exit paths set `_lastAllocatedTimeMs = 0` because no search time was allocated - the move was determined instantly. The actual time shown is overhead of checking threats, not search time.

## Version

- Target: v1.61.0
- Previous: v1.60.0
