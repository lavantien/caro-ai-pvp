using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for staging area storage of raw self-play data before verification.
/// This is SEPARATE from the main opening book (IOpeningBookStore).
///
/// Design principles:
/// - Store games in SGF format (one row per game, not per position)
/// - Phase 1: Workers write to flat files (buffer_*.sgf) for zero lock contention
/// - Phase 1.5: Ingestion script bulk-inserts into SQLite
/// - Phase 2: Verification replays move sequences to reconstruct positions
/// </summary>
public interface IStagingBookStore : IDisposable
{
    /// <summary>
    /// Record a completed game in SGF format.
    /// This is the PRIMARY storage method - one row per game.
    /// </summary>
    /// <param name="gameRecord">Complete game record with moves in SGF format</param>
    void RecordGame(SelfPlayGameRecord gameRecord);

    /// <summary>
    /// Record a single move from self-play (legacy method for backward compatibility).
    /// Moves are buffered and flushed when buffer reaches capacity.
    /// </summary>
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
    /// Get all games for batch processing.
    /// </summary>
    /// <param name="limit">Maximum number of games to return</param>
    /// <param name="offset">Offset for pagination</param>
    /// <returns>List of game records</returns>
    List<SelfPlayGameRecord> GetGames(int limit = 1000, int offset = 0);

    /// <summary>
    /// Get games filtered by result.
    /// </summary>
    /// <param name="result">Game result: 1=Red win, 0=Draw, -1=Blue win</param>
    /// <param name="limit">Maximum number of games</param>
    /// <returns>List of game records</returns>
    List<SelfPlayGameRecord> GetGamesByResult(int result, int limit = 1000);

    /// <summary>
    /// Get all positions for verification, filtered by maximum ply.
    /// Only positions up to maxPly are returned (opening phase).
    /// Reconstructed from SGF move sequences.
    /// </summary>
    IEnumerable<StagingPosition> GetPositionsForVerification(int maxPly = 16);

    /// <summary>
    /// Get aggregated statistics for all positions.
    /// Groups by (canonical hash, direct hash, player) and computes win rates.
    /// </summary>
    Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> GetPositionStatistics();

    /// <summary>
    /// Get all moves played for a specific position.
    /// </summary>
    List<StagingMove> GetMovesForPosition(ulong canonicalHash, ulong directHash, Player player);

    /// <summary>
    /// Flush any buffered data to persistent storage.
    /// </summary>
    void Flush();

    /// <summary>
    /// Clear all staging data.
    /// </summary>
    void Clear();

    /// <summary>
    /// Initialize the staging store.
    /// </summary>
    void Initialize();

    /// <summary>
    /// Get total count of staged games.
    /// </summary>
    long GetGameCount();

    /// <summary>
    /// Get total count of staged positions.
    /// </summary>
    long GetPositionCount();
}

/// <summary>
/// Complete record of a self-play game in SGF format.
/// One row per game - efficient storage for Phase 1.
/// </summary>
public sealed class SelfPlayGameRecord
{
    /// <summary>
    /// Unique game identifier.
    /// </summary>
    public long GameId { get; set; }

    /// <summary>
    /// Game moves in SGF format: "B[hh];W[ii];..." where hh, ii are coordinates.
    /// Coordinates are 2-letter codes: aa=0,0 to pp=15,15.
    /// </summary>
    public required string SgfMoves { get; set; }

    /// <summary>
    /// Winner of the game: Red, Blue, or None (draw).
    /// </summary>
    public Player Winner { get; set; }

    /// <summary>
    /// Total number of moves in the game.
    /// </summary>
    public int TotalMoves { get; set; }

    /// <summary>
    /// Time control used (e.g., "7+5" for 7 min + 5 sec increment).
    /// </summary>
    public string? TimeControl { get; set; }

    /// <summary>
    /// Temperature used for move selection.
    /// </summary>
    public double Temperature { get; set; }

    /// <summary>
    /// Difficulty level used for self-play.
    /// </summary>
    public AIDifficulty Difficulty { get; set; }

    /// <summary>
    /// Timestamp when game was recorded.
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;

    /// <summary>
    /// List of moves as (x, y) tuples for reconstruction.
    /// </summary>
    public List<(int X, int Y)> MoveList { get; set; } = new();

    /// <summary>
    /// Create SGF-formatted move string from move list.
    /// </summary>
    public static string ToSgf(List<(int X, int Y)> moves)
    {
        var sb = new System.Text.StringBuilder();
        for (int i = 0; i < moves.Count; i++)
        {
            var (x, y) = moves[i];
            var player = i % 2 == 0 ? 'B' : 'W';  // Red=Black first
            sb.Append($"{player}[{ToSgfCoord(x)}{ToSgfCoord(y)}];");
        }
        return sb.ToString();
    }

    /// <summary>
    /// Parse SGF move string to move list.
    /// </summary>
    public static List<(int X, int Y)> FromSgf(string sgf)
    {
        var moves = new List<(int, int)>();
        if (string.IsNullOrEmpty(sgf)) return moves;

        // Parse "B[ab];W[cd];..." format
        var span = sgf.AsSpan();
        int i = 0;
        while (i < span.Length - 4)
        {
            // Skip to next move marker
            if (span[i] != 'B' && span[i] != 'W')
            {
                i++;
                continue;
            }

            if (span[i + 1] == '[' && i + 4 < span.Length)
            {
                var x = FromSgfCoord(span[i + 2]);
                var y = FromSgfCoord(span[i + 3]);
                if (x >= 0 && y >= 0)
                {
                    moves.Add((x, y));
                }
                i += 5; // Skip past "B[xy];"
            }
            else
            {
                i++;
            }
        }

        return moves;
    }

    private static char ToSgfCoord(int coord)
    {
        // 0-15 -> a-p
        return (char)('a' + coord);
    }

    private static int FromSgfCoord(char c)
    {
        // a-p -> 0-15
        if (c >= 'a' && c <= 'p') return c - 'a';
        if (c >= 'A' && c <= 'P') return c - 'A';
        return -1;
    }
}

/// <summary>
/// A single position in the staging area.
/// </summary>
public sealed record StagingPosition
{
    public required ulong CanonicalHash { get; init; }
    public required ulong DirectHash { get; init; }
    public required Player Player { get; init; }
    public required int Ply { get; init; }
    public required int MoveX { get; init; }
    public required int MoveY { get; init; }
    public required int TimeBudgetMs { get; init; }
    public required int GameResult { get; init; }
}

/// <summary>
/// Aggregated statistics for a position across all self-play games.
/// </summary>
public sealed record PositionStatistics
{
    public required int PlayCount { get; init; }
    public required int WinCount { get; init; }
    public required double WinRate { get; init; }
    public required int AvgTimeBudgetMs { get; init; }
    public required int DrawCount { get; init; }
    public required int LossCount { get; init; }
}

/// <summary>
/// A single move record in the staging area.
/// </summary>
public sealed record StagingMove
{
    public required int MoveX { get; init; }
    public required int MoveY { get; init; }
    public required int Ply { get; init; }
    public required int GameResult { get; init; }
    public required int PlayCount { get; init; }
    public required double WinRate { get; init; }
}
