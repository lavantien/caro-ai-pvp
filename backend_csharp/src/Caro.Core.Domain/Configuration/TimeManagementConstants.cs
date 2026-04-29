namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized time management tuning parameters.
/// Controls how the AI allocates search time across different game phases.
/// </summary>
public static class TimeManagementConstants
{
    // Default time controls
    public const int DefaultInitialTimeSeconds = 420;
    public const int DefaultIncrementSeconds = 5;
    public const int DefaultInitialTimeMs = DefaultInitialTimeSeconds * 1000;
    public const int ShortTimeControlThresholdMs = 180_000;
    public const int EmergencyThresholdMs = 10_000;
    public const int CriticalEmergencyThresholdMs = 3000;
    public const int IncrementOnlySoftBoundMs = 1000;
    public const int IncrementOnlyHardBoundMs = 2000;
    public const int MinSoftBoundMs = 500;

    // Phase thresholds (move numbers)
    public const int PhaseMidThreshold = 25;

    // Multipliers and ratios
    public const double TimeMultiplier = 1.3;
    public const double ShortTimeMultiplier = 0.8;
    public const double IncrementUsageRatio = 0.6;
    public const double OptimalTimeRatio = 0.8;
    public const double HardBoundMultiplier = 1.3;
    public const double HardBoundSafetyRatio = 0.2;
    public const double SoftBoundIncrementRatio = 0.5;
    public const double HardBoundIncrementRatio = 0.75;

    // AdaptiveTimeManager PID controller weights
    public const double IntegralGain = 0.1;
    public const double IntegralClampValue = 0.5;
    public const double ProportionalWeight = 0.6;
    public const double IntegralWeight = 0.3;
    public const double DerivativeWeight = 0.1;
    public const double TimePressureBlendFactor = 0.7;
    public const double MultiplierSmoothingOld = 0.7;
    public const double MultiplierSmoothingNew = 0.3;
    public const double MaxMultiplierSuddenDeath = 1.2;
    public const double MaxMultiplierNormal = 3.0;
    public const double MinMultiplier = 0.2;

    // Time scramble thresholds
    public const int SuddenDeathScrambleMs = 20_000;
    public const int NormalScrambleMs = 30_000;
    public const int IncrementScrambleMultiplier = 3;
    public const double SafetyFactor = 0.8;

    // Adaptive scaling
    public const double InitialTimePhaseDivisor = 25.0;
    public const double IncrementScalingFactor = 1.5;
    public const double ComplexityCap = 1.5;
    public const double MaxTimePercentage = 0.4;
    public const int MinAllocatableMs = 300;
    public const int MaxAllocatableTimeDivisor = 6;
    public const double HardBoundBufferRatio = 0.01;
    public const int HardBoundBufferAbsoluteMs = 100;

    // Emergency
    public const int EmergencyThresholdMinimumMs = 2000;
    public const int EmergencyTimeDivisor = 20;

    // Multiplier adjustments
    public const double MultiplierBoostOnWin = 1.05;
    public const double MultiplierPenaltyOnLoss = 0.95;
    public const double MultiplierResetFactor = 0.5;

    // TimeBudgetDepthManager
    public const double DefaultEstimatedNps = 100_000;
    public const double DefaultEffectiveBranchingFactor = 2.5;
    public const double NpsSmoothingOld = 0.5;
    public const double NpsSmoothingNew = 0.5;
    public const double MinBranchingFactor = 1.5;
    public const double MaxBranchingFactor = 5.0;
    public const double BranchingFactorSmoothingOld = 0.8;
    public const double BranchingFactorSmoothingNew = 0.2;
    public const double MinEffectiveTime = 0.01;
    public const int DepthBonusMultiplier = 4;
    public const int MaxCalculatedDepth = 15;
    public const double TimeContinuationThreshold = 0.8;
}
