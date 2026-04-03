using System.Net.WebSockets;
using System.Text;
using Caro.Api;
using Caro.Api.Logging;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.TimeManagement;
using Microsoft.AspNetCore.Mvc;
using System.Collections.Concurrent;
using System.Collections.Immutable;

var builder = WebApplication.CreateBuilder(args);

// Register GameLogService with lazy async initialization
builder.Services.AddSingleton<GameLogService>(sp =>
{
    var logger = sp.GetRequiredService<ILogger<GameLogService>>();
    // Use GetDataPath to store logs in a consistent location
    var dbPath = Path.Combine(AppContext.BaseDirectory, "Data", "GameLogs.db");

    // GetAwaiter().GetResult() preferred over .Wait()/.Result:
    // unwraps AggregateException into the actual exception for cleaner diagnostics.
    // Safe in ASP.NET Core startup (no SynchronizationContext).
    return GameLogService.CreateAsync(dbPath, logger).GetAwaiter().GetResult();
});

// Register MinimaxAI
builder.Services.AddSingleton<MinimaxAI>(sp =>
{
    var logger = sp.GetRequiredService<ILogger<MinimaxAI>>();
    return new MinimaxAI(logger: logger);
});

// Register UCIHandler for WebSocket UCI protocol
builder.Services.AddSingleton<UCIHandler>();

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
app.UseWebSockets();

// WebSocket endpoint for UCI protocol
app.Map("/ws/uci", async (HttpContext context, UCIHandler handler) =>
{
    if (context.WebSockets.IsWebSocketRequest)
    {
        using var webSocket = await context.WebSockets.AcceptWebSocketAsync();
        var buffer = new byte[4096];
        var logger = context.RequestServices.GetRequiredService<ILogger<Program>>();

        logger.LogInformation("UCI WebSocket connection established");

        handler.SendToClient = async msg =>
        {
            if (webSocket.State == WebSocketState.Open)
            {
                var bytes = Encoding.UTF8.GetBytes(msg);
                await webSocket.SendAsync(
                    new ArraySegment<byte>(bytes),
                    WebSocketMessageType.Text,
                    true,
                    CancellationToken.None);
            }
        };

        try
        {
            while (webSocket.State == WebSocketState.Open)
            {
                var result = await webSocket.ReceiveAsync(buffer, CancellationToken.None);

                if (result.MessageType == WebSocketMessageType.Close)
                {
                    await webSocket.CloseAsync(WebSocketCloseStatus.NormalClosure, "Closing", CancellationToken.None);
                    break;
                }

                if (result.MessageType == WebSocketMessageType.Text)
                {
                    var message = Encoding.UTF8.GetString(buffer, 0, result.Count).Trim();
                    if (!string.IsNullOrEmpty(message))
                    {
                        logger.LogDebug("UCI command received: {Message}", message);

                        var response = await handler.HandleMessageAsync(message);
                        if (!string.IsNullOrEmpty(response))
                        {
                            var responseBytes = Encoding.UTF8.GetBytes(response);
                            await webSocket.SendAsync(
                                new ArraySegment<byte>(responseBytes),
                                WebSocketMessageType.Text,
                                true,
                                CancellationToken.None
                            );
                        }
                    }
                }
            }
        }
        finally
        {
            handler.SendToClient = null;
        }

        logger.LogInformation("UCI WebSocket connection closed");
    }
    else
    {
        context.Response.StatusCode = 400;
    }
});

// In-memory game storage with per-game locks (concurrent-safe)
// Using ConcurrentDictionary eliminates the need for a global lock
var games = new ConcurrentDictionary<string, GameSession>();

// POST /api/game/new - Create new game
app.MapPost("/api/game/new", (CreateGameRequest? request) =>
{
    var gameId = Guid.NewGuid().ToString();

    // Parse time control from request
    TimeControl timeControl = (request?.TimeControl?.ToLowerInvariant()) switch
    {
        "1+0" or "bullet" => TimeControl.Bullet,
        "3+2" or "blitz" => TimeControl.Blitz,
        "15+10" or "classical" => TimeControl.Classical,
        _ => TimeControl.Rapid  // default to 7+5
    };

    // Parse game mode (default to PvP)
    var gameMode = request?.GameMode?.ToLowerInvariant() switch
    {
        "pvai" => GameMode.PvAI,
        "aivai" => GameMode.AivAI,
        _ => GameMode.PvP
    };

    var session = new GameSession(
        timeControl.Name,
        timeControl.InitialTimeMs,
        timeControl.IncrementSeconds,
        gameMode);
    games[gameId] = session;

    Console.WriteLine($"[GAME] Created {gameId}: mode={gameMode}, tc={timeControl.Name}");
    return Results.Ok(new { gameId, state = session.GetResponse() });
});

