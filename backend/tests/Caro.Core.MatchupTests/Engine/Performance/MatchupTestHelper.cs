using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

public static class MatchupTestHelper
{
    public static MatchupStatistics RunMatchupWithStatistics(
        AIDifficulty redDiff,
        AIDifficulty blueDiff,
        int games,
        ITestOutputHelper? output = null,
        int initialTimeSeconds = MatchupTestConfig.InitialTimeSeconds,
        int incrementSeconds = MatchupTestConfig.IncrementSeconds)
    {
        var results = new List<(bool redIsPlayer, Player winner)>();
        var redPlayerWins = 0;
        var bluePlayerWins = 0;
        var draws = 0;

        var engine = CreateEngine();

        for (int i = 0; i < games; i++)
        {
            bool swapColors = i % 2 == 1;

            var actualRed = swapColors ? blueDiff : redDiff;
            var actualBlue = swapColors ? redDiff : blueDiff;

            var result = engine.RunGame(
                redDifficulty: actualRed,
                blueDifficulty: actualBlue,
                maxMoves: MatchupTestConfig.MaxMoves,
                initialTimeSeconds: initialTimeSeconds,
                incrementSeconds: incrementSeconds,
                ponderingEnabled: true,
                parallelSearchEnabled: true);

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

            output?.WriteLine($"  Game {i + 1}: {actualRed} vs {actualBlue} (swap={swapColors}) => {result.Winner} in {result.TotalMoves} moves");
        }

        var stats = new MatchupStatistics
        {
            RedDifficulty = redDiff,
            BlueDifficulty = blueDiff,
            TotalGames = games,
            RedPlayerWins = redPlayerWins,
            BluePlayerWins = bluePlayerWins,
            Draws = draws
        };

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

        var los = StatisticalAnalyzer.CalculateLOS(redPlayerWins, bluePlayerWins, draws);
        var (eloDiff, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(redPlayerWins, bluePlayerWins, draws);
        var pValue = StatisticalAnalyzer.BinomialTestPValue(redPlayerWins, games, 0.5);

        stats.LikelihoodOfSuperiority = los;
        stats.EloDifference = eloDiff;
        stats.ConfidenceIntervalLower = lowerCI;
        stats.ConfidenceIntervalUpper = upperCI;
        stats.PValue = pValue;

        var (hasColorAdv, effectSize, _) = StatisticalAnalyzer.DetectColorAdvantage(results);
        stats.HasColorAdvantage = hasColorAdv;
        stats.ColorAdvantageEffectSize = effectSize;

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

    public static StatisticalAnalyzer.SPRTResult CheckSPRT(
        int redWins, int blueWins, int draws,
        double elo0 = MatchupTestConfig.SprtElo0,
        double elo1 = MatchupTestConfig.SprtElo1Adjacent)
    {
        return StatisticalAnalyzer.SPRT(redWins, blueWins, draws, elo0, elo1);
    }

    private static TournamentEngine CreateEngine()
    {
        return AITestHelper.CreateNonDeterministicEngine();
    }
}
