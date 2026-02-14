using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;
using Microsoft.Extensions.Logging;
using System;
using System.Text;

namespace Caro.UCI;

/// <summary>
/// UCI protocol handler for console I/O.
/// </summary>
public sealed class UCIProtocol
{
    private readonly MinimaxAI _ai;
    private readonly ILogger _logger;
    private readonly UCISearchController _searchController;
    private readonly UCIEngineOptions _options;
    private Board? _currentBoard;
    private Player _currentPlayer = Player.Red;
    private bool _isRunning = true;

    public UCIProtocol(MinimaxAI ai, ILogger logger)
    {
        _ai = ai ?? throw new ArgumentNullException(nameof(ai));
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
        _options = new UCIEngineOptions();
        _searchController = new UCISearchController(ai, _options);
        _currentBoard = new Board();

        _searchController.OnSearchInfo += OnSearchInfo;
        _searchController.OnBestMove += OnBestMove;
    }

    private void OnBestMove((int x, int y) move)
    {
        var (x, y) = move;
        var moveStr = $"bestmove {UCIMoveNotation.ToUCI(x, y)}";
        Console.Out.WriteLine(moveStr);
        Console.Out.Flush();
    }

    /// <summary>
    /// Run the UCI protocol loop.
    /// </summary>
    public async Task RunAsync(TextReader input, TextWriter output, CancellationToken ct)
    {
        await output.WriteLineAsync("Caro AI UCI Engine 1.0");
        await output.WriteLineAsync("Type 'uci' to initialize or 'quit' to exit.");
        await output.FlushAsync();

        while (_isRunning && !ct.IsCancellationRequested)
        {
            var line = await input.ReadLineAsync();
            if (line == null)
                break;

            var trimmed = line.Trim();
            if (string.IsNullOrEmpty(trimmed))
                continue;

            var response = HandleCommand(trimmed);
            foreach (var resp in response)
            {
                await output.WriteLineAsync(resp);
            }
            await output.FlushAsync();
        }
    }

    /// <summary>
    /// Stop the protocol loop.
    /// </summary>
    public void Stop()
    {
        _isRunning = false;
        _searchController.StopSearch();
    }

    /// <summary>
    /// Handle a single UCI command.
    /// </summary>
    public string[] HandleCommand(string command)
    {
        var parts = command.Split(' ', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        if (parts.Length == 0)
            return Array.Empty<string>();

        var cmd = parts[0].ToLowerInvariant();
        var args = parts.Skip(1).ToArray();

        try
        {
            return cmd switch
            {
                "uci" => HandleUci(),
                "isready" => HandleIsReady(),
                "ucinewgame" => HandleUciNewGame(),
                "position" => HandlePosition(args),
                "go" => HandleGo(args),
                "stop" => HandleStop(),
                "setoption" => HandleSetOption(args),
                "quit" => HandleQuit(),
                "echo" => new[] { string.Join(" ", args) },  // For debugging
                _ => new[] { $"Unknown command: {cmd}" }
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error handling command: {Command}", command);
            return new[] { $"Error: {ex.Message}" };
        }
    }

    private string[] HandleUci()
    {
        var responses = new List<string>
        {
            "id name Caro AI 1.52.0",
            "id author Caro AI Project"
        };

        // Add all option declarations
        responses.AddRange(UCIEngineOptions.GetOptionDeclarations());
        responses.Add("uciok");

        return responses.ToArray();
    }

    private string[] HandleIsReady()
    {
        return new[] { "readyok" };
    }

    private string[] HandleUciNewGame()
    {
        // Reset for new game
        _currentBoard = new Board();
        _currentPlayer = Player.Red;
        _ai.ClearAllState();

        return Array.Empty<string>();
    }

    private string[] HandlePosition(string[] args)
    {
        if (args.Length == 0)
            return new[] { "Error: position command requires arguments" };

        try
        {
            var positionCommand = "position " + string.Join(" ", args);
            _currentBoard = UCIPositionConverter.ParsePosition(positionCommand);

            // Determine current player from move count
            // Red moves on odd-numbered moves (1, 3, 5...), Blue on even (2, 4, 6...)
            // Start from empty, count moves to find who moves next
            int moveCount = 0;
            foreach (var cell in _currentBoard.Cells)
            {
                if (!cell.IsEmpty)
                    moveCount++;
            }

            _currentPlayer = (moveCount % 2) switch
            {
                0 => Player.Red,   // Even moves played, Red to move
                _ => Player.Blue    // Odd moves played, Blue to move
            };

            return Array.Empty<string>();
        }
        catch (Exception ex)
        {
            return new[] { $"Error parsing position: {ex.Message}" };
        }
    }

    private string[] HandleGo(string[] args)
    {
        if (_currentBoard == null)
            return new[] { "Error: No position set" };

        var goParams = UCIGoParameters.Parse(args);

        // Start search asynchronously
        _searchController.StartSearch(_currentBoard, _currentPlayer, goParams);

        // Return immediately - bestmove will be sent when ready
        return Array.Empty<string>();
    }

    private string[] HandleStop()
    {
        var bestMove = _searchController.StopSearch();

        if (bestMove.HasValue)
        {
            var (x, y) = bestMove.Value;
            return new[] { $"bestmove {UCIMoveNotation.ToUCI(x, y)}" };
        }

        return Array.Empty<string>();
    }

    private string[] HandleSetOption(string[] args)
    {
        // Parse: setoption name <name> [value <value>]
        // Names can have spaces (e.g., "Skill Level", "Use Opening Book")
        if (args.Length < 2)
            return new[] { "Error: setoption requires name" };

        var nameIndex = Array.IndexOf(args, "name");
        if (nameIndex == -1 || nameIndex + 1 >= args.Length)
            return new[] { "Error: setoption requires name" };

        var valueIndex = Array.IndexOf(args, "value");

        // Extract name (everything between "name" and "value", or end if no value)
        string name;
        string? value = null;

        int nameEndIndex = valueIndex != -1 ? valueIndex : args.Length;
        var nameParts = new List<string>();
        for (int i = nameIndex + 1; i < nameEndIndex; i++)
        {
            nameParts.Add(args[i]);
        }
        name = string.Join(" ", nameParts);

        // Extract value if present
        if (valueIndex != -1 && valueIndex + 1 < args.Length)
        {
            value = args[valueIndex + 1];
        }

        if (_options.SetOption(name, value))
            return Array.Empty<string>();

        return new[] { $"Error: Unknown option '{name}'" };
    }

    private string[] HandleQuit()
    {
        // Wait for any ongoing search to complete before quitting
        if (_searchController.IsSearching)
        {
            Task.Run(async () =>
            {
                try
                {
                    await _searchController.WaitForCompletion();
                }
                catch
                {
                    // Ignore exceptions during shutdown
                }
                Stop();
                Environment.Exit(0);
            }).Wait(TimeSpan.FromSeconds(30));
        }
        Stop();
        Environment.Exit(0);
        return Array.Empty<string>();
    }

    private void OnSearchInfo(SearchInfo info)
    {
        // Emit info message to console
        var sb = new StringBuilder("info");

        if (info.Depth > 0)
            sb.Append($" depth {info.Depth}");

        if (info.Nodes > 0)
            sb.Append($" nodes {info.Nodes}");

        if (info.TimeMs > 0)
            sb.Append($" time {info.TimeMs}");

        if (info.Score != 0)
            sb.Append($" score cp {info.Score}");

        if (info.PV.Length > 0)
            sb.Append($" pv {string.Join(" ", info.PV)}");

        Console.Out.WriteLine(sb.ToString());
        Console.Out.Flush();
    }
}