// POST /api/game/{id}/move - Make a move
app.MapPost("/api/game/{id}/move", (string id, MoveRequest request) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    return session.ExecuteMove(game =>
    {
        if (game.IsGameOver)
            return (game, Results.BadRequest("Game is over"));

        var board = game.Board;
        var validator = new OpenRuleValidator();

        if (!validator.IsValidSecondMove(board, request.X, request.Y))
            return (game, Results.BadRequest("Open Rule violation: Second 'O' cannot be in center 3x3 zone"));

        try
        {
            game = game.WithMove(request.X, request.Y);

            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);

            if (result.HasWinner)
            {
                game = game.WithGameOver(result.Winner, result.WinningLine.ToImmutableArray());
            }

            return (game, (IResult?)null);
        }
        catch (ArgumentOutOfRangeException)
        {
            return (game, Results.BadRequest("Position out of bounds"));
        }
        catch (InvalidOperationException)
        {
            return (game, Results.BadRequest("Cell already occupied"));
        }
    });
});

// POST /api/game/{id}/undo - Undo last move
app.MapPost("/api/game/{id}/undo", (string id) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    return session.ExecuteMove(game =>
    {
        try
        {
            game = game.UndoMove();
            return (game, (IResult?)null);
        }
        catch (InvalidOperationException ex)
        {
            return (game, Results.BadRequest(ex.Message));
        }
    });
});

