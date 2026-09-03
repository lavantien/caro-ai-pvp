namespace Caro.Engine;

public struct SearchConfig
{
    public int MaxDepth { get; set; }
    public long TimeLimitMs { get; set; }
    /// <summary>Stop starting new depths once elapsed passes this; 0 disables.</summary>
    public long SoftLimitMs { get; set; }
    public int Threads { get; set; }
    public bool UseVCF { get; set; }
    /// <summary>Attacker moves the VCF solver may chain; 0 = engine default.</summary>
    public int VCFMaxDepth { get; set; }
    public double TimeFraction { get; set; }
}

public struct SearchStats
{
    public SearchStats()
    {
    }

    public int DepthAchieved { get; set; }
    public long NodesSearched { get; set; }
    public double NodesPerSecond { get; set; }
    public int SearchScore { get; set; }
    public string MoveType { get; set; } = "";
    public double TableHitRate { get; set; }
    public long AllocatedTimeMs { get; set; }
    public int ThreadCount { get; set; }
    public int? VcfDepth { get; set; }
    public long? VcfNodes { get; set; }
}

public enum VCFResult
{
    NoWin = 0,
    Win = 1,
    Timeout = 2,
}

/// <summary>
/// Solver outcome with observability counters: one node per attacker
/// placement tried, ChainDepth = attacker moves in the winning forced
/// chain (0 when no win).
/// </summary>
public readonly record struct VcfSearchResult(
    int X,
    int Y,
    VCFResult Result,
    long NodesSearched,
    int ChainDepth);
