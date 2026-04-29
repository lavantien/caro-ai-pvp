# Go 1.26 Onboarding Guide

Welcome to the **Caro AI PvP** codebase! This is a grandmaster-level Caro (Gomoku variant) AI built with Go 1.26 and Go-idiomatic package layout.

## Quick Project Overview

### Package Structure
```
backend/
├── cmd/
│   ├── server/main.go             # API server entry point
│   └── engine/main.go             # Standalone UCI console engine
├── internal/
│   ├── domain/                    # Pure game entities (no dependencies)
│   ├── engine/                    # AI engine (depends on domain)
│   ├── uci/                       # UCI protocol handler
│   ├── api/                       # HTTP/WebSocket API
│   └── persistence/               # SQLite game logs
├── go.mod
└── Makefile
```

### Key Technologies
- Go 1.26 (Green Tea GC, context.Context, errors.AsType)
- net/http ServeMux with method matching (Go 1.22+)
- gorilla/websocket, mattn/go-sqlite3 (CGO_ENABLED=1)
- stretchr/testify for assertions
- experimental simd/archsimd for vectorized evaluation

### Architecture Principles

- **Immutability**: Domain types are immutable structs; operations return new instances
- **Value semantics**: Small structs (Position, Cell) passed by value
- **Interface segregation**: Small, focused interfaces; no DI container
- **Package separation**: Dependency flow domain <- engine <- uci/api

---

## Part 1: Go 1.26 Features Used

### 1.1 Green Tea GC

Default in Go 1.26. Provides 10-40% reduction in GC overhead. No configuration needed.

### 1.2 errors.AsType[T]()

Type-safe error matching without reflection:

```go
// Before (errors.As)
var appErr *AppError
if errors.As(err, &appErr) { ... }

// After (errors.AsType)
if appErr, ok := errors.AsType[*AppError](err); ok {
    log.Error("app error", "code", appErr.Code)
}
```

### 1.3 Expression-Based new()

Create pointers to literal values inline:

```go
type CreateGameRequest struct {
    RedDifficulty  *int `json:"red_difficulty"`
    BlueDifficulty *int `json:"blue_difficulty"`
}

// Before: temp variable
level := 3
req := CreateGameRequest{RedDifficulty: &level}

// After: expression-based new
req := CreateGameRequest{RedDifficulty: new(3)}
```

### 1.4 Self-Referential Generics

Generic types can reference themselves in constraints:

```go
type Comparable[T Comparable[T]] interface {
    CompareTo(T) int
}
```

### 1.5 Goroutine Leak Detection

Debug builds detect leaked goroutines:

```bash
GOEXPERIMENT=goroutineleakprofile go test ./...
```

### 1.6 experimental/simd

Vectorized operations for BitBoard evaluation (amd64, build tag `goexperiment.simd`):

```go
// +build goexperiment.simd

import "simd/archsimd"

func evalSIMD(a, b simd.Uint64x2) int {
    // Vectorized bitwise operations
}
```

---

## Part 2: Immutable Domain Patterns

### 2.1 Value Types

Small structs passed by value (no pointers needed):

```go
type Position struct {
    X int
    Y int
}

func (p Position) IsValid() bool {
    return p.X >= 0 && p.X < BoardSize && p.Y >= 0 && p.Y < BoardSize
}
```

### 2.2 Immutable Board

Board operations return new instances. Original is never modified:

```go
type Board struct {
    cells    [256]Cell
    redBits  [4]uint64
    blueBits [4]uint64
    hash     uint64
}

func (b Board) PlaceStone(x, y int, player Player) Board {
    // Returns new Board with updated cells, bitboard, and hash
    newBoard := b // copy (array values copied)
    // ... update newBoard fields
    return newBoard
}
```

### 2.3 GameState Transitions

All state transitions return new instances:

```go
type GameState struct {
    Board           Board
    CurrentPlayer   Player
    MoveNumber      int
    IsGameOver      bool
    Winner          Player
    WinningLine     []Position
    BoardHistory    []Board
    MoveHistory     []Position
    TimeControl     string
    InitialTimeMs   int64
    IncrementSeconds int
    GameMode        GameMode
}

func (g GameState) WithMove(x, y int) GameState {
    newBoard := g.Board.PlaceStone(x, y, g.CurrentPlayer)
    history := append([]Board{g.Board}, g.BoardHistory...)
    return GameState{
        Board:         newBoard,
        CurrentPlayer: g.CurrentPlayer.Opponent(),
        MoveNumber:    g.MoveNumber + 1,
        BoardHistory:  history,
        // ... copy other fields
    }
}
```

### 2.4 Domain Errors

Sentinel errors for each domain violation:

