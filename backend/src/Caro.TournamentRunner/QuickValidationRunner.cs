using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick validation runner for testing AI strength with different time controls.
/// </summary>
public class QuickValidationRunner
{
    public static async Task RunAsync(int initialSeconds, int incrementSeconds, int gamesPerMatchup = 10)
    {
        var engine = new TournamentEngine();
        var tcName = $"{initialSeconds / 60}+{incrementSeconds}";

        Console.WriteLine($"=== AI Strength Validation: {tcName} Time Control ===");
        Console.WriteLine($"Games per matchup: {gamesPerMatchup}");
        Console.WriteLine();

        // Test adjacent difficulties
        var matchups = new[]
        {
            (Red: AIDifficulty.Hard, Blue: AIDifficulty.Medium, Name: "Hard vs Medium"),
            (Red: AIDifficulty.Grandmaster, Blue: AIDifficulty.Hard, Name: "GM vs Hard"),
            (Red: AIDifficulty.Medium, Blue: AIDifficulty.Easy, Name: "Medium vs Easy"),
        };

        foreach (var (redDiff, blueDiff, name) in matchups)
        {
            var redWins = 0;
            var blueWins = 0;
            var draws = 0;

            Console.WriteLine($"--- {name} ---");

            for (int i = 0; i < gamesPerMatchup; i++)
            {
                // Alternate colors for fairness
                bool swapColors = (i % 2 == 1);
                var actualRed = swapColors ? blueDiff : redDiff;
                var actualBlue = swapColors ? redDiff : blueDiff;

                var result = engine.RunGame(
                    redDifficulty: actualRed,
                    blueDifficulty: actualBlue,
                    maxMoves: 200,
                    initialTimeSeconds: initialSeconds,
                    incrementSeconds: incrementSeconds,
                    ponderingEnabled: true);

                if (result.IsDraw)
                {
                    draws++;
                }
                else if (result.Winner == Player.Red)
                {
                    if (swapColors)
                        blueWins++;
                    else
                        redWins++;
                }
                else
                {
                    if (swapColors)
                        redWins++;
                    else
                        blueWins++;
                }
            }

            var total = redWins + blueWins + draws;
            var redWinRate = (double)redWins / total;
            var blueWinRate = (double)blueWins / total;

            Console.WriteLine($"  Result: {redDiff} {redWins} - {blueWins} {blueDiff} (Draws: {draws})");
            Console.WriteLine($"  Win rates: {redDiff} {redWinRate:P0} vs {blueDiff} {blueWinRate:P0}");

            // Expected: higher difficulty wins more
            var expectedWinner = redDiff > blueDiff ? redDiff : blueDiff;
            AIDifficulty? actualWinner = redWins > blueWins ? redDiff : (blueWins > redWins ? blueDiff : null);
            var passed = actualWinner.HasValue && actualWinner.Value == expectedWinner;

            Console.WriteLine($"  Status: {(passed ? "PASS" : "FAIL")} - Expected {expectedWinner} to win");
            Console.WriteLine();
        }

        Console.WriteLine("=== Validation Complete ===");
    }
}
