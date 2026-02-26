using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using System.Text;

namespace Caro.TournamentRunner;

/// <summary>
/// Runs baseline benchmark suite with 12 standardized matchups (32 games each).
/// Generates comprehensive statistics with mode, median, and mean aggregation.
/// </summary>
public static class BaselineBenchmarkRunner
{
    // Short codes for difficulty names
    private static readonly Dictionary<AIDifficulty, string> DifficultyCodes = new()
    {
        { AIDifficulty.Braindead, "bd" },
        { AIDifficulty.Easy, "ez" },
        { AIDifficulty.Medium, "md" },
        { AIDifficulty.Hard, "hd" },
        { AIDifficulty.Grandmaster, "gm" }
    };

    // 12 standardized matchups: 6 difficulty pairs × 2 time controls
    private static readonly (AIDifficulty First, AIDifficulty Second, int Time, int Inc, string TimeControl)[] Matchups =
    {
        (AIDifficulty.Braindead, AIDifficulty.Easy, 60, 0, "bullet"),
        (AIDifficulty.Braindead, AIDifficulty.Easy, 180, 2, "blitz"),
        (AIDifficulty.Braindead, AIDifficulty.Medium, 60, 0, "bullet"),
        (AIDifficulty.Braindead, AIDifficulty.Medium, 180, 2, "blitz"),
        (AIDifficulty.Braindead, AIDifficulty.Grandmaster, 60, 0, "bullet"),
        (AIDifficulty.Braindead, AIDifficulty.Grandmaster, 180, 2, "blitz"),
        (AIDifficulty.Easy, AIDifficulty.Hard, 60, 0, "bullet"),
        (AIDifficulty.Easy, AIDifficulty.Hard, 180, 2, "blitz"),
        (AIDifficulty.Medium, AIDifficulty.Grandmaster, 60, 0, "bullet"),
        (AIDifficulty.Medium, AIDifficulty.Grandmaster, 180, 2, "blitz"),
        (AIDifficulty.Hard, AIDifficulty.Grandmaster, 60, 0, "bullet"),
        (AIDifficulty.Hard, AIDifficulty.Grandmaster, 180, 2, "blitz"),
    };

    private const int GamesPerMatchup = 32;

    public static async Task RunAsync()
    {
        var startTime = DateTime.UtcNow;
        var allResults = new List<(string FileName, BaselineStatistics Stats, string TimeControl)>();

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  BASELINE BENCHMARK SUITE");
        Console.WriteLine($"  Started: {startTime:yyyy-MM-dd HH:mm:ss}");
        Console.WriteLine($"  12 matchups × {GamesPerMatchup} games each = {Matchups.Length * GamesPerMatchup} total games");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine();

        int matchupIndex = 1;
        foreach (var (first, second, time, inc, timeControl) in Matchups)
        {
            var fileName = $"baseline_{timeControl}_{DifficultyCodes[first]}_{DifficultyCodes[second]}.txt";

            Console.WriteLine($"[{matchupIndex}/{Matchups.Length}] {first} vs {second} ({time}+{inc} {timeControl})");
            Console.WriteLine($"    Output: {fileName}");

            var stats = await RunMatchup(first, second, time, inc, fileName);

            allResults.Add((fileName, stats, timeControl));

            Console.WriteLine($"    → Higher wins: {stats.HigherDifficultyWins}, Lower wins: {stats.LowerDifficultyWins}, Draws: {stats.Draws}");
            Console.WriteLine();

            matchupIndex++;
        }

        // Generate final summary
        var summaryFileName = "baseline_summary.txt";
        await GenerateSummary(allResults, summaryFileName, startTime);

        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
        Console.WriteLine("  BASELINE BENCHMARK COMPLETE");
        Console.WriteLine($"  Duration: {(DateTime.UtcNow - startTime).TotalMinutes:F1} minutes");
        Console.WriteLine($"  Summary: {summaryFileName}");
        Console.WriteLine("═══════════════════════════════════════════════════════════════════");
    }

