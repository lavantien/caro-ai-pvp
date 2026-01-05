using Caro.Core.Entities;
using Caro.Core.GameLogic;
using System.Diagnostics;

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
        if (x < 0 || x >= 15 || y < 0 || y >= 15)
            return false;

        // Open Rule: Red's second move (move #3) cannot be in center 3x3
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
/// </summary>
public class TournamentEngine
{
    private readonly MinimaxAI _ai = new();
    private readonly MoveValidator _moveValidator = new();

    /// <summary>
    /// Run a single game between two AI opponents with chess clock time controls
    /// </summary>
    public MatchResult RunGame(
        AIDifficulty redDifficulty,
        AIDifficulty blueDifficulty,
        int maxMoves = 225,
        int initialTimeSeconds = 180,  // 3 minutes per player
        int incrementSeconds = 2,      // +2 seconds per move
        bool ponderingEnabled = true,   // Enable pondering
        MoveCallback? onMove = null,    // Callback when a move is played
        BoardCallback? onBoardUpdate = null,  // Callback for board state updates
        LogCallback? onLog = null,      // Callback for structured logging
        string redBotName = "Red",
        string blueBotName = "Blue"
    )
    {
        // Clear all AI state to prevent cross-contamination between games
        // This is CRITICAL when bots of different difficulties play in sequence
        _ai.ClearAllState();

        // Log game start
        onLog?.Invoke("info", "system", $"Game started: {redBotName} ({redDifficulty}) vs {blueBotName} ({blueDifficulty}) | Time: {initialTimeSeconds}s+{incrementSeconds}s | Max moves: {maxMoves}");

        var board = new Board();
        var game = new GameState();
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
            var difficulty = currentPlayer == Player.Red ? redDifficulty : blueDifficulty;
            var isRed = currentPlayer == Player.Red;
            var moveNumber = totalMoves + 1;  // 1-indexed move number

            // Time the AI move
            var moveStopwatch = Stopwatch.StartNew();
            var (x, y) = _ai.GetBestMove(board, currentPlayer, difficulty,
                isRed ? redTimeRemainingMs : blueTimeRemainingMs, moveNumber: moveNumber,
                ponderingEnabled: ponderingEnabled);
            moveStopwatch.Stop();

            // Validate move (single source of truth for all move validation)
            if (!_moveValidator.IsValidMove(board, x, y, currentPlayer, moveNumber))
            {
                // Move is invalid - AI loses for making illegal move
                onLog?.Invoke("error", currentPlayer.ToString().ToLower(), $"ILLEGAL MOVE at ({x},{y}) move #{moveNumber}");
                Console.WriteLine($"[ILLEGAL MOVE] {currentPlayer} attempted invalid move ({x}, {y}) at move #{moveNumber}");
                game.EndGame(
                    currentPlayer == Player.Red ? Player.Blue : Player.Red,
                    new List<Position>()
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
                game.RecordMove(board, x, y);

                // Get search statistics for this move
                var (depthAchieved, nodesSearched, nodesPerSecond, tableHitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched) = _ai.GetSearchStatistics();
                var stats = new MoveStats(depthAchieved, nodesSearched, nodesPerSecond, tableHitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched);

                // Log the move with stats
                var timeStr = $"{moveTimeMs}ms";
                var ponderStr = ponderingActive ? " [pondering]" : "";
                onLog?.Invoke("info", currentPlayer.ToString().ToLower(),
                    $"Move #{moveNumber}: ({x},{y}) | {timeStr}{ponderStr} | Depth: {depthAchieved} | Nodes: {nodesSearched:N0} | NPS: {nodesPerSecond:N0} | VCF: {vcfDepthAchieved}d/{vcfNodesSearched:N0}n");

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
                    game.EndGame(result.Winner, result.WinningLine);
                    break;
                }

                totalMoves++;
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
            winnerDifficulty = winner == Player.Red ? redDifficulty : blueDifficulty;
            loserDifficulty = loser == Player.Red ? redDifficulty : blueDifficulty;
        }
        else if (endedByTimeout)
        {
            // Timeout = loss for the player who timed out
            winner = timeoutPlayer == Player.Red ? Player.Blue : Player.Red;
            loser = timeoutPlayer;
            winnerDifficulty = winner == Player.Red ? redDifficulty : blueDifficulty;
            loserDifficulty = loser == Player.Red ? redDifficulty : blueDifficulty;

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
            for (int x = 0; x < 15; x++)
            {
                for (int y = 0; y < 15; y++)
                {
                    if (board.GetCell(x, y).Player != Player.None)
                        occupiedCells++;
                }
            }

            // Debug output for draws
            onLog?.Invoke("info", "system",
                $"DRAW: {redDifficulty} vs {blueDifficulty} | Moves: {totalMoves}/{maxMoves} | Board: {occupiedCells}/225 cells");
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
