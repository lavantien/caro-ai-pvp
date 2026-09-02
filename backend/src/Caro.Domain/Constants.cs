namespace Caro.Domain;

public static class Constants
{
    public const int BoardSize = 16;
    public const int WinLength = 5;
    public const int MaxMoves = BoardSize * BoardSize;
    public const int OpenRuleMin = 3;

    public const int Infinity = 100_000;
    public const int MaxEval = 25_000;

    public const int MaxConcurrentGames = 4;
    // Mirrored by hand in Caro.Server.runtimeconfig.template.json
    // (System.GC.HeapHardLimitBytes); update both together.
    public const long HeapHardLimitBytes = 2L * 1024 * 1024 * 1024;
    public const int AbandonedTimeoutMinutes = 30;

    public const int DefaultTTSizeMB = 1024;
    public const double VCFTimeFraction = 0.20;

    public const int MaxSearchRadius = 2;
    public const int AbsoluteMaxDepth = 50;
    public const int AspirationWindowSize = 1_500;
    public const int MaxAspirationAttempts = 3;
    public const int NullMoveMinDepth = 4;
    public const int NullMoveReduction = 2;
    public const int MaxQuiescenceDepth = 4;

    public const int LMRMinDepth = 3;
    public const int LMRFullDepthMoves = 4;

    public const int WinScore = 30_000;

    // TT configuration
    public const int TTShardCount = 16;

    // VCF solver
    public const int VCFSearchDepth = 12;

    // Time management
    public const double TimePhaseDivisorEarly = 25.0;
    public const double TimePhaseDivisorLate = 30.0;
    public const int TimePhaseSwitchMove = 25;
    public const double TimeIncContribFactor = 0.6;
    public const long TimeMinOptimalMs = 300;
    public const double TimeMaxFraction = 0.4;
    public const double TimeHardBoundMultiplier = 1.3;
    public const double TimeBufferFraction = 0.01;
    public const long TimeMinBufferMs = 100;
    public const long TimeReserveMs = 50;
    public const double TimeSoftBoundFraction = 0.8;

    // Pondering (L5 background search on the predicted reply)
    public const int PonderMinCompletedDepth = 1;
}
