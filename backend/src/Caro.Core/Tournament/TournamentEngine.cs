using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Threading.Channels;
using System.Threading.Tasks;

namespace Caro.Core.Tournament;

/// <summary>
/// Validates moves according to Caro rules including open rule
/// </summary>
public class MoveValidator
{
    private readonly OpenRuleValidator _openRuleValidator = new();

    /// <summary>
    /// Validate a move according to all game rules
    /// </summary>
    public bool IsValidMove(Board board, int x, int y, Player player, int moveNumber)
    {
        // Check if cell is empty
        if (board.GetCell(x, y).Player != Player.None)
            return false;

        // Check coordinates are valid
        if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
            return false;

        // Open Rule: Red's second move (move #3) must be at least 3 intersections away from first red stone
        // This creates a 5x5 exclusion zone centered on the first move
        if (player == Player.Red && moveNumber == 3)
        {
            if (!_openRuleValidator.IsValidSecondMove(board, x, y))
                return false;
        }

        return true;
    }
}

/// <summary>
/// Callback for when a move is played
/// </summary>
public delegate void MoveCallback(int x, int y, Player player, int moveNumber, long redTimeMs, long blueTimeMs, MoveStats? stats);

/// <summary>
/// Callback for when board state updates (includes move info for atomic updates)
/// </summary>
public delegate void BoardCallback(Board board, int moveNumber, long redTimeMs, long blueTimeMs, int lastMoveX, int lastMoveY, Player lastMovePlayer);

/// <summary>
/// Callback for structured game logging
/// </summary>
public delegate void LogCallback(string level, string source, string message);

/// <summary>
/// Runs automated AI vs AI tournaments
/// Uses stats publisher-subscriber pattern for receiving move statistics
/// </summary>
public class TournamentEngine
{
    private readonly MinimaxAI _botA;
    private readonly MinimaxAI _botB;
    private readonly MoveValidator _moveValidator = new();

    /// <summary>
    /// Constructor with dependency injection for AI instances.
    /// </summary>
    public TournamentEngine(MinimaxAI botA, MinimaxAI botB)
    {
        _botA = botA ?? throw new ArgumentNullException(nameof(botA));
        _botB = botB ?? throw new ArgumentNullException(nameof(botB));
    }

    /// <summary>
    /// Create a TournamentEngine with default AI instances for standalone testing.
    /// This factory method creates MinimaxAI instances with default settings.
    /// </summary>
    public static TournamentEngine CreateDefault()
    {
        var botA = new MinimaxAI();
        var botB = new MinimaxAI();
        return new TournamentEngine(botA, botB);
    }

    // Stats subscriber tasks
    private CancellationTokenSource? _statsCts;
    private Task? _botAStatsTask;
    private Task? _botBStatsTask;

    // Cached ponder stats from async subscriber (indexed by Player color, not bot)
    private long _redPonderNodes;
    private double _redPonderNps;
    private int _redPonderDepth;
    private long _bluePonderNodes;
    private double _bluePonderNps;
    private int _bluePonderDepth;

