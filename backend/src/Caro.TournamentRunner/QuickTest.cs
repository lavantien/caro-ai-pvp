using Caro.Core.Entities;
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
        var engine = new TournamentEngine();
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
                maxMoves: 225,
                initialTimeSeconds: 180,  // 3+2 time control (faster iteration)
                incrementSeconds: 2,
                ponderingEnabled: true,
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

        // TEST 1: D11 vs D11 - Verify both can reach full depth
        Console.WriteLine("\n📊 TEST GROUP 1: MAXIMUM DEPTH VERIFICATION (D11 vs D11)");
        RunMatchup(AIDifficulty.Legend, AIDifficulty.Legend, games: 1);

        // TEST 2: D11 vs D10 - Verify both can reach full depth
        Console.WriteLine("\n📊 TEST GROUP 2: FULL DEPTH VERIFICATION (D11 vs D10)");
        RunMatchup(AIDifficulty.Legend, AIDifficulty.Grandmaster, games: 1);

        // TEST 3: D10 vs D8 - Both high difficulty levels
        Console.WriteLine("\n📊 TEST GROUP 3: HIGH DIFFICULTY (D10 vs D8)");
        RunMatchup(AIDifficulty.Grandmaster, AIDifficulty.Expert, games: 1);

        // TEST 4: D11 vs D6 - Legend vs mid-level
        Console.WriteLine("\n📊 TEST GROUP 4: VERY HIGH vs MID (D11 vs D6)");
        RunMatchup(AIDifficulty.Legend, AIDifficulty.Harder, games: 1);

        // TEST 5: Both use SEQUENTIAL search (D4 vs D6)
        Console.WriteLine("\n📊 TEST GROUP 5: SEQUENTIAL SEARCH (D4 vs D6)");
        RunMatchup(AIDifficulty.Medium, AIDifficulty.Harder, games: 1);

        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  ALL TESTS COMPLETE");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();
    }
}
