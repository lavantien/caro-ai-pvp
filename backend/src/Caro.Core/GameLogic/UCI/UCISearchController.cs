using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using System.Diagnostics;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Bridges UCI go command parameters to MinimaxAI search.
/// Manages search lifecycle with cancellation support.
/// </summary>
public sealed class UCISearchController
{
    private readonly MinimaxAI _ai;
    private readonly UCIEngineOptions _options;
    private CancellationTokenSource? _searchCts;
    private Task<(int x, int y)>? _searchTask;
    private Board? _currentBoard;
    private Player _currentPlayer;

    /// <summary>
    /// Search statistics for info messages.
    /// </summary>
    public SearchStats LastStats { get; private set; } = new();

    /// <summary>
    /// Event raised when search info becomes available.
    /// </summary>
    public event Action<SearchInfo>? OnSearchInfo;

    /// <summary>
    /// Event raised when search completes with a best move.
    /// </summary>
    public event Action<(int x, int y)>? OnBestMove;

    public UCISearchController(MinimaxAI ai, UCIEngineOptions options)
    {
        _ai = ai ?? throw new ArgumentNullException(nameof(ai));
        _options = options ?? throw new ArgumentNullException(nameof(options));
    }

    /// <summary>
    /// Start a search with the given parameters.
    /// </summary>
    public void StartSearch(Board board, Player player, UCIGoParameters goParams)
    {
        // Stop any ongoing search
        StopSearch();

        _currentBoard = board;
        _currentPlayer = player;
        _searchCts = new CancellationTokenSource();

        // Start search in background
        _searchTask = Task.Run(() => ExecuteSearch(board, player, goParams, _searchCts.Token));

        // Fire event when search completes
        _searchTask.ContinueWith(t =>
        {
            if (t.IsCompletedSuccessfully)
            {
                OnBestMove?.Invoke(t.Result);
            }
        }, TaskContinuationOptions.OnlyOnRanToCompletion);
    }

    /// <summary>
    /// Stop the current search and return the best move found so far.
    /// </summary>
    public (int x, int y)? StopSearch()
    {
        if (_searchCts != null && !_searchCts.IsCancellationRequested)
        {
            _searchCts.Cancel();

            if (_searchTask != null && _searchTask.Status == TaskStatus.RanToCompletion)
            {
                var result = _searchTask.Result;
                _searchTask = null;
                return result;
            }
        }

        _searchTask = null;
        return null;
    }

    /// <summary>
    /// Wait for the current search to complete.
    /// </summary>
    public async Task<(int x, int y)> WaitForCompletion()
    {
        if (_searchTask == null)
            throw new InvalidOperationException("No search in progress");

        var result = await _searchTask;
        _searchTask = null;
        return result;
    }

    /// <summary>
    /// Check if a search is currently in progress.
    /// </summary>
    public bool IsSearching => _searchTask != null &&
                               !_searchTask.IsCompleted &&
                               !_searchTask.IsCanceled &&
                               !_searchTask.IsFaulted;

    private (int x, int y) ExecuteSearch(Board board, Player player, UCIGoParameters goParams, CancellationToken ct)
    {
        var stopwatch = Stopwatch.StartNew();
        var difficulty = _options.GetDifficulty();

        try
        {
            // Calculate time allocation from go parameters
            long? timeRemainingMs = null;

            if (player == Player.Red && goParams.WhiteTimeMs.HasValue)
            {
                timeRemainingMs = goParams.WhiteTimeMs.Value;
            }
            else if (player == Player.Blue && goParams.BlackTimeMs.HasValue)
            {
                timeRemainingMs = goParams.BlackTimeMs.Value;
            }

            // If time control provided, calculate remaining time after increment
            if (timeRemainingMs.HasValue)
            {
                var incrementMs = (player == Player.Red ? goParams.WhiteIncrementMs : goParams.BlackIncrementMs) ?? 0;
                // Don't add increment here - it will be added after move completes
                timeRemainingMs = timeRemainingMs.Value;
            }

            // Get best move from AI
            var (x, y) = _ai.GetBestMove(
                board,
                player,
                difficulty,
                timeRemainingMs,
                moveNumber: 0,
                ponderingEnabled: _options.Ponder,
                parallelSearchEnabled: _options.Threads > 1
            );

            stopwatch.Stop();

            // Update final stats
            var (depthAchieved, nodesSearched, _, _, _, _, _, _, _, _, _, _) = _ai.GetSearchStatistics();
            LastStats = new SearchStats
            {
                Depth = depthAchieved,
                Nodes = nodesSearched,
                TimeMs = stopwatch.ElapsedMilliseconds,
                BestMove = (x, y)
            };

            // Emit final info
            OnSearchInfo?.Invoke(new SearchInfo
            {
                Depth = LastStats.Depth,
                Nodes = LastStats.Nodes,
                TimeMs = LastStats.TimeMs,
                Score = 0,  // Caro doesn't have centipawn scores
                PV = new[] { UCIMoveNotation.ToUCI(x, y) }
            });

            return (x, y);
        }
        catch (OperationCanceledException)
        {
            // Search was stopped - return current best or center
            return GetFallbackMove(board);
        }
    }