```go
var (
    ErrCellOccupied   = errors.New("cell already occupied")
    ErrPositionBounds = errors.New("position out of bounds")
    ErrGameOver       = errors.New("game is over")
    ErrOpenRule       = errors.New("open rule violation")
)
```

---

## Part 3: Concurrency Patterns

### 3.1 Goroutine Worker Pool

Per-search pool dispatched via channels (not persistent):

```go
type WorkerPool struct {
    jobs    chan SearchJob
    results chan SearchResult
    wg      sync.WaitGroup
    cancel  context.CancelFunc
}

func (p *WorkerPool) Dispatch(ctx context.Context, jobs []SearchJob) []SearchResult {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    pool := &WorkerPool{
        jobs:    make(chan SearchJob, len(jobs)),
        results: make(chan SearchResult, len(jobs)),
        cancel:  cancel,
    }

    // Start workers
    for i := 0; i < p.numWorkers; i++ {
        pool.wg.Add(1)
        go func(id int) {
            defer pool.wg.Done()
            for job := range pool.jobs {
                select {
                case <-ctx.Done():
                    return
                default:
                    pool.results <- search(ctx, job)
                }
            }
        }(i)
    }

    // Send jobs
    for _, job := range jobs {
        pool.jobs <- job
    }
    close(pool.jobs)

    // Wait and collect
    go func() {
        pool.wg.Wait()
        close(pool.results)
    }()

    var results []SearchResult
    for r := range pool.results {
        results = append(results, r)
    }
    return results
}
```

### 3.2 Mutex-Based Game Sessions

Each game session has its own mutex for thread safety:

```go
type GameSession struct {
    mu          sync.Mutex
    game        *domain.GameState
    redTimeMs   int64
    blueTimeMs  int64
    lastMoveAt  time.Time
    redAI       *engine.MinimaxAI
    blueAI      *engine.MinimaxAI
}
```

### 3.3 Context Propagation

Cancellation flows from HTTP request through AI search:

```go
func (s *GameSession) MakeAIMove(ctx context.Context) (GameResponse, error) {
    // Extract data under lock (minimal lock time)
    s.mu.Lock()
    board := s.game.Board
    player := s.game.CurrentPlayer
    s.mu.Unlock()

    // AI computation outside lock (can take seconds)
    ai := s.GetOrCreateAI(player)
    x, y := ai.GetBestMove(board, player, opts, ctx) // ctx propagates cancellation

    // Apply move under lock
    s.mu.Lock()
    defer s.mu.Unlock()
    // ... apply move, update time
}
```

### 3.4 SeqLock Transposition Table

Lock-free reads with atomic version counters:

```go
type TTEntry struct {
    hash    uint64
    data    uint32
    meta    uint32
    version atomic.Uint32 // odd=writing, even=stable
}

func (t *TranspositionTable) Store(entry TTEntry) {
    v := entry.version
    entry.version.Add(1)       // make odd (writing)
    // write fields...
    entry.version.Add(1)       // make even (stable)
}

func (t *TranspositionTable) Load(hash uint64) (TTEntry, bool) {
    v1 := entry.version.Load()
    if v1%2 != 0 { return TTEntry{}, false } // writing, retry
    copied := entry // copy
    if entry.version.Load() != v1 { return TTEntry{}, false } // changed during copy
    return copied, true
}
```

### 3.5 sync.Pool for Reusable Objects

Reduce GC pressure in hot paths:

```go
var searchBoardPool = sync.Pool{
    New: func() any { return &SearchBoard{} },
}

func acquireSearchBoard() *SearchBoard {
    sb := searchBoardPool.Get().(*SearchBoard)
    sb.Reset()
    return sb
}

func releaseSearchBoard(sb *SearchBoard) {
    searchBoardPool.Put(sb)
}
```

---

## Part 4: Error Handling

### 4.1 Domain Errors

Sentinel errors at package boundaries:

```go
// internal/domain/errors.go
var (
    ErrCellOccupied   = errors.New("cell already occupied")
    ErrPositionBounds = errors.New("position out of bounds")
    ErrGameOver       = errors.New("game is over")
    ErrOpenRule       = errors.New("open rule violation")
)

// internal/api/errors.go
var (
    ErrGameNotFound = errors.New("game not found")
    ErrTooManyGames = errors.New("too many concurrent games")
)
```

