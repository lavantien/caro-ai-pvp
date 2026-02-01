using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Result type for VCF (Victory by Continuous Fours) detection
/// </summary>
public enum VCFResultType
{
    /// <summary>
    /// No VCF found - continue normal search
    /// </summary>
    NoVCF,

    /// <summary>
    /// Winning sequence found - we can force a win
    /// </summary>
    WinningSequence,

    /// <summary>
    /// Losing sequence - opponent can force a win, we must defend
    /// </summary>
    LosingSequence
}

/// <summary>
/// Result from VCF solver check at a node
/// Immutable, thread-safe
/// </summary>
public sealed class VCFNodeResult
{
    public VCFResultType Type { get; init; }
    public int Score { get; init; }
    public List<(int x, int y)> ForcingMoves { get; init; } = new();
    public int Depth { get; init; }
    public long NodesSearched { get; init; }

    /// <summary>
    /// Winning score constant - high enough to exceed any normal evaluation
    /// </summary>
    public const int WinScore = 1000000;

    /// <summary>
    /// Create a "no VCF found" result
    /// </summary>
    public static VCFNodeResult None => new()
    {
        Type = VCFResultType.NoVCF,
        Score = 0,
        ForcingMoves = new List<(int x, int y)>(),
        Depth = 0,
        NodesSearched = 0
    };

    /// <summary>
    /// Create a winning VCF result
    /// </summary>
    public static VCFNodeResult Winning(List<(int x, int y)> moves, int depth, long nodes) => new()
    {
        Type = VCFResultType.WinningSequence,
        Score = WinScore - depth * 100,  // Prefer shorter wins
        ForcingMoves = moves,
        Depth = depth,
        NodesSearched = nodes
    };

    /// <summary>
    /// Create a losing VCF result (opponent can force win)
    /// </summary>
    public static VCFNodeResult Losing(List<(int x, int y)> defenses, int depth, long nodes) => new()
    {
        Type = VCFResultType.LosingSequence,
        Score = -WinScore + depth * 100,  // Prefer longer losses
        ForcingMoves = defenses,
        Depth = depth,
        NodesSearched = nodes
    };
}

/// <summary>
/// Cache entry for VCF results
/// Thread-safe for concurrent dictionary access
/// </summary>
internal sealed class VCFCacheEntry
{
    public required ulong Hash { get; init; }
    public VCFResultType ResultType { get; init; }
    public int Score { get; init; }
    public byte Depth { get; init; }
    public byte Age { get; init; }
}
