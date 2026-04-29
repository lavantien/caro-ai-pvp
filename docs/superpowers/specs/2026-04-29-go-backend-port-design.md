# Go Backend Port Design

Port the C# backend (Caro AI PvP) to Go 1.26 using Go-native idioms.

## Decisions

- **API**: Redesigned with RESTful collection naming, snake_case JSON, Go-style error responses. Frontend will need updates.
- **Architecture**: Go-idiomatic flat layout (`internal/domain`, `internal/engine`, `internal/api`, `internal/uci`, `internal/persistence`, `cmd/`)
- **Build order**: API layer first, then AI engine
- **SIMD**: Use experimental `simd/archsimd` package for BitBoard evaluation where available
- **Persistence**: SQLite + FTS5 via `mattn/go-sqlite3` (CGO_ENABLED=1)
- **Location**: Replace `backend/` directory
- **Approach**: Go-native redesign (Approach 2)

## Go 1.26 Features Used

| Feature | Usage |
|---------|-------|
| Green Tea GC | Automatic - 10-40% GC overhead reduction |
| `errors.AsType[T]()` | Type-safe error matching in API handlers |
| Expression-based `new()` | Optional struct fields (difficulty, etc.) |
| Self-referential generics | Type constraints for engine generics |
| Goroutine leak detection | Debug builds with `GOEXPERIMENT=goroutineleakprofile` |
| `net/http.ServeMux` method matching | Native routing, no third-party router |
| `log/slog` | Structured logging (replaces `ILogger<T>`) |
| `context.Context` | Cancellation propagation from HTTP to AI search |
| `experimental/simd` | BitBoard pattern evaluation vectorization |

## Project Structure

```
backend/
├── go.mod                          # module caro-ai-pvp
├── go.sum
├── cmd/
│   ├── server/main.go              # API server entry point
│   └── engine/main.go              # Standalone UCI console engine
├── internal/
│   ├── domain/                     # Pure game domain (no dependencies)
│   │   ├── board.go                # Immutable board with bitboard
│   │   ├── board_test.go
│   │   ├── game.go                 # GameState, state transitions
│   │   ├── game_test.go
│   │   ├── player.go               # Player enum + Opponent()
│   │   ├── position.go             # Position value type
│   │   ├── zobrist.go              # Hash key generation
│   │   ├── constants.go            # BoardSize, WinLength, directions
│   │   ├── errors.go               # Domain errors (ErrCellOccupied, etc.)
│   │   └── win.go                  # WinDetector, OpenRuleValidator
│   ├── engine/                     # AI engine (depends on domain)
│   │   ├── minimax.go              # MinimaxAI entry point
│   │   ├── search.go               # Iterative deepening + PVS + alpha-beta
│   │   ├── search_test.go
│   │   ├── parallel.go             # Lazy SMP with goroutine pool
│   │   ├── evaluation.go           # BitBoard evaluator interface
│   │   ├── evaluation_test.go
│   │   ├── evaluation_simd.go      # SIMD-optimized eval (GOEXPERIMENT=simd)
│   │   ├── evaluation_scalar.go    # Scalar fallback (default)
│   │   ├── transposition.go        # Lock-free sharded TT with SeqLock
│   │   ├── transposition_test.go
│   │   ├── movepicker.go           # Staged move ordering
│   │   ├── candidate.go            # Candidate generation
│   │   ├── vcf.go                  # VCF solver (20% of allocated time)
│   │   ├── vcf_test.go
│   │   ├── heuristics.go           # Killer moves, history, continuation
│   │   ├── timemanager.go          # PID time management
│   │   ├── timemonitor.go          # Search time monitoring
│   │   ├── difficulty.go           # Difficulty profiles L1-L5
│   │   ├── searchboard.go          # Mutable board for search hot path
│   │   └── bitboard.go             # BitBoard type + operations
│   ├── uci/                        # UCI protocol layer
│   │   ├── handler.go              # UCI command dispatcher
│   │   ├── handler_test.go
│   │   ├── notation.go             # Move notation conversion
│   │   ├── position.go             # Position parsing
│   │   └── options.go              # Engine options
│   ├── api/                        # HTTP/WebSocket API
│   │   ├── server.go               # ServeMux setup, middleware
│   │   ├── handlers.go             # REST endpoint handlers
│   │   ├── handlers_test.go
│   │   ├── websocket.go            # WebSocket UCI bridge
│   │   ├── session.go              # GameSession with mutex + time tracking
│   │   ├── store.go                # InMemoryStore
│   │   ├── requests.go             # Request/response types
│   │   ├── middleware.go           # CORS, logging, recovery
│   │   └── errors.go               # API error types
│   └── persistence/                # SQLite game logs
│       ├── gamelog.go              # GameLogService
│       ├── gamelog_test.go
│       └── schema.sql              # FTS5 schema
├── data/                           # Runtime data (SQLite DB, logs)
└── Makefile                        # Build, test, lint targets
```

