using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick test runner for isolated matchups with detailed logging
/// Use this to verify AI fixes without running full tournaments
/// </summary>
public static class QuickTest
{
    public static void RunMatchup(AIDifficulty redDiff, AIDifficulty blueDiff, int games = 5)
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();
        var results = new List<MatchResult>();

        Console.WriteLine();
        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║  QUICK TEST: {redDiff} (Red) vs {blueDiff} (Blue) - {games} games              ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        for (int i = 1; i <= games; i++)
        {
            Console.WriteLine($"--- Game {i}/{games} ---");

            var moves = new List<string>();
            var allMoves = new List<(int x, int y, Player player)>();

            var result = engine.RunGame(
                redDiff,
                blueDiff,
                maxMoves: 1024,
                initialTimeSeconds: 420,  // 7+5 time control (like before)
                incrementSeconds: 5,
                ponderingEnabled: false,  // Disabled: parallel search has bugs
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    var playerChar = player == Player.Red ? 'R' : 'B';
                    moves.Add($"M{moveNumber}:{playerChar}({x},{y})");
                    allMoves.Add((x, y, player));
                },
                onLog: (level, source, message) =>
                {
                    if (level == "warn" || level == "error")
                    {
                        Console.WriteLine($"  [{level.ToUpper()}] {source}: {message}");
                    }
                }
            );

            results.Add(result);

            // Show board positions for critical moves
            Console.WriteLine($"  Winner: {result.Winner} ({(result.Winner == Player.Red ? redDiff : blueDiff)}) in {result.TotalMoves} moves");
            Console.WriteLine($"  Duration: {result.DurationMs / 1000:F1}s");

            // Show last few moves
            if (moves.Count > 0)
            {
                var lastMoves = moves.TakeLast(Math.Min(8, moves.Count)).ToList();
                Console.WriteLine($"  Final moves: {string.Join(", ", lastMoves)}");
            }

            Console.WriteLine();
        }

        // Summary
        var redWins = results.Count(r => r.Winner == Player.Red);
        var blueWins = results.Count(r => r.Winner == Player.Blue);
        var draws = results.Count(r => r.IsDraw);

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine($"  RESULT: {redDiff} (Red) {redWins} - {draws} - {blueWins} {blueDiff} (Blue)");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");

        // Expected: Higher difficulty should win more
        var higherDiff = redDiff > blueDiff ? redDiff : blueDiff;
        var lowerDiff = redDiff < blueDiff ? redDiff : blueDiff;
        var higherWins = redDiff > blueDiff ? redWins : blueWins;
        var lowerWins = redDiff < blueDiff ? redWins : blueWins;

        if (higherWins > lowerWins)
        {
            Console.WriteLine($"  ✅ PASS: Higher difficulty ({higherDiff}) won {higherWins}/{games} games");
        }
        else if (lowerWins > higherWins)
        {
            Console.WriteLine($"  ❌ FAIL: Lower difficulty ({lowerDiff}) won {lowerWins}/{games} games - AI strength inversion detected!");
        }
        else
        {
            Console.WriteLine($"  ⚠️  TIED: Both difficulties won {higherWins} games");
        }
        Console.WriteLine();
    }

    public static void RunAllTests()
    {
        Console.WriteLine();
        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine("║           AI STRENGTH VERIFICATION - QUICK TEST SUITE              ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");

        // TEST 1: D5 vs D5 - Verify both can reach full depth
        Console.WriteLine("\n📊 TEST GROUP 1: MAXIMUM DEPTH VERIFICATION (Grandmaster vs Grandmaster)");
        RunMatchup(AIDifficulty.Grandmaster, AIDifficulty.Grandmaster, games: 1);

        // TEST 1.5: D4 vs D4 - Check for Blue advantage (symmetric test)
        Console.WriteLine("\n📊 TEST GROUP 1.5: BLUE ADVANTAGE CHECK (Hard vs Hard)");
        RunMatchup(AIDifficulty.Hard, AIDifficulty.Hard, games: 1);

        // TEST 2: D5 vs D4 - Verify strength ordering
        Console.WriteLine("\n📊 TEST GROUP 2: FULL DEPTH VERIFICATION (Grandmaster vs Hard)");
        RunMatchup(AIDifficulty.Grandmaster, AIDifficulty.Hard, games: 3);

        // TEST 3: D4 vs D3 - Both good difficulty levels
        Console.WriteLine("\n📊 TEST GROUP 3: HIGH DIFFICULTY (Hard vs Medium)");
        RunMatchup(AIDifficulty.Hard, AIDifficulty.Medium, games: 1);

        // TEST 4: D5 vs D2 - Grandmaster vs low-level
        Console.WriteLine("\n📊 TEST GROUP 4: VERY HIGH vs LOW (Grandmaster vs Easy)");
        RunMatchup(AIDifficulty.Grandmaster, AIDifficulty.Easy, games: 1);

        // TEST 5: Both use SEQUENTIAL search (Easy vs Medium)
        Console.WriteLine("\n📊 TEST GROUP 5: SEQUENTIAL SEARCH (Easy vs Medium)");
        RunMatchup(AIDifficulty.Easy, AIDifficulty.Medium, games: 1);

        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  ALL TESTS COMPLETE");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();
    }
}
