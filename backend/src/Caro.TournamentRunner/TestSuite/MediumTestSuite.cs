using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class MediumTestSuite : ITestSuite
{
    public string Name => "Medium";

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Medium vs Braindead"] = new(90.0, true),
            ["Medium vs Easy"] = new(80.0, true),
            ["Medium vs Medium"] = new(0.0, true) // self-play, no threshold
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var matchups = new List<MatchupConfig>
        {
            new(AIDifficulty.Medium, AIDifficulty.Braindead, 20),
            new(AIDifficulty.Medium, AIDifficulty.Easy, 20),
            new(AIDifficulty.Medium, AIDifficulty.Medium, 20)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output);
    }
}
