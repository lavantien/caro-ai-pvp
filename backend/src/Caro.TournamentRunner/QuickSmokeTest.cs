using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick smoke test for verifying AI strength ordering and book integration.
/// 5 matchups, 2 games each = 10 total games with 3+2 time control.
/// Shows detailed stats per move for debugging.
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
        Console.WriteLine("  QUICK SMOKE TEST: AI Strength Verification");
        Console.WriteLine($"  {GamesPerMatchup} games per matchup, 3+2 time control");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        var totalRedWins = 0;
        var totalBlueWins = 0;
        var totalDraws = 0;
        var gameNumber = 0;

        foreach (var (red, blue, name) in matchups)
        {
            Console.WriteLine($"\n=== {name} ===");
            var redWins = 0;
            var blueWins = 0;
            var draws = 0;

            for (int i = 0; i < GamesPerMatchup; i++)
            {
                gameNumber++;
                var swapColors = i % 2 == 1;
                var moveCount = 0;
                var gameStartMs = DateTime.UtcNow.Ticks / 10000;

                var result = engine.RunGame(
                    redDifficulty: red,
                    blueDifficulty: blue,
                    initialTimeSeconds: InitialTimeSeconds,
                    incrementSeconds: IncrementSeconds,
                    ponderingEnabled: true,
                    parallelSearchEnabled: true,
                    swapColors: swapColors,
                    onMove: (x, y, player, moveNum, redTimeMs, blueTimeMs, stats) =>
                    {
                        var actualDiff = swapColors
                            ? (player == Player.Red ? blue : red)  // Swapped: Red plays blue's difficulty
                            : (player == Player.Red ? red : blue);
                        Console.WriteLine(GameStatsFormatter.FormatMoveLine(
                            gameNumber, moveNum, x, y, player, actualDiff, stats));
                        moveCount = moveNum;
                    });

                var gameDurationSec = (DateTime.UtcNow.Ticks / 10000 - gameStartMs) / 1000.0;

                if (result.IsDraw)
                {
                    draws++;
                    totalDraws++;
                    Console.WriteLine(GameStatsFormatter.FormatGameResult(gameNumber, AIDifficulty.Easy, moveCount, gameDurationSec, isDraw: true));
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
                    Console.WriteLine(GameStatsFormatter.FormatGameResult(gameNumber, result.WinnerDifficulty, moveCount, gameDurationSec, result.Winner));
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