## API Design

### REST Endpoints

```
POST   /api/games                      # Create game
GET    /api/games/{id}                  # Get game state
POST   /api/games/{id}/moves            # Make a human move
POST   /api/games/{id}/ai-moves         # Request AI move
POST   /api/games/{id}/undo             # Undo last move
DELETE /api/games/{id}                  # Delete game
GET    /ws/uci                          # WebSocket UCI bridge
```

### Request Types

```go
type CreateGameRequest struct {
    TimeControl    string `json:"time_control"`
    GameMode       string `json:"game_mode"`
    Difficulty     *int   `json:"difficulty"`
    RedDifficulty  *int   `json:"red_difficulty"`
    BlueDifficulty *int   `json:"blue_difficulty"`
}

type MoveRequest struct {
    X int `json:"x"`
    Y int `json:"y"`
}
```

### Response Types

```go
type GameResponse struct {
    Board             [][]string  `json:"board"`
    CurrentPlayer     string      `json:"current_player"`
    MoveNumber        int         `json:"move_number"`
    IsGameOver        bool        `json:"is_game_over"`
    Winner            string      `json:"winner"`
    WinningLine       []Position  `json:"winning_line"`
    RedTimeRemaining  float64     `json:"red_time_remaining"`
    BlueTimeRemaining float64     `json:"blue_time_remaining"`
    TimeControl       string      `json:"time_control"`
    InitialTime       int         `json:"initial_time"`
    Increment         int         `json:"increment"`
    GameMode          string      `json:"game_mode"`
    RedDifficulty     *int        `json:"red_difficulty"`
    BlueDifficulty    *int        `json:"blue_difficulty"`
}

type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}
```