    /// <summary>
    /// Run a single game between two AI opponents with chess clock time controls
    /// </summary>
    public MatchResult RunGame(
        AIDifficulty redDifficulty,
        AIDifficulty blueDifficulty,
        int maxMoves = 1024,
        int initialTimeSeconds = 180,  // 3 minutes per player
        int incrementSeconds = 2,      // +2 seconds per move
        bool ponderingEnabled = false,
        bool parallelSearchEnabled = false,
        MoveCallback? onMove = null,    // Callback when a move is played
        BoardCallback? onBoardUpdate = null,  // Callback for board state updates
        LogCallback? onLog = null,      // Callback for structured logging
        string redBotName = "Red",
        string blueBotName = "Blue",
        bool swapColors = false  // If true, BotA plays as Blue, BotB plays as Red
    )
    {
        // Map bots to colors based on swap parameter
        // swapColors=false: BotA (with redDifficulty capabilities) plays Red, BotB (with blueDifficulty) plays Blue
        // swapColors=true: BotA plays Blue, BotB plays Red (each bot keeps its difficulty capabilities)
        var botAIsRed = !swapColors;
        var redAI = botAIsRed ? _botA : _botB;
        var blueAI = botAIsRed ? _botB : _botA;

        // Map difficulties to bots (not to colors)
        // BotA always uses redDifficulty config, BotB always uses blueDifficulty config
        // When swapped, Red gets BotB (with blueDifficulty), Blue gets BotA (with redDifficulty)
        var botADifficulty = redDifficulty;   // BotA's difficulty
        var botBDifficulty = blueDifficulty;  // BotB's difficulty

        // Clear all AI state to prevent cross-contamination between games
        // This is CRITICAL when bots of different difficulties play in sequence
        _botA.ClearAllState();
        _botB.ClearAllState();

        // Reset cached ponder stats
        _redPonderNodes = 0;
        _redPonderNps = 0;
        _redPonderDepth = 0;
        _bluePonderNodes = 0;
        _bluePonderNps = 0;
        _bluePonderDepth = 0;

        // Get per-player difficulty settings from centralized config
        var redSettings = AIDifficultyConfig.Instance.GetSettings(redDifficulty);
        var blueSettings = AIDifficultyConfig.Instance.GetSettings(blueDifficulty);

        // Start stats subscriber tasks - subscribe based on which bot is playing which color
        _statsCts = new CancellationTokenSource();
        _botAStatsTask = Task.Run(() => SubscribeStatsAsync(_botA, _botA.StatsChannel, botAIsRed ? Player.Red : Player.Blue, _statsCts.Token));
        _botBStatsTask = Task.Run(() => SubscribeStatsAsync(_botB, _botB.StatsChannel, botAIsRed ? Player.Blue : Player.Red, _statsCts.Token));

        // Log game start with actual color assignments
        onLog?.Invoke("info", "system", $"Game started: {redBotName} ({redDifficulty}) vs {blueBotName} ({blueDifficulty}) | Time: {initialTimeSeconds}s+{incrementSeconds}s | Max moves: {maxMoves}");

        var game = GameState.CreateInitial();
        var board = game.Board;
        var moveTimesMs = new List<long>();
        var stopwatch = Stopwatch.StartNew();
        var totalMoves = 0;
        var endedByTimeout = false;
        var timeoutPlayer = Player.None;

        // Chess clock time banks (in milliseconds)
        long redTimeRemainingMs = (long)initialTimeSeconds * 1000;
        long blueTimeRemainingMs = (long)initialTimeSeconds * 1000;
        long incrementMs = (long)incrementSeconds * 1000;

        // Run game until win, draw, max moves, or time runs out
        while (totalMoves < maxMoves && !game.IsGameOver)
        {
            var currentPlayer = game.CurrentPlayer;
            var isRed = currentPlayer == Player.Red;
            var moveNumber = totalMoves + 1;  // 1-indexed move number

            // Use the appropriate AI instance for this player
            var currentAI = isRed ? redAI : blueAI;

            // Determine which bot is playing, and use that bot's difficulty
            // BotA always uses botADifficulty, BotB always uses botBDifficulty
            var currentBotIsA = (isRed && botAIsRed) || (!isRed && !botAIsRed);
            var difficulty = currentBotIsA ? botADifficulty : botBDifficulty;

            // Use THIS BOT's difficulty settings for pondering and parallel search
            var currentSettings = currentBotIsA ? redSettings : blueSettings;
            var playerPonderingEnabled = ponderingEnabled && currentSettings.PonderingEnabled;
            var playerParallelEnabled = parallelSearchEnabled && currentSettings.ParallelSearchEnabled;

            // Stop current player's pondering (from during opponent's previous move) and get their stats
            // Pass the AI's actual color (currentPlayer) for correct stats attribution
            currentAI.StopPondering(currentPlayer);
            var (ponderDepth, ponderNodes, ponderNps, _) = currentAI.GetLastPonderStats(currentPlayer);

            // Start opponent's pondering IMMEDIATELY (opponent will think during current player's move)
            var opponentAI = isRed ? blueAI : redAI;
            var opponentBotIsA = (isRed && !botAIsRed) || (!isRed && botAIsRed);  // Opponent is the other bot
            var opponentSettings = opponentBotIsA ? redSettings : blueSettings;
            var opponentColor = isRed ? Player.Blue : Player.Red;  // Explicit opponent color
            var opponentPonderingEnabled = ponderingEnabled && opponentSettings.PonderingEnabled;
            if (opponentPonderingEnabled)
            {
                onLog?.Invoke("debug", "system", $"Starting pondering: opponent={opponentColor}, difficulty={opponentSettings.Difficulty}");
                opponentAI.StartPonderingNow(board, currentPlayer, opponentSettings.Difficulty, opponentColor);
            }

            // Time the AI move (opponent is pondering in background)
            var moveStopwatch = Stopwatch.StartNew();
            var (x, y) = currentAI.GetBestMove(board, currentPlayer, difficulty,
                isRed ? redTimeRemainingMs : blueTimeRemainingMs, moveNumber: moveNumber,
                ponderingEnabled: false, parallelSearchEnabled: playerParallelEnabled);
            moveStopwatch.Stop();

            // Validate move (single source of truth for all move validation)
            if (!_moveValidator.IsValidMove(board, x, y, currentPlayer, moveNumber))
            {
                // Move is invalid - AI loses for making illegal move
                onLog?.Invoke("error", currentPlayer.ToString().ToLower(), $"ILLEGAL MOVE at ({x},{y}) move #{moveNumber}");
                Console.WriteLine($"[ILLEGAL MOVE] {currentPlayer} attempted invalid move ({x}, {y}) at move #{moveNumber}");
                game = game.WithGameOver(
                    currentPlayer == Player.Red ? Player.Blue : Player.Red,
                    ImmutableArray<Position>.Empty
                );
                break;
            }

            var moveTimeMs = moveStopwatch.ElapsedMilliseconds;
            moveTimesMs.Add(moveTimeMs);

            // Update time bank
            if (isRed)
            {
                redTimeRemainingMs -= moveTimeMs;
                redTimeRemainingMs += incrementMs;
            }
            else
            {
                blueTimeRemainingMs -= moveTimeMs;
                blueTimeRemainingMs += incrementMs;
            }

            // Check for timeout (player ran out of time)
            if ((isRed && redTimeRemainingMs <= 0) || (!isRed && blueTimeRemainingMs <= 0))
            {
                endedByTimeout = true;
                timeoutPlayer = currentPlayer;
                break;
            }

            // Make the move
            try
            {
                game = game.WithMove(x, y);
                board = game.Board;

                // Get search statistics for this move from the correct AI
                var (depthAchieved, nodesSearched, nodesPerSecond, tableHitRate, _, vcfDepthAchieved, vcfNodesSearched, threadCount, parallelDiagnostics, masterTTPercent, helperAvgDepth, allocatedTimeMs, bookUsed) = currentAI.GetSearchStatistics();

                // Determine if current player supports pondering and actually did ponder work
                bool ponderingActive = playerPonderingEnabled && (ponderNodes > 0 || currentSettings.Difficulty >= AIDifficulty.Hard);

                var stats = new MoveStats(depthAchieved, nodesSearched, nodesPerSecond, tableHitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched, threadCount, parallelDiagnostics, moveTimeMs, masterTTPercent, helperAvgDepth, allocatedTimeMs, ponderNodes, ponderNps, ponderDepth, bookUsed);

                // Log the move with stats
                var timeStr = $"{moveTimeMs}ms";
                var ponderStr = ponderingActive ? " [pondering]" : "";
                onLog?.Invoke("info", currentPlayer.ToString().ToLower(),
                    $"Move #{moveNumber}: ({x},{y}) | {timeStr}{ponderStr} | Depth: {depthAchieved} | Nodes: {nodesSearched:N0} | NPS: {nodesPerSecond:N0} | Threads: {threadCount} | VCF: {vcfDepthAchieved}d/{vcfNodesSearched:N0}n");

                // Call move callback with stats
                onMove?.Invoke(x, y, currentPlayer, totalMoves + 1, redTimeRemainingMs, blueTimeRemainingMs, stats);

                // Call board update callback with move info for atomic updates
                onBoardUpdate?.Invoke(board, totalMoves + 1, redTimeRemainingMs, blueTimeRemainingMs, x, y, currentPlayer);

                // Check for win
                var detector = new WinDetector();
                var result = detector.CheckWin(board);
                if (result.HasWinner)
                {
                    var winBy = result.Winner == Player.Red ? redBotName : blueBotName;
                    var winByDiff = result.Winner == Player.Red ? redDifficulty : blueDifficulty;
                    onLog?.Invoke("info", "system", $"WIN by {winBy} ({winByDiff}) at move #{moveNumber} | Line: {string.Join(", ", result.WinningLine.Select(p => $"({p.X},{p.Y})"))}");
                    game = game.WithGameOver(result.Winner, result.WinningLine.ToImmutableArray());
                    break;
                }

                totalMoves++;

                // Start current player's pondering for opponent's response (runs during opponent's next turn)
                if (playerPonderingEnabled)
                {
                    onLog?.Invoke("debug", "system", $"Starting pondering after move: player={currentPlayer}, difficulty={currentSettings.Difficulty}");
                    currentAI.StartPonderingAfterMove(board, currentPlayer, currentPlayer, currentSettings.Difficulty);
                }
            }
            catch (Exception ex)
            {
                // Log the exception with details
                onLog?.Invoke("error", currentPlayer.ToString().ToLower(),
                    $"EXCEPTION at move #{moveNumber}: {ex.GetType().Name} - {ex.Message}");
                onLog?.Invoke("error", "system", $"Stack trace: {ex.StackTrace?.Substring(0, Math.Min(200, ex.StackTrace.Length))}...");

                // Determine outcome - exception doesn't mean forfeit, could be system error
                // For now, treat as system error resulting in draw
                endedByTimeout = false;
                break;
            }
        }

        stopwatch.Stop();

        // Determine winner
        Player winner;
        Player loser;
        AIDifficulty winnerDifficulty;
        AIDifficulty loserDifficulty;

        if (game.IsGameOver)
        {
            winner = game.Winner;
            loser = winner == Player.Red ? Player.Blue : Player.Red;
            // Map color to bot difficulty (Red may be BotA or BotB depending on swapColors)
            var winnerBotIsA = (winner == Player.Red && botAIsRed) || (winner == Player.Blue && !botAIsRed);
            winnerDifficulty = winnerBotIsA ? botADifficulty : botBDifficulty;
            var loserBotIsA = (loser == Player.Red && botAIsRed) || (loser == Player.Blue && !botAIsRed);
            loserDifficulty = loserBotIsA ? botADifficulty : botBDifficulty;
        }
        else if (endedByTimeout)
        {
            // Timeout = loss for the player who timed out
            winner = timeoutPlayer == Player.Red ? Player.Blue : Player.Red;
            loser = timeoutPlayer;
            // Map color to bot difficulty (Red may be BotA or BotB depending on swapColors)
            var winnerBotIsA = (winner == Player.Red && botAIsRed) || (winner == Player.Blue && !botAIsRed);
            winnerDifficulty = winnerBotIsA ? botADifficulty : botBDifficulty;
            var loserBotIsA = (loser == Player.Red && botAIsRed) || (loser == Player.Blue && !botAIsRed);
            loserDifficulty = loserBotIsA ? botADifficulty : botBDifficulty;

            onLog?.Invoke("warning", "system",
                $"TIMEOUT: {loserDifficulty} ({loser}) ran out of time | {winnerDifficulty} ({winner}) wins by default | Moves: {totalMoves}");
            Console.WriteLine($"\n TIMEOUT: {loserDifficulty} ({loser}) ran out of time");
            Console.WriteLine($"    {winnerDifficulty} ({winner}) wins by default");
            Console.WriteLine($"    Moves before timeout: {totalMoves}");
        }
        else
        {
            // True draw - board full or max moves reached
            var occupiedCells = 0;
            int boardSize = board.BoardSize;
            int totalCells = boardSize * boardSize;
            for (int x = 0; x < boardSize; x++)
            {
                for (int y = 0; y < boardSize; y++)
                {
                    if (board.GetCell(x, y).Player != Player.None)
                        occupiedCells++;
                }
            }

            // Debug output for draws
            onLog?.Invoke("info", "system",
                $"DRAW: {redDifficulty} vs {blueDifficulty} | Moves: {totalMoves}/{maxMoves} | Board: {occupiedCells}/{totalCells} cells");
            Console.WriteLine($"\n DRAW: {redDifficulty} vs {blueDifficulty}");
            Console.WriteLine($"    Total Moves: {totalMoves}/{maxMoves}");
            Console.WriteLine($"    Board Occupied: {occupiedCells}/225 cells");

            winner = Player.None;
            loser = Player.None;
            winnerDifficulty = AIDifficulty.Easy; // placeholder
            loserDifficulty = AIDifficulty.Easy; // placeholder
        }

        // Determine winner and loser bot names
        string winnerBotName = winner != Player.None
            ? (winner == Player.Red ? redBotName : blueBotName)
            : "Draw";
        string loserBotName = loser != Player.None
            ? (loser == Player.Red ? redBotName : blueBotName)
            : "Draw";

        // Final game summary
        var resultStr = winner != Player.None
            ? $"Winner: {winnerBotName} ({winnerDifficulty})"
            : "Draw";
        onLog?.Invoke("info", "system",
            $"Game ended: {resultStr} | Duration: {stopwatch.ElapsedMilliseconds / 1000.0:F1}s | Moves: {totalMoves}");

        // Stop stats subscriber tasks
        _statsCts?.Cancel();
        try
        {
            Task.WhenAll(_botAStatsTask ?? Task.CompletedTask, _botBStatsTask ?? Task.CompletedTask)
                .Wait(TimeSpan.FromSeconds(1));
        }
        catch (AggregateException)
        {
            // Tasks were cancelled - expected
        }
        finally
        {
            _statsCts?.Dispose();
            _statsCts = null;
        }

        return new MatchResult
        {
            Winner = winner,
            Loser = loser,
            TotalMoves = totalMoves,
            DurationMs = stopwatch.ElapsedMilliseconds,
            MoveTimesMs = moveTimesMs,
            WinnerDifficulty = winnerDifficulty,
            LoserDifficulty = loserDifficulty,
            FinalBoard = board,
            WinnerBotName = winnerBotName,
            LoserBotName = loserBotName,
            IsDraw = !game.IsGameOver && !endedByTimeout,
            EndedByTimeout = endedByTimeout
        };
    }

