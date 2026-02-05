using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Caro.TournamentRunner.ReportGenerators;
using System.Diagnostics;

namespace Caro.TournamentRunner;

/// <summary>
/// Command-line runner for AI Strength Validation tests.
/// Provides automated testing with configurable parameters and HTML report generation.
/// </summary>
public static class AIStrengthTestRunner
{
    /// <summary>
    /// Default configuration for validation tests.
    /// </summary>
    public class TestConfig
    {
        public int GamesPerMatchup { get; set; } = 25;
        public int InitialTimeSeconds { get; set; } = 120;
        public int IncrementSeconds { get; set; } = 1;
        public bool EnablePondering { get; set; } = true;
        public string OutputDirectory { get; set; } = "./TestResults";
        public bool GenerateHtmlReport { get; set; } = true;
        public bool Verbose { get; set; } = false;
    }

    /// <summary>
    /// Run all validation phases with the specified configuration.
    /// </summary>
    public static async Task RunAllAsync(TestConfig config)
    {
        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine("║              AI STRENGTH VALIDATION TEST SUITE                    ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();
        Console.WriteLine($"Configuration:");
        Console.WriteLine($"  Games per matchup: {config.GamesPerMatchup}");
        Console.WriteLine($"  Time control: {config.InitialTimeSeconds}+{config.IncrementSeconds}");
        Console.WriteLine($"  Pondering: {(config.EnablePondering ? "Enabled" : "Disabled")}");
        Console.WriteLine($"  Output: {config.OutputDirectory}");
        Console.WriteLine();

        var stopwatch = Stopwatch.StartNew();
        var results = new List<MatchupStatistics>();

        // Phase 1: Adjacent Difficulty Testing
        results.AddRange(await RunPhase1Async(config));

        // Phase 2: Cross-Level Testing
        results.AddRange(await RunPhase2Async(config));

        // Phase 3: Color Advantage Detection
        results.AddRange(await RunPhase3Async(config));

        stopwatch.Stop();

        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine($"VALIDATION COMPLETE - Total time: {stopwatch.Elapsed.TotalMinutes:F2} minutes");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        // Print summary
        PrintSummary(results);

        // Generate HTML report
        if (config.GenerateHtmlReport)
        {
            await GenerateReportAsync(results, config.OutputDirectory);
        }
    }

    /// <summary>
    /// Phase 1: Adjacent Difficulty Testing
    /// Tests neighboring difficulty levels to detect inversions.
    /// </summary>
    private static async Task<List<MatchupStatistics>> RunPhase1Async(TestConfig config)
    {
        Console.WriteLine("PHASE 1: Adjacent Difficulty Testing");
        Console.WriteLine("─────────────────────────────────────────────────────────────────");

        var results = new List<MatchupStatistics>();
        var engine = TournamentEngine.CreateDefault();

        var matchups = new (AIDifficulty higher, AIDifficulty lower)[]
        {
            (AIDifficulty.Grandmaster, AIDifficulty.Hard),
            (AIDifficulty.Hard, AIDifficulty.Medium),
            (AIDifficulty.Medium, AIDifficulty.Easy),
            (AIDifficulty.Easy, AIDifficulty.Braindead),
        };

        foreach (var (higher, lower) in matchups)
        {
            var stat = await RunMatchupAsync(engine, higher, lower, config);
            results.Add(stat);
            Console.WriteLine($"  {stat.ToSummaryString()}");
        }

        Console.WriteLine();
        return results;
    }

    /// <summary>
    /// Phase 2: Cross-Level Testing
    /// Tests large difficulty gaps.
    /// </summary>
    private static async Task<List<MatchupStatistics>> RunPhase2Async(TestConfig config)
    {
        Console.WriteLine("PHASE 2: Cross-Level Testing (Large Gaps)");
        Console.WriteLine("─────────────────────────────────────────────────────────────────");

        var results = new List<MatchupStatistics>();
        var engine = TournamentEngine.CreateDefault();

        var matchups = new (AIDifficulty higher, AIDifficulty lower)[]
        {
            (AIDifficulty.Grandmaster, AIDifficulty.Medium),
            (AIDifficulty.Grandmaster, AIDifficulty.Easy),
            (AIDifficulty.Hard, AIDifficulty.Easy),
        };

        foreach (var (higher, lower) in matchups)
        {
            var stat = await RunMatchupAsync(engine, higher, lower, config);
            results.Add(stat);
            Console.WriteLine($"  {stat.ToSummaryString()}");
        }

        Console.WriteLine();
        return results;
    }

    /// <summary>
    /// Phase 3: Color Advantage Detection
    /// Tests symmetric matchups to detect color bias.
    /// </summary>
    private static async Task<List<MatchupStatistics>> RunPhase3Async(TestConfig config)
    {
        Console.WriteLine("PHASE 3: Color Advantage Detection");
        Console.WriteLine("─────────────────────────────────────────────────────────────────");

        var results = new List<MatchupStatistics>();
        var engine = TournamentEngine.CreateDefault();

        var difficulties = new[]
        {
            AIDifficulty.Grandmaster,
            AIDifficulty.Hard,
            AIDifficulty.Medium,
        };

        foreach (var diff in difficulties)
        {
            var stat = await RunMatchupAsync(engine, diff, diff, config);
            results.Add(stat);
            Console.WriteLine($"  {diff}: Red wins {stat.RedColorWins}, Blue wins {stat.BlueColorWins} " +
                            $"(Red rate: {stat.RedColorWinRate:P1})");
        }

        Console.WriteLine();
        return results;
    }

    /// <summary>
    /// Run a single matchup with color swapping and collect statistics.
    /// </summary>
    private static async Task<MatchupStatistics> RunMatchupAsync(
        TournamentEngine engine,
        AIDifficulty redDiff,
        AIDifficulty blueDiff,
        TestConfig config)
    {
        var results = new List<(bool redDiffPlayedAsRed, Player winner)>();
        var redPlayerWins = 0;
        var bluePlayerWins = 0;
        var draws = 0;

        for (int i = 0; i < config.GamesPerMatchup; i++)
        {
            bool swapColors = (i % 2 == 1);

            var actualRed = swapColors ? blueDiff : redDiff;
            var actualBlue = swapColors ? redDiff : blueDiff;

            var result = engine.RunGame(
                redDifficulty: actualRed,
                blueDifficulty: actualBlue,
                maxMoves: 361,
                initialTimeSeconds: config.InitialTimeSeconds,
                incrementSeconds: config.IncrementSeconds,
                ponderingEnabled: config.EnablePondering);

            if (result.IsDraw)
            {
                draws++;
            }
            else if (result.Winner == Player.Red)
            {
                if (swapColors)
                    bluePlayerWins++;
                else
                    redPlayerWins++;
            }
            else
            {
                if (swapColors)
                    redPlayerWins++;
                else
                    bluePlayerWins++;
            }

            if (!result.IsDraw)
            {
                results.Add((!swapColors, result.Winner));
            }

            // Progress update
            if (config.Verbose || i % 5 == 0)
            {
                Console.Write($"\r    {redDiff} vs {blueDiff}: {i + 1}/{config.GamesPerMatchup}");
            }
        }

        Console.Write($"\r{' ',60}\r"); // Clear progress line

        // Build stats object
        var stats = new MatchupStatistics
        {
            RedDifficulty = redDiff,
            BlueDifficulty = blueDiff,
            TotalGames = config.GamesPerMatchup,
            RedPlayerWins = redPlayerWins,
            BluePlayerWins = bluePlayerWins,
            Draws = draws
        };

        // Calculate color-specific stats
        foreach (var (redDiffPlayedAsRed, winner) in results)
        {
            if (redDiffPlayedAsRed)
            {
                if (winner == Player.Red)
                    stats.RedAsRed_Wins++;
                else
                    stats.BlueAsBlue_Wins++;
            }
            else
            {
                if (winner == Player.Red)
                    stats.BlueAsRed_Wins++;
                else
                    stats.RedAsBlue_Wins++;
            }
        }

        // Calculate statistics
        stats.LikelihoodOfSuperiority = StatisticalAnalyzer.CalculateLOS(redPlayerWins, bluePlayerWins, draws);
        var (eloDiff, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(redPlayerWins, bluePlayerWins, draws);
        stats.EloDifference = eloDiff;
        stats.ConfidenceIntervalLower = lowerCI;
        stats.ConfidenceIntervalUpper = upperCI;
        stats.PValue = StatisticalAnalyzer.BinomialTestPValue(redPlayerWins, config.GamesPerMatchup, 0.5);

        var (hasColorAdv, effectSize, _) = StatisticalAnalyzer.DetectColorAdvantage(results);
        stats.HasColorAdvantage = hasColorAdv;
        stats.ColorAdvantageEffectSize = effectSize;

        // Determine test result
        var higherDiff = redDiff > blueDiff ? redDiff : blueDiff;
        var lowerDiff = redDiff < blueDiff ? redDiff : blueDiff;
        stats.ExpectedResult = $"{higherDiff} should beat {lowerDiff}";

        if (redDiff > blueDiff)
        {
            stats.TestPassed = redPlayerWins > bluePlayerWins && stats.PValue < 0.1;
            stats.Conclusion = stats.TestPassed
                ? $"{redDiff} is significantly stronger than {blueDiff}"
                : $"FAILED: {redDiff} vs {blueDiff} - Expected {redDiff} to win, but {(bluePlayerWins > redPlayerWins ? $"{blueDiff}" : "neither")} won more";
        }
        else if (blueDiff > redDiff)
        {
            stats.TestPassed = bluePlayerWins > redPlayerWins && stats.PValue < 0.1;
            stats.Conclusion = stats.TestPassed
                ? $"{blueDiff} is significantly stronger than {redDiff}"
                : $"FAILED: {blueDiff} vs {redDiff} - Expected {blueDiff} to win, but {(redPlayerWins > bluePlayerWins ? $"{redDiff}" : "neither")} won more";
        }
        else
        {
            stats.TestPassed = Math.Abs(redPlayerWins - bluePlayerWins) <= 3;
            stats.Conclusion = stats.TestPassed
                ? "Equal difficulties performed as expected"
                : $"Equal difficulties but {(redPlayerWins > bluePlayerWins ? "Red" : "Blue")} won more";
        }

        return stats;
    }

    private static void PrintSummary(IReadOnlyList<MatchupStatistics> results)
    {
        var totalTests = results.Count;
        var passedTests = results.Count(r => r.TestPassed);
        var totalGames = results.Sum(r => r.TotalGames);

        Console.WriteLine("SUMMARY:");
        Console.WriteLine($"  Total Tests: {totalTests}");
        Console.WriteLine($"  Passed: {passedTests}");
        Console.WriteLine($"  Failed: {totalTests - passedTests}");
        Console.WriteLine($"  Total Games: {totalGames}");
        Console.WriteLine();

        // Show failures
        var failures = results.Where(r => !r.TestPassed).ToList();
        if (failures.Any())
        {
            Console.WriteLine("FAILED TESTS:");
            foreach (var failure in failures)
            {
                Console.WriteLine($"  - {failure.RedDifficulty} vs {failure.BlueDifficulty}: {failure.Conclusion}");
            }
            Console.WriteLine();
        }
    }

    private static async Task GenerateReportAsync(IReadOnlyList<MatchupStatistics> results, string outputDirectory)
    {
        try
        {
            if (!Directory.Exists(outputDirectory))
            {
                Directory.CreateDirectory(outputDirectory);
            }

            var timestamp = DateTime.Now.ToString("yyyyMMdd_HHmmss");
            var filename = $"AI_Strength_Validation_{timestamp}.html";
            var fullPath = Path.Combine(outputDirectory, filename);

            var html = HtmlReportGenerator.GenerateReport(results, "AI Strength Validation Report");
            await File.WriteAllTextAsync(fullPath, html);

            Console.WriteLine($"HTML report saved to: {fullPath}");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Failed to generate HTML report: {ex.Message}");
        }
    }

    /// <summary>
    /// Run a quick subset of tests (for faster validation).
    /// </summary>
    public static async Task RunQuickAsync(TestConfig? config = null)
    {
        config ??= new TestConfig { GamesPerMatchup = 10, Verbose = true, EnablePondering = false };

        Console.WriteLine("QUICK VALIDATION MODE (10 games per matchup)");
        Console.WriteLine($"  Pondering: {(config.EnablePondering ? "Enabled" : "Disabled")}");
        Console.WriteLine();

        var results = new List<MatchupStatistics>();
        var engine = TournamentEngine.CreateDefault();

        // Test only a few key matchups
        var keyMatchups = new (AIDifficulty, AIDifficulty)[]
        {
            (AIDifficulty.Grandmaster, AIDifficulty.Hard),
            (AIDifficulty.Hard, AIDifficulty.Medium),
            (AIDifficulty.Medium, AIDifficulty.Easy),
            (AIDifficulty.Grandmaster, AIDifficulty.Medium),  // Cross-level
            (AIDifficulty.Grandmaster, AIDifficulty.Grandmaster),  // Color advantage
        };

        foreach (var (red, blue) in keyMatchups)
        {
            var stat = await RunMatchupAsync(engine, red, blue, config);
            results.Add(stat);
            Console.WriteLine($"  {stat.ToSummaryString()}");
        }

        Console.WriteLine();
        PrintSummary(results);
    }
}
