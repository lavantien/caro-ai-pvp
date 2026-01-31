using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner.TestSuite;

public class FullIntegratedTestSuite : ITestSuite
{
    public string Name => "FullIntegrated";

    public TestSuiteExpectations Expectations => new(new Dictionary<string, WinRateThreshold>());

    public TestSuiteResult Run(TextWriter output)
    {
        var suites = new ITestSuite[]
        {
            new GrandmasterTestSuite(),
            new HardTestSuite(),
            new MediumTestSuite(),
            new EasyTestSuite(),
            new BraindeadTestSuite()
        };

        var allMatchups = new List<MatchupResult>();
        int totalPassed = 0;
        int totalFailed = 0;

        foreach (var suite in suites)
        {
            var result = suite.Run(output);
            allMatchups.AddRange(result.Matchups);
            totalPassed += result.PassedCount;
            totalFailed += result.FailedCount;
        }

        return new TestSuiteResult(Name, allMatchups, totalPassed, totalFailed);
    }
}