### Routing (Go 1.22+ ServeMux)

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /api/games", api.CreateGame)
mux.HandleFunc("GET /api/games/{id}", api.GetGame)
mux.HandleFunc("POST /api/games/{id}/moves", api.MakeMove)
mux.HandleFunc("POST /api/games/{id}/ai-moves", api.MakeAIMove)
mux.HandleFunc("POST /api/games/{id}/undo", api.UndoMove)
mux.HandleFunc("DELETE /api/games/{id}", api.DeleteGame)
```

## Domain Model

### Board (Immutable)

- 16x16 grid represented as `Cell` array + dual bitboard (4 `uint64` per player)
- `PlaceStone(x, y, player)` returns new Board with O(1) bitboard/hash incremental update
- Zobrist hashing with SplitMix64 PRNG, table `[16][16][2]uint64`
- Bit layout: `bitIndex = y * 16 + x`, stored in `uint64[bitIndex/64]`

### GameState (Immutable)

- All state transitions return new instances: `WithMove()`, `UndoMove()`, `WithGameOver()`
- Slice-based undo history (replaces C# ImmutableStack)
- Fields: Board, CurrentPlayer, MoveNumber, IsGameOver, Winner, WinningLine, BoardHistory, MoveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode

### Domain Errors

```go
var (
    ErrCellOccupied   = errors.New("cell already occupied")
    ErrPositionBounds = errors.New("position out of bounds")
    ErrGameOver       = errors.New("game is over")
    ErrOpenRule       = errors.New("open rule violation")
    ErrGameNotFound   = errors.New("game not found")
    ErrTooManyGames   = errors.New("too many concurrent games")
    ErrInvalidLevel   = errors.New("difficulty must be 1-5")
)
```

## AI Engine

### Difficulty Profiles

| Level | Name | Time Budget | Goroutines | VCF Solver | Pondering |
|-------|------|-------------|------------|------------|-----------|
| 1 | Novice | 5% | 1 | No | No |
| 2 | Beginner | 15% | 1 | No | No |
| 3 | Intermediate | 40% | 2 | Yes | No |
| 4 | Advanced | 70% | Pow2((N-2)/2) / 2 | Yes | No |
| 5 | Grandmaster | 100% | Pow2((N-2)/2) | Yes | Yes |

Where N = `runtime.GOMAXPROCS(0)`, Pow2 = largest power of 2 <= value.

L4 uses half of L5's goroutine count (next power of 2 down).

Examples:

| CPU Cores | (N-2)/2 | L5 | L4 | L3 | L1-L2 |
|-----------|---------|-----|-----|-----|-------|
| 4 | 1 | 1 | 1 | 2 | 1 |
| 8 | 3 | 2 | 1 | 2 | 1 |
| 12 | 5 | 4 | 2 | 2 | 1 |
| 16 | 7 | 4 | 2 | 2 | 1 |
| 20 | 9 | 8 | 4 | 2 | 1 |
| 32 | 15 | 8 | 4 | 2 | 1 |
| 64 | 31 | 16 | 8 | 2 | 1 |

Under load, goroutine count is divided by active game count (minimum 1).

### Per-Player AI Isolation

Each player in a game gets its own `MinimaxAI` instance with completely isolated:
- Transposition table (64MB default, private allocation)
- Search heuristics (killer moves, history tables, continuation history)
- VCF solver (private cache, 10K entries)
- Ponderer state
- Goroutine pool (per-search, destroyed after)
- Node counters and stats

Zero state sharing between red and blue AI instances. In AI vs AI matches, there is no possibility of cross-contamination.

### Resource Limits

- **TT per AI instance**: 64MB default (configurable)
- **VCF cache per AI**: ~1MB (10K entries)
- **Heuristics per AI**: ~200KB
- **Heap hard limit**: 2GB (`debug.SetMemoryLimit(2 * 1024 * 1024 * 1024)`)
- **Max concurrent games**: 4

### Search Architecture

```
MinimaxAI.GetBestMove()
├── VCF Solver (pre-search, 20% of allocated time)
│   └── Recursive threat sequence detection
├── Iterative Deepening loop
│   ├── Aspiration Windows (start at +/-50)
│   ├── Parallel Search (Lazy SMP)
│   │   ├── Master goroutine: full depth, writes TT at all depths
│   │   └── Helper goroutines: explore variants, write TT at depth >= 3
│   ├── PVS (Principal Variation Search)
│   │   ├── Full window for first move
│   │   └── Null-window for remaining, re-search on fail-high
│   ├── Alpha-Beta with:
│   │   ├── Null Move Pruning (depth > 3, reduce by 3)
│   │   ├── Adaptive LMR (based on depth, move type, history)
│   │   └── Quiescence Search (max 4 plies)
│   └── Move Picker (staged)
│       ├── Stage 1: TT move
│       ├── Stage 2: Must-block (opponent open four)
│       ├── Stage 3: Winning moves (open four / double threat)
│       ├── Stage 4: Threat creation (open three, broken four)
│       ├── Stage 5: Killer + Counter moves
│       ├── Stage 6: Good quiet (history > 0)
│       └── Stage 7: Bad quiet (remaining)
├── Time Management
│   ├── PID Controller for allocation
│   ├── Soft/hard bounds (1.5x multiplier)
│   └── context.WithCancel for search termination
└── Result selection (prefer deeper + better score)
```

### Lock-Free Transposition Table

SeqLock pattern using `atomic.Uint32` for version counters:
- 16 independent shards to reduce cache contention
- Writer: increment version (odd), write fields, increment version (even)
- Reader: read version, copy entry, verify version unchanged
- Depth-age replacement strategy

### VCF Solver Budget

VCF pre-search uses 20% of the allocated search time (not a fixed 100ms cap). This scales proportionally across all time controls:

| Time Control | Typical Allocation | VCF Budget (20%) |
|-------------|-------------------|-------------------|
| Bullet (1+0) | ~2-4s | 400-800ms |
| Blitz (3+2) | ~5-15s | 1-3s |
| Rapid (7+5) | ~15-45s | 3-9s |
| Classical (15+10) | ~30-120s | 6-24s |

### SIMD Evaluation

Two build paths:
- `evaluation_scalar.go` (default): uses `math/bits` for `OnesCount64`, `RotateLeft64`
- `evaluation_simd.go` (build tag `goexperiment.simd`): uses `simd/archsimd` for vectorized pattern detection

### Goroutine Pool

Per-search channel-based pool (not persistent):
- Go goroutines have ~2us startup cost, no need for persistent workers
- Each search creates a pool, dispatches work via channels, collects results
- Context cancellation terminates all goroutines cleanly

### Key Go Optimizations

1. `sync.Pool` for SearchBoard instances to reduce GC pressure
2. `unsafe.Pointer` for direct TT entry access in hot paths
3. `debug.SetMemoryLimit()` tuned for large TT allocations
4. `errors.AsType[T]()` for type-safe error matching
5. Expression-based `new()` for optional config fields
6. Green Tea GC handles large allocations efficiently

## Data Flow

### AI Move Request Flow

```
Client -> POST /api/games/{id}/ai-moves
  |
  +-- session.ExtractForAI() [under mutex]
  |   +-- Returns immutable board + time + difficulty
  |
  +-- ai.GetBestMove(board, player, opts, ctx) [outside mutex]
  |   +-- VCF pre-search (20% of allocated time)
  |   +-- Launch search goroutines via channels
  |   +-- Master: iterative deepening with PVS
  |   +-- Helpers: explore variants, share via TT
  |   +-- Return (x, y) + stats
  |
  +-- session.ApplyMove(x, y) [under mutex]
      +-- Validate (bounds, occupied, game over)
      +-- Apply to game state (immutable: new instance)
      +-- Check win condition
      +-- Update time tracking
      +-- Return updated state
