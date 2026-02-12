namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized move ordering constants for AI search.
/// These values determine the priority of moves during search ordering.
/// Higher values = higher priority.
/// </summary>
public static class MoveOrderingConstants
{
    /// <summary>
    /// Score for moves that must block opponent's winning threat
    /// </summary>
    public const int MustBlockScore = 2_000_000;

    /// <summary>
    /// Score for moves that create a winning position
    /// </summary>
    public const int WinningMoveScore = 1_500_000;

    /// <summary>
    /// Score for transposition table move (already proven good)
    /// </summary>
    public const int TtMoveScore = 1_000_000;

    /// <summary>
    /// Score for moves that create threats (open three, broken four)
    /// </summary>
    public const int ThreatCreateScore = 800_000;

    /// <summary>
    /// Score for first killer move (quiet move that caused beta cutoff)
    /// </summary>
    public const int KillerScore1 = 500_000;

    /// <summary>
    /// Score for second killer move
    /// </summary>
    public const int KillerScore2 = 400_000;

    /// <summary>
    /// Maximum score for counter-move history bonus
    /// </summary>
    public const int CounterMoveScore = 150_000;

    /// <summary>
    /// Maximum score for continuation history bonus
    /// </summary>
    public const int ContinuationScoreMax = 300_000;

    /// <summary>
    /// Maximum score for history heuristic bonus
    /// </summary>
    public const int HistoryScoreMax = 20_000;

    /// <summary>
    /// Threshold for separating good vs bad quiet moves based on history score
    /// </summary>
    public const int GoodQuietThreshold = 500;
}
