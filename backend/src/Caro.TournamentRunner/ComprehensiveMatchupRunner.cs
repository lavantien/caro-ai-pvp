using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Comprehensive matchup runner with 20 matchups (10 games each)
/// 15 standard matchups + 5 experimental matchups
/// All with alternating colors, full parallel, full pondering
/// </summary>
public class ComprehensiveMatchupRunner
{
    public class MatchupConfig
    {
        public AIDifficulty RedDifficulty { get; set; }
        public AIDifficulty BlueDifficulty { get; set; }
        public int Games { get; set; } = 10;
        public int InitialTimeSeconds { get; set; } = 420;
        public int IncrementSeconds { get; set; } = 5;
        public bool EnablePondering { get; set; } = true;
        public bool EnableParallel { get; set; } = true;
        public string? Description { get; set; }
        public bool IsExperimental { get; set; } = false;
    }

    public static async Task RunAsync(int timeSeconds = 420, int incSeconds = 5, int gamesPerMatchup = 10)
    {
        var engine = new TournamentEngine();
        var tcName = $"{timeSeconds / 60}+{incSeconds}";

        Console.WriteLine($"╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine($"║     COMPREHENSIVE MATCHUP RUNNER: {tcName} Time Control               ║");
        Console.WriteLine($"║     {gamesPerMatchup} games per matchup (alternating colors)              ║");
        Console.WriteLine($"║     Full parallel search, Full pondering                             ║");
        Console.WriteLine($"╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        var matchups = GenerateMatchups(gamesPerMatchup, timeSeconds, incSeconds);

        // Group by standard vs experimental
        var standardMatchups = matchups.Where(m => !m.IsExperimental).ToList();
        var experimentalMatchups = matchups.Where(m => m.IsExperimental).ToList();

        Console.WriteLine($"Standard matchups: {standardMatchups.Count}");
        Console.WriteLine($"Experimental matchups: {experimentalMatchups.Count}");
        Console.WriteLine($"Total matchups: {matchups.Count}");
        Console.WriteLine($"Total games: {matchups.Sum(m => m.Games)}");
        Console.WriteLine();

        var results = new List<MatchupResult>();

        // Run standard matchups
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  STANDARD MATCHUPS");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        foreach (var matchup in standardMatchups)
        {
            var result = await RunMatchup(engine, matchup);
            results.Add(result);
        }

        // Run experimental matchups
        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  EXPERIMENTAL MATCHUPS");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        foreach (var matchup in experimentalMatchups)
        {
            var result = await RunMatchup(engine, matchup);
            results.Add(result);
        }

        // Final summary
        Console.WriteLine();
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  FINAL SUMMARY");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        PrintFinalSummary(results);
    }

    private static List<MatchupConfig> GenerateMatchups(int games, int time, int inc)
    {
        var matchups = new List<MatchupConfig>();

        // Standard matchups (15)
        // 1-4. Grandmaster vs all
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "1. Grandmaster vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Easy,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "2. Grandmaster vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Medium,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "3. Grandmaster vs Medium"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Hard,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "4. Grandmaster vs Hard"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Grandmaster,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "5. Grandmaster vs Grandmaster"
        });

        // 6-9. Hard vs all (except Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "6. Hard vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Easy,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "7. Hard vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Medium,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "8. Hard vs Medium"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Hard,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = true,
            Description = "9. Hard vs Hard"
        });

        // 10-12. Medium vs all (except Hard, Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = false,
            Description = "10. Medium vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Easy,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = false,
            Description = "11. Medium vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Medium,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = true,
            EnableParallel = false,
            Description = "12. Medium vs Medium"
        });

        // 13-14. Easy vs all (except Medium, Hard, Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Easy,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = false,
            EnableParallel = false,
            Description = "13. Easy vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Easy,
            BlueDifficulty = AIDifficulty.Easy,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = false,
            EnableParallel = false,
            Description = "14. Easy vs Easy"
        });

        // 15. Braindead vs Braindead
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Braindead,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = false,
            EnableParallel = false,
            Description = "15. Braindead vs Braindead"
        });

        // Experimental matchups (16-20)
        // These can be customized for different AI configurations

        // 16. Grandmaster with reduced time (faster, more mistakes)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Hard,
            Games = games,
            InitialTimeSeconds = time / 2,
            IncrementSeconds = inc / 2,
            EnablePondering = true,
            EnableParallel = true,
            Description = "16. EXP: Grandmaster (half time) vs Hard",
            IsExperimental = true
        });

        // 17. Hard with extended time (deeper search)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Medium,
            Games = games,
            InitialTimeSeconds = time * 2,
            IncrementSeconds = inc * 2,
            EnablePondering = true,
            EnableParallel = true,
            Description = "17. EXP: Hard (double time) vs Medium",
            IsExperimental = true
        });

        // 18. Medium with pondering disabled (baseline comparison)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Medium,
            Games = games,
            InitialTimeSeconds = time,
            IncrementSeconds = inc,
            EnablePondering = false,
            EnableParallel = false,
            Description = "18. EXP: Medium vs Medium (no pondering)",
            IsExperimental = true
        });

        // 19. Short time control blitz
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Grandmaster,
            Games = games,
            InitialTimeSeconds = 60,
            IncrementSeconds = 1,
            EnablePondering = true,
            EnableParallel = true,
            Description = "19. EXP: Grandmaster vs Grandmaster (1+0 blitz)",
            IsExperimental = true
        });

        // 20. Long time control (maximum depth)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Hard,
            Games = games / 2, // Fewer games due to longer time
            InitialTimeSeconds = 1200, // 20 minutes
            IncrementSeconds = 10,
            EnablePondering = true,
            EnableParallel = true,
            Description = "20. EXP: Grandmaster vs Hard (20+10 long control)",
            IsExperimental = true
        });

        return matchups;
    }

    private static async Task<MatchupResult> RunMatchup(TournamentEngine engine, MatchupConfig config)
    {
        var redWins = 0;
        var blueWins = 0;
        var draws = 0;
        var totalMoves = 0;
        var totalTimeMs = 0L;

        var redDiff = config.RedDifficulty;
        var blueDiff = config.BlueDifficulty;

        Console.WriteLine($"┌─────────────────────────────────────────────────────────────────────┐");
        Console.WriteLine($"│ {config.Description,-71} │");
        Console.WriteLine($"├─────────────────────────────────────────────────────────────────────┤");
        Console.WriteLine($"│ Time: {config.InitialTimeSeconds / 60}+{config.IncrementSeconds} | Pondering: {(config.EnablePondering ? "ON" : "OFF"),3} | Parallel: {(config.EnableParallel ? "ON" : "OFF"),3} │");
        Console.WriteLine($"└─────────────────────────────────────────────────────────────────────┘");
        Console.WriteLine();

        for (int game = 1; game <= config.Games; game++)
        {
            // Alternate colors
            bool swapColors = (game % 2 == 1);
            var gameRedDiff = swapColors ? blueDiff : redDiff;
            var gameBlueDiff = swapColors ? redDiff : blueDiff;

            var moveCount = 0;
            var gameStartMs = DateTime.UtcNow.Ticks / 10000;

            var result = engine.RunGame(
                redDifficulty: gameRedDiff,
                blueDifficulty: gameBlueDiff,
                maxMoves: 361,
                initialTimeSeconds: config.InitialTimeSeconds,
                incrementSeconds: config.IncrementSeconds,
                ponderingEnabled: config.EnablePondering,
                parallelSearchEnabled: config.EnableParallel,
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    var diff = player == Player.Red ? gameRedDiff : gameBlueDiff;
                    var color = player == Player.Red ? "R" : "B";

                    // Format stats for one-line output
                    var timeStr = FormatTime(stats?.MoveTimeMs ?? 0);
                    var allocStr = FormatTime((stats?.AllocatedTimeMs ?? 0));
                    var depthStr = stats != null ? $"D{stats.DepthAchieved}" : "D-";
                    var nStr = stats != null ? FormatLargeNumber(stats.NodesSearched) : "N/A";
                    var npsStr = stats != null ? FormatLargeNumber((long)stats.NodesPerSecond) : "N/A";
                    var ttStr = stats != null ? $"{stats.TableHitRate:F1}% " : "N/A";
                    var masterStr = stats != null ? $"{stats.MasterTTPercent:F1}% " : "N/A";
                    var helperStr = stats != null ? $"{stats.HelperAvgDepth:F1}" : "N/A";
                    var ponderStr = stats?.PonderingActive == true ? "Y" : "N";
                    var threadsStr = stats?.ThreadCount.ToString() ?? "1";

                    // One-line aligned output
                    Console.WriteLine(
                        $"    G{game,2} M{moveNumber,3} | {color}({x},{y}) by {diff,-12} | " +
                        $"T: {timeStr,-9}/{allocStr,-8} | " +
                        $"Th: {threadsStr} | " +
                        $"{depthStr,-3} | " +
                        $"N: {nStr,13} | " +
                        $"NPS: {npsStr,12} | " +
                        $"TT: {ttStr,-5} | " +
                        $"%M: {masterStr,-5} | " +
                        $"HD: {helperStr,-4} | " +
                        $"P: {ponderStr}");

                    moveCount = moveNumber;
                },
                onLog: (level, source, message) =>
                {
                    if (level == "warn" || level == "error")
                    {
                        Console.WriteLine($"    [{level.ToUpper()}] {source}: {message}");
                    }
                });

            var gameDurationMs = DateTime.UtcNow.Ticks / 10000 - gameStartMs;
            totalMoves += moveCount;
            totalTimeMs += gameDurationMs;

            // Determine winner
            if (result.IsDraw)
            {
                draws++;
                Console.WriteLine($"    → Game {game}: DRAW after {moveCount} moves ({gameDurationMs / 1000:F1}s)");
            }
            else
            {
                var winner = result.Winner;
                var winningDiff = winner == Player.Red ? gameRedDiff : gameBlueDiff;
                var winningColor = winner == Player.Red ? "Red" : "Blue";

                if (winner == Player.Red)
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

                var whichDiff = (winner == Player.Red) == swapColors ? blueDiff : redDiff;
                Console.WriteLine($"    → Game {game}: {whichDiff} ({winningColor}) wins on move {moveCount} ({gameDurationMs / 1000:F1}s)");
            }

            Console.WriteLine();
        }

        // Summary for this matchup
        var total = redWins + blueWins + draws;
        var redWinRate = (double)redWins / total;
        var blueWinRate = (double)blueWins / total;

        Console.WriteLine($"  ───────────────────────────────────────────────────────────────────");
        Console.WriteLine($"  SUMMARY: {redDiff} {redWins} - {blueWins} {blueDiff} - {draws} draws");
        Console.WriteLine($"  Win rates: {redDiff} {redWinRate:P1} | {blueDiff} {blueWinRate:P1}");
        Console.WriteLine($"  Avg moves: {(double)totalMoves / total:F1} | Avg time: {totalTimeMs / total / 1000:F1}s/game");
        Console.WriteLine();
        Console.WriteLine();

        return new MatchupResult
        {
            RedDifficulty = redDiff,
            BlueDifficulty = blueDiff,
            RedWins = redWins,
            BlueWins = blueWins,
            Draws = draws,
            TotalGames = total,
            AverageMoves = (double)totalMoves / total,
            AverageTimeSeconds = totalTimeMs / total / 1000.0
        };
    }

    private static void PrintFinalSummary(List<MatchupResult> results)
    {
        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine("║                       MATCHUP RESULTS                             ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        Console.WriteLine("┌─────────────────────────────────┬──────┬──────┬──────┬────────┬────────┐");
        Console.WriteLine("│ Matchup                         │ Red  │ Blue │ Draw │  Avg   │  Avg   │");
        Console.WriteLine("│                                 │ Wins │ Wins │      │ Moves  │ Time(s)│");
        Console.WriteLine("├─────────────────────────────────┼──────┼──────┼──────┼────────┼────────┤");

        foreach (var result in results)
        {
            var matchupName = $"{result.RedDifficulty} vs {result.BlueDifficulty}";
            var paddedName = matchupName.Length > 31 ? matchupName[..31] : matchupName.PadRight(31);

            Console.WriteLine($"│ {paddedName} │ {result.RedWins,4} │ {result.BlueWins,4} │ {result.Draws,4} │ {result.AverageMoves,6:F1} │ {result.AverageTimeSeconds,6:F1} │");
        }

        Console.WriteLine("└─────────────────────────────────┴──────┴──────┴──────┴────────┴────────┘");
        Console.WriteLine();

        // Calculate overall stats by difficulty
        var allDifficulties = new[] {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Medium,
            AIDifficulty.Hard,
            AIDifficulty.Grandmaster
        };

        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine("║                      PERFORMANCE BY DIFFICULTY                   ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        Console.WriteLine("┌───────────────┬────────┬────────┬────────┬────────┐");
        Console.WriteLine("│ Difficulty    │  Wins  │ Losses │  Draws │  Win % │");
        Console.WriteLine("├───────────────┼────────┼────────┼────────┼────────┤");

        foreach (var diff in allDifficulties)
        {
            var wins = 0;
            var losses = 0;
            var draws = 0;

            foreach (var result in results)
            {
                if (result.RedDifficulty == diff)
                {
                    wins += result.RedWins;
                    losses += result.BlueWins;
                    draws += result.Draws;
                }
                if (result.BlueDifficulty == diff)
                {
                    wins += result.BlueWins;
                    losses += result.RedWins;
                    draws += result.Draws;
                }
            }

            var total = wins + losses + draws;
            var winRate = total > 0 ? (double)wins / total : 0;

            Console.WriteLine($"│ {diff,-13} │ {wins,6} │ {losses,6} │ {draws,6} │ {winRate,6:P1} │");
        }

        Console.WriteLine("└───────────────┴────────┴────────┴────────┴────────┘");
        Console.WriteLine();
    }

    private static string FormatTime(long ms)
    {
        if (ms < 1000)
            return $"{ms}ms";
        if (ms < 60000)
            return $"{ms / 1000.0:F1}s";
        return $"{ms / 60000.0:F1}m";
    }

    private static string FormatLargeNumber(long n)
    {
        if (n < 1_000)
            return n.ToString();
        if (n < 1_000_000)
            return $"{n / 1000.0:F1}K";
        if (n < 1_000_000_000)
            return $"{n / 1_000_000.0:F1}M";
        return $"{n / 1_000_000_000.0:F1}B";
    }

    public class MatchupResult
    {
        public AIDifficulty RedDifficulty { get; set; }
        public AIDifficulty BlueDifficulty { get; set; }
        public int RedWins { get; set; }
        public int BlueWins { get; set; }
        public int Draws { get; set; }
        public int TotalGames { get; set; }
        public double AverageMoves { get; set; }
        public double AverageTimeSeconds { get; set; }
    }
}