    private (int x, int y) GetFallbackMove(Board board)
    {
        // Try to find empty cell near center
        int center = board.BoardSize / 2;

        // Search in expanding spiral from center
        for (int radius = 0; radius < board.BoardSize; radius++)
        {
            for (int dx = -radius; dx <= radius; dx++)
            {
                for (int dy = -radius; dy <= radius; dy++)
                {
                    int x = center + dx;
                    int y = center + dy;
                    if (x >= 0 && x < board.BoardSize && y >= 0 && y < board.BoardSize)
                    {
                        if (board.IsEmpty(x, y))
                            return (x, y);
                    }
                }
            }
        }

        return (center, center);
    }
}

/// <summary>
/// Parameters parsed from UCI go command.
/// </summary>
public sealed class UCIGoParameters
{
    public long? WhiteTimeMs { get; set; }
    public long? BlackTimeMs { get; set; }
    public int? WhiteIncrementMs { get; set; }
    public int? BlackIncrementMs { get; set; }
    public int? MoveTimeMs { get; set; }
    public int? Depth { get; set; }
    public int? Nodes { get; set; }
    public bool Infinite { get; set; }

    /// <summary>
    /// Parse go command parameters.
    /// </summary>
    /// <example>
    /// "go wtime 180000 btime 180000 winc 2000 binc 2000"
    /// </example>
    public static UCIGoParameters Parse(string[] args)
    {
        var result = new UCIGoParameters();

        for (int i = 0; i < args.Length; i++)
        {
            switch (args[i].ToLowerInvariant())
            {
                case "wtime":
                    if (i + 1 < args.Length && long.TryParse(args[i + 1], out long wtime))
                        result.WhiteTimeMs = wtime;
                    break;

                case "btime":
                    if (i + 1 < args.Length && long.TryParse(args[i + 1], out long btime))
                        result.BlackTimeMs = btime;
                    break;

                case "winc":
                    if (i + 1 < args.Length && int.TryParse(args[i + 1], out int winc))
                        result.WhiteIncrementMs = winc;
                    break;

                case "binc":
                    if (i + 1 < args.Length && int.TryParse(args[i + 1], out int binc))
                        result.BlackIncrementMs = binc;
                    break;

                case "movetime":
                    if (i + 1 < args.Length && int.TryParse(args[i + 1], out int movetime))
                        result.MoveTimeMs = movetime;
                    break;

                case "depth":
                    if (i + 1 < args.Length && int.TryParse(args[i + 1], out int depth))
                        result.Depth = depth;
                    break;

                case "nodes":
                    if (i + 1 < args.Length && int.TryParse(args[i + 1], out int nodes))
                        result.Nodes = nodes;
                    break;

                case "infinite":
                    result.Infinite = true;
                    break;
            }
        }

        return result;
    }
}

/// <summary>
/// Search statistics for reporting.
/// </summary>
public sealed record SearchStats
{
    public int Depth { get; init; }
    public long Nodes { get; init; }
    public long TimeMs { get; init; }
    public (int x, int y)? BestMove { get; init; }
}

/// <summary>
/// Search info for UCI info messages.
/// </summary>
public sealed class SearchInfo
{
    public int Depth { get; init; }
    public long Nodes { get; init; }
    public long TimeMs { get; init; }
    public int Score { get; init; }
    public string[] PV { get; init; } = Array.Empty<string>();
}
