using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Options for configuring the comprehensive matchup runner
/// </summary>
public class ComprehensiveOptions
{
    /// <summary>
    /// Specific matchups to run. If empty, runs all standard matchups.
    /// </summary>
    public List<(AIDifficulty First, AIDifficulty Second)> Matchups { get; set; } = new();

    /// <summary>
    /// Time control: initial time in seconds
    /// </summary>
    public int InitialTimeSeconds { get; set; } = 180;

    /// <summary>
    /// Time control: increment per move in seconds
    /// </summary>
    public int IncrementSeconds { get; set; } = 2;

    /// <summary>
    /// Number of games per matchup
    /// </summary>
    public int GamesPerMatchup { get; set; } = 20;

    /// <summary>
    /// Enable pondering (thinking on opponent's time)
    /// </summary>
    public bool EnablePondering { get; set; } = true;

    /// <summary>
    /// Enable parallel search (Lazy SMP)
    /// </summary>
    public bool EnableParallel { get; set; } = true;
}

/// <summary>
/// Comprehensive matchup runner with 20 matchups (10 GamesPerMatchup each)
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
        public int InitialTimeSeconds { get; set; } = 180;
        public int IncrementSeconds { get; set; } = 2;
        public bool EnablePondering { get; set; } = true;
        public bool EnableParallel { get; set; } = true;
        public string? Description { get; set; }
        public bool IsExperimental { get; set; } = false;
    }

    private static void LogWrite(string? message = null)
    {
        Console.WriteLine(message);
    }

    public static async Task RunAsync()
    {
        await RunAsyncInternal(null);
    }

    public static async Task RunAsync(ComprehensiveOptions? options)
    {
        await RunAsyncInternal(options);
    }

    private const int TimeSeconds = 180;
    private const int IncSeconds = 2;
    private const int GamesPerMatchupDefault = 20;

    private static async Task RunAsyncInternal(ComprehensiveOptions? options)
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();

        // Use options or defaults
        var timeSeconds = options?.InitialTimeSeconds ?? TimeSeconds;
        var incSeconds = options?.IncrementSeconds ?? IncSeconds;
        var gamesPerMatchup = options?.GamesPerMatchup ?? GamesPerMatchupDefault;
        var enablePondering = options?.EnablePondering ?? true;
        var enableParallel = options?.EnableParallel ?? true;
        var tcName = $"{timeSeconds}+{incSeconds}";

        LogWrite($"═══════════════════════════════════════════════════════════════════");
        LogWrite($"  COMPREHENSIVE MATCHUP RUNNER: {tcName} Time Control");
        LogWrite($"  {gamesPerMatchup} games per matchup (alternating colors)");
        LogWrite($"  Parallel: {(enableParallel ? "ON" : "OFF")} | Pondering: {(enablePondering ? "ON" : "OFF")}");
        LogWrite($"═══════════════════════════════════════════════════════════════════");
        LogWrite();

        // Use custom matchups from options, or generate default set
        var matchups = options?.Matchups.Count > 0
            ? GenerateCustomMatchups(options.Matchups, gamesPerMatchup, timeSeconds, incSeconds, enablePondering, enableParallel)
            : GenerateMatchups();

        // Group by standard vs experimental
        var standardMatchups = matchups.Where(m => !m.IsExperimental).ToList();
        var experimentalMatchups = matchups.Where(m => m.IsExperimental).ToList();

        LogWrite($"Standard matchups: {standardMatchups.Count}");
        LogWrite($"Experimental matchups: {experimentalMatchups.Count}");
        LogWrite($"Total matchups: {matchups.Count}");
        LogWrite($"Total GamesPerMatchup: {matchups.Sum(m => m.Games)}");
        LogWrite();

        var results = new List<MatchupResult>();

        // Run standard matchups
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite("  STANDARD MATCHUPS");
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite();

        foreach (var matchup in standardMatchups)
        {
            var result = await RunMatchup(engine, matchup);
            results.Add(result);
        }

        // Run experimental matchups
        LogWrite();
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite("  EXPERIMENTAL MATCHUPS");
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite();

        foreach (var matchup in experimentalMatchups)
        {
            var result = await RunMatchup(engine, matchup);
            results.Add(result);
        }

        // Final summary
        LogWrite();
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite("  FINAL SUMMARY");
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite();

        PrintFinalSummary(results);
    }

    /// <summary>
    /// Generate custom matchups from options
    /// </summary>
    private static List<MatchupConfig> GenerateCustomMatchups(
        List<(AIDifficulty First, AIDifficulty Second)> matchups,
        int games,
        int timeSeconds,
        int incSeconds,
        bool enablePondering,
        bool enableParallel)
    {
        var configs = new List<MatchupConfig>();
        int index = 1;

        foreach (var (first, second) in matchups)
        {
            configs.Add(new MatchupConfig
            {
                RedDifficulty = first,
                BlueDifficulty = second,
                Games = games,
                InitialTimeSeconds = timeSeconds,
                IncrementSeconds = incSeconds,
                EnablePondering = enablePondering,
                EnableParallel = enableParallel,
                Description = $"{index}. {first} vs {second}",
                IsExperimental = false
            });
            index++;
        }

        return configs;
    }

    private static List<MatchupConfig> GenerateMatchups()
    {
        var matchups = new List<MatchupConfig>();

        // Standard matchups (15)
        // 1-4. Grandmaster vs all
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "1. Grandmaster vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Easy,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "2. Grandmaster vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Medium,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "3. Grandmaster vs Medium"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Hard,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "4. Grandmaster vs Hard"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Grandmaster,
            BlueDifficulty = AIDifficulty.Grandmaster,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "5. Grandmaster vs Grandmaster"
        });

        // 6-9. Hard vs all (except Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "6. Hard vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Easy,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "7. Hard vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Medium,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "8. Hard vs Medium"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Hard,
            BlueDifficulty = AIDifficulty.Hard,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = true,
            Description = "9. Hard vs Hard"
        });

        // 10-12. Medium vs all (except Hard, Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = false,
            Description = "10. Medium vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Easy,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = false,
            Description = "11. Medium vs Easy"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Medium,
            BlueDifficulty = AIDifficulty.Medium,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = true,
            EnableParallel = false,
            Description = "12. Medium vs Medium"
        });

        // 13-14. Easy vs all (except Medium, Hard, Grandmaster)
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Easy,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = false,
            EnableParallel = false,
            Description = "13. Easy vs Braindead"
        });

        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Easy,
            BlueDifficulty = AIDifficulty.Easy,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
            EnablePondering = false,
            EnableParallel = false,
            Description = "14. Easy vs Easy"
        });

        // 15. Braindead vs Braindead
        matchups.Add(new MatchupConfig
        {
            RedDifficulty = AIDifficulty.Braindead,
            BlueDifficulty = AIDifficulty.Braindead,
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
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
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds / 2,
            IncrementSeconds = IncSeconds / 2,
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
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds * 2,
            IncrementSeconds = IncSeconds * 2,
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
            Games = GamesPerMatchupDefault,
            InitialTimeSeconds = TimeSeconds,
            IncrementSeconds = IncSeconds,
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
            Games = GamesPerMatchupDefault,
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
            Games = GamesPerMatchupDefault / 2, // Fewer games due to longer time
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

        LogWrite($"┌─────────────────────────────────────────────────────────────────────┐");
        LogWrite($"│ {config.Description,-71} │");
        LogWrite($"├─────────────────────────────────────────────────────────────────────┤");
        LogWrite($"│ Time: {config.InitialTimeSeconds}+{config.IncrementSeconds} | Pondering: {(config.EnablePondering ? "ON" : "OFF"),3} | Parallel: {(config.EnableParallel ? "ON" : "OFF"),3} │");
        LogWrite($"└─────────────────────────────────────────────────────────────────────┘");
        LogWrite();

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
                maxMoves: 1024,
                initialTimeSeconds: config.InitialTimeSeconds,
                incrementSeconds: config.IncrementSeconds,
                ponderingEnabled: config.EnablePondering,
                parallelSearchEnabled: config.EnableParallel,
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    var diff = player == Player.Red ? gameRedDiff : gameBlueDiff;
                    LogWrite(GameStatsFormatter.FormatMoveLine(game, moveNumber, x, y, player, diff, stats));
                    moveCount = moveNumber;
                },
                onLog: (level, source, message) =>
                {
                    if (level == "warn" || level == "error")
                    {
                        LogWrite($"    [{level.ToUpper()}] {source}: {message}");
                    }
                });

            var gameDurationMs = DateTime.UtcNow.Ticks / 10000 - gameStartMs;
            totalMoves += moveCount;
            totalTimeMs += gameDurationMs;

            // Determine winner
            if (result.IsDraw)
            {
                draws++;
                LogWrite($"    → Game {game}: DRAW after {moveCount} moves ({gameDurationMs / 1000:F1}s)");
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
                LogWrite($"    → Game {game}: {whichDiff} ({winningColor}) wins on move {moveCount} ({gameDurationMs / 1000:F1}s)");
            }

            LogWrite();
        }

        // Summary for this matchup
        var total = redWins + blueWins + draws;
        var redWinRate = (double)redWins / total;
        var blueWinRate = (double)blueWins / total;

        LogWrite($"  ───────────────────────────────────────────────────────────────────");
        LogWrite($"  SUMMARY: {redDiff} {redWins} - {blueWins} {blueDiff} - {draws} draws");
        LogWrite($"  Win rates: {redDiff} {redWinRate:P1} | {blueDiff} {blueWinRate:P1}");
        LogWrite($"  Avg moves: {(double)totalMoves / total:F1} | Avg time: {totalTimeMs / total / 1000:F1}s/game");
        LogWrite();
        LogWrite();

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
        LogWrite("╔═══════════════════════════════════════════════════════════════════╗");
        LogWrite("║                       MATCHUP RESULTS                             ║");
        LogWrite("╚═══════════════════════════════════════════════════════════════════╝");
        LogWrite();

        LogWrite("┌─────────────────────────────────┬──────┬──────┬──────┬────────┬────────┐");
        LogWrite("│ Matchup                         │ 1st  │ 2nd  │ Draw │  Avg   │  Avg   │");
        LogWrite("│                                 │ Wins │ Wins │      │ Moves  │ Time(s)│");
        LogWrite("├─────────────────────────────────┼──────┼──────┼──────┼────────┼────────┤");

        foreach (var result in results)
        {
            var matchupName = $"{result.RedDifficulty} vs {result.BlueDifficulty}";
            var paddedName = matchupName.Length > 31 ? matchupName[..31] : matchupName.PadRight(31);

            LogWrite($"│ {paddedName} │ {result.RedWins,4} │ {result.BlueWins,4} │ {result.Draws,4} │ {result.AverageMoves,6:F1} │ {result.AverageTimeSeconds,6:F1} │");
        }

        LogWrite("└─────────────────────────────────┴──────┴──────┴──────┴────────┴────────┘");
        LogWrite();

        // Calculate overall stats by difficulty
        var allDifficulties = new[] {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Medium,
            AIDifficulty.Hard,
            AIDifficulty.Grandmaster
        };

        LogWrite("╔═══════════════════════════════════════════════════════════════════╗");
        LogWrite("║                      PERFORMANCE BY DIFFICULTY                   ║");
        LogWrite("╚═══════════════════════════════════════════════════════════════════╝");
        LogWrite();

        LogWrite("┌───────────────┬────────┬────────┬────────┬────────┐");
        LogWrite("│ Difficulty    │  Wins  │ Losses │  Draws │  Win % │");
        LogWrite("├───────────────┼────────┼────────┼────────┼────────┤");

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

            LogWrite($"│ {diff,-13} │ {wins,6} │ {losses,6} │ {draws,6} │ {winRate,6:P1} │");
        }

        LogWrite("└───────────────┴────────┴────────┴────────┴────────┘");
        LogWrite();
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
