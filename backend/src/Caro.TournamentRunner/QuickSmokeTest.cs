using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick smoke test for verifying AI strength ordering and book integration.
/// 5 matchups, 2 games each = 10 total games with 3+2 time control.
/// Shows detailed stats per move for debugging.
/// </summary>
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

        // Track wins by bot name (first/second in matchup), not by color
        var totalFirstBotWins = 0;
        var totalSecondBotWins = 0;
        var totalDraws = 0;
        var gameNumber = 0;

        foreach (var (firstBot, secondBot, name) in matchups)
        {
            Console.WriteLine($"\n=== {name} ===");
            var firstBotWins = 0;
            var secondBotWins = 0;
            var draws = 0;

            for (int i = 0; i < GamesPerMatchup; i++)
            {
                gameNumber++;
                var swapColors = i % 2 == 1;
                var moveCount = 0;
                var gameStartMs = DateTime.UtcNow.Ticks / 10000;

                // When swapColors is true, firstBot plays as Blue, secondBot plays as Red
                var redDiff = swapColors ? secondBot : firstBot;
                var blueDiff = swapColors ? firstBot : secondBot;

                var result = engine.RunGame(
                    redDifficulty: redDiff,
                    blueDifficulty: blueDiff,
                    initialTimeSeconds: InitialTimeSeconds,
                    incrementSeconds: IncrementSeconds,
                    ponderingEnabled: true,
                    parallelSearchEnabled: true,
                    swapColors: false,  // We handle color swapping via difficulty assignment
                    onMove: (x, y, player, moveNum, redTimeMs, blueTimeMs, stats) =>
                    {
                        var actualDiff = player == Player.Red ? redDiff : blueDiff;
                        Console.WriteLine(GameStatsFormatter.FormatMoveLine(
                            gameNumber, moveNum, x, y, player, actualDiff, stats));
                        moveCount = moveNum;
                    });

                var gameDurationSec = (DateTime.UtcNow.Ticks / 10000 - gameStartMs) / 1000.0;

                if (result.IsDraw)
                {
                    draws++;
                    totalDraws++;
                    Console.WriteLine(GameStatsFormatter.FormatGameResult(gameNumber, firstBot, moveCount, gameDurationSec, isDraw: true));
                }
                else
                {
                    // Determine which bot won (first or second in matchup name)
                    var winnerIsFirstBot = (result.Winner == Player.Red && !swapColors) ||
                                          (result.Winner == Player.Blue && swapColors);

                    if (winnerIsFirstBot)
                    {
                        firstBotWins++;
                        totalFirstBotWins++;
                    }
                    else
                    {
                        secondBotWins++;
                        totalSecondBotWins++;
                    }

                    var winningBot = winnerIsFirstBot ? firstBot : secondBot;
                    Console.WriteLine(GameStatsFormatter.FormatGameResult(gameNumber, winningBot, moveCount, gameDurationSec, result.Winner));
                }
            }

            Console.WriteLine($"  Matchup result: {firstBot} {firstBotWins} - {secondBot} {secondBotWins} - Draw {draws}");
        }

        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine($"  TOTAL: FirstBot {totalFirstBotWins} - SecondBot {totalSecondBotWins} - Draw {totalDraws}");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
    }
}
