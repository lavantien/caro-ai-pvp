using Caro.Api;
using Caro.Api.Logging;
using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Microsoft.AspNetCore.Mvc;

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

// In-memory game storage (for prototype)
var games = new Dictionary<string, GameState>();
var gamesLock = new object();

// POST /api/game/new - Create new game
app.MapPost("/api/game/new", () =>
{
    var gameId = Guid.NewGuid().ToString();
    var game = new GameState();

    lock (gamesLock)
    {
        games[gameId] = game;
    }

    return Results.Ok(new { gameId, state = MapToResponse(game) });
});

// POST /api/game/{id}/move - Make a move
app.MapPost("/api/game/{id}/move", (string id, MoveRequest request) =>
{
    lock (gamesLock)
    {
        if (!games.TryGetValue(id, out var game))
            return Results.NotFound("Game not found");

        if (game.IsGameOver)
            return Results.BadRequest("Game is over");

        var board = game.Board;
        var validator = new OpenRuleValidator();

        if (!validator.IsValidSecondMove(board, request.X, request.Y))
            return Results.BadRequest("Open Rule violation: Second 'O' cannot be in center 3x3 zone");

        try
        {
            game.RecordMove(board, request.X, request.Y);
            game.ApplyTimeIncrement();

            var detector = new WinDetector();
            var result = detector.CheckWin(board);

            if (result.HasWinner)
            {
                game.EndGame(result.Winner, result.WinningLine);
            }

            return Results.Ok(new { state = MapToResponse(game) });
        }
        catch (ArgumentOutOfRangeException)
        {
            return Results.BadRequest("Position out of bounds");
        }
        catch (InvalidOperationException)
        {
            return Results.BadRequest("Cell already occupied");
        }
    }
});

// POST /api/game/{id}/undo - Undo last move
app.MapPost("/api/game/{id}/undo", (string id) =>
{
    lock (gamesLock)
    {
        if (!games.TryGetValue(id, out var game))
            return Results.NotFound("Game not found");

        try
        {
            var board = game.Board;
            game.UndoMove(board);

            return Results.Ok(new { state = MapToResponse(game) });
        }
        catch (InvalidOperationException ex)
        {
            return Results.BadRequest(ex.Message);
        }
    }
});

// POST /api/game/{id}/ai-move - Get AI move and make it
app.MapPost("/api/game/{id}/ai-move", (string id, AIMoveRequest request) =>
{
    lock (gamesLock)
    {
        if (!games.TryGetValue(id, out var game))
            return Results.NotFound("Game not found");

        if (game.IsGameOver)
            return Results.BadRequest("Game is over");

        var board = game.Board;

        // Parse difficulty
        if (!Enum.TryParse<AIDifficulty>(request.Difficulty, true, out var difficulty))
        {
            return Results.BadRequest("Invalid difficulty. Use: Easy, Medium, Hard, or Expert");
        }

        // Get AI move
        var ai = new MinimaxAI();
        var (x, y) = ai.GetBestMove(board, game.CurrentPlayer, difficulty);

        // Validate and apply the move
        var validator = new OpenRuleValidator();

        if (!validator.IsValidSecondMove(board, x, y))
            return Results.BadRequest("AI move violates Open Rule");

        try
        {
            game.RecordMove(board, x, y);
            game.ApplyTimeIncrement();

            var detector = new WinDetector();
            var result = detector.CheckWin(board);

            if (result.HasWinner)
            {
                game.EndGame(result.Winner, result.WinningLine);
            }

            return Results.Ok(new { state = MapToResponse(game) });
        }
        catch (ArgumentOutOfRangeException)
        {
            return Results.BadRequest("AI returned invalid position");
        }
        catch (InvalidOperationException)
        {
            return Results.BadRequest("AI tried to occupy already occupied cell");
        }
    }
});

// GET /api/game/{id} - Get game state
app.MapGet("/api/game/{id}", (string id) =>
{
    lock (gamesLock)
    {
        if (!games.TryGetValue(id, out var game))
            return Results.NotFound("Game not found");

        return Results.Ok(new { state = MapToResponse(game) });
    }
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

static object MapToResponse(GameState game) => new
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
    redTimeRemaining = game.RedTimeRemaining.TotalSeconds,
    blueTimeRemaining = game.BlueTimeRemaining.TotalSeconds
};

record MoveRequest(int X, int Y);
record AIMoveRequest(string Difficulty);
