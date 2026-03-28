namespace Caro.Core.MatchupTests.Engine.Performance;

public static class MatchupTestConfig
{
    public const int GamesPerAdjacentPair = 20;
    public const int GamesPerCrossLevel = 15;
    public const int GamesPerSelfPlay = 20;
    public const int GamesPerSmokePair = 4;

    public const int InitialTimeSeconds = 420;
    public const int IncrementSeconds = 5;

    public const double SprtElo0 = 0;
    public const double SprtElo1Adjacent = 50;
    public const double SprtElo1CrossLevel = 100;
    public const int SprtMinGames = 10;

    public const double SelfPlayMinRedWinRate = 0.35;
    public const double SelfPlayMaxRedWinRate = 0.65;

    public const int MaxMoves = 256;
}
