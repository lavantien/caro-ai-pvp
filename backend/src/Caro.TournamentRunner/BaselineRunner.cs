using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Comprehensive baseline test for all AI difficulties with detailed logging
/// </summary>
public class BaselineRunner
{
    public static async Task RunAsync(int initialSeconds, int incrementSeconds, int gamesPerMatchup = 10)
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();
        var tcName = $"{initialSeconds / 60}+{incrementSeconds}";

        Console.WriteLine($"╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║     COMPREHENSIVE AI BASELINE: {tcName} Time Control                  ║");
        Console.WriteLine($"║     Games per matchup: {gamesPerMatchup} (alternating colors)              ║");
        Console.WriteLine($"╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        // All difficulty matchups from lowest to highest
        var allDifficulties = new[]
        {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Medium,
            AIDifficulty.Hard,
            AIDifficulty.Grandmaster
        };

        // Test each adjacent pair
        for (int i = 0; i < allDifficulties.Length - 1; i++)
        {
            var lowerDiff = allDifficulties[i];
            var higherDiff = allDifficulties[i + 1];

            await RunMatchup(engine, lowerDiff, higherDiff, gamesPerMatchup, initialSeconds, incrementSeconds);
        }

        // Also test Grandmaster vs Medium for wider gap
        await RunMatchup(engine, AIDifficulty.Medium, AIDifficulty.Grandmaster, gamesPerMatchup, initialSeconds, incrementSeconds);

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  BASELINE TEST COMPLETE");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
    }

    private static async Task RunMatchup(
        TournamentEngine engine,
        AIDifficulty lowerDiff,
        AIDifficulty higherDiff,
        int gamesPerMatchup,
        int initialTimeSeconds,
        int incrementSeconds)
    {
        var higherWins = 0;
        var lowerWins = 0;
        var draws = 0;

        var matchupName = $"{higherDiff} vs {lowerDiff}";

        Console.WriteLine($"╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║  {matchupName,-67} ║");
        Console.WriteLine($"╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        for (int game = 1; game <= gamesPerMatchup; game++)
        {
            // Alternate colors: odd games have colors swapped
            bool swapColors = (game % 2 == 1);
            var redDiff = swapColors ? lowerDiff : higherDiff;
            var blueDiff = swapColors ? higherDiff : lowerDiff;

            Console.WriteLine($"  Game {game}/{gamesPerMatchup}: Red={redDiff,-12} Blue={blueDiff,-12} {(swapColors ? "(swapped)" : "")}");

            // Track move count and final result
            int totalMoves = 0;
            Player? winner = null;
            int winningMove = 0;

            var result = engine.RunGame(
                redDifficulty: redDiff,
                blueDifficulty: blueDiff,
                maxMoves: 1024,
                initialTimeSeconds: initialTimeSeconds,
                incrementSeconds: incrementSeconds,
                ponderingEnabled: false,
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    var diff = player == Player.Red ? redDiff : blueDiff;
                    var color = player == Player.Red ? "R" : "B";
                    var moveTimeMs = stats?.MoveTimeMs ?? 0;

                    // Log every 5th move and final moves to reduce noise
                    if (moveNumber % 5 == 0 || moveNumber < 10)
                    {
                        var depthInfo = stats != null ? $" D{stats.DepthAchieved}" : "";
                        var npsInfo = stats != null ? $" {stats.NodesPerSecond:F0}nps" : "";
                        Console.WriteLine($"    M{moveNumber,3}: {color}({x},{y}) by {diff,-12} T:{moveTimeMs / 1000:F1}s{depthInfo}{npsInfo}");
                    }

                    totalMoves = moveNumber;
                },
                onLog: (level, source, message) =>
                {
                    // Only show warnings and errors
                    if (level == "warn" || level == "error")
                    {
                        Console.WriteLine($"    [{level.ToUpper()}] {source}: {message}");
                    }
                });

            winner = result.Winner;
            winningMove = totalMoves;

            // Determine which difficulty won
            if (result.IsDraw)
            {
                draws++;
                Console.WriteLine($"    → DRAW after {totalMoves} moves");
            }
            else if (winner == Player.Red)
            {
                if (swapColors)
                {
                    lowerWins++;
                    Console.WriteLine($"    → {lowerDiff} (as Blue) wins on move {winningMove}");
                }
                else
                {
                    higherWins++;
                    Console.WriteLine($"    → {higherDiff} (as Red) wins on move {winningMove}");
                }
            }
            else // Blue won
            {
                if (swapColors)
                {
                    higherWins++;
                    Console.WriteLine($"    → {higherDiff} (as Blue) wins on move {winningMove}");
                }
                else
                {
                    lowerWins++;
                    Console.WriteLine($"    → {lowerDiff} (as Red) wins on move {winningMove}");
                }
            }

            Console.WriteLine($"    Duration: {result.DurationMs / 1000:F1}s | Timeout: {result.Winner == Player.None && !result.IsDraw}");
            Console.WriteLine();
        }

        // Summary
        var total = higherWins + lowerWins + draws;
        var higherWinRate = (double)higherWins / total;
        var lowerWinRate = (double)lowerWins / total;

        Console.WriteLine($"  ───────────────────────────────────────────────────────────────────");
        Console.WriteLine($"  SUMMARY: {higherDiff} {higherWins} - {lowerWins} {lowerDiff} - {draws} draws");
        Console.WriteLine($"  Win rates: {higherDiff} {higherWinRate:P1} | {lowerDiff} {lowerWinRate:P1}");

        var expectedWinner = higherDiff;
        AIDifficulty? actualWinner = higherWins > lowerWins ? higherDiff : (lowerWins > higherWins ? lowerDiff : null);
        var passed = actualWinner.HasValue && actualWinner.Value == expectedWinner;

        Console.WriteLine($"  Status: {(passed ? "✅ PASS" : "❌ FAIL")} - Expected {expectedWinner} to win more");

        if (!passed)
        {
            if (actualWinner == null)
            {
                Console.WriteLine($"  Note: All games drawn - higher difficulty should be more aggressive");
            }
            else
            {
                Console.WriteLine($"  WARNING: Lower difficulty ({actualWinner}) won more games!");
            }
        }

        Console.WriteLine();
        Console.WriteLine();
    }
}
