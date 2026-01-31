using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using System.Text;

namespace Caro.TournamentRunner.TestSuite;

public class GrandmasterTestSuite : ITestSuite
{
    public string Name => "Grandmaster";

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Grandmaster vs Braindead"] = new(100.0, true),
            ["Grandmaster vs Easy"] = new(95.0, true),
            ["Grandmaster vs Medium"] = new(90.0, true),
            ["Grandmaster vs Hard"] = new(80.0, true),
            ["Grandmaster vs Grandmaster"] = new(0.0, true) // self-play, no threshold
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var matchups = new List<MatchupConfig>
        {
            new(AIDifficulty.Grandmaster, AIDifficulty.Braindead, 20),
            new(AIDifficulty.Grandmaster, AIDifficulty.Easy, 20),
            new(AIDifficulty.Grandmaster, AIDifficulty.Medium, 20),
            new(AIDifficulty.Grandmaster, AIDifficulty.Hard, 20),
            new(AIDifficulty.Grandmaster, AIDifficulty.Grandmaster, 20)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output);
    }
}
