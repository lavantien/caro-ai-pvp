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

        // Eval-side aliases: an exactly-5 pattern scores as a win, and the
        // corrected eval clamps to MaxEval.
        public const int FiveScore = WinScore;
        public const int MaxCorrectedEval = MaxEval;
    }

    public static class Eval
    {
        public const int Flex4WinBonus = 15_000;
        public const int DoubleB4Bonus = 14_000;
        public const int B4F3Bonus = 13_000;
        public const int DoubleF3Bonus = 12_000;
        public const int Flex4Score = 10_000;
        public const int Block4Score = 5_000;
        public const int Flex3Score = 1_000;
        public const int Block3Score = 100;
        public const int Flex2Score = 100;
        public const int Block2Score = 30;
        public const int Flex1Score = 10;
        public const int CenterBonusWeight = 2;
    }

    public static class Search
    {
        public const int AbsoluteMaxDepth = 50;
        public const int AspirationWindowSize = 1_500;
        public const int MaxAspirationAttempts = 3;
        public const int AspirationWidenFactor = 2;
        public const int NullMoveMinDepth = 4;
        public const int NullMoveReduction = 2;
        public const int MaxQuiescenceDepth = 4;
        public const int LMRMinDepth = 3;
        public const int LMRFullDepthMoves = 4;
        public const int LMRDeepMoveThreshold = 8;
        public const int LmrBaseReduction = 1;
        public const int LmrDeepReduction = 2;
        public const int StartDepthStagger = 2;
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

    public static class History
    {
        public const int MaxKillerDepth = 64;
        public const int KillerPrimaryScore = 500_000;
        public const int KillerSecondaryScore = 400_000;
        public const int HistoryMax = 1_000_000;
        public const int ContHistMax = 30_000;
        public const int ContHistBonusScale = 300;
        public const int BoardCells = Board.Size * Board.Size;
    }

    public static class Ordering
    {
        public const int HistoryScoreCap = 300_000;
        public const int HistoryMultiplier = 2;
        public const int CenterWeight = 100;
        public const int ProximityWeight = 10;
        public const int CenterDistScaleBase = Board.Size * 2 - 4;
        public const int NeighborStoneScore = 3;
        public const int OwnOpenFourScore = 700_000;
        public const int OwnFourScore = 400_000;
        public const int OwnFlex3Score = 300_000;
        public const int OppOpenFourScore = 500_000;
        public const int OppFourScore = 350_000;
        public const int OppFlex3Score = 200_000;
    }

    public static class Pattern
    {
        public const int TwosScanRadius = 2;
        public const int ClusterAnchorDistance = 2;
        public const int TwosMinNoneCount = 3;
        public const int TwosCount = 2;
        public const int SinglesCount = 1;
        public const int TacticalMinCompletions = 1;
        public const int TacticalDoubleThreatDirs = 2;
    }

    public static class Iteration
    {
        public const double GrowthMin = 1.5;
        public const double GrowthMax = 6.0;
        public const double GrowthDefault = 4.0;
    }

    public static class Transposition
    {
        public const int ShardCount = 16;
        public const int DefaultSizeMB = 1024;
        public const int DefaultSessionSizeMB = 256;
        public const int AgeDecayPerGeneration = 8;
    }

    public static class Watchdog
    {
        public const int PollIntervalMs = 10;
        public const int WatchJoinTimeoutMs = 500;
    }

    public static class Capacity
    {
        // Empty-board seed block: a square spanning this many cells per side
        // around the center.
        public const int EmptyBoardSeedSpan = 3;
        public const int DefaultCandidateCapacity = 64;
        public const int InitialUndoCapacity = 64;
    }

    public static class Limits
    {
        public const int MaxConcurrentGames = 4;

        // Mirrored by hand in Caro.Server.runtimeconfig.template.json
        // (System.GC.HeapHardLimitBytes); update both together.
        public const long HeapHardLimitBytes = 2L * 1024 * 1024 * 1024;

        public const int AbandonedTimeoutMinutes = 30;
    }

    public static class Opening
    {
        // Half-width of the seeded opening scatter around the center.
        public const int SpreadRadius = 3;
    }

    public static class Difficulty
    {
        public const int MinLevel = 1;
        public const int MaxLevel = 5;
    }

    public static class TimeControl
    {
        public const string Default = "7+5";
        public const long DefaultInitialTimeMs = 420_000;
        public const int DefaultIncrementSeconds = 5;
    }
}
