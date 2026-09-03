namespace Caro.Domain;

public static partial class Constants
{
    public static class Board
    {
        public const int Size = 16;
        public const int WinLength = 5;
        public const int LineLength = 2 * WinLength + 1;
        public const int MaxMoves = Size * Size;
        public const int OpenRuleMin = 3;
        public const int MaxSearchRadius = 2;
    }

    public static class Score
    {
        public const int Infinity = 100_000;
        public const int MaxEval = 25_000;
        public const int WinScore = 30_000;
    }

    public static class Search
    {
        public const int AbsoluteMaxDepth = 50;
        public const int AspirationWindowSize = 1_500;
        public const int MaxAspirationAttempts = 3;
        public const int NullMoveMinDepth = 4;
        public const int NullMoveReduction = 2;
        public const int MaxQuiescenceDepth = 4;
        public const int LMRMinDepth = 3;
        public const int LMRFullDepthMoves = 4;
        public const int LMRDeepMoveThreshold = 8;
    }

    public static class Vcf
    {
        public const int SearchDepth = 12;
        public const double TimeFraction = 0.20;
        public const double BlockFraction = TimeFraction / 2;
        public const double BlockCheckFraction = TimeFraction / 4;
    }

    public static class TimeManagement
    {
        public const double PhaseDivisorEarly = 25.0;
        public const double PhaseDivisorLate = 30.0;
        public const int PhaseSwitchMove = 25;
        public const double IncContribFactor = 0.6;
        public const long MinOptimalMs = 300;
        public const double MaxFraction = 0.4;
        public const double HardBoundMultiplier = 1.3;
        public const double BufferFraction = 0.01;
        public const long MinBufferMs = 100;
        public const long ReserveMs = 50;
        public const double SoftBoundFraction = 0.8;
    }

    public static class Ponder
    {
        public const int MinCompletedDepth = 1;
    }

    public static class Transposition
    {
        public const int ShardCount = 16;
        public const int DefaultSizeMB = 1024;
    }

    public static class Limits
    {
        public const int MaxConcurrentGames = 4;

        // Mirrored by hand in Caro.Server.runtimeconfig.template.json
        // (System.GC.HeapHardLimitBytes); update both together.
        public const long HeapHardLimitBytes = 2L * 1024 * 1024 * 1024;

        public const int AbandonedTimeoutMinutes = 30;
    }
}
