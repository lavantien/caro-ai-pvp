using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions through self-play (engine vs engine).
/// Records moves that led to wins/draws for later analysis and book inclusion.
/// </summary>
public sealed class SelfPlayGenerator
{
    private readonly IOpeningBookStore _store;
    private readonly ILogger<SelfPlayGenerator> _logger;
    private readonly ILoggerFactory _loggerFactory;

    public SelfPlayGenerator(
        IOpeningBookStore store,
        ILoggerFactory? loggerFactory = null)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<SelfPlayGenerator>();
    }

    /// <summary>
    /// Generate self-play games and record results.
    /// </summary>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        int gameCount,
        int timeControlMs = 1000,
        int maxMoves = 100,
        CancellationToken cancellationToken = default)
    {
        var summary = new SelfPlaySummary();

        for (int gameId = 0; gameId < gameCount && !cancellationToken.IsCancellationRequested; gameId++)
        {
            try
            {
                var game = await PlayGameAsync(gameId, timeControlMs, maxMoves, cancellationToken);

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

        _logger.LogInformation("Self-play complete: {Total} games, R:{Red} B:{Blue} D:{Draw}",
            gameCount, summary.RedWins, summary.BlueWins, summary.Draws);

        return summary;
    }

    /// <summary>
    /// Play a single game between two AI players.
    /// </summary>
    private async Task<(Player Winner, int MoveCount)> PlayGameAsync(
        int gameId,
        int timeControlMs,
        int maxMoves,
        CancellationToken cancellationToken)
    {
        var board = new Board();
        var currentPlayer = Player.Red;
        var ai1 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var ai2 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());

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

        return (winner, moveCount);
    }
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

    public int TotalGames => RedWins + BlueWins + Draws;
    public double AverageMoves => TotalGames > 0 ? (double)TotalMoves / TotalGames : 0;
}
