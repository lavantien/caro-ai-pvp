using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions through self-play (engine vs engine).
///
/// Separated Pipeline Architecture (Actor Phase):
/// - Records ALL moves to staging store (not main book)
/// - Staging is verified later by MoveVerifier (Critic phase)
/// - Only verified moves enter the main book
///
/// Design principles:
/// - All buffer sizes are powers of 2
/// - Fast search for diversity (not deep for accuracy)
/// - No judgment on move quality (that's verification's job)
/// </summary>
public sealed class SelfPlayGenerator
{
    // Default time per move (2^10 ms)
    private const int DefaultTimeControlMs = 1024;

    // Maximum ply to record (opening moves only)
    private const int DefaultMaxPly = 16;

    private readonly IStagingBookStore _stagingStore;
    private readonly IOpeningBookStore? _bookStore;  // Optional: for reading during play
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly ILogger<SelfPlayGenerator> _logger;
    private readonly ILoggerFactory _loggerFactory;

    /// <summary>
    /// Create a self-play generator that records to staging store.
    /// </summary>
    /// <param name="stagingStore">Staging store for raw self-play data</param>
    /// <param name="canonicalizer">Position canonicalizer for symmetry reduction</param>
    /// <param name="loggerFactory">Logger factory for creating loggers</param>
    /// <param name="bookStore">Optional book store for reading during play (not writing)</param>
    public SelfPlayGenerator(
        IStagingBookStore stagingStore,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null,
        IOpeningBookStore? bookStore = null)
    {
        _stagingStore = stagingStore ?? throw new ArgumentNullException(nameof(stagingStore));
        _canonicalizer = canonicalizer ?? new PositionCanonicalizer();
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<SelfPlayGenerator>();
        _bookStore = bookStore;
    }

    /// <summary>
    /// Legacy constructor for backward compatibility.
    /// Creates a staging store wrapper around the book store.
    /// </summary>
    [Obsolete("Use constructor with IStagingBookStore for separated pipeline")]
    public SelfPlayGenerator(
        IOpeningBookStore store,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null)
    {
        // This should not be used in the new separated pipeline
        throw new InvalidOperationException(
            "SelfPlayGenerator now requires IStagingBookStore. " +
            "Use the constructor with IStagingBookStore for the separated pipeline.");
    }

