using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class EasyTestSuite : ITestSuite
{
    public string Name => "Easy";

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Easy vs Braindead"] = new(80.0, true),
            ["Easy vs Easy"] = new(0.0, true) // self-play, no threshold
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var matchups = new List<MatchupConfig>
        {
            new(AIDifficulty.Easy, AIDifficulty.Braindead, 20),
            new(AIDifficulty.Easy, AIDifficulty.Easy, 20)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output);
    }
}
