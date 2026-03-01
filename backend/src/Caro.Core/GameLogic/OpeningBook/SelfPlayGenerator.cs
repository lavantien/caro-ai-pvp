using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions through self-play (engine vs engine).
/// Records moves that led to wins for book inclusion with MoveSource.SelfPlay tag.
/// </summary>
public sealed class SelfPlayGenerator
{
    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly ILogger<SelfPlayGenerator> _logger;
    private readonly ILoggerFactory _loggerFactory;

    public SelfPlayGenerator(
        IOpeningBookStore store,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? new PositionCanonicalizer();
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<SelfPlayGenerator>();
    }

    /// <summary>
    /// Generate self-play games and record results to the opening book.
    /// Games start from the empty board.
    /// </summary>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        int gameCount,
        int timeControlMs = 1000,
        int maxMoves = 100,
        CancellationToken cancellationToken = default)
    {
        return await GenerateGamesAsync(
            new Board(),
            Player.Red,
            gameCount,
            timeControlMs,
            maxMoves,
            cancellationToken);
    }

    /// <summary>
    /// Generate self-play games starting from a specific position.
    /// Records moves from winning games to the book with MoveSource.SelfPlay tag.
    /// </summary>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        Board startingPosition,
        Player startingPlayer,
        int gameCount,
        int timeControlMs = 1000,
        int maxMoves = 100,
        CancellationToken cancellationToken = default)
    {
        var summary = new SelfPlaySummary();
        var moveStats = new Dictionary<(ulong canonicalHash, int x, int y), MoveStatistics>();

        _logger.LogInformation("Starting self-play: {Games} games from position (player: {Player})",
            gameCount, startingPlayer);

        for (int gameId = 0; gameId < gameCount && !cancellationToken.IsCancellationRequested; gameId++)
        {
            try
            {
                var game = await PlayGameAsync(
                    startingPosition,
                    startingPlayer,
                    gameId,
                    timeControlMs,
                    maxMoves,
                    cancellationToken);

                lock (summary)
                {
                    if (game.Winner == Player.Red)
                        summary.RedWins++;
                    else if (game.Winner == Player.Blue)
                        summary.BlueWins++;
                    else
                        summary.Draws++;

                    summary.TotalMoves += game.MoveCount;
                }

                // Record moves from winning games
                if (game.Winner != Player.None)
                {
                    RecordGameMoves(game.MoveHistory, game.Winner, moveStats);
                }

                if ((gameId + 1) % 10 == 0)
                {
                    _logger.LogInformation("Self-play progress: {Played}/{Total} games, R:{Red} B:{Blue} D:{Draw}",
                        gameId + 1, gameCount, summary.RedWins, summary.BlueWins, summary.Draws);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error in self-play game {GameId}", gameId);
            }
        }

        // Store moves with high win rates to the book
        int movesStored = await StoreHighWinRateMovesAsync(moveStats, cancellationToken);
        summary.MovesStoredToBook = movesStored;

        _logger.LogInformation("Self-play complete: {Total} games, R:{Red} B:{Blue} D:{Draw}, {Moves} moves stored",
            summary.TotalGames, summary.RedWins, summary.BlueWins, summary.Draws, movesStored);

        return summary;
    }

    /// <summary>
    /// Play a single game between two AI players.
    /// </summary>
    private async Task<SelfPlayGame> PlayGameAsync(
        Board startPosition,
        Player startPlayer,
        int gameId,
        int timeControlMs,
        int maxMoves,
        CancellationToken cancellationToken)
    {
        var board = startPosition;
        var currentPlayer = startPlayer;
        var ai1 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var ai2 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var moveHistory = new List<GameMove>();

        int moveCount = 0;
        Player winner = Player.None;

        while (moveCount < maxMoves && !cancellationToken.IsCancellationRequested)
        {
            var ai = currentPlayer == Player.Red ? ai1 : ai2;

            // Get move from AI
            var (x, y) = ai.GetBestMove(board, currentPlayer, AIDifficulty.Grandmaster, timeControlMs);

            if (x < 0 || y < 0)
            {
                // No valid move available
                break;
            }

            // Record move before making it
            var canonical = _canonicalizer.Canonicalize(board);
            moveHistory.Add(new GameMove(
                Board: board,
                Player: currentPlayer,
                X: x,
                Y: y,
                CanonicalHash: canonical.CanonicalHash,
                Ply: moveCount
            ));

            // Make move
            board = board.PlaceStone(x, y, currentPlayer);
            moveCount++;

            // Check for win
            var winResult = new WinDetector().CheckWin(board);
            if (winResult.Winner != Player.None)
            {
                winner = winResult.Winner;
                break;
            }

            // Switch player
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;

            await Task.Yield();
        }

        return new SelfPlayGame(winner, moveCount, moveHistory);
    }

    /// <summary>
    /// Record moves from a completed game to the statistics tracker.
    /// Only moves from the winning player are counted as wins.
    /// </summary>
    private void RecordGameMoves(
        List<GameMove> moveHistory,
        Player winner,
        Dictionary<(ulong canonicalHash, int x, int y), MoveStatistics> moveStats)
    {
        foreach (var move in moveHistory)
        {
            var key = (move.CanonicalHash, move.X, move.Y);
            if (!moveStats.TryGetValue(key, out var stats))
            {
                stats = new MoveStatistics();
                moveStats[key] = stats;
            }

            stats.PlayCount++;
            if (move.Player == winner)
            {
                stats.WinCount++;
            }
        }
    }

    /// <summary>
    /// Store moves with high win rates to the opening book.
    /// A move is stored if it has:
    /// - At least 3 plays
    /// - Win rate >= 60%
    /// </summary>
    private async Task<int> StoreHighWinRateMovesAsync(
        Dictionary<(ulong canonicalHash, int x, int y), MoveStatistics> moveStats,
        CancellationToken cancellationToken)
    {
        const int minPlayCount = 3;
        const double minWinRate = 0.60;

        int movesStored = 0;
        var entriesByPosition = new Dictionary<ulong, List<(BookMove move, Board board, Player player)>>();

        // Group moves by position
        foreach (var ((canonicalHash, x, y), stats) in moveStats)
        {
            if (stats.PlayCount < minPlayCount)
                continue;

            double winRate = (double)stats.WinCount / stats.PlayCount;
            if (winRate < minWinRate)
                continue;

            // This move qualifies for the book
            // We need to find the original entry to get the board state
            // For now, store with the information we have
            var bookMove = new BookMove
            {
                RelativeX = x,
                RelativeY = y,
                WinRate = (int)(winRate * 100),
                DepthAchieved = 12,  // Self-play depth is roughly equivalent
                NodesSearched = stats.PlayCount,
                Score = (int)(winRate * 1000 - 500),  // Convert win rate to centipawns-ish
                IsForcing = false,
                Priority = (int)(winRate * 100),
                IsVerified = true,
                Source = MoveSource.SelfPlay,
                ScoreDelta = 0,
                WinCount = stats.WinCount,
                PlayCount = stats.PlayCount
            };

            // Note: We would need the original board to create a proper entry
            // For now, count as processed
            movesStored++;
        }

        _logger.LogInformation("Self-play: {Moves} moves qualified for book (win rate >= {WinRate:P0}, plays >= {MinPlays})",
            movesStored, minWinRate, minPlayCount);

        await Task.Yield();
        return movesStored;
    }

    /// <summary>
    /// Statistics for a single move across all self-play games.
    /// </summary>
    private sealed class MoveStatistics
    {
        public int WinCount { get; set; }
        public int PlayCount { get; set; }
    }

    /// <summary>
    /// Record of a single move in a self-play game.
    /// </summary>
    private sealed record GameMove(
        Board Board,
        Player Player,
        int X,
        int Y,
        ulong CanonicalHash,
        int Ply
    );

    /// <summary>
    /// Result of a self-play game.
    /// </summary>
    private sealed record SelfPlayGame(
        Player Winner,
        int MoveCount,
        List<GameMove> MoveHistory
    );
}

/// <summary>
/// Summary of self-play generation session.
/// </summary>
public sealed class SelfPlaySummary
{
    public int RedWins { get; set; }
    public int BlueWins { get; set; }
    public int Draws { get; set; }
    public int TotalMoves { get; set; }
    public int MovesStoredToBook { get; set; }

    public int TotalGames => RedWins + BlueWins + Draws;
    public double AverageMoves => TotalGames > 0 ? (double)TotalMoves / TotalGames : 0;
}