    private static async Task<BaselineStatistics> RunMatchup(
        AIDifficulty first,
        AIDifficulty second,
        int timeSeconds,
        int incSeconds,
        string outputFileName)
    {
        var aggregator = new BaselineStatisticsAggregator
        {
            HigherDifficulty = second, // Second is always higher in our matchups
            LowerDifficulty = first
        };

        var engine = TournamentEngineFactory.CreateWithOpeningBook();

        // Set up file output with console tee
        using var fileWriter = new StreamWriter(outputFileName, false, Encoding.UTF8);
        var originalOut = Console.Out;
        using var teeWriter = new TeeTextWriter(fileWriter, originalOut);
        Console.SetOut(teeWriter);

        try
        {
            Console.WriteLine("═══════════════════════════════════════════════════════════════════");
            Console.WriteLine($"  BASELINE BENCHMARK: {first} vs {second} ({timeSeconds}+{incSeconds})");
            Console.WriteLine($"  {GamesPerMatchup} games | Date: {DateTime.UtcNow:yyyy-MM-dd HH:mm:ss}");
            Console.WriteLine("═══════════════════════════════════════════════════════════════════");
            Console.WriteLine();

            for (int game = 1; game <= GamesPerMatchup; game++)
            {
                // Alternate colors
                bool swapColors = (game % 2 == 1);
                var redDiff = swapColors ? second : first;
                var blueDiff = swapColors ? first : second;

                var moveCount = 0;
                var gameStartMs = DateTime.UtcNow.Ticks / 10000;

                var result = engine.RunGame(
                    redDifficulty: redDiff,
                    blueDifficulty: blueDiff,
                    maxMoves: 1024,
                    initialTimeSeconds: timeSeconds,
                    incrementSeconds: incSeconds,
                    ponderingEnabled: true,
                    parallelSearchEnabled: true,
                    onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                    {
                        var diff = player == Player.Red ? redDiff : blueDiff;
                        Console.WriteLine(GameStatsFormatter.FormatMoveLine(game, moveNumber, x, y, player, diff, stats));
                        if (stats != null)
                            aggregator.RecordMove((MoveStats)stats, diff);
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

                // Determine winner
                AIDifficulty? winnerDiff = null;
                if (result.IsDraw)
                {
                    Console.WriteLine($"    → Game {game}: DRAW after {moveCount} moves ({gameDurationMs / 1000:F1}s)");
                }
                else
                {
                    var winner = result.Winner;
                    winnerDiff = winner == Player.Red ? redDiff : blueDiff;
                    var winningColor = winner == Player.Red ? "Red" : "Blue";
                    Console.WriteLine($"    → Game {game}: {winnerDiff} ({winningColor}) wins on move {moveCount} ({gameDurationMs / 1000:F1}s)");
                }

                aggregator.RecordGameResult(winnerDiff ?? first, moveCount, result.IsDraw);
                Console.WriteLine();
            }

            var stats = aggregator.GetStatistics();
            PrintMatchupStatistics(stats);

            return stats;
        }
        finally
        {
            Console.SetOut(originalOut);
        }
    }

    private static void PrintMatchupStatistics(BaselineStatistics stats)
    {
        Console.WriteLine("───────────────────────────────────────────────────────────────────");
        Console.WriteLine("  MATCHUP STATISTICS");
        Console.WriteLine("───────────────────────────────────────────────────────────────────");
        Console.WriteLine();

        Console.WriteLine("DISCRETE METRICS (Mode meaningful):");
        Console.WriteLine("| Metric       | Mode | Median | Mean   |");
        Console.WriteLine("|--------------|------|--------|--------|");
        Console.WriteLine($"| Move Count   | {stats.MoveCount.Mode,4:F0} | {stats.MoveCount.Median,6:F1} | {stats.MoveCount.Mean,6:F1} |");
        Console.WriteLine($"| Master Depth | {stats.MasterDepth.Mode,4:F0} | {stats.MasterDepth.Median,6:F1} | {stats.MasterDepth.Mean,6:F1} |");
        Console.WriteLine($"| FMC (%)      | {stats.FirstMoveCutoffPercent.Mode,4:F0} | {stats.FirstMoveCutoffPercent.Median,6:F1} | {stats.FirstMoveCutoffPercent.Mean,6:F1} |");
        Console.WriteLine();

        Console.WriteLine("CONTINUOUS METRICS (Median/Mean only):");
        Console.WriteLine("| Metric           | Median  | Mean    |");
        Console.WriteLine("|------------------|---------|---------|");
        Console.WriteLine($"| NPS              | {FormatNPS(stats.NPS.Median),7} | {FormatNPS(stats.NPS.Mean),7} |");
        Console.WriteLine($"| Helper Avg Depth | {stats.HelperAvgDepth.Median,7:F1} | {stats.HelperAvgDepth.Mean,7:F1} |");
        Console.WriteLine($"| Time Used (ms)   | {FormatTime(stats.TimeUsedMs.Median),7} | {FormatTime(stats.TimeUsedMs.Mean),7} |");
        Console.WriteLine($"| Time Alloc (ms)  | {FormatTime(stats.TimeAllocatedMs.Median),7} | {FormatTime(stats.TimeAllocatedMs.Mean),7} |");
        Console.WriteLine($"| TT Hit Rate (%)  | {stats.TTHitRate.Median,7:F1} | {stats.TTHitRate.Mean,7:F1} |");
        Console.WriteLine($"| EBF              | {stats.EffectiveBranchingFactor.Median,7:F1} | {stats.EffectiveBranchingFactor.Mean,7:F1} |");
        Console.WriteLine();

        if (stats.VCFTriggers.Count > 0)
        {
            Console.WriteLine($"VCF TRIGGERS ({stats.VCFTriggers.Count} total):");
            foreach (var trigger in stats.VCFTriggers.Take(10))
            {
                Console.WriteLine($"  Game {trigger.Game}, Move {trigger.Move}: depth={trigger.Depth}, nodes={FormatLargeNumber(trigger.Nodes)}");
            }
            if (stats.VCFTriggers.Count > 10)
                Console.WriteLine($"  ... and {stats.VCFTriggers.Count - 10} more");
            Console.WriteLine();
        }

        if (stats.MoveTypeDistribution.Count > 0)
        {
            Console.WriteLine("MOVE TYPE FREQUENCIES:");
            foreach (var (moveType, (count, percentage)) in stats.MoveTypeDistribution.OrderByDescending(x => x.Value.Count))
            {
                Console.WriteLine($"  {moveType}: {count} ({percentage:F1}%)");
            }
        }
    }

    private static async Task GenerateSummary(
        List<(string FileName, BaselineStatistics Stats, string TimeControl)> results,
        string summaryFileName,
        DateTime startTime)
    {
        using var writer = new StreamWriter(summaryFileName, false, Encoding.UTF8);

        await writer.WriteLineAsync("═══════════════════════════════════════════════════════════════════");
        await writer.WriteLineAsync("  BASELINE BENCHMARK COMPLETE - SUMMARY");
        await writer.WriteLineAsync($"  Date: {startTime:yyyy-MM-dd}");
        await writer.WriteLineAsync("═══════════════════════════════════════════════════════════════════");
        await writer.WriteLineAsync();

        // Win rates table
        await writer.WriteLineAsync("## Win Rates");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Higher Win | Draw | Lower Win |");
        await writer.WriteLineAsync("|---------|------|------------|------|-----------|");

        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.HigherDifficultyWins} | {stats.Draws} | {stats.LowerDifficultyWins} |");
        }
        await writer.WriteLineAsync();

        // Per-Matchup Statistics
        await writer.WriteLineAsync("## Per-Matchup Statistics");
        await writer.WriteLineAsync();

        // Move Count
        await writer.WriteLineAsync("### Move Count (Discrete - Mode meaningful)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Mode | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.MoveCount.Mode:F0} | {stats.MoveCount.Median:F1} | {stats.MoveCount.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // Master Depth
        await writer.WriteLineAsync("### Master Depth (Discrete - Mode meaningful)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Mode | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.MasterDepth.Mode:F0} | {stats.MasterDepth.Median:F1} | {stats.MasterDepth.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // FMC%
        await writer.WriteLineAsync("### FMC% (Discrete - Mode meaningful)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Mode | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.FirstMoveCutoffPercent.Mode:F0} | {stats.FirstMoveCutoffPercent.Median:F1} | {stats.FirstMoveCutoffPercent.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // NPS
        await writer.WriteLineAsync("### NPS (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {FormatNPS(stats.NPS.Median)} | {FormatNPS(stats.NPS.Mean)} |");
        }
        await writer.WriteLineAsync();

        // EBF
        await writer.WriteLineAsync("### EBF (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.EffectiveBranchingFactor.Median:F1} | {stats.EffectiveBranchingFactor.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // Helper Avg Depth
        await writer.WriteLineAsync("### Helper Avg Depth (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.HelperAvgDepth.Median:F1} | {stats.HelperAvgDepth.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // Time Used
        await writer.WriteLineAsync("### Time Used (ms) (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {FormatTime(stats.TimeUsedMs.Median)} | {FormatTime(stats.TimeUsedMs.Mean)} |");
        }
        await writer.WriteLineAsync();

        // Time Allocated
        await writer.WriteLineAsync("### Time Allocated (ms) (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {FormatTime(stats.TimeAllocatedMs.Median)} | {FormatTime(stats.TimeAllocatedMs.Mean)} |");
        }
        await writer.WriteLineAsync();

        // TT Hit Rate
        await writer.WriteLineAsync("### TT Hit Rate (%) (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var (_, stats, tc) in results)
        {
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.TTHitRate.Median:F1} | {stats.TTHitRate.Mean:F1} |");
        }
        await writer.WriteLineAsync();

        // VCF Trigger Summary
        await writer.WriteLineAsync("### VCF Trigger Summary (Per Matchup)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Total | Details |");
        await writer.WriteLineAsync("|---------|------|-------|---------|");
        foreach (var (_, stats, tc) in results)
        {
            var details = stats.VCFTriggers.Count > 0
                ? string.Join(", ", stats.VCFTriggers.Take(5).Select(t => $"G{t.Game}M{t.Move}(d{t.Depth}/n{FormatLargeNumber(t.Nodes)})"))
                : "-";
            if (stats.VCFTriggers.Count > 5)
                details += $" ... +{stats.VCFTriggers.Count - 5} more";
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {stats.VCFTriggers.Count} | {details} |");
        }
        await writer.WriteLineAsync();

        // Move Type Distribution
        await writer.WriteLineAsync("### Move Type Distribution (Per Matchup)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Normal | Book | BookValidated | ImmediateWin | ImmediateBlock | ErrorRate |");
        await writer.WriteLineAsync("|---------|------|--------|------|---------------|--------------|----------------|-----------|");
        foreach (var (_, stats, tc) in results)
        {
            var normal = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.Normal, (0, 0.0)).Item2;
            var book = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.Book, (0, 0.0)).Item2;
            var bookVal = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.BookValidated, (0, 0.0)).Item2;
            var immWin = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.ImmediateWin, (0, 0.0)).Item2;
            var immBlock = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.ImmediateBlock, (0, 0.0)).Item2;
            var error = stats.MoveTypeDistribution.GetValueOrDefault(MoveType.ErrorRate, (0, 0.0)).Item2;
            await writer.WriteLineAsync($"| {stats.HigherDifficulty} vs {stats.LowerDifficulty} | {tc} | {normal:F1}% | {book:F1}% | {bookVal:F1}% | {immWin:F1}% | {immBlock:F1}% | {error:F1}% |");
        }
        await writer.WriteLineAsync();

        // Per-Difficulty Statistics (aggregated across all matchups where each difficulty played)
        await writer.WriteLineAsync("## Per-Difficulty Statistics");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("Statistics aggregated across all matchups where each difficulty played.");
        await writer.WriteLineAsync();

        // Aggregate per-difficulty stats across all results, grouped by time control
        var bulletDiffStats = new Dictionary<AIDifficulty, List<PerDifficultyStatistics>>();
        var blitzDiffStats = new Dictionary<AIDifficulty, List<PerDifficultyStatistics>>();

        foreach (var (_, stats, tc) in results)
        {
            var targetDict = tc == "bullet" ? bulletDiffStats : blitzDiffStats;
            foreach (var (diff, diffStats) in stats.PerDifficultyStats)
            {
                if (!targetDict.ContainsKey(diff))
                    targetDict[diff] = new List<PerDifficultyStatistics>();
                targetDict[diff].Add(diffStats);
            }
        }

        // All difficulties in order
        var allDifficulties = new[]
        {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Medium,
            AIDifficulty.Hard,
            AIDifficulty.Grandmaster
        };

        foreach (var difficulty in allDifficulties)
        {
            var bulletStats = bulletDiffStats.GetValueOrDefault(difficulty, new List<PerDifficultyStatistics>());
            var blitzStats = blitzDiffStats.GetValueOrDefault(difficulty, new List<PerDifficultyStatistics>());

            if (bulletStats.Count == 0 && blitzStats.Count == 0)
                continue;

            await writer.WriteLineAsync($"### {difficulty}");
            await writer.WriteLineAsync();

            // Discrete metrics table
            await writer.WriteLineAsync("**Discrete Metrics (Mode meaningful):**");
            await writer.WriteLineAsync();
            await writer.WriteLineAsync("| Time Control | Move Count | Master Depth | FMC% |");
            await writer.WriteLineAsync("|--------------|------------|--------------|------|");

            if (bulletStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(bulletStats);
                await writer.WriteLineAsync($"| Bullet (60+0) | {agg.TotalMoves} moves | {agg.MasterDepth.Mode:F0}/{agg.MasterDepth.Median:F1}/{agg.MasterDepth.Mean:F1} | {agg.FirstMoveCutoffPercent.Mode:F0}/{agg.FirstMoveCutoffPercent.Median:F1}/{agg.FirstMoveCutoffPercent.Mean:F1} |");
            }

            if (blitzStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(blitzStats);
                await writer.WriteLineAsync($"| Blitz (180+2) | {agg.TotalMoves} moves | {agg.MasterDepth.Mode:F0}/{agg.MasterDepth.Median:F1}/{agg.MasterDepth.Mean:F1} | {agg.FirstMoveCutoffPercent.Mode:F0}/{agg.FirstMoveCutoffPercent.Median:F1}/{agg.FirstMoveCutoffPercent.Mean:F1} |");
            }
            await writer.WriteLineAsync();

            // Continuous metrics table
            await writer.WriteLineAsync("**Continuous Metrics (Median/Mean only):**");
            await writer.WriteLineAsync();
            await writer.WriteLineAsync("| Time Control | NPS | Helper Depth | Time Used | Time Alloc | TT Hit Rate | EBF |");
            await writer.WriteLineAsync("|--------------|-----|--------------|-----------|------------|-------------|-----|");

            if (bulletStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(bulletStats);
                await writer.WriteLineAsync($"| Bullet (60+0) | {FormatNPS(agg.NPS.Median)}/{FormatNPS(agg.NPS.Mean)} | {agg.HelperAvgDepth.Median:F1}/{agg.HelperAvgDepth.Mean:F1} | {FormatTime(agg.TimeUsedMs.Median)}/{FormatTime(agg.TimeUsedMs.Mean)} | {FormatTime(agg.TimeAllocatedMs.Median)}/{FormatTime(agg.TimeAllocatedMs.Mean)} | {agg.TTHitRate.Median:F1}%/{agg.TTHitRate.Mean:F1}% | {agg.EffectiveBranchingFactor.Median:F1}/{agg.EffectiveBranchingFactor.Mean:F1} |");
            }

            if (blitzStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(blitzStats);
                await writer.WriteLineAsync($"| Blitz (180+2) | {FormatNPS(agg.NPS.Median)}/{FormatNPS(agg.NPS.Mean)} | {agg.HelperAvgDepth.Median:F1}/{agg.HelperAvgDepth.Mean:F1} | {FormatTime(agg.TimeUsedMs.Median)}/{FormatTime(agg.TimeUsedMs.Mean)} | {FormatTime(agg.TimeAllocatedMs.Median)}/{FormatTime(agg.TimeAllocatedMs.Mean)} | {agg.TTHitRate.Median:F1}%/{agg.TTHitRate.Mean:F1}% | {agg.EffectiveBranchingFactor.Median:F1}/{agg.EffectiveBranchingFactor.Mean:F1} |");
            }
            await writer.WriteLineAsync();

            // Move type distribution
            await writer.WriteLineAsync("**Move Types:**");
            await writer.WriteLineAsync();

            if (bulletStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(bulletStats);
                var moveTypesStr = FormatMoveTypeDistribution(agg.MoveTypeDistribution);
                await writer.WriteLineAsync($"- Bullet: {moveTypesStr}");
            }

            if (blitzStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(blitzStats);
                var moveTypesStr = FormatMoveTypeDistribution(agg.MoveTypeDistribution);
                await writer.WriteLineAsync($"- Blitz: {moveTypesStr}");
            }
            await writer.WriteLineAsync();

            // VCF triggers
            await writer.WriteLineAsync("**VCF Triggers:**");
            await writer.WriteLineAsync();

            if (bulletStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(bulletStats);
                await writer.WriteLineAsync($"- Bullet: {agg.VCFTriggers.Count} total");
            }

            if (blitzStats.Count > 0)
            {
                var agg = AggregatePerDifficultyStats(blitzStats);
                await writer.WriteLineAsync($"- Blitz: {agg.VCFTriggers.Count} total");
            }
            await writer.WriteLineAsync();
        }

        // Also print summary to console
        Console.WriteLine();
        Console.WriteLine("Summary written to: " + summaryFileName);
    }

    private static string FormatNPS(double nps)
    {
        if (nps >= 1_000_000)
            return $"{nps / 1_000_000:F1}M";
        if (nps >= 1_000)
            return $"{nps / 1_000:F1}K";
        return $"{nps:F0}";
    }

    private static string FormatTime(double ms)
    {
        if (ms >= 60000)
            return $"{ms / 60000:F1}m";
        if (ms >= 1000)
            return $"{ms / 1000:F1}s";
        return $"{ms:F0}ms";
    }

    private static string FormatLargeNumber(long n)
    {
        if (n >= 1_000_000_000)
            return $"{n / 1_000_000_000.0:F1}B";
        if (n >= 1_000_000)
            return $"{n / 1_000_000.0:F1}M";
        if (n >= 1_000)
            return $"{n / 1_000.0:F1}K";
        return n.ToString();
    }

    /// <summary>
    /// Aggregate multiple PerDifficultyStatistics into a single combined view.
    /// This combines stats from multiple matchups where the same difficulty played.
    /// </summary>
    private static PerDifficultyStatistics AggregatePerDifficultyStats(List<PerDifficultyStatistics> statsList)
    {
        if (statsList.Count == 0)
            return new PerDifficultyStatistics();

        if (statsList.Count == 1)
            return statsList[0];

        // Combine all moves for proper aggregation
        var allMasterDepths = new List<double>();
        var allFMC = new List<double>();
        var allNPS = new List<double>();
        var allHelperDepth = new List<double>();
        var allTimeUsed = new List<double>();
        var allTimeAlloc = new List<double>();
        var allTTHitRate = new List<double>();
        var allEBF = new List<double>();
        var allMoveTypes = new Dictionary<MoveType, int>();
        var allVCFTriggers = new List<VCFTriggerRecord>();
        var totalMoves = 0;

        foreach (var stats in statsList)
        {
            totalMoves += stats.TotalMoves;
            // For discrete metrics, we need to weigh by the count of moves
            // Since we don't have individual values, we use the median as representative
            // This is an approximation - ideally we'd have access to raw values
            allMasterDepths.Add(stats.MasterDepth.Median);
            allFMC.Add(stats.FirstMoveCutoffPercent.Median);
            allNPS.Add(stats.NPS.Median);
            allHelperDepth.Add(stats.HelperAvgDepth.Median);
            allTimeUsed.Add(stats.TimeUsedMs.Median);
            allTimeAlloc.Add(stats.TimeAllocatedMs.Median);
            allTTHitRate.Add(stats.TTHitRate.Median);
            allEBF.Add(stats.EffectiveBranchingFactor.Median);

            foreach (var (moveType, (count, _)) in stats.MoveTypeDistribution)
            {
                if (!allMoveTypes.ContainsKey(moveType))
                    allMoveTypes[moveType] = 0;
                allMoveTypes[moveType] += count;
            }

            allVCFTriggers.AddRange(stats.VCFTriggers);
        }

        // Compute aggregated move type distribution
        var totalMoveTypeCount = allMoveTypes.Values.Sum();
        var moveTypeDistribution = totalMoveTypeCount > 0
            ? allMoveTypes.ToDictionary(kvp => kvp.Key, kvp => (kvp.Value, (double)kvp.Value / totalMoveTypeCount * 100))
            : new Dictionary<MoveType, (int, double)>();

        return new PerDifficultyStatistics
        {
            Difficulty = statsList[0].Difficulty,
            TotalMoves = totalMoves,
            MasterDepth = ComputeDiscreteStats(allMasterDepths),
            FirstMoveCutoffPercent = ComputeDiscreteStats(allFMC),
            NPS = ComputeContinuousStats(allNPS),
            HelperAvgDepth = ComputeContinuousStats(allHelperDepth),
            TimeUsedMs = ComputeContinuousStats(allTimeUsed),
            TimeAllocatedMs = ComputeContinuousStats(allTimeAlloc),
            TTHitRate = ComputeContinuousStats(allTTHitRate),
            EffectiveBranchingFactor = ComputeContinuousStats(allEBF),
            MoveTypeDistribution = moveTypeDistribution,
            VCFTriggers = allVCFTriggers
        };
    }

    private static DiscreteMetricStats ComputeDiscreteStats(List<double> values)
    {
        if (values.Count == 0)
            return new DiscreteMetricStats { Mode = 0, Median = 0, Mean = 0 };

        return new DiscreteMetricStats
        {
            Mode = ComputeMode(values),
            Median = ComputeMedian(values),
            Mean = values.Average()
        };
    }

    private static ContinuousMetricStats ComputeContinuousStats(List<double> values)
    {
        if (values.Count == 0)
            return new ContinuousMetricStats { Median = 0, Mean = 0 };

        return new ContinuousMetricStats
        {
            Median = ComputeMedian(values),
            Mean = values.Average()
        };
    }

    private static double ComputeMode(List<double> values)
    {
        if (values.Count == 0) return 0;

        var roundedValues = values.Select(v => Math.Round(v)).GroupBy(v => v)
            .OrderByDescending(g => g.Count())
            .ThenBy(g => g.Key)
            .ToList();

        return roundedValues.Count > 0 ? roundedValues[0].Key : 0;
    }

    private static double ComputeMedian(List<double> values)
    {
        if (values.Count == 0) return 0;

        var sorted = values.OrderBy(v => v).ToList();
        int mid = sorted.Count / 2;

        if (sorted.Count % 2 == 0)
            return (sorted[mid - 1] + sorted[mid]) / 2.0;
        return sorted[mid];
    }

    private static string FormatMoveTypeDistribution(Dictionary<MoveType, (int Count, double Percentage)> distribution)
    {
        if (distribution.Count == 0)
            return "N/A";

        var parts = distribution
            .OrderByDescending(x => x.Value.Count)
            .Select(kvp => $"{kvp.Key}: {kvp.Value.Percentage:F1}%")
            .Take(4);
        return string.Join(", ", parts);
    }
}

