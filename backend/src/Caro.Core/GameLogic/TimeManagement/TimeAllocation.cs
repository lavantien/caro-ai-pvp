namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// Represents the time allocation for a single move search
/// Uses two-level time control: soft bound (optimal target) and hard bound (maximum allowed)
/// </summary>
public readonly struct TimeAllocation
{
    /// <summary>
    /// Soft bound - the target time to aim for
    /// Search can continue past this if move is unstable or position is complex
    /// </summary>
    public long SoftBoundMs { get; init; }

    /// <summary>
    /// Hard bound - absolute maximum time allowed for this move
    /// Search MUST stop when this is reached
    /// </summary>
    public long HardBoundMs { get; init; }

    /// <summary>
    /// Optimal time - 80% of soft bound
    /// Ideal target when position is stable and not complex
    /// </summary>
    public long OptimalTimeMs { get; init; }

    /// <summary>
    /// Whether this is emergency mode (very low time remaining)
    /// In emergency mode, use TT move or VCF solver instead of full search
    /// </summary>
    public bool IsEmergency { get; init; }

    /// <summary>
    /// Current game phase for time allocation decisions
    /// </summary>
    public GamePhase Phase { get; init; }

    /// <summary>
    /// Complexity multiplier applied (0.5x to 2.0x)
    /// Higher values indicate more complex positions requiring more time
    /// </summary>
    public double ComplexityMultiplier { get; init; }

    /// <summary>
    /// Create a default time allocation with minimum values
    /// </summary>
    public static TimeAllocation Default => new()
    {
        SoftBoundMs = 5_000,
        HardBoundMs = 15_000,
        OptimalTimeMs = 4_000,
        IsEmergency = false,
        Phase = GamePhase.EarlyMid,
        ComplexityMultiplier = 1.0
    };
}

/// <summary>
/// Game phases for time management
/// Different phases use different percentages of the base time allocation
/// </summary>
public enum GamePhase
{
    /// <summary>
    /// Opening: Moves 1-10
    /// Use 50% of base time - save time for later complexity
    /// </summary>
    Opening = 0,

    /// <summary>
    /// Early Midgame: Moves 11-25
    /// Use 80% of base time - position developing
    /// </summary>
    EarlyMid = 1,

    /// <summary>
    /// Late Midgame: Moves 26-45
    /// Use 120% of base time - peak tactical complexity
    /// </summary>
    LateMid = 2,

    /// <summary>
    /// Endgame: Moves 46+
    /// Use 100% of base time - precise play needed
    /// </summary>
    Endgame = 3
}