### 4.2 API Error Responses

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
        slog.Error("internal error", "err", err)
        writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal", Message: "Internal server error"})
    }
}
```

---

## Part 5: Testing

### 5.1 Test Stack

| Component | Purpose |
|-----------|---------|
| `testing` (stdlib) | Test framework |
| `github.com/stretchr/testify` | Assertions (assert, require, mock) |
| `net/http/httptest` | HTTP handler testing |
| Race detector | `go test -race ./...` |
| Goroutine leak detection | `GOEXPERIMENT=goroutineleakprofile go test ./...` |

### 5.2 Table-Driven Tests

Go's primary testing pattern:

```go
func TestPositionIsValid(t *testing.T) {
    tests := []struct {
        name     string
        pos      Position
        expected bool
    }{
        {"origin", Position{0, 0}, true},
        {"center", Position{8, 8}, true},
        {"corner", Position{15, 15}, true},
        {"negative_x", Position{-1, 0}, false},
        {"over_y", Position{0, 16}, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.pos.IsValid())
        })
    }
}
```

### 5.3 Testing Immutable Types

```go
func TestBoardPlaceStoneImmutable(t *testing.T) {
    original := domain.NewBoard()

    placed := original.PlaceStone(8, 8, domain.PlayerRed)

    // Original unchanged
    assert.Equal(t, domain.PlayerNone, original.GetCell(8, 8).Player)
    // New board updated
    assert.Equal(t, domain.PlayerRed, placed.GetCell(8, 8).Player)
    // Hash differs
    assert.NotEqual(t, original.Hash(), placed.Hash())
}
```

### 5.4 Testing AI Engine

```go
func TestMinimaxFindsWinningMove(t *testing.T) {
    board := setupBoardWithFourInRow() // 4 red stones in a row
    ai := engine.NewMinimaxAI(engine.Config{MaxThreads: 1})

    x, y := ai.GetBestMove(board, domain.PlayerRed, opts, context.Background())

    // Should find the winning fifth stone
    assert.True(t, isValidWinningCompletion(board, x, y, domain.PlayerRed))
}
```

### 5.5 Testing HTTP Handlers

```go
func TestCreateGame(t *testing.T) {
    store := api.NewInMemoryStore()
    handler := api.NewHandler(store)

    body := `{"time_control":"3+2","game_mode":"aivai","difficulty":5}`
    req := httptest.NewRequest(http.MethodPost, "/api/games", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    handler.CreateGame(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    var resp api.GameResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, "aivai", resp.GameMode)
    assert.NotNil(t, resp.RedDifficulty)
    assert.Equal(t, 5, *resp.RedDifficulty)
}
```

### 5.6 Testing Concurrency

```go
func TestConcurrentMoveRequests(t *testing.T) {
    session := api.NewGameSession(api.SessionConfig{GameMode: "pvp"})

    var wg sync.WaitGroup
    errors := make([]error, 100)

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            _, err := session.MakeMove(idx%16, idx%16)
            errors[idx] = err
        }(i)
    }

    wg.Wait()

    // Exactly one should succeed per cell, rest should get ErrCellOccupied
    successCount := 0
    for _, err := range errors {
        if err == nil { successCount++ }
    }
    assert.Greater(t, successCount, 0)
}
```

### 5.7 Running Tests

```bash
# All tests with race detector
cd backend && CGO_ENABLED=1 go test -race ./...

# Specific package
go test -v ./internal/domain/...

# With coverage
go test -cover ./internal/engine/...

# Goroutine leak detection (debug)
GOEXPERIMENT=goroutineleakprofile go test ./internal/engine/...

# Benchmark
go test -bench=BenchmarkEvaluation -benchmem ./internal/engine/...
```

---

## Part 6: Build and Run

### Development

```bash
# Build
cd backend && go build ./...

# Run API server
go run ./cmd/server

# Run standalone UCI engine
go run ./cmd/engine

# Build with SIMD support
GOEXPERIMENT=simd go build ./...
```

### Makefile Targets

```bash
make build       # Build all binaries
make test        # Run tests with race detector
make lint        # Run golangci-lint
make fmt         # Format with gofmt
make vet         # Run go vet
```

---

## Part 7: Package Dependency Rules

Dependency flow is strictly one-directional:

```
cmd/server -> internal/api -> internal/engine -> internal/domain
                              internal/persistence -> internal/domain
                              internal/uci -> internal/engine, internal/domain
```

**Rules:**
- `internal/domain` imports only stdlib
- `internal/engine` imports only `internal/domain` and stdlib
- `internal/uci` imports `internal/engine` and `internal/domain`
- `internal/api` imports all other internal packages
- `internal/persistence` imports only `internal/domain`
- No circular imports
- No external dependencies in `internal/domain`

---

## Summary Checklist

1. **Understand package layout**: internal/domain -> engine -> uci/api
2. **Immutability**: Domain types return new instances on mutation
3. **Concurrency**: goroutines + channels + context.Context + sync.Mutex
4. **Error handling**: Sentinel errors with errors.Is() at API boundary
5. **Testing**: Table-driven tests with testify, race detector, httptest
6. **Build**: `CGO_ENABLED=1` required for SQLite; `go run ./cmd/server` to start
7. **When tests fail**: Check table-driven test case name to identify which input caused the failure
8. **Assertions**: Use `github.com/stretchr/testify/assert` and `require`
