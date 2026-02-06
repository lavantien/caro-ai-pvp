using System.Text;
using System.Text.Json;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;
using Microsoft.Extensions.Logging;

namespace Caro.Api;

/// <summary>
/// WebSocket-based UCI protocol handler for API integration.
/// Allows the frontend to communicate directly with the UCI engine.
/// </summary>
public sealed class UCIHandler
{
    private readonly MinimaxAI _ai;
    private readonly UCISearchController _searchController;
    private readonly UCIEngineOptions _options;
    private readonly ILogger<UCIHandler> _logger;
    private Board? _currentBoard;
    private Player _currentPlayer = Player.Red;

    public UCIHandler(MinimaxAI ai, ILogger<UCIHandler> logger)
    {
        _ai = ai ?? throw new ArgumentNullException(nameof(ai));
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
        _options = new UCIEngineOptions();
        _searchController = new UCISearchController(ai, _options);
        _currentBoard = new Board();

        _searchController.OnSearchInfo += OnSearchInfo;
        _searchController.OnBestMove += OnBestMove;
    }

    /// <summary>
    /// Handle incoming WebSocket message.
    /// </summary>
    public async Task<string> HandleMessageAsync(string message)
    {
        try
        {
            var command = JsonSerializer.Deserialize<UCICommand>(message);
            if (command == null)
                return JsonSerializer.SerializeToElement(new UCIError("Invalid command")).ToString();

            var response = HandleCommand(command);
            return JsonSerializer.Serialize(response);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error handling UCI command: {Message}", message);
            return JsonSerializer.Serialize(new UCIError(ex.Message));
        }
    }

    /// <summary>
    /// Handle a UCI command synchronously.
    /// </summary>
    public object HandleCommand(UCICommand command)
    {
        if (command == null || string.IsNullOrEmpty(command.Command))
            return new UCIError("Missing command");

        var cmd = command.Command.ToLowerInvariant();

        try
        {
            return cmd switch
            {
                "uci" => HandleUci(),
                "isready" => HandleIsReady(),
                "ucinewgame" => HandleUciNewGame(),
                "position" => HandlePosition(command),
                "go" => HandleGo(command),
                "stop" => HandleStop(),
                "setoption" => HandleSetOption(command),
                _ => new UCIError($"Unknown command: {cmd}")
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error handling command: {Command}", cmd);
            return new UCIError(ex.Message);
        }
    }

    private object HandleUci()
    {
        return new UCIResponse
        {
            Id = new[] { "Caro AI 1.0", "Caro AI Project" },
            Options = UCIEngineOptions.GetOptionDeclarations(),
            UciOk = true
        };
    }

    private object HandleIsReady()
    {
        return new UCIResponse { ReadyOk = true };
    }

    private object HandleUciNewGame()
    {
        _currentBoard = new Board();
        _currentPlayer = Player.Red;
        _ai.ClearAllState();
        return new UCIResponse { Ok = true };
    }

    private object HandlePosition(UCICommand command)
    {
        if (command.Position == null && command.Moves == null)
            return new UCIError("Position command requires position or moves");

        try
        {
            // Parse position - if empty, start from empty board
            if (command.Position == null || command.Position == "startpos")
            {
                _currentBoard = new Board();
            }
            else
            {
                _currentBoard = UCIPositionConverter.ParsePosition(command.Position);
            }

            // Apply moves if provided
            if (command.Moves != null && command.Moves.Length > 0)
            {
                var (board, nextPlayer) = UCIPositionConverter.ApplyMoves(_currentBoard, command.Moves);
                _currentBoard = board;
                _currentPlayer = nextPlayer;
            }
            else
            {
                // Determine current player from move count
                int moveCount = _currentBoard.Cells.Count(c => !c.IsEmpty);
                _currentPlayer = (moveCount % 2) == 0 ? Player.Red : Player.Blue;
            }

            return new UCIResponse { Ok = true };
        }
        catch (Exception ex)
        {
            return new UCIError($"Error parsing position: {ex.Message}");
        }
    }

    private object HandleGo(UCICommand command)
    {
        if (_currentBoard == null)
            return new UCIError("No position set");

        var goParams = new UCIGoParameters
        {
            WhiteTimeMs = command.WhiteTime,
            BlackTimeMs = command.BlackTime,
            WhiteIncrementMs = command.WhiteIncrement,
            BlackIncrementMs = command.BlackIncrement,
            MoveTimeMs = command.MoveTime,
            Depth = command.Depth,
            Nodes = command.Nodes,
            Infinite = command.Infinite
        };

        // Start search asynchronously
        _searchController.StartSearch(_currentBoard, _currentPlayer, goParams);

        return new UCIResponse { Searching = true };
    }

    private object HandleStop()
    {
        var bestMove = _searchController.StopSearch();

        if (bestMove.HasValue)
        {
            var (x, y) = bestMove.Value;
            return new UCIResponse { BestMove = UCIMoveNotation.ToUCI(x, y) };
        }

        return new UCIResponse { Stopped = true };
    }

    private object HandleSetOption(UCICommand command)
    {
        if (string.IsNullOrEmpty(command.Name))
            return new UCIError("setoption requires name");

        if (_options.SetOption(command.Name, command.Value))
            return new UCIResponse { Ok = true };

        return new UCIError($"Unknown option '{command.Name}'");
    }

    private void OnSearchInfo(SearchInfo info)
    {
        // Could emit via WebSocket to connected clients
        _logger.LogDebug("Search info: depth={Depth}, nodes={Nodes}, time={TimeMs}ms",
            info.Depth, info.Nodes, info.TimeMs);
    }

    private void OnBestMove((int x, int y) move)
    {
        // Could emit via WebSocket to connected clients
        _logger.LogDebug("Best move: {Move}", UCIMoveNotation.ToUCI(move.x, move.y));
    }
}

/// <summary>
/// UCI command from frontend.
/// </summary>
public sealed record UCICommand
{
    public string? Command { get; init; }
    public string? Position { get; init; }
    public string[]? Moves { get; init; }

    // Go command parameters
    public long? WhiteTime { get; init; }
    public long? BlackTime { get; init; }
    public int? WhiteIncrement { get; init; }
    public int? BlackIncrement { get; init; }
    public int? MoveTime { get; init; }
    public int? Depth { get; init; }
    public int? Nodes { get; init; }
    public bool Infinite { get; init; }

    // SetOption parameters
    public string? Name { get; init; }
    public string? Value { get; init; }
}

/// <summary>
/// UCI response to frontend.
/// </summary>
public sealed record UCIResponse
{
    public string[]? Id { get; init; }
    public string[]? Options { get; init; }
    public bool UciOk { get; init; }
    public bool ReadyOk { get; init; }
    public bool Ok { get; init; }
    public bool Searching { get; init; }
    public bool Stopped { get; init; }
    public string? BestMove { get; init; }
    public SearchInfoDto? Info { get; init; }
}

/// <summary>
/// Search info for frontend.
/// </summary>
public sealed record SearchInfoDto
{
    public int Depth { get; init; }
    public long Nodes { get; init; }
    public long TimeMs { get; init; }
    public int Score { get; init; }
    public string[] PV { get; init; } = Array.Empty<string>();
}

/// <summary>
/// UCI error response.
/// </summary>
public sealed record UCIError
{
    public string Error { get; }

    public UCIError(string error)
    {
        Error = error;
    }

    public bool IsError => true;
}
