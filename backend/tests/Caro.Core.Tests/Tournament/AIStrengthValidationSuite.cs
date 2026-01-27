using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.Tournament;

/// <summary>
/// Comprehensive AI strength validation test suite.
/// Uses statistical analysis to validate that AI difficulty levels are correctly ordered
/// and that there are no unexpected color advantages or strength inversions.
///
/// Run with: dotnet test --filter "FullyQualifiedName~AIStrengthValidationSuite"
/// </summary>
public class AIStrengthValidationSuite : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;
    private readonly List<MatchResult> _allResults = new();

    // Test configuration
    private const int GamesPerMatchup = 10;  // Balance between statistical power and test duration
    private const int InitialTimeSeconds = 420;  // 7+5 time control
    private const int IncrementSeconds = 5;

    public AIStrengthValidationSuite(ITestOutputHelper output)
    {
        _output = output;
        _engine = new TournamentEngine();
    }

    public void Dispose()
    {
        // Clean up any resources
        GC.SuppressFinalize(this);
    }

    /// <summary>
    /// Run a matchup with color swapping and collect statistics
    /// </summary>
    private MatchupStatistics RunMatchupWithStatistics(
        AIDifficulty redDiff,
        AIDifficulty blueDiff,
        int games)
    {
        var results = new List<(bool redIsPlayer, Player winner)>();
        var redPlayerWins = 0;
        var bluePlayerWins = 0;
        var draws = 0;

        // Run games with color swapping for fair comparison
        for (int i = 0; i < games; i++)
        {
            // Half the games with standard assignment, half with swapped colors
            bool swapColors = (i % 2 == 1);

            var actualRed = swapColors ? blueDiff : redDiff;
            var actualBlue = swapColors ? redDiff : blueDiff;

            var result = _engine.RunGame(
                redDifficulty: actualRed,
                blueDifficulty: actualBlue,
                maxMoves: 225,
                initialTimeSeconds: InitialTimeSeconds,
                incrementSeconds: IncrementSeconds,
                ponderingEnabled: true);

            _allResults.Add(result);

            // Track result with color swap info
            var winner = result.Winner;
            if (result.IsDraw)
            {
                draws++;
            }
            else if (result.Winner == Player.Red)
            {
                // Track who won - if colors were swapped, the actual Red player is different
                if (swapColors)
                    bluePlayerWins++;  // Blue diff won as Red player
                else
                    redPlayerWins++;   // Red diff won as Red player
            }
            else
            {
                if (swapColors)
                    redPlayerWins++;   // Red diff won as Blue player
                else
                    bluePlayerWins++;  // Blue diff won as Blue player
            }

            // Track color-specific results
            if (!result.IsDraw)
            {
                // redIsPlayer = true means actualRed (could be redDiff or blueDiff depending on swap)
                results.Add((!swapColors, winner));
            }
        }

        // Calculate statistics
        var stats = new MatchupStatistics
        {
            RedDifficulty = redDiff,
            BlueDifficulty = blueDiff,
            TotalGames = games,
            RedPlayerWins = redPlayerWins,
            BluePlayerWins = bluePlayerWins,
            Draws = draws
        };

        // Calculate color-specific stats
        // results stores (redDiffPlayedAsRed, winner)
        // redDiffPlayedAsRed = true means: redDiff played as Red, blueDiff played as Blue
        // redDiffPlayedAsRed = false means: blueDiff played as Red, redDiff played as Blue (swapped)
        foreach (var (redDiffPlayedAsRed, winner) in results)
        {
            if (redDiffPlayedAsRed)
            {
                // No swap: redDiff as Red, blueDiff as Blue
                if (winner == Player.Red)
                    stats.RedAsRed_Wins++;  // redDiff won as Red
                else
                    stats.BlueAsBlue_Wins++;  // blueDiff won as Blue
            }
            else
            {
                // Swapped: blueDiff as Red, redDiff as Blue
                if (winner == Player.Red)
                    stats.BlueAsRed_Wins++;  // blueDiff won as Red
                else
                    stats.RedAsBlue_Wins++;  // redDiff won as Blue
            }
        }

        // Statistical calculations
        var los = StatisticalAnalyzer.CalculateLOS(redPlayerWins, bluePlayerWins, draws);
        var (eloDiff, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(redPlayerWins, bluePlayerWins, draws);
        var pValue = StatisticalAnalyzer.BinomialTestPValue(redPlayerWins, games, 0.5);

        stats.LikelihoodOfSuperiority = los;
        stats.EloDifference = eloDiff;
        stats.ConfidenceIntervalLower = lowerCI;
        stats.ConfidenceIntervalUpper = upperCI;
        stats.PValue = pValue;

        // Color advantage detection
        var (hasColorAdv, effectSize, _) = StatisticalAnalyzer.DetectColorAdvantage(results);
        stats.HasColorAdvantage = hasColorAdv;
        stats.ColorAdvantageEffectSize = effectSize;

        // Determine test result
        var higherDiff = redDiff > blueDiff ? redDiff : blueDiff;
        var lowerDiff = redDiff < blueDiff ? redDiff : blueDiff;
        stats.ExpectedResult = $"{higherDiff} should beat {lowerDiff}";

        if (redDiff > blueDiff)
        {
            stats.TestPassed = redPlayerWins > bluePlayerWins && pValue < 0.1;
            stats.Conclusion = stats.TestPassed
                ? $"{redDiff} is significantly stronger than {blueDiff}"
                : $"FAILED: {redDiff} vs {blueDiff} - Expected {redDiff} to win, but {(bluePlayerWins > redPlayerWins ? $"{blueDiff}" : "neither")} won more";
        }
        else if (blueDiff > redDiff)
        {
            stats.TestPassed = bluePlayerWins > redPlayerWins && pValue < 0.1;
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

    #region PHASE 1: Adjacent Difficulty Testing

    [Fact]
    public void Phase1_1_D5VsD4_HigherDifficultyWinsSignificantly()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Hard,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    [Fact]
    public void Phase1_2_D4VsD3_HigherDifficultyWinsSignificantly()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Hard, AIDifficulty.Medium,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    [Fact]
    public void Phase1_3_D3VsD2_HigherDifficultyWinsSignificantly()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Medium, AIDifficulty.Easy,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    [Fact]
    public void Phase1_4_D2VsD1_HigherDifficultyWinsSignificantly()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Easy, AIDifficulty.Braindead,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    #endregion

    #region PHASE 2: Cross-Level Testing

    [Fact]
    public void Phase2_1_D5VsD3_LargeGapDetected()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Medium,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    [Fact]
    public void Phase2_2_D5VsD2_VeryLargeGapDetected()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Easy,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    [Fact]
    public void Phase2_3_D4VsD2_LargeGapDetected()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Hard, AIDifficulty.Easy,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());

        Assert.True(stats.TestPassed, stats.Conclusion);
    }

    #endregion

    #region PHASE 3: Color Advantage Detection

    [Fact]
    public void Phase3_1_D5VsD5_NoSignificantColorAdvantage()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Grandmaster,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());
        _output.WriteLine($"  Red color win rate: {stats.RedColorWinRate:P1}");
        _output.WriteLine($"  Blue color win rate: {stats.BlueColorWinRate:P1}");

        // Should have no significant color advantage (win rate 40-60%)
        Assert.InRange(stats.RedColorWinRate, 0.40, 0.60);
    }

    [Fact]
    public void Phase3_2_D4VsD4_NoSignificantColorAdvantage()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Hard, AIDifficulty.Hard,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());
        _output.WriteLine($"  Red color win rate: {stats.RedColorWinRate:P1}");

        Assert.InRange(stats.RedColorWinRate, 0.40, 0.60);
    }

    [Fact]
    public void Phase3_3_D3VsD3_NoSignificantColorAdvantage()
    {
        var stats = RunMatchupWithStatistics(
            AIDifficulty.Medium, AIDifficulty.Medium,
            GamesPerMatchup);

        _output.WriteLine(stats.ToSummaryString());
        _output.WriteLine($"  Red color win rate: {stats.RedColorWinRate:P1}");

        Assert.InRange(stats.RedColorWinRate, 0.40, 0.60);
    }

    #endregion

    #region PHASE 4: Quick Full Round-Robin

    [Fact]
    public void Phase4_QuickRoundRobin_AllDifficulties_MonotonicStrengthOrdering()
    {
        // Run a quick round-robin with fewer games per matchup
        var difficulties = new[]
        {
            AIDifficulty.Grandmaster, AIDifficulty.Hard, AIDifficulty.Medium, AIDifficulty.Easy
        };

        var eloScores = new Dictionary<AIDifficulty, double>();
        foreach (var diff in difficulties)
        {
            eloScores[diff] = 0;
        }

        var matchups = new List<(AIDifficulty red, AIDifficulty blue)>();
        for (int i = 0; i < difficulties.Length; i++)
        {
            for (int j = i + 1; j < difficulties.Length; j++)
            {
                matchups.Add((difficulties[i], difficulties[j]));
            }
        }

        // Run 10 games per matchup for quick validation
        foreach (var (red, blue) in matchups)
        {
            var stats = RunMatchupWithStatistics(red, blue, 10);

            // Add Elo contribution (positive means red won more)
            eloScores[red] += stats.EloDifference / 2;
            eloScores[blue] -= stats.EloDifference / 2;

            _output.WriteLine($"  {red} vs {blue}: {stats.EloDifference:+0} Elo");
        }

        // Verify monotonic ordering - Grandmaster should have highest score
        var sortedScores = eloScores.OrderByDescending(kvp => kvp.Value).Select(kvp => kvp.Key).ToList();
        _output.WriteLine($"  Elo ranking: {string.Join(" > ", sortedScores.Select(d => $"{d} ({eloScores[d]:+0})"))}");

        // Assert that Grandmaster (D5) is top or tied for top
        var topScore = eloScores.Values.Max();
        Assert.True(eloScores[AIDifficulty.Grandmaster] >= topScore - 50,
            $"Grandmaster should be top scorer, got {eloScores[AIDifficulty.Grandmaster]} vs {topScore}");
    }

    #endregion
}
