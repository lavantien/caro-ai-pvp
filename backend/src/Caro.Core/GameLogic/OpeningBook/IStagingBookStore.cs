using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for staging area storage of raw self-play data before verification.
/// This is SEPARATE from the main opening book (IOpeningBookStore).
///
/// Design principles:
/// - All buffer sizes are powers of 2 for optimal performance
/// - Thread-safe for concurrent self-play game recording
/// - Positions are recorded without judgment (Actor phase)
/// - Verification (Critic phase) happens separately via MoveVerifier
/// </summary>
public interface IStagingBookStore : IDisposable
{
    /// <summary>
    /// Record a single move from self-play to the staging area.
    /// Moves are buffered and flushed when buffer reaches capacity.
    /// </summary>
    /// <param name="canonicalHash">Canonical position hash after symmetry reduction</param>
    /// <param name="directHash">Direct hash of the board for exact identification</param>
    /// <param name="player">Player who made the move</param>
    /// <param name="ply">Ply number (0 = first move)</param>
    /// <param name="moveX">X coordinate of the move</param>
    /// <param name="moveY">Y coordinate of the move</param>
    /// <param name="gameResult">Result for the move's player: 1=win, 0=draw, -1=loss</param>
    /// <param name="gameId">Unique identifier for the game</param>
    /// <param name="timeBudgetMs">Time budget used for this move in milliseconds</param>
    void RecordMove(
        ulong canonicalHash,
        ulong directHash,
        Player player,
        int ply,
        int moveX,
        int moveY,
        int gameResult,
        long gameId,
        int timeBudgetMs);

    /// <summary>
    /// Get all positions for verification, filtered by maximum ply.
    /// Only positions up to maxPly are returned (opening phase).
    /// </summary>
    /// <param name="maxPly">Maximum ply to include (default: 16 for opening moves)</param>
    /// <returns>Enumerable of staging positions</returns>
    IEnumerable<StagingPosition> GetPositionsForVerification(int maxPly = 16);

    /// <summary>
    /// Get aggregated statistics for all positions.
    /// Groups by (canonical hash, direct hash, player) and computes win rates.
    /// </summary>
    /// <returns>Dictionary mapping position key to statistics</returns>
    Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> GetPositionStatistics();

    /// <summary>
    /// Get all moves played for a specific position.
    /// </summary>
    /// <param name="canonicalHash">Canonical position hash</param>
    /// <param name="directHash">Direct position hash</param>
    /// <param name="player">Player to move</param>
    /// <returns>List of staging moves for this position</returns>
    List<StagingMove> GetMovesForPosition(ulong canonicalHash, ulong directHash, Player player);

    /// <summary>
    /// Flush any buffered moves to persistent storage.
    /// Called automatically when buffer reaches capacity.
    /// </summary>
    void Flush();

    /// <summary>
    /// Clear all staging data.
    /// Called after successful integration to main book.
    /// </summary>
    void Clear();

    /// <summary>
    /// Initialize the staging store (create tables if needed).
    /// Called before any other operations.
    /// </summary>
    void Initialize();

    /// <summary>
    /// Get total count of staged positions.
    /// </summary>
    /// <returns>Count of positions in staging</returns>
    long GetPositionCount();

    /// <summary>
    /// Get total count of games recorded.
    /// </summary>
    /// <returns>Count of unique games</returns>
    long GetGameCount();
}

/// <summary>
/// A single position in the staging area.
/// Represents one move from a self-play game.
/// </summary>
public sealed record StagingPosition
{
    /// <summary>
    /// Canonical position hash after symmetry reduction.
    /// </summary>
    public required ulong CanonicalHash { get; init; }

    /// <summary>
    /// Direct hash of the board for exact identification.
    /// </summary>
    public required ulong DirectHash { get; init; }

    /// <summary>
    /// Player who made the move.
    /// </summary>
    public required Player Player { get; init; }

    /// <summary>
    /// Ply number (0 = first move of game).
    /// </summary>
    public required int Ply { get; init; }

    /// <summary>
    /// X coordinate of the move.
    /// </summary>
    public required int MoveX { get; init; }

    /// <summary>
    /// Y coordinate of the move.
    /// </summary>
    public required int MoveY { get; init; }

    /// <summary>
    /// Time budget used for this move in milliseconds.
    /// </summary>
    public required int TimeBudgetMs { get; init; }
}

/// <summary>
/// Aggregated statistics for a position across all self-play games.
/// </summary>
public sealed record PositionStatistics
{
    /// <summary>
    /// Total number of times this position was encountered.
    /// </summary>
    public required int PlayCount { get; init; }

    /// <summary>
    /// Number of times the move led to a win for the moving player.
    /// </summary>
    public required int WinCount { get; init; }

    /// <summary>
    /// Win rate (WinCount / PlayCount).
    /// </summary>
    public required double WinRate { get; init; }

    /// <summary>
    /// Average time budget used in milliseconds.
    /// </summary>
    public required int AvgTimeBudgetMs { get; init; }

    /// <summary>
    /// Number of draws.
    /// </summary>
    public required int DrawCount { get; init; }

    /// <summary>
    /// Number of losses.
    /// </summary>
    public required int LossCount { get; init; }
}

/// <summary>
/// A single move record in the staging area.
/// </summary>
public sealed record StagingMove
{
    /// <summary>
    /// X coordinate of the move.
    /// </summary>
    public required int MoveX { get; init; }

    /// <summary>
    /// Y coordinate of the move.
    /// </summary>
    public required int MoveY { get; init; }

    /// <summary>
    /// Ply number of this move.
    /// </summary>
    public required int Ply { get; init; }

    /// <summary>
    /// Game result for the moving player: 1=win, 0=draw, -1=loss.
    /// </summary>
    public required int GameResult { get; init; }

    /// <summary>
    /// Number of times this specific move was played.
    /// </summary>
    public required int PlayCount { get; init; }

    /// <summary>
    /// Win rate for this move.
    /// </summary>
    public required double WinRate { get; init; }
}
