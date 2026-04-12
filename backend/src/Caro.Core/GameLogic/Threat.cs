using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Threat types for Caro Gomoku threat detection
/// Critical for VCF (Victory by Continuous Four) solver
/// </summary>
public enum ThreatType
{
    /// <summary>
    /// Straight Four: XXXX_ - 4 consecutive stones with at least one open end
    /// Winning threat: requires immediate response from opponent
    /// </summary>
    StraightFour,

    /// <summary>
    /// Broken Four: XXX_X - 4 stones with one gap
    /// Can create two simultaneous threats (double attack)
    /// </summary>
    BrokenFour,

    /// <summary>
    /// Straight Three: XXX__ - 3 consecutive stones with both ends open
    /// Creates unstoppable Straight Four on next move
    /// </summary>
    StraightThree,

    /// <summary>
    /// Broken Three: XX_X_ - 3 stones with one gap, at least one open end
    /// Can become Straight Three or Broken Four
    /// </summary>
    BrokenThree
}

/// <summary>
/// Represents a threat on the board for VCF calculation
/// </summary>
public class Threat
{
    public ThreatType Type { get; init; }
    public Player Owner { get; init; }

    /// <summary>
    /// Squares where the owner can play to complete this threat
    /// For S4: single winning square
    /// For B4: gap square or extension squares
    /// For S3/B3: squares to extend toward five
    /// </summary>
    public List<(int x, int y)> GainSquares { get; init; } = new();

    /// <summary>
    /// Squares where opponent must play to defend against this threat
    /// For S4: exactly one defense square (the gain square)
    /// For B4: one or more defense squares
    /// </summary>
    public List<(int x, int y)> CostSquares { get; init; } = new();

    /// <summary>
    /// All stone positions that form this threat
    /// Used for validation and visualization
    /// </summary>
    public List<(int x, int y)> StonePositions { get; init; } = new();

    /// <summary>
    /// Direction of this threat (dx, dy) for pattern matching
    /// </summary>
    public (int dx, int dy) Direction { get; init; }

    /// <summary>
    /// Priority for move ordering in VCF search
    /// Higher values = more urgent threats
    /// </summary>
    public int Priority => Type switch
    {
        ThreatType.StraightFour => 100,    // Immediate win threat
        ThreatType.BrokenFour => 80,       // Double attack potential
        ThreatType.StraightThree => 60,    // Strong forcing move
        ThreatType.BrokenThree => 40,      // Potential threat
        _ => 0
    };
}
