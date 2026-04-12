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

    private double _timeFraction = 1.0;

    /// <summary>
    /// Fraction of allocated time to use (0.0-1.0). Applied to TimeAllocation output post-PID.
    /// Default 1.0 for backward compatibility.
    /// </summary>
    public double TimeFraction
    {
        get => _timeFraction;
        init => _timeFraction = value is < 0.0 or > 1.0
            ? throw new ArgumentOutOfRangeException(nameof(value), $"Must be 0.0-1.0, got {value}")
            : value;
    }

    /// <summary>
    /// Whether to run dedicated pre-search VCF solver. Does NOT affect in-tree VCF.
    /// Default true for backward compatibility.
    /// </summary>
    public bool UseVCF { get; init; } = true;

    public static SearchOptions Default { get; } = new();
}
