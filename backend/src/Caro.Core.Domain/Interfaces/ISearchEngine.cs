namespace Caro.Core.Domain.Interfaces;

/// <summary>
/// Interface for AI search engines.
/// Provides stateless search functionality where all state is passed in.
/// </summary>
public interface ISearchEngine
{
    /// <summary>
    /// Find the best move for the given position.
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player to search for</param>
    /// <param name="options">Search options including difficulty and time constraints</param>
    /// <param name="transpositionTable">Transposition table for caching (per-game scoped)</param>
    /// <param name="cancellationToken">Cancellation token for stopping the search</param>
    /// <returns>Search result containing the best move and statistics</returns>
    SearchResult Search(
        Entities.Board board,
        Entities.Player player,
        SearchOptions options,
        ITranspositionTable transpositionTable,
        System.Threading.CancellationToken cancellationToken = default);
}

/// <summary>
/// Options for AI search
/// </summary>
public sealed record SearchOptions(
    AIDifficulty Difficulty,
    long? TimeRemainingMs,
    int MoveNumber,
    int MaxDepth,
    int FixedThreadCount,
    bool EnablePondering,
    bool EnableVCF
)
{
    /// <summary>
    /// Create search options for the given difficulty
    /// </summary>
    public static SearchOptions ForDifficulty(AIDifficulty difficulty) =>
        new(
            difficulty,
            null,  // Time remaining calculated by time manager
            0,     // Move number
            difficulty == AIDifficulty.Grandmaster ? 12 : 6,  // Max depth
            -1,    // Auto-calculate thread count
            false, // Pondering disabled for single search
            difficulty >= AIDifficulty.Hard  // VCF only for Hard+
        );
}

/// <summary>
/// Result of an AI search
/// </summary>
public sealed record SearchResult(
    int X,
    int Y,
    int DepthAchieved,
    long NodesSearched,
    double ElapsedSeconds,
    int Score,
    int TableHits,
    int TableLookups,
    int ThreadCount,
    bool WasCancelled
)
{
    /// <summary>
    /// Nodes per second
    /// </summary>
    public double NodesPerSecond => ElapsedSeconds > 0
        ? NodesSearched / ElapsedSeconds
        : 0;

    /// <summary>
    /// Table hit rate (0-1)
    /// </summary>
    public double HitRate => TableLookups > 0
        ? (double)TableHits / TableLookups
        : 0;

    /// <summary>
    /// The best move as a tuple
    /// </summary>
    public (int x, int y) Move => (X, Y);
}
