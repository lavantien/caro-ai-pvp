using Caro.Core.Domain.Entities;

namespace Caro.Core.Domain.Interfaces;

/// <summary>
/// Interface for publishing AI search statistics.
/// Used for telemetry and real-time updates.
/// </summary>
public interface IStatsPublisher
{
    /// <summary>
    /// Unique identifier for this publisher
    /// </summary>
    string PublisherId { get; }

    /// <summary>
    /// Publish statistics for a completed move
    /// </summary>
    /// <param name="stats">The statistics to publish</param>
    void PublishMoveStats(MoveStats stats);

    /// <summary>
    /// Subscribe to statistics from this publisher
    /// </summary>
    /// <param name="subscriber">The subscriber to receive stats</param>
    void Subscribe(IMoveStatsSubscriber subscriber);

    /// <summary>
    /// Unsubscribe a statistics subscriber
    /// </summary>
    /// <param name="subscriber">The subscriber to remove</param>
    void Unsubscribe(IMoveStatsSubscriber subscriber);
}

/// <summary>
/// Interface for receiving move statistics
/// </summary>
public interface IMoveStatsSubscriber
{
    /// <summary>
    /// Called when new move statistics are available
    /// </summary>
    /// <param name="stats">The statistics received</param>
    void OnMoveStats(MoveStats stats);

    /// <summary>
    /// Unique identifier for this subscriber
    /// </summary>
    string SubscriberId { get; }
}

/// <summary>
/// Statistics for a single move/search
/// </summary>
public sealed record MoveStats(
    string PublisherId,
    Player Player,
    StatsType Type,
    int X,
    int Y,
    int DepthAchieved,
    long NodesSearched,
    double NodesPerSecond,
    double TableHitRate,
    int TableHits,
    int TableLookups,
    int Score,
    int ThreadCount,
    long MoveTimeMs,
    long TimestampMs,
    bool PonderingActive,
    int VCFDepthAchieved,
    long VCFNodesSearched
)
{
    /// <summary>
    /// Create main search statistics
    /// </summary>
    public static MoveStats MainSearch(
        string publisherId,
        Player player,
        int x, int y,
        int depth,
        long nodes,
        double nps,
        int hits,
        int lookups,
        int score,
        int threads,
        long moveTimeMs) =>
        new(
            publisherId, player, StatsType.MainSearch,
            x, y, depth, nodes, nps,
            lookups > 0 ? (double)hits / lookups : 0,
            hits, lookups, score, threads, moveTimeMs,
            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            false, 0, 0
        );

    /// <summary>
    /// Create pondering statistics
    /// </summary>
    public static MoveStats Pondering(
        string publisherId,
        int depth,
        long nodes,
        double nps) =>
        new(
            publisherId, Player.None, StatsType.Pondering,
            -1, -1, depth, nodes, nps,
            0, 0, 0, 0, 0, 0,
            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            true, 0, 0
        );

    /// <summary>
    /// Create VCF search statistics
    /// </summary>
    public static MoveStats VCFSearch(
        string publisherId,
        int depth,
        long nodes) =>
        new(
            publisherId, Player.None, StatsType.VCFSearch,
            -1, -1, depth, nodes, 0,
            0, 0, 0, 0, 0, 0,
            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            false, depth, nodes
        );
}

/// <summary>
/// Type of search statistics
/// </summary>
public enum StatsType
{
    /// <summary>Main search for best move</summary>
    MainSearch,

    /// <summary>Background pondering search</summary>
    Pondering,

    /// <summary>VCF (Victory by Continuous Four) search</summary>
    VCFSearch
}
