using Caro.Core.Entities;
using Caro.Core.GameLogic;

var builder = WebApplication.CreateBuilder(args);

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
