using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public interface ITestSuite
{
    string Name { get; }
    TestSuiteResult Run(TextWriter output);
    TestSuiteExpectations Expectations { get; }
}

public record TestSuiteExpectations(
    Dictionary<string, WinRateThreshold> MatchupExpectations
);

public record WinRateThreshold(
    double MinWinRate,
    bool AllowDraws
);

public record MatchupResult(
    string RedAI,
    string BlueAI,
    int RedWins,
    int Draws,
    int BlueWins,
    double ActualWinRate,
    bool Passed,
    string ExpectedDisplay,
    int GameCount
);

public record TestSuiteResult(
    string SuiteName,
    List<MatchupResult> Matchups,
    int PassedCount,
    int FailedCount
)
{
    public int TotalCount => Matchups.Count;
}

public record MatchupConfig(
    AIDifficulty RedDifficulty,
    AIDifficulty BlueDifficulty,
    int GameCount,
    bool AlternateColors = true
);
