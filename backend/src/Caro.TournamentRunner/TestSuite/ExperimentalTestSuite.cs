using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class ExperimentalTestSuite : ITestSuite
{
    public string Name => "Experimental";

    public AIDifficulty CustomDifficulty { get; set; } = AIDifficulty.Grandmaster;

    public TestSuiteExpectations Expectations => new(
        new Dictionary<string, WinRateThreshold>
        {
            ["Custom vs Braindead"] = new(100.0, true),
            ["Custom vs Easy"] = new(95.0, true),
            ["Custom vs Medium"] = new(90.0, true),
            ["Custom vs Hard"] = new(85.0, true),
            ["Custom vs Grandmaster"] = new(80.0, true)
        }
    );

    public TestSuiteResult Run(TextWriter output)
    {
        var customName = CustomDifficulty.ToString();
        var matchups = new List<MatchupConfig>
        {
            new(CustomDifficulty, AIDifficulty.Braindead, 10),
            new(CustomDifficulty, AIDifficulty.Easy, 10),
            new(CustomDifficulty, AIDifficulty.Medium, 10),
            new(CustomDifficulty, AIDifficulty.Hard, 10),
            new(CustomDifficulty, AIDifficulty.Grandmaster, 10)
        };

        return TestSuiteRunner.RunMatchups(this, matchups, output, customName);
    }
}