```

### Context Propagation

```
HTTP request context -> session mutex boundary -> AI search context
                              |                        |
                              |  context.WithCancel    |
                              +--- for TT writes ------+
                              |                        |
                              +-- TimeMonitor cancels -+
                                    on hard bound hit
```

### Error Handling

API handlers use `errors.Is()` for domain error matching:

```go
func writeError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, ErrGameNotFound):
        writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
    case errors.Is(err, ErrTooManyGames):
        writeJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "too_many_games", Message: err.Error()})
    case errors.Is(err, ErrCellOccupied), errors.Is(err, ErrPositionBounds),
         errors.Is(err, ErrGameOver), errors.Is(err, ErrOpenRule):
        writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
    default:
        writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal", Message: "Internal server error"})
    }
}
```

## Persistence

### SQLite + FTS5

Using `mattn/go-sqlite3` with build tag `sqlite_fts5`. Requires `CGO_ENABLED=1`.

Schema:
```sql
CREATE TABLE IF NOT EXISTS game_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    data TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE VIRTUAL TABLE IF NOT EXISTS game_logs_fts USING fts5(
    game_id, event_type, data, content=game_logs, content_rowid=id
);
```

### Structured Logging

Using `log/slog` (standard library) with JSON handler for production, text handler for development.

## Infrastructure

### Game Cleanup

- Periodic timer every 5 minutes removes completed/abandoned games
- Graceful shutdown disposes all remaining game sessions
- Max 4 concurrent games enforced at creation

### Graceful Shutdown

- `signal.Notify` for SIGINT/SIGTERM
- `http.Server.Shutdown()` with 10-second timeout drains in-flight requests
- Cleanup goroutine disposes all remaining AI instances and game sessions

### CORS

Allow localhost origins with credentials for local development.

## Frontend Impact

The SvelteKit frontend needs updates for:
- Endpoint paths: `/api/game/new` -> `/api/games`, `/api/game/{id}` -> `/api/games/{id}`, etc.
- JSON field names: camelCase -> snake_case
- Error response shape: `{ error: string, message: string }`

## UCI Protocol

Standalone console engine via `cmd/engine/main.go`:
- Commands: uci, isready, ucinewgame, position, go, stop, quit, setoption
- Engine options: Threads, Hash, Ponder, Skill Level
- WebSocket bridge via `/ws/uci` endpoint

## Testing Strategy

| Package | Focus |
|---------|-------|
| `internal/domain` | Board, GameState, WinDetector, Zobrist hashing |
| `internal/engine` | Search, evaluation, TT, VCF, move ordering |
| `internal/uci` | UCI command parsing, notation conversion |
| `internal/api` | HTTP handlers, WebSocket, session management |
| `internal/persistence` | SQLite game log CRUD |

Testing tools:
- `testing` package (standard library)
- `net/http/httptest` for API handler tests
- `github.com/stretchr/testify` for assertions
- Race detector: `CGO_ENABLED=1 go test -race ./...`
- Goroutine leak detection: `GOEXPERIMENT=goroutineleakprofile go test ./...`
