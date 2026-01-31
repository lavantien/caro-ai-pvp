using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class BraindeadTestSuite : ITestSuite
{
    public string Name => "Braindead";

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Braindead vs Braindead"] = new(0.0, true) // self-play, random baseline
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var matchups = new List<MatchupConfig>
        {
            new(AIDifficulty.Braindead, AIDifficulty.Braindead, 20)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output);
    }
}
