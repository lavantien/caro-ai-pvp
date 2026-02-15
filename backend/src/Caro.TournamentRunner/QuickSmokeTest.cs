using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick smoke test for verifying opening book integration.
/// 5 matchups, 2 games each = 10 total games with 3+2 time control.
/// </summary>
public static class QuickSmokeTest
{
    private const int GamesPerMatchup = 2;  // 2 games each = 10 total
    private const int InitialTimeSeconds = 180;  // 3+2 blitz
    private const int IncrementSeconds = 2;

    public static void Run()
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();

        var matchups = new[]
        {
            (AIDifficulty.Braindead, AIDifficulty.Easy, "Braindead vs Easy"),
            (AIDifficulty.Easy, AIDifficulty.Braindead, "Easy vs Braindead"),
            (AIDifficulty.Grandmaster, AIDifficulty.Hard, "GM vs Hard"),
            (AIDifficulty.Hard, AIDifficulty.Grandmaster, "Hard vs GM"),
            (AIDifficulty.Grandmaster, AIDifficulty.Grandmaster, "GM vs GM"),
        };

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  QUICK SMOKE TEST: Opening Book Verification");
        Console.WriteLine($"  {GamesPerMatchup} games per matchup, 3+2 time control");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        var totalRedWins = 0;
        var totalBlueWins = 0;
        var totalDraws = 0;

        foreach (var (red, blue, name) in matchups)
        {
            Console.WriteLine($"\n=== {name} ===");
            var redWins = 0;
            var blueWins = 0;
            var draws = 0;

            for (int i = 0; i < GamesPerMatchup; i++)
            {
                var swapColors = i % 2 == 1;
                var result = engine.RunGame(
                    redDifficulty: red,
                    blueDifficulty: blue,
                    initialTimeSeconds: InitialTimeSeconds,
                    incrementSeconds: IncrementSeconds,
                    ponderingEnabled: true,
                    parallelSearchEnabled: true,
                    swapColors: swapColors);

                if (result.IsDraw)
                {
                    draws++;
                    totalDraws++;
                    Console.WriteLine($"  Game {i + 1}: DRAW");
                }
                else
                {
                    if (result.Winner == Player.Red)
                    {
                        redWins++;
                        totalRedWins++;
                    }
                    else
                    {
                        blueWins++;
                        totalBlueWins++;
                    }
                    Console.WriteLine($"  Game {i + 1}: {result.WinnerDifficulty} wins ({result.Winner})");
                }
            }

            Console.WriteLine($"  Matchup result: Red {redWins} - Blue {blueWins} - Draw {draws}");
        }

        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine($"  TOTAL: Red {totalRedWins} - Blue {totalBlueWins} - Draw {totalDraws}");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
    }
}