/// <summary>
/// TextWriter that writes to both a file and the console (tee behavior)
/// </summary>
internal sealed class TeeTextWriter : TextWriter
{
    private readonly TextWriter _file;
    private readonly TextWriter _console;
    private bool _disposed;

    public TeeTextWriter(TextWriter file, TextWriter console)
    {
        _file = file;
        _console = console;
    }

    public override Encoding Encoding => _console.Encoding;

    public override void Write(char value)
    {
        _file.Write(value);
        _console.Write(value);
    }

    public override void Write(string? value)
    {
        _file.Write(value);
        _console.Write(value);
    }

    public override void WriteLine(string? value)
    {
        _file.WriteLine(value);
        _console.WriteLine(value);
    }

    public override void WriteLine()
    {
        _file.WriteLine();
        _console.WriteLine();
    }

    public override async Task WriteAsync(char value)
    {
        await _file.WriteAsync(value);
        await _console.WriteAsync(value);
    }

    public override async Task WriteAsync(string? value)
    {
        await _file.WriteAsync(value);
        await _console.WriteAsync(value);
    }

    public override async Task WriteLineAsync(string? value)
    {
        await _file.WriteLineAsync(value);
        await _console.WriteLineAsync(value);
    }

    public override async Task WriteLineAsync()
    {
        await _file.WriteLineAsync();
        await _console.WriteLineAsync();
    }

    public override void Flush()
    {
        _file.Flush();
        _console.Flush();
    }

    public override async Task FlushAsync()
    {
        await _file.FlushAsync();
        await _console.FlushAsync();
    }

    protected override void Dispose(bool disposing)
    {
        if (!_disposed)
        {
            if (disposing)
            {
                _file.Dispose();
            }
            _disposed = true;
        }
        base.Dispose(disposing);
    }
}