    /// <summary>
    /// Subscribe to stats events from an AI instance
    /// Caches ponder stats for use by the game loop
    /// </summary>
    private async Task SubscribeStatsAsync(MinimaxAI ai, Channel<MoveStatsEvent> channel, Player player, CancellationToken cancellationToken)
    {
        await foreach (var statsEvent in channel.Reader.ReadAllAsync(cancellationToken))
        {
            if (statsEvent.Type == StatsType.Pondering)
            {
                // Cache ponder stats for this player
                if (player == Player.Red)
                {
                    _redPonderNodes = statsEvent.NodesSearched;
                    _redPonderNps = statsEvent.NodesPerSecond;
                    _redPonderDepth = statsEvent.DepthAchieved;
                }
                else
                {
                    _bluePonderNodes = statsEvent.NodesSearched;
                    _bluePonderNps = statsEvent.NodesPerSecond;
                    _bluePonderDepth = statsEvent.DepthAchieved;
                }
            }
        }
    }

    /// <summary>
    /// Run a tournament with multiple games between different AI difficulties
    /// </summary>
    public List<MatchResult> RunTournament(
        Dictionary<(AIDifficulty, AIDifficulty), int> matchups,
        IProgress<TournamentProgress>? progress = null)
    {
        var results = new List<MatchResult>();
        var totalGames = matchups.Values.Sum();
        var completedGames = 0;

        foreach (var (difficulties, gameCount) in matchups)
        {
            var (redDiff, blueDiff) = difficulties;

            for (int i = 0; i < gameCount; i++)
            {
                var result = RunGame(redDiff, blueDiff);
                results.Add(result);
                completedGames++;

                progress?.Report(new TournamentProgress
                {
                    CompletedGames = completedGames,
                    TotalGames = totalGames,
                    CurrentMatch = $"{redDiff} vs {blueDiff}",
                    LatestResult = result
                });
            }
        }

        return results;
    }
}

/// <summary>
/// Tournament progress information
/// </summary>
public class TournamentProgress
{
    public int CompletedGames { get; init; }
    public int TotalGames { get; init; }
    public required string CurrentMatch { get; init; }
    public required MatchResult LatestResult { get; init; }

    public double ProgressPercent => TotalGames > 0
        ? (double)CompletedGames / TotalGames * 100
        : 0;
}
