using Caro.Api;
using Caro.Api.Logging;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Caro.Core.Tournament;
using Microsoft.AspNetCore.Mvc;
using System.Collections.Concurrent;

var builder = WebApplication.CreateBuilder(args);

// Add SignalR
builder.Services.AddSignalR();

// Register GameLogService with lazy async initialization
builder.Services.AddSingleton<GameLogService>(sp =>
{
    var logger = sp.GetRequiredService<ILogger<GameLogService>>();
    // Use GetDataPath to store logs in a consistent location
    var dbPath = Path.Combine(AppContext.BaseDirectory, "Data", "GameLogs.db");

    // Block on initialization since we're in the DI container setup
    // This runs once at app startup
    var task = GameLogService.CreateAsync(dbPath, logger);
    task.Wait(); // Safe here since we're not in async context yet

    return task.Result;
});

// Register TournamentManager as both a singleton (for injection) and hosted service
builder.Services.AddSingleton<TournamentManager>();
builder.Services.AddHostedService(sp => sp.GetRequiredService<TournamentManager>());

// Register SqliteOpeningBookStore
builder.Services.AddSingleton<SqliteOpeningBookStore>(sp =>
{
    var logger = sp.GetRequiredService<ILogger<SqliteOpeningBookStore>>();
    // From bin/Debug/net10.0/, go up 6 levels to reach repo root
    var dbPath = Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "..", "..", "..", "..", "opening_book.db");
    return new SqliteOpeningBookStore(dbPath, logger);
});

// Register OpeningBook with SQLite store
builder.Services.AddSingleton<OpeningBook>(sp =>
{
    var store = sp.GetRequiredService<SqliteOpeningBookStore>();
    store.Initialize(); // Ensure tables exist
    var canonicalizer = new PositionCanonicalizer();
    var validator = new OpeningBookValidator();
    var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);
    return new OpeningBook(store, canonicalizer, lookupService);
});

// Register MinimaxAI with OpeningBook dependency
builder.Services.AddSingleton<MinimaxAI>(sp =>
{
    var openingBook = sp.GetRequiredService<OpeningBook>();
    var logger = sp.GetRequiredService<ILogger<MinimaxAI>>();
    return new MinimaxAI(logger: logger, openingBook: openingBook);
});

// CORS for local development - allow any localhost port
builder.Services.AddCors(options =>
{
    options.AddDefaultPolicy(policy =>
    {
        policy.SetIsOriginAllowed(origin => new Uri(origin).Host == "localhost")
              .AllowAnyHeader()
              .AllowAnyMethod()
              .AllowCredentials();
    });
});

var app = builder.Build();
app.UseCors();

// Map SignalR hub
app.MapHub<TournamentHub>("/hubs/tournament");

// In-memory game storage with per-game locks (concurrent-safe)
// Using ConcurrentDictionary eliminates the need for a global lock
var games = new ConcurrentDictionary<string, GameSession>();

// POST /api/game/new - Create new game
app.MapPost("/api/game/new", () =>
{
    var gameId = Guid.NewGuid().ToString();
    var session = new GameSession();
    games[gameId] = session;

    return Results.Ok(new { gameId, state = session.GetResponse() });
});

// POST /api/game/{id}/move - Make a move
app.MapPost("/api/game/{id}/move", (string id, MoveRequest request) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    return session.ExecuteUnderLock(game =>
    {
        if (game.IsGameOver)
            return Results.BadRequest("Game is over");

        var board = game.Board;
        var validator = new OpenRuleValidator();

        if (!validator.IsValidSecondMove(board, request.X, request.Y))
            return Results.BadRequest("Open Rule violation: Second 'O' cannot be in center 3x3 zone");

        try
        {
            game.RecordMove(request.X, request.Y);

            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);

            if (result.HasWinner)
            {
                game.EndGame(result.Winner, result.WinningLine);
            }

            return Results.Ok(new { state = session.GetResponse() });
        }
        catch (ArgumentOutOfRangeException)
        {
            return Results.BadRequest("Position out of bounds");
        }
        catch (InvalidOperationException)
        {
            return Results.BadRequest("Cell already occupied");
        }
    });
});

// POST /api/game/{id}/undo - Undo last move
app.MapPost("/api/game/{id}/undo", (string id) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    return session.ExecuteUnderLock(game =>
    {
        try
        {
            game.UndoMove();

            return Results.Ok(new { state = session.GetResponse() });
        }
        catch (InvalidOperationException ex)
        {
            return Results.BadRequest(ex.Message);
        }
    });
});

