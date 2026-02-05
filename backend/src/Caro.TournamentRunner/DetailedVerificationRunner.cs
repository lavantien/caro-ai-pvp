using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Detailed verification runner with per-move diagnostics
/// Shows: time taken, threads, nodes, NPS, table hit rate
/// </summary>
public class DetailedVerificationRunner
{
    public static async Task RunAsync(int initialSeconds = 420, int incrementSeconds = 5, int gamesPerMatchup = 10)
    {
        var engine = TournamentEngine.CreateDefault();
        var tcName = $"{initialSeconds / 60}+{incrementSeconds}";

        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║     DETAILED VERIFICATION: {tcName} Time Control                    ║");
        Console.WriteLine($"║     Games per matchup: {gamesPerMatchup} (alternating colors)              ║");
        Console.WriteLine($"║     Parallel: ENABLED | Pondering: ENABLED                          ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        // All specified matchups
        var matchups = new[]
        {
            (Red: AIDifficulty.Grandmaster, Blue: AIDifficulty.Medium, Name: "Grandmaster vs Medium"),
            (Red: AIDifficulty.Grandmaster, Blue: AIDifficulty.Hard, Name: "Grandmaster vs Hard"),
            (Red: AIDifficulty.Grandmaster, Blue: AIDifficulty.Grandmaster, Name: "Grandmaster vs Grandmaster"),
            (Red: AIDifficulty.Hard, Blue: AIDifficulty.Easy, Name: "Hard vs Easy"),
            (Red: AIDifficulty.Hard, Blue: AIDifficulty.Medium, Name: "Hard vs Medium"),
            (Red: AIDifficulty.Hard, Blue: AIDifficulty.Hard, Name: "Hard vs Hard"),
            (Red: AIDifficulty.Medium, Blue: AIDifficulty.Braindead, Name: "Medium vs Braindead"),
            (Red: AIDifficulty.Medium, Blue: AIDifficulty.Easy, Name: "Medium vs Easy"),
            (Red: AIDifficulty.Medium, Blue: AIDifficulty.Medium, Name: "Medium vs Medium"),
            (Red: AIDifficulty.Easy, Blue: AIDifficulty.Braindead, Name: "Easy vs Braindead"),
            (Red: AIDifficulty.Easy, Blue: AIDifficulty.Easy, Name: "Easy vs Easy"),
            (Red: AIDifficulty.Braindead, Blue: AIDifficulty.Braindead, Name: "Braindead vs Braindead"),
        };

        foreach (var (redDiff, blueDiff, name) in matchups)
        {
            await RunMatchup(engine, redDiff, blueDiff, name, gamesPerMatchup, initialSeconds, incrementSeconds);
        }

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  VERIFICATION COMPLETE");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
    }

    private static async Task RunMatchup(
        TournamentEngine engine,
        AIDifficulty redDiff,
        AIDifficulty blueDiff,
        string matchupName,
        int gamesPerMatchup,
        int initialTimeSeconds,
        int incrementSeconds)
    {
        var redWins = 0;
        var blueWins = 0;
        var draws = 0;

        // Track cumulative stats
        long totalRedNodes = 0;
        long totalBlueNodes = 0;
        long totalRedTime = 0;
        long totalBlueTime = 0;
        int totalRedMoves = 0;
        int totalBlueMoves = 0;

        Console.WriteLine($"╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║  {matchupName,-67} ║");
        Console.WriteLine($"╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        for (int game = 1; game <= gamesPerMatchup; game++)
        {
            // Alternate colors: odd games have colors swapped
            bool swapColors = (game % 2 == 1);
            var actualRed = swapColors ? blueDiff : redDiff;
            var actualBlue = swapColors ? redDiff : blueDiff;

            var redSwapName = swapColors ? $"({blueDiff})" : "";
            var blueSwapName = swapColors ? $"({redDiff})" : "";

            Console.WriteLine($"  Game {game}/{gamesPerMatchup}: Red={actualRed,-12}{redSwapName} Blue={actualBlue,-12}{blueSwapName}");

            var result = engine.RunGame(
                redDifficulty: actualRed,
                blueDifficulty: actualBlue,
                maxMoves: 361,
                initialTimeSeconds: initialTimeSeconds,
                incrementSeconds: incrementSeconds,
                ponderingEnabled: true,
                parallelSearchEnabled: true,
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    var diff = player == Player.Red ? actualRed : actualBlue;
                    var color = player == Player.Red ? "R" : "B";
                    var colorName = player == Player.Red ? "red" : "blue";
                    var moveTimeMs = stats?.MoveTimeMs ?? 0;
                    var pondering = stats?.PonderingActive ?? false ? "P" : "";

                    // Track cumulative stats
                    if (player == Player.Red)
                    {
                        totalRedNodes += stats?.NodesSearched ?? 0;
                        totalRedTime += moveTimeMs;
                        totalRedMoves++;
                    }
                    else
                    {
                        totalBlueNodes += stats?.NodesSearched ?? 0;
                        totalBlueTime += moveTimeMs;
                        totalBlueMoves++;
                    }

                    // Log each move with detailed diagnostics
                    var depth = stats?.DepthAchieved ?? 0;
                    var nodes = stats?.NodesSearched ?? 0;
                    var nps = stats?.NodesPerSecond ?? 0;
                    var threads = stats?.ThreadCount ?? 1;
                    var hitRate = stats?.TableHitRate ?? 0;
                    var masterTT = stats?.MasterTTPercent ?? 0;
                    var helperAvg = stats?.HelperAvgDepth ?? 0;
                    var allocatedMs = stats?.AllocatedTimeMs ?? moveTimeMs;
                    var vcfDepth = stats?.VCFDepthAchieved ?? 0;
                    var vcfNodes = stats?.VCFNodesSearched ?? 0;
                    var ponderNodes = stats?.PonderNodesSearched ?? 0;
                    var ponderNps = stats?.PonderNodesPerSecond ?? 0;

                    // Format time as h:mm:ss.mmm or just ms if short
                    string timeStr;
                    if (moveTimeMs >= 1000)
                    {
                        var moveSec = moveTimeMs / 1000.0;
                        timeStr = $"{moveSec:F3}s";
                    }
                    else
                    {
                        timeStr = $"{moveTimeMs}ms";
                    }
                    string allocStr;
                    if (allocatedMs >= 1000)
                    {
                        var allocSec = allocatedMs / 1000.0;
                        allocStr = $"{allocSec:F3}s";
                    }
                    else
                    {
                        allocStr = $"{allocatedMs}ms";
                    }

                    // Calculate total time for this player
                    var totalTime = player == Player.Red ? redTimeMs : blueTimeMs;
                    var totalSec = totalTime / 1000.0;
                    var min = (int)(totalSec / 60);
                    var remainingSec = totalSec % 60;
                    var ms = totalTime % 1000;
                    var timeDisplay = $"{min}:{remainingSec:00}:{ms:D3}";

                    // Build N: main/ponder string
                    string nStr, npsStr;
                    if (ponderNodes > 0)
                    {
                        nStr = $"{nodes:N0}/{ponderNodes:N0}";
                        npsStr = $"{nps:F0}/{ponderNps:F0}";
                    }
                    else
                    {
                        nStr = $"{nodes:N0}";
                        npsStr = $"{nps:F0}";
                    }

                    // Build VCF string
                    string vcfStr;
                    if (vcfDepth > 0 || vcfNodes > 0)
                    {
                        vcfStr = $"{vcfDepth}d/{vcfNodes:N0}n";
                    }
                    else
                    {
                        vcfStr = "-";
                    }

                    Console.WriteLine(
                        $"    M{moveNumber,2} | T: {timeDisplay} - {diff,-12}({colorName}): {color}({x},{y}) | " +
                        $"Time: {timeStr,-9}/{allocStr,-8} | " +
                        $"Thr: {threads,1} | " +
                        $"D: {depth,2} | " +
                        $"N: {nStr,13} | " +
                        $"NPS: {npsStr,12} | " +
                        $"TT: {hitRate,5:F1}% | " +
                        $"%M: {masterTT,5:F1}% | " +
                        $"HD: {helperAvg,4:F1} | " +
                        $"P: {pondering,-1} | " +
                        $"VCF: {vcfStr}");
                },
                onLog: (level, source, message) =>
                {
                    // Only show warnings and errors
                    if (level == "warn" || level == "error")
                    {
                        Console.WriteLine($"    [{level.ToUpper()}] {source}: {message}");
                    }
                });

            // Determine outcome
            if (result.IsDraw)
            {
                draws++;
                Console.WriteLine($"    -> DRAW");
            }
            else if (result.Winner == Player.Red)
            {
                if (swapColors)
                    blueWins++;
                else
                    redWins++;
                var winningDiff = swapColors ? blueDiff : redDiff;
                Console.WriteLine($"    -> {winningDiff} (Red) wins");
            }
            else
            {
                if (swapColors)
                    redWins++;
                else
                    blueWins++;
                var winningDiff = swapColors ? redDiff : blueDiff;
                Console.WriteLine($"    -> {winningDiff} (Blue) wins");
            }

            Console.WriteLine($"    Duration: {result.DurationMs / 1000:F1}s");
            Console.WriteLine();
        }

        // Summary for this matchup
        var total = redWins + blueWins + draws;
        var redWinRate = (double)redWins / total;
        var blueWinRate = (double)blueWins / total;

        // Calculate averages
        double avgRedNodes = totalRedMoves > 0 ? totalRedNodes / (double)totalRedMoves : 0;
        double avgBlueNodes = totalBlueMoves > 0 ? totalBlueNodes / (double)totalBlueMoves : 0;
        double avgRedTime = totalRedMoves > 0 ? totalRedTime / (double)totalRedMoves : 0;
        double avgBlueTime = totalBlueMoves > 0 ? totalBlueTime / (double)totalBlueMoves : 0;
        double avgRedNps = avgRedTime > 0 ? avgRedNodes * 1000 / avgRedTime : 0;
        double avgBlueNps = avgBlueTime > 0 ? avgBlueNodes * 1000 / avgBlueTime : 0;

        Console.WriteLine($"  ───────────────────────────────────────────────────────────────────");
        Console.WriteLine($"  MATCHUP RESULT: {redDiff} {redWins} - {blueWins} {blueDiff} - {draws} draws");
        Console.WriteLine($"  Win rates: {redDiff} {redWinRate:P1} | {blueDiff} {blueWinRate:P1}");
        Console.WriteLine();
        Console.WriteLine($"  AVG STATS ({redDiff} as Red):  Nodes: {avgRedNodes:N0} | Time: {avgRedTime:F0}ms | NPS: {avgRedNps:N0}");
        Console.WriteLine($"  AVG STATS ({blueDiff} as Blue): Nodes: {avgBlueNodes:N0} | Time: {avgBlueTime:F0}ms | NPS: {avgBlueNps:N0}");

        // Expected: higher difficulty wins more
        AIDifficulty? expectedWinner = null;
        if (redDiff > blueDiff) expectedWinner = redDiff;
        else if (blueDiff > redDiff) expectedWinner = blueDiff;

        AIDifficulty? actualWinner = null;
        if (redWins > blueWins) actualWinner = redDiff;
        else if (blueWins > redWins) actualWinner = blueDiff;

        if (expectedWinner.HasValue)
        {
            var passed = actualWinner.HasValue && actualWinner.Value == expectedWinner.Value;
            Console.WriteLine($"  Status: {(passed ? "PASS" : "FAIL")} - Expected {expectedWinner.Value} to win more");
        }
        else
        {
            Console.WriteLine($"  Status: N/A - Same difficulty matchup");
        }

        Console.WriteLine();
        Console.WriteLine();
    }
}