    /// <summary>
    /// Generate self-play games and record results to staging.
    /// Games start from the empty board.
    /// </summary>
    /// <param name="gameCount">Number of games to play</param>
    /// <param name="timeControlMs">Time per move in milliseconds (default: 1024)</param>
    /// <param name="maxMoves">Maximum moves per game</param>
    /// <param name="maxPly">Maximum ply to record (default: 16 for opening moves)</param>
    /// <param name="cancellationToken">Cancellation token</param>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        int gameCount,
        int timeControlMs = DefaultTimeControlMs,
        int maxMoves = 100,
        int maxPly = DefaultMaxPly,
        CancellationToken cancellationToken = default)
    {
        return await GenerateGamesAsync(
            new Board(),
            Player.Red,
            gameCount,
            timeControlMs,
            maxMoves,
            maxPly,
            cancellationToken);
    }

    /// <summary>
    /// Generate self-play games starting from a specific position.
    /// Records moves to staging store for later verification.
    /// </summary>
    /// <param name="startingPosition">Starting board position</param>
    /// <param name="startingPlayer">Player to move first</param>
    /// <param name="gameCount">Number of games to play</param>
    /// <param name="timeControlMs">Time per move in milliseconds (default: 1024)</param>
    /// <param name="maxMoves">Maximum moves per game</param>
    /// <param name="maxPly">Maximum ply to record (default: 16 for opening moves)</param>
    /// <param name="cancellationToken">Cancellation token</param>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        Board startingPosition,
        Player startingPlayer,
        int gameCount,
        int timeControlMs = DefaultTimeControlMs,
        int maxMoves = 100,
        int maxPly = DefaultMaxPly,
        CancellationToken cancellationToken = default)
    {
        var summary = new SelfPlaySummary
        {
            MaxPlyRecorded = maxPly
        };

        _logger.LogInformation(
            "Starting self-play: {Games} games from position (player: {Player}), " +
            "time: {TimeMs}ms, max ply: {MaxPly}",
            gameCount, startingPlayer, timeControlMs, maxPly);

        // Initialize staging store
        _stagingStore.Initialize();

        // Get starting game ID from staging store
        long gameOffset = _stagingStore.GetGameCount();

        for (int gameIndex = 0; gameIndex < gameCount && !cancellationToken.IsCancellationRequested; gameIndex++)
        {
            try
            {
                var gameId = gameOffset + gameIndex;
                var game = await PlayGameAsync(
                    startingPosition,
                    startingPlayer,
                    gameId,
                    timeControlMs,
                    maxMoves,
                    cancellationToken);

                // Update summary statistics
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

                // Record moves to staging store (Actor phase - no judgment)
                RecordGameMovesToStaging(game.MoveHistory, game.Winner, gameId, timeControlMs, maxPly);
                summary.StagingMovesRecorded += Math.Min(game.MoveHistory.Count, maxPly);

                // Progress reporting (every 10 games or at end)
                if ((gameIndex + 1) % 10 == 0 || gameIndex == gameCount - 1)
                {
                    _logger.LogInformation(
                        "Self-play progress: {Played}/{Total} games, R:{Red} B:{Blue} D:{Draw}, " +
                        "staging moves: {StagingMoves}",
                        gameIndex + 1, gameCount, summary.RedWins, summary.BlueWins, summary.Draws,
                        summary.StagingMovesRecorded);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error in self-play game {GameId}", gameIndex);
            }
        }

        // Flush staging store to ensure all data is persisted
        _stagingStore.Flush();

        _logger.LogInformation(
            "Self-play complete: {Total} games, R:{Red} B:{Blue} D:{Draw}, {Moves} moves to staging",
            summary.TotalGames, summary.RedWins, summary.BlueWins, summary.Draws,
            summary.StagingMovesRecorded);

        return summary;
    }

    /// <summary>
    /// Play a single game between two AI players.
    /// Uses shallow search for diversity (not depth for accuracy).
    /// </summary>
    private async Task<SelfPlayGame> PlayGameAsync(
        Board startPosition,
        Player startPlayer,
        long gameId,
        int timeControlMs,
        int maxMoves,
        CancellationToken cancellationToken)
    {
        var board = startPosition;
        var currentPlayer = startPlayer;

        // Shallow AI for diversity (depth 8 - fast exploration)
        var ai1 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var ai2 = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());

        var moveHistory = new List<GameMoveRecord>();
        int moveCount = 0;
        Player winner = Player.None;

        while (moveCount < maxMoves && !cancellationToken.IsCancellationRequested)
        {
            var ai = currentPlayer == Player.Red ? ai1 : ai2;

            // Get move from AI (use book if available for variety)
            (int x, int y) move;
            if (_bookStore != null)
            {
                var canonical = _canonicalizer.Canonicalize(board);
                var entry = _bookStore.GetEntry(canonical.CanonicalHash, currentPlayer);
                if (entry != null && entry.Moves.Length > 0)
                {
                    // Use book move with some randomization for variety
                    var random = new Random();
                    var bookMoves = entry.Moves.Take(4).ToList();  // Top 4 moves
                    var selectedMove = bookMoves[random.Next(bookMoves.Count)];
                    move = (selectedMove.RelativeX, selectedMove.RelativeY);
                }
                else
                {
                    move = ai.GetBestMove(board, currentPlayer, AIDifficulty.Grandmaster, timeControlMs);
                }
            }
            else
            {
                move = ai.GetBestMove(board, currentPlayer, AIDifficulty.Grandmaster, timeControlMs);
            }

            if (move.x < 0 || move.y < 0)
            {
                // No valid move available
                break;
            }

            // Record move before making it
            var canonicalResult = _canonicalizer.Canonicalize(board);
            var directHash = board.GetHash();

            moveHistory.Add(new GameMoveRecord(
                Board: board,
                Player: currentPlayer,
                X: move.x,
                Y: move.y,
                CanonicalHash: canonicalResult.CanonicalHash,
                DirectHash: directHash,
                Ply: moveCount
            ));

            // Make move
            board = board.PlaceStone(move.x, move.y, currentPlayer);
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
    /// Record moves from a completed game to the staging store.
    /// All moves are recorded (Actor phase) - verification (Critic) happens later.
    /// </summary>
    private void RecordGameMovesToStaging(
        List<GameMoveRecord> moveHistory,
        Player winner,
        long gameId,
        int timeBudgetMs,
        int maxPly)
    {
        foreach (var move in moveHistory)
        {
            // Only record opening moves (ply filter)
            if (move.Ply >= maxPly)
                continue;

            // Calculate game result from the perspective of the move's player
            int gameResult = winner == move.Player ? 1 :
                             winner == Player.None ? 0 : -1;

            _stagingStore.RecordMove(
                canonicalHash: move.CanonicalHash,
                directHash: move.DirectHash,
                player: move.Player,
                ply: move.Ply,
                moveX: move.X,
                moveY: move.Y,
                gameResult: gameResult,
                gameId: gameId,
                timeBudgetMs: timeBudgetMs
            );
        }
    }

    /// <summary>
    /// Record of a single move in a self-play game.
    /// </summary>
    private sealed record GameMoveRecord(
        Board Board,
        Player Player,
        int X,
        int Y,
        ulong CanonicalHash,
        ulong DirectHash,
        int Ply
    );

    /// <summary>
    /// Result of a self-play game.
    /// </summary>
    private sealed record SelfPlayGame(
        Player Winner,
        int MoveCount,
        List<GameMoveRecord> MoveHistory
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
    public int StagingMovesRecorded { get; set; }
    public int MaxPlyRecorded { get; set; }

    public int TotalGames => RedWins + BlueWins + Draws;
    public double AverageMoves => TotalGames > 0 ? (double)TotalMoves / TotalGames : 0;

    [Obsolete("Use StagingMovesRecorded for separated pipeline")]
    public int MovesStoredToBook { get; set; }
}
