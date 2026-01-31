using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class HardTestSuite : ITestSuite
{
    public string Name => "Hard";

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Hard vs Braindead"] = new(95.0, true),
            ["Hard vs Easy"] = new(90.0, true),
            ["Hard vs Medium"] = new(80.0, true),
            ["Hard vs Hard"] = new(0.0, true) // self-play, no threshold
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var matchups = new List<MatchupConfig>
        {
            new(AIDifficulty.Hard, AIDifficulty.Braindead, 20),
            new(AIDifficulty.Hard, AIDifficulty.Easy, 20),
            new(AIDifficulty.Hard, AIDifficulty.Medium, 20),
            new(AIDifficulty.Hard, AIDifficulty.Hard, 20)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output);
    }
}