// POST /api/game/{id}/ai-move - Get AI move and make it
// AI calculation is performed OUTSIDE the lock using a cloned board
// This prevents blocking other game requests during AI thinking time
app.MapPost("/api/game/{id}/ai-move", (
    string id,
    [FromServices] MinimaxAI ai) =>
{
    if (!games.TryGetValue(id, out var session))
        return Results.NotFound("Game not found");

    // Step 1: Extract game data under lock (minimal lock time)
    var (boardClone, currentPlayer, isGameOver, timeRemainingMs, incrementSeconds) = session.ExtractForAI();

    if (isGameOver)
        return Results.BadRequest("Game is over");

    // Step 2: AI calculation OUTSIDE lock (can take seconds without blocking other games)
    Console.WriteLine($"[AI] {currentPlayer} thinking... (timeRemaining={timeRemainingMs}ms, increment={incrementSeconds}s)");
    var searchOptions = new SearchOptions
    {
        TimeRemainingMs = timeRemainingMs,
        IncrementSeconds = incrementSeconds,
        PonderingEnabled = true,
        ParallelSearchEnabled = true,
    };
    var (x, y) = ai.GetBestMove(boardClone, currentPlayer, searchOptions);
    var stats = ai.GetSearchStatistics();
    Console.WriteLine($"[AI] {currentPlayer} -> ({x},{y})");
    Console.WriteLine($"[AI] stats depth={stats.DepthAchieved} nodes={stats.NodesSearched} nps={stats.NodesPerSecond:F0} score={stats.SearchScore} moveType={stats.MoveType} ttHit={stats.TableHitRate:F1}% time={stats.AllocatedTimeMs}ms threads={stats.ThreadCount}");

    // Step 3: Validate and apply the move under lock
    return session.ExecuteMove(game =>
    {
        // Double-check game didn't end while we were calculating
        if (game.IsGameOver)
            return (game, Results.BadRequest("Game ended while AI was thinking"));

        try
        {
            game = game.WithMove(x, y);

            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);

            if (result.HasWinner)
            {
                game = game.WithGameOver(result.Winner, result.WinningLine.ToImmutableArray());
            }

            return (game, (IResult?)null);
        }
        catch (ArgumentOutOfRangeException)
        {
            return (game, Results.BadRequest("AI returned invalid position"));
        }
        catch (InvalidOperationException)
        {
            return (game, Results.BadRequest("AI tried to occupy already occupied cell"));
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

app.Run();

/// <summary>
/// Thread-safe game session with per-game locking and time tracking.
/// Each game has its own lock, allowing concurrent games to proceed independently.
/// </summary>
public sealed class GameSession
{
    private readonly object _lock = new();
    private GameState _game;
    private long _redTimeRemainingMs;
    private long _blueTimeRemainingMs;
    private DateTime _lastMoveTimestamp;

    public GameSession(
        string timeControl = "7+5",
        long initialTimeMs = 420_000,
        int incrementSeconds = 5,
        GameMode gameMode = GameMode.PvP)
    {
        _game = GameState.CreateInitial(
            timeControl: timeControl,
            initialTimeMs: initialTimeMs,
            incrementSeconds: incrementSeconds,
            gameMode: gameMode
        );
        _redTimeRemainingMs = initialTimeMs;
        _blueTimeRemainingMs = initialTimeMs;
        _lastMoveTimestamp = DateTime.UtcNow;
    }

    /// <summary>
    /// Executes a move under lock, automatically tracking player time.
    /// Time is deducted from the moving player and increment is added.
    /// Returns the error result if action returns one, otherwise success
    /// with full game state response including timing.
    /// </summary>
    public IResult ExecuteMove(Func<GameState, (GameState updated, IResult? error)> action)
    {
        lock (_lock)
        {
            var previousMoveNumber = _game.MoveNumber;
            var movingPlayer = _game.CurrentPlayer;
            var (updated, error) = action(_game);

            if (error != null)
                return error;

            if (updated.MoveNumber > previousMoveNumber)
            {
                var now = DateTime.UtcNow;
                var elapsedMs = (long)(now - _lastMoveTimestamp).TotalMilliseconds;
                long inc = updated.IncrementSeconds * 1000L;
                if (movingPlayer == Player.Red)
                    _redTimeRemainingMs = Math.Max(0, _redTimeRemainingMs - elapsedMs + inc);
                else
                    _blueTimeRemainingMs = Math.Max(0, _blueTimeRemainingMs - elapsedMs + inc);
                _lastMoveTimestamp = now;
            }

            _game = updated;
            return Results.Ok(new { state = BuildResponse() });
        }
    }

    /// <summary>
    /// Extracts data needed for AI calculation WITHOUT holding the lock.
    /// Board is immutable, so no cloning is needed.
    /// Includes time remaining for the current player and increment for time-managed search.
    /// </summary>
    public (Board BoardClone, Player CurrentPlayer, bool IsGameOver, long TimeRemainingMs, int IncrementSeconds) ExtractForAI()
    {
        lock (_lock)
        {
            long timeRemaining = _game.CurrentPlayer == Player.Red
                ? _redTimeRemainingMs
                : _blueTimeRemainingMs;
            return (_game.Board, _game.CurrentPlayer, _game.IsGameOver, timeRemaining, _game.IncrementSeconds);
        }
    }

    public object GetResponse()
    {
        lock (_lock)
        {
            return BuildResponse();
        }
    }

    private object BuildResponse() => new
    {
        board = from x in Enumerable.Range(0, 16)
                from y in Enumerable.Range(0, 16)
                let cell = _game.Board.GetCell(x, y)
                select new
                {
                    x,
                    y,
                    player = cell.Player.ToLowerString()
                },
        currentPlayer = _game.CurrentPlayer.ToLowerString(),
        moveNumber = _game.MoveNumber,
        isGameOver = _game.IsGameOver,
        winner = _game.Winner.ToLowerString(),
        winningLine = _game.WinningLine.Select(p => new { x = p.X, y = p.Y }),
        redTimeRemaining = _redTimeRemainingMs / 1000.0,
        blueTimeRemaining = _blueTimeRemainingMs / 1000.0,
        timeControl = _game.TimeControl,
        initialTime = _game.InitialTimeMs / 1000,
        increment = _game.IncrementSeconds,
        gameMode = _game.GameMode.ToLowerString()
    };
}

record CreateGameRequest(string? TimeControl, string? GameMode);
record MoveRequest(int X, int Y);
