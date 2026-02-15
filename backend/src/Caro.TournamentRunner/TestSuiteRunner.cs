using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Caro.TournamentRunner.TestSuite;
using System.Text;

namespace Caro.TournamentRunner;

public class TestSuiteRunner
{
    private readonly Dictionary<string, ITestSuite> _suites;

    public TestSuiteRunner()
    {
        _suites = new(StringComparer.OrdinalIgnoreCase)
        {
            ["grandmaster"] = new GrandmasterTestSuite(),
            ["hard"] = new HardTestSuite(),
            ["medium"] = new MediumTestSuite(),
            ["easy"] = new EasyTestSuite(),
            ["braindead"] = new BraindeadTestSuite(),
            ["experimental"] = new ExperimentalTestSuite(),
            ["full"] = new FullIntegratedTestSuite()
        };
    }

    public bool Run(string suiteName, string outputPath)
    {
        if (!_suites.TryGetValue(suiteName, out var suite))
        {
            Console.WriteLine($"Available test suites: {string.Join(", ", _suites.Keys)}");
            return true;
        }

        using var writer = new StreamWriter(outputPath, append: false, Encoding.UTF8)
        {
            AutoFlush = true
        };

        WriteHeader(writer, suiteName);
        var result = suite.Run(writer);
        WriteSummary(writer, result);

        Console.SetOut(Console.Out);
        Console.WriteLine($"Results written to: {outputPath}");
        Console.WriteLine($"Passed: {result.PassedCount} | Failed: {result.FailedCount}");

        return true;
    }

    internal static TestSuiteResult RunMatchups(
        ITestSuite suite,
        List<MatchupConfig> matchups,
        TextWriter output,
        string? customRedName = null)
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();
        var results = new List<MatchupResult>();

        foreach (var config in matchups)
        {
            var result = RunMatchup(engine, config, suite.Expectations, output, customRedName);
            results.Add(result);
        }

        var passed = results.Count(r => r.Passed);
        var failed = results.Count(r => !r.Passed);

        WriteSuiteSummary(output, suite.Name, results, passed, failed);

        return new TestSuiteResult(suite.Name, results, passed, failed);
    }

    private static MatchupResult RunMatchup(
        TournamentEngine engine,
        MatchupConfig config,
        TestSuiteExpectations expectations,
        TextWriter output,
        string? customRedName)
    {
        var redName = customRedName ?? config.RedDifficulty.ToString();
        var blueName = config.BlueDifficulty.ToString();
        var matchupKey = $"{redName} vs {blueName}";

        output.WriteLine($"Matchup: {matchupKey} ({config.GameCount} games)");

        var redWins = 0;
        var blueWins = 0;
        var draws = 0;

        int currentGame = 1;
        for (int i = 0; i < config.GameCount; i++)
        {
            // Alternate colors each game: odd games (i=1,3,5...) swap colors
            bool swapColors = (i % 2 == 1);

            // Difficulties stay constant with their bot instances
            var redDiff = config.RedDifficulty;  // Difficulty for first bot
            var blueDiff = config.BlueDifficulty; // Difficulty for second bot

            // Bot names stay constant to their color positions for result tracking
            var redBotName = redName;
            var blueBotName = blueName;

            var result = engine.RunGame(
                redDiff,
                blueDiff,
                maxMoves: 1024,
                initialTimeSeconds: 60,
                incrementSeconds: 1,
                ponderingEnabled: true,
                parallelSearchEnabled: true,
                redBotName: redBotName,
                blueBotName: blueBotName,
                swapColors: swapColors,
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    // Determine actual difficulty based on which bot is playing which color
                    // When swapped: Red slot has Blue's bot (with blueDiff), Blue slot has Red's bot (with redDiff)
                    var actualDifficulty = swapColors
                        ? (player == Player.Red ? blueDiff : redDiff)
                        : (player == Player.Red ? redDiff : blueDiff);
                    output.WriteLine(GameStatsFormatter.FormatMoveLine(
                        currentGame, moveNumber, x, y, player, actualDifficulty, stats));
                }
            );

            output.WriteLine(GameStatsFormatter.FormatGameResult(
                currentGame, result.WinnerDifficulty, result.TotalMoves, result.DurationMs / 1000.0, result.Winner, result.IsDraw));

            currentGame++;

            if (result.IsDraw)
                draws++;
            else if (result.Winner == Player.Red)
                redWins++;
            else
                blueWins++;
        }

        double winRate = config.RedDifficulty == config.BlueDifficulty
            ? 50.0
            : ((double)redWins / config.GameCount) * 100.0;

        bool passed = CheckThreshold(matchupKey, winRate, draws, config.GameCount, expectations);

        string expectedDisplay = GetExpectedDisplay(matchupKey, expectations);
        string actualDisplay = $"{redWins} - {draws} - {blueWins}";

        output.WriteLine($"  Result: {redName} {actualDisplay} {blueName}");
        output.WriteLine($"  Win Rate: {winRate:F1}% | Expected: {expectedDisplay} | {(passed ? "PASS" : "FAIL")}");
        output.WriteLine();

        return new MatchupResult(
            redName,
            blueName,
            redWins,
            draws,
            blueWins,
            winRate,
            passed,
            expectedDisplay,
            config.GameCount
        );
    }

    private static bool CheckThreshold(
        string matchupKey,
        double winRate,
        int draws,
        int gameCount,
        TestSuiteExpectations expectations)
    {
        if (!expectations.MatchupExpectations.TryGetValue(matchupKey, out var threshold))
            return true;

        if (threshold.MinWinRate == 0)
            return true;

        double winDrawRate = ((double)draws / gameCount) * 100.0 + winRate;
        double required = threshold.AllowDraws ? threshold.MinWinRate : winRate;

        return threshold.AllowDraws
            ? winDrawRate >= threshold.MinWinRate
            : winRate >= threshold.MinWinRate;
    }

    private static string GetExpectedDisplay(string matchupKey, TestSuiteExpectations expectations)
    {
        if (!expectations.MatchupExpectations.TryGetValue(matchupKey, out var threshold))
            return "N/A";

        if (threshold.MinWinRate == 0)
            return "~50% (self-play baseline)";

        var type = threshold.AllowDraws ? "win+draw" : "win";
        return $">={threshold.MinWinRate:F0}% ({type})";
    }

    private static void WriteHeader(TextWriter writer, string suiteName)
    {
        writer.WriteLine("=== CARO AI TEST SUITE RESULTS ===");
        writer.WriteLine($"Timestamp: {DateTime.Now:yyyy-MM-dd HH:mm:ss}");
        writer.WriteLine($"Host: {Environment.ProcessorCount} logical processors");
        writer.WriteLine();
    }

    private static void WriteSuiteSummary(
        TextWriter output,
        string suiteName,
        List<MatchupResult> results,
        int passed,
        int failed)
    {
        output.WriteLine(new string('-', 36));
        output.WriteLine($"{suiteName.ToUpper()} SUITE: {passed}/{passed + failed} PASS");
        output.WriteLine();
    }

    private static void WriteSummary(TextWriter writer, TestSuiteResult result)
    {
        writer.WriteLine(new string('=', 36));
        writer.WriteLine("SUMMARY");
        writer.WriteLine(new string('=', 36));
        writer.WriteLine($"Total Matchups: {result.TotalCount}");
        writer.WriteLine($"Passed: {result.PassedCount} | Failed: {result.FailedCount}");
    }
}
