namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Encapsulates search parameters for AI move calculation.
/// </summary>
public sealed record SearchOptions
{
    public long? TimeRemainingMs { get; init; }
    public int MoveNumber { get; init; }
    public bool PonderingEnabled { get; init; } = true;
    public bool ParallelSearchEnabled { get; init; } = true;
    public int? IncrementSeconds { get; init; }
    public int? ThreadCount { get; init; }
    public int? MaxDepth { get; init; }
    public long? MaxNodes { get; init; }
    public int? MaxTimeMs { get; init; }

    public static SearchOptions Default { get; } = new();
}