// POST /api/game/{id}/ai-move - Get AI move and make it
// AI calculation is performed OUTSIDE the lock using a cloned board
// This prevents blocking other game requests during AI thinking time
app.MapPost("/api/game/{id}/ai-move", (
    string id,
    AIMoveRequest request,
    [FromServices] MinimaxAI ai) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    // Step 1: Extract game data under lock (minimal lock time)
    var (boardClone, currentPlayer, isGameOver) = session.ExtractForAI();

    if (isGameOver)
        return Results.BadRequest("Game is over");

    // Step 2: Parse difficulty
    if (!Enum.TryParse<AIDifficulty>(request.Difficulty, true, out var difficulty))
    {
        return Results.BadRequest("Invalid difficulty. Use: Easy, Medium, Hard, or Expert");
    }

    // Step 3: AI calculation OUTSIDE lock (can take seconds without blocking other games)
    var (x, y) = ai.GetBestMove(boardClone, currentPlayer, difficulty);

    // Step 4: Validate and apply the move under lock
    return session.ExecuteUnderLock(game =>
    {
        // Double-check game didn't end while we were calculating
        if (game.IsGameOver)
            return Results.BadRequest("Game ended while AI was thinking");

        var validator = new OpenRuleValidator();

        if (!validator.IsValidSecondMove(game.Board, x, y))
            return Results.BadRequest("AI move violates Open Rule");

        try
        {
            game.RecordMove(x, y);

            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);

            if (result.HasWinner)
            {
                game.EndGame(result.Winner, result.WinningLine);
            }

            return Results.Ok(new { state = session.GetResponse() });
        }
        catch (ArgumentOutOfRangeException)
        {
            return Results.BadRequest("AI returned invalid position");
        }
        catch (InvalidOperationException)
        {
            return Results.BadRequest("AI tried to occupy already occupied cell");
        }
    });
});

// GET /api/game/{id} - Get game state
app.MapGet("/api/game/{id}", (string id) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    return Results.Ok(new { state = session.GetResponse() });
});

// ==================== Tournament API Endpoints ====================

// GET /api/tournament/state - Get current tournament state
app.MapGet("/api/tournament/state", ([FromServices] TournamentManager manager) =>
{
    return Results.Ok(manager.GetState());
});

// POST /api/tournament/start - Start the tournament
app.MapPost("/api/tournament/start", async ([FromServices] TournamentManager manager) =>
{
    var started = await manager.StartTournamentAsync();
    return started
        ? Results.Ok(new { message = "Tournament started", state = manager.GetState() })
        : Results.BadRequest(new { message = "Tournament already running" });
});

// POST /api/tournament/pause - Pause the tournament
app.MapPost("/api/tournament/pause", async ([FromServices] TournamentManager manager) =>
{
    var paused = await manager.PauseTournamentAsync();
    return paused
        ? Results.Ok(new { message = "Tournament paused", state = manager.GetState() })
        : Results.BadRequest(new { message = "Cannot pause - tournament not running" });
});

// POST /api/tournament/resume - Resume the tournament
app.MapPost("/api/tournament/resume", async ([FromServices] TournamentManager manager) =>
{
    var resumed = await manager.ResumeTournamentAsync();
    return resumed
        ? Results.Ok(new { message = "Tournament resumed", state = manager.GetState() })
        : Results.BadRequest(new { message = "Cannot resume - tournament not paused" });
});

app.Run();

/// <summary>
/// Thread-safe game session with per-game locking.
/// Each game has its own lock, allowing concurrent games to proceed independently.
/// This eliminates the global lock bottleneck in the original implementation.
/// </summary>
public sealed class GameSession
{
    private readonly object _lock = new();
    private GameState _game = new();

    /// <summary>
    /// Executes an action under the per-game lock.
    /// Returns the result of the action.
    /// </summary>
    public TResult ExecuteUnderLock<TResult>(Func<GameState, TResult> action)
    {
        lock (_lock)
        {
            return action(_game);
        }
    }

    /// <summary>
    /// Executes an action under the per-game lock.
    /// </summary>
    public void ExecuteUnderLock(Action<GameState> action)
    {
        lock (_lock)
        {
            action(_game);
        }
    }

    /// <summary>
    /// Extracts data needed for AI calculation WITHOUT holding the lock.
    /// Returns a cloned board so AI can compute without blocking other requests.
    /// </summary>
    public (Board BoardClone, Player CurrentPlayer, bool IsGameOver) ExtractForAI()
    {
        lock (_lock)
        {
            // Clone the board to allow AI calculation outside the lock
            return (_game.Board.Clone(), _game.CurrentPlayer, _game.IsGameOver);
        }
    }

    /// <summary>
    /// Gets the current game state as a response object (under lock)
    /// </summary>
    public object GetResponse()
    {
        lock (_lock)
        {
            var game = _game;
            return new
            {
                board = from x in Enumerable.Range(0, 15)
                        from y in Enumerable.Range(0, 15)
                        let cell = game.Board.GetCell(x, y)
                        select new
                        {
                            x,
                            y,
                            player = cell.Player.ToString().ToLower()
                        },
                currentPlayer = game.CurrentPlayer.ToString().ToLower(),
                moveNumber = game.MoveNumber,
                isGameOver = game.IsGameOver,
                winner = game.Winner.ToString().ToLower(),
                winningLine = game.WinningLine.Select(p => new { x = p.X, y = p.Y }),
                redTimeRemaining = 0.0,  // Time tracking moved to application layer
                blueTimeRemaining = 0.0
            };
        }
    }
}

record MoveRequest(int X, int Y);
record AIMoveRequest(string Difficulty);
