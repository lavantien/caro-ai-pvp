using Caro.Core.Entities;
using Caro.Core.GameLogic;
using System.Diagnostics;

namespace Caro.Core.Tournament;

/// <summary>
/// Runs automated AI vs AI tournaments
/// </summary>
public class TournamentEngine
{
    private readonly MinimaxAI _ai = new();

    /// <summary>
    /// Run a single game between two AI opponents with chess clock time controls
    /// </summary>
    public MatchResult RunGame(
        AIDifficulty redDifficulty,
        AIDifficulty blueDifficulty,
        int maxMoves = 225,
        int initialTimeSeconds = 180,  // 3 minutes per player
        int incrementSeconds = 2)      // +2 seconds per move
    {
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

            // Time the AI move
            var moveStopwatch = Stopwatch.StartNew();
            var (x, y) = _ai.GetBestMove(board, currentPlayer, difficulty,
                isRed ? redTimeRemainingMs : blueTimeRemainingMs);  // Pass remaining time
            moveStopwatch.Stop();

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

                // Check for win
                var detector = new WinDetector();
                var result = detector.CheckWin(board);
                if (result.HasWinner)
                {
                    game.EndGame(result.Winner, result.WinningLine);
                    break;
                }

                totalMoves++;
            }
            catch
            {
                // Invalid move - AI loses
                game.EndGame(
                    currentPlayer == Player.Red ? Player.Blue : Player.Red,
                    new List<Position>()
                );
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

            Console.WriteLine($"\n⏰ TIMEOUT: {loserDifficulty} ({loser}) ran out of time");
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
            Console.WriteLine($"\n⚠️  DRAW: {redDifficulty} vs {blueDifficulty}");
            Console.WriteLine($"    Total Moves: {totalMoves}/{maxMoves}");
            Console.WriteLine($"    Board Occupied: {occupiedCells}/225 cells");

            winner = Player.None;
            loser = Player.None;
            winnerDifficulty = AIDifficulty.Easy; // placeholder
            loserDifficulty = AIDifficulty.Easy; // placeholder
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
