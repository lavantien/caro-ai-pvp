namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized search heuristic constants for AI evaluation and move ordering.
/// </summary>
public static class SearchHeuristicConstants
{
    // Threat scoring weights (used in MinimaxAI)
    public const int WinningSquarePenalty = 10000;
    public const int FourThreatPenalty = 5000;
    public const int ThreeThreatPenalty = 500;
    public const int FourThreatBonus = 8000;
    public const int MultipleThreePenalty = 2000;
    public const int MultipleThreeThreshold = 2;

    // Search bounds
    public const int AlphaBetaMargin = 1000;
    public const int AspirationWindow = 50;
    public const int MaxAspirationAttempts = 3;
    public const int ProbCutMargin = 200;
    public const int PVSWindowThreshold = 50;

    // Depth controls
    public const int LMRFullDepthMoves = 4;
    public const int PVSEnabledDepth = 2;
    public const int MaxSearchDepth = 50;
    public const int MaxQuiescenceDepth = 4;
    public const int MaxAIMaxDepth = 64;

    // Move ordering
    public const int PriorityMoveCount = 4;
    public const int RandomBonusRange = 100;

    // Score thresholds
    public const int WinScore = 30_000;
    public const long ReasonableScoreThreshold = -100_000;

    // Time allocation ratios (in MinimaxAI)
    public const double SoftBoundRatio = 0.8;
    public const double OptimalBoundRatio = 0.6;
    public const double HardBoundTimeRatio = 1.3;
    public const double OptimalTimeFractionNumerator = 8;
    public const double HardTimeCheckRatio = 0.9;
    public const double IterationAbortRatio = 0.5;
    public const double IterationCautionRatio = 0.4;

    // Depth estimation
    public const int DepthEstimationBaseline = 5;
    public const int DepthEstimationMultiplier = 200;
    public const int IterationTimeEstimateAggressive = 5;
    public const int IterationTimeEstimateNormal = 2;

    // Effective increment estimation
    public const int MinEffectiveIncrementSeconds = 2;
    public const double InitialTimeToIncrementDivisor = 90.0;

    // VCF time thresholds
    public const int EmergencyVcfCapMs = 2500;
    public const double VcfTimeRemainingThreshold = 0.1;
    public const int VcfMinimumTimeMs = 1;
    public const double VcfInitialTimeFraction = 0.05;

    // Open rule
    public const int OpenRuleDistance = 3;

    // Center distance (for move ordering)
    public const int CenterIndex = 7; // Center of 0-15 range for 16x16

    // EBF for time estimation
    public const double EffectiveBranchingFactorEstimate = 2.5;
}
