using System.Text.RegularExpressions;

namespace Caro.TournamentRunner;

/// <summary>
/// Parses existing baseline benchmark files and regenerates the summary with Per-Difficulty Statistics.
/// Run this when you have matchup files but need an updated summary format.
/// </summary>
public static class BaselineSummaryRegenerator
{
    private static readonly Dictionary<string, string> DifficultyCodeToName = new()
    {
        { "bd", "Braindead" },
        { "ez", "Easy" },
        { "md", "Medium" },
        { "hd", "Hard" },
        { "gm", "Grandmaster" }
    };

    private static readonly Dictionary<string, int> DifficultyOrder = new()
    {
        { "Braindead", 0 },
        { "Easy", 1 },
        { "Medium", 2 },
        { "Hard", 3 },
        { "Grandmaster", 4 }
    };

    public static async Task RegenerateSummaryAsync(string directory = ".")
    {
        var files = Directory.GetFiles(directory, "baseline_*.txt")
            .Where(f => !f.Contains("summary"))
            .OrderBy(f => f)
            .ToList();

        if (files.Count == 0)
        {
            Console.WriteLine("No baseline matchup files found.");
            return;
        }

        Console.WriteLine($"Found {files.Count} matchup files to parse...");

        var allResults = new List<ParsedMatchup>();

        foreach (var file in files)
        {
            var result = await ParseMatchupFile(file);
            if (result != null)
            {
                allResults.Add(result);
                Console.WriteLine($"  Parsed: {Path.GetFileName(file)} - {result.HigherDifficulty} vs {result.LowerDifficulty} ({result.Moves.Count} moves)");
            }
        }

        // Generate new summary
        var summaryPath = Path.Combine(directory, "baseline_summary.txt");
        await GenerateSummary(allResults, summaryPath);
        Console.WriteLine($"\nGenerated: {summaryPath}");
    }

    private static async Task<ParsedMatchup?> ParseMatchupFile(string filePath)
    {
        var fileName = Path.GetFileNameWithoutExtension(filePath);
        var parts = fileName.Split('_');

        if (parts.Length < 4) return null;

        var timeControl = parts[1]; // bullet or blitz
        var diff1Code = parts[2];
        var diff2Code = parts[3];

        if (!DifficultyCodeToName.TryGetValue(diff1Code, out var diff1) ||
            !DifficultyCodeToName.TryGetValue(diff2Code, out var diff2))
            return null;

        // Determine higher/lower difficulty
        var (higherDiff, lowerDiff) = DifficultyOrder[diff1] > DifficultyOrder[diff2]
            ? (diff1, diff2)
            : (diff2, diff1);

        var content = await File.ReadAllTextAsync(filePath);
        var lines = content.Split('\n');

        var moves = new List<ParsedMove>();
        var higherWins = 0;
        var lowerWins = 0;
        var draws = 0;
        var gameMoveCounts = new List<int>();
        var currentGameMoves = 0;
        var vcfTriggers = new List<VCFTriggerInfo>();
        var moveTypeCounts = new Dictionary<string, int>();

        foreach (var rawLine in lines)
        {
            var line = rawLine.Trim();

            // Parse move lines - look for pattern like "G 1 M  1 |"
            if (line.StartsWith("G ") && line.Contains(" M ") && line.Contains("|"))
            {
                currentGameMoves++;

                var move = ParseMoveLine(line);
                if (move != null)
                {
                    moves.Add(move);

                    // Track move types
                    if (!moveTypeCounts.ContainsKey(move.MoveType))
                        moveTypeCounts[move.MoveType] = 0;
                    moveTypeCounts[move.MoveType]++;

                    // Check for VCF (non-empty VCF field)
                    if (line.Contains("VCF:") && Regex.IsMatch(line, @"VCF:\s*[^-]"))
                    {
                        var gameMatch = Regex.Match(line, @"G\s*(\d+)");
                        var moveMatch = Regex.Match(line, @"M\s*(\d+)");
                        if (gameMatch.Success && moveMatch.Success)
                        {
                            vcfTriggers.Add(new VCFTriggerInfo
                            {
                                Game = int.Parse(gameMatch.Groups[1].Value),
                                Move = int.Parse(moveMatch.Groups[1].Value),
                                Difficulty = move.Difficulty
                            });
                        }
                    }
                }
            }

            // Parse game results
            if (line.Contains("wins on move"))
            {
                var winnerMatch = Regex.Match(line, @"(\w+)\s+\((\w+)\)\s+wins");
                if (winnerMatch.Success)
                {
                    var winner = winnerMatch.Groups[1].Value;
                    if (winner == higherDiff) higherWins++;
                    else if (winner == lowerDiff) lowerWins++;

                    gameMoveCounts.Add(currentGameMoves);
                    currentGameMoves = 0;
                }
            }
            else if (line.Contains("DRAW after"))
            {
                draws++;
                gameMoveCounts.Add(currentGameMoves);
                currentGameMoves = 0;
            }
        }

        return new ParsedMatchup
        {
            FileName = fileName,
            TimeControl = timeControl,
            HigherDifficulty = higherDiff,
            LowerDifficulty = lowerDiff,
            HigherDifficultyWins = higherWins,
            LowerDifficultyWins = lowerWins,
            Draws = draws,
            Moves = moves,
            GameMoveCounts = gameMoveCounts,
            VCFTriggers = vcfTriggers,
            MoveTypeCounts = moveTypeCounts
        };
    }

    private static ParsedMove? ParseMoveLine(string line)
    {
        try
        {
            // Extract difficulty - comes after "by" and before next "|"
            var byMatch = Regex.Match(line, @"by\s+(\w+)\s+\|");
            if (!byMatch.Success) return null;
            var difficulty = byMatch.Groups[1].Value;

            // Extract depth - pattern like "D2" or "D10"
            var depthMatch = Regex.Match(line, @"\bD(\d+)\b");
            var depth = depthMatch.Success ? int.Parse(depthMatch.Groups[1].Value) : 0;

            // Extract NPS - pattern like "NPS: 86.0K" or "NPS: 1.2M"
            var npsMatch = Regex.Match(line, @"NPS:\s*([\d.]+)([KM]?)");
            var nps = 0.0;
            if (npsMatch.Success)
            {
                var value = double.Parse(npsMatch.Groups[1].Value);
                var suffix = npsMatch.Groups[2].Value;
                nps = suffix switch
                {
                    "K" => value * 1000,
                    "M" => value * 1_000_000,
                    _ => value
                };
            }

            // Extract TT hit rate - pattern like "TT: 0.0%"
            var ttMatch = Regex.Match(line, @"TT:\s*([\d.]+)%");
            var ttHitRate = ttMatch.Success ? double.Parse(ttMatch.Groups[1].Value) : 0;

            // Extract Helper Depth - pattern like "HD: 1.0"
            var hdMatch = Regex.Match(line, @"HD:\s*([\d.]+)");
            var helperDepth = hdMatch.Success ? double.Parse(hdMatch.Groups[1].Value) : 0;

            // Extract EBF - pattern like "EBF: 2.5"
            var ebfMatch = Regex.Match(line, @"EBF:\s*([\d.]+)");
            var ebf = ebfMatch.Success ? double.Parse(ebfMatch.Groups[1].Value) : 0;

            // Extract FMC% - pattern like "FMC: 0%" or "FMC: 100%"
            var fmcMatch = Regex.Match(line, @"FMC:\s*(\d+)%");
            var fmc = fmcMatch.Success ? double.Parse(fmcMatch.Groups[1].Value) : 0;

            // Extract move type - single/double char code before "Th:" field
            // Pattern: "| -    |" or "| Bk   |" or "| Wn   |" etc.
            var typeMatch = Regex.Match(line, @"\|\s*(\S{1,2})\s+\|\s*Th:");
            var moveType = typeMatch.Success ? typeMatch.Groups[1].Value.Trim() : "-";

            return new ParsedMove
            {
                Difficulty = difficulty,
                NPS = nps,
                TTHitRate = ttHitRate,
                HelperDepth = helperDepth,
                EBF = ebf,
                FMC = fmc,
                Depth = depth,
                MoveType = moveType
            };
        }
        catch
        {
            return null;
        }
    }

    private static async Task GenerateSummary(List<ParsedMatchup> results, string outputPath)
    {
        using var writer = new StreamWriter(outputPath);

        await writer.WriteLineAsync("═══════════════════════════════════════════════════════════════════");
        await writer.WriteLineAsync("  BASELINE BENCHMARK COMPLETE - SUMMARY");
        await writer.WriteLineAsync($"  Date: {DateTime.UtcNow:yyyy-MM-dd}");
        await writer.WriteLineAsync("═══════════════════════════════════════════════════════════════════");
        await writer.WriteLineAsync();

        // Sort results: bullet first, then blitz, by matchup order
        results = results.OrderBy(r => r.TimeControl == "bullet" ? 0 : 1)
            .ThenBy(r => DifficultyOrder.GetValueOrDefault(r.LowerDifficulty, 0))
            .ThenBy(r => DifficultyOrder.GetValueOrDefault(r.HigherDifficulty, 0))
            .ToList();

        // Win rates
        await writer.WriteLineAsync("## Win Rates");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Higher Win | Draw | Lower Win |");
        await writer.WriteLineAsync("|---------|------|------------|------|-----------|");
        foreach (var r in results)
        {
            await writer.WriteLineAsync($"| {r.HigherDifficulty} vs {r.LowerDifficulty} | {r.TimeControl} | {r.HigherDifficultyWins} | {r.Draws} | {r.LowerDifficultyWins} |");
        }
        await writer.WriteLineAsync();

        // Per-Matchup Statistics
        await writer.WriteLineAsync("## Per-Matchup Statistics");
        await writer.WriteLineAsync();

        // Move Count
        await WriteDiscreteMetricTable(writer, results, "Move Count",
            r => r.GameMoveCounts.Select(x => (double)x).ToList());

        // Master Depth
        await WriteDiscreteMetricTable(writer, results, "Master Depth",
            r => r.Moves.Select(m => (double)m.Depth).ToList(), " (Discrete - Mode meaningful)");

        // FMC%
        await WriteDiscreteMetricTable(writer, results, "FMC%",
            r => r.Moves.Select(m => m.FMC).ToList(), " (Discrete - Mode meaningful)");

        // NPS
        await WriteContinuousMetricTable(writer, results, "NPS",
            r => r.Moves.Select(m => m.NPS).ToList(), true);

        // EBF
        await WriteContinuousMetricTable(writer, results, "EBF",
            r => r.Moves.Select(m => m.EBF).ToList());

        // Helper Depth
        await WriteContinuousMetricTable(writer, results, "Helper Avg Depth",
            r => r.Moves.Select(m => m.HelperDepth).ToList());

        // TT Hit Rate
        await WriteContinuousMetricTable(writer, results, "TT Hit Rate (%)",
            r => r.Moves.Select(m => m.TTHitRate).ToList());

        // VCF Trigger Summary
        await writer.WriteLineAsync("### VCF Trigger Summary (Per Matchup)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Total | Details |");
        await writer.WriteLineAsync("|---------|------|-------|---------|");
        foreach (var r in results)
        {
            var details = r.VCFTriggers.Count > 0
                ? string.Join(", ", r.VCFTriggers.Take(5).Select(t => $"G{t.Game}M{t.Move}(d0/n0)"))
                : "-";
            if (r.VCFTriggers.Count > 5)
                details += $" ... +{r.VCFTriggers.Count - 5} more";
            await writer.WriteLineAsync($"| {r.HigherDifficulty} vs {r.LowerDifficulty} | {r.TimeControl} | {r.VCFTriggers.Count} | {details} |");
        }
        await writer.WriteLineAsync();

        // Move Type Distribution
        await writer.WriteLineAsync("### Move Type Distribution (Per Matchup)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Normal | Book | BookValidated | ImmediateWin | ImmediateBlock | ErrorRate |");
        await writer.WriteLineAsync("|---------|------|--------|------|---------------|--------------|----------------|-----------|");
        foreach (var r in results)
        {
            var total = r.MoveTypeCounts.Values.Sum();
            if (total == 0) total = 1;
            var normal = GetMoveTypePercent(r.MoveTypeCounts, "-", total);
            var book = GetMoveTypePercent(r.MoveTypeCounts, "Bk", total);
            var bookVal = GetMoveTypePercent(r.MoveTypeCounts, "Bv", total);
            var immWin = GetMoveTypePercent(r.MoveTypeCounts, "Wn", total);
            var immBlock = GetMoveTypePercent(r.MoveTypeCounts, "Bl", total);
            var error = GetMoveTypePercent(r.MoveTypeCounts, "Er", total);
            await writer.WriteLineAsync($"| {r.HigherDifficulty} vs {r.LowerDifficulty} | {r.TimeControl} | {normal:F1}% | {book:F1}% | {bookVal:F1}% | {immWin:F1}% | {immBlock:F1}% | {error:F1}% |");
        }
        await writer.WriteLineAsync();

        // Per-Difficulty Statistics
        await writer.WriteLineAsync("## Per-Difficulty Statistics");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("Aggregated across all matchups where each difficulty played.");
        await writer.WriteLineAsync();

        var perDifficulty = AggregatePerDifficulty(results);

        foreach (var diff in new[] { "Braindead", "Easy", "Medium", "Hard", "Grandmaster" })
        {
            if (!perDifficulty.ContainsKey(diff)) continue;

            var stats = perDifficulty[diff];
            await writer.WriteLineAsync($"### {diff}");
            await writer.WriteLineAsync();

            // Discrete metrics table
            await writer.WriteLineAsync("**Discrete Metrics (Mode/Median/Mean):**");
            await writer.WriteLineAsync();
            await writer.WriteLineAsync("| Time Control | Master Depth | FMC% |");
            await writer.WriteLineAsync("|--------------|--------------|------|");

            if (stats.BulletMoves.Count > 0)
            {
                var depth = ComputeDiscreteStats(stats.BulletMoves.Select(m => (double)m.Depth).ToList());
                var fmc = ComputeDiscreteStats(stats.BulletMoves.Select(m => m.FMC).ToList());
                await writer.WriteLineAsync($"| Bullet | {depth.Mode:F0}/{depth.Median:F1}/{depth.Mean:F1} | {fmc.Mode:F0}/{fmc.Median:F1}/{fmc.Mean:F1} |");
            }

            if (stats.BlitzMoves.Count > 0)
            {
                var depth = ComputeDiscreteStats(stats.BlitzMoves.Select(m => (double)m.Depth).ToList());
                var fmc = ComputeDiscreteStats(stats.BlitzMoves.Select(m => m.FMC).ToList());
                await writer.WriteLineAsync($"| Blitz | {depth.Mode:F0}/{depth.Median:F1}/{depth.Mean:F1} | {fmc.Mode:F0}/{fmc.Median:F1}/{fmc.Mean:F1} |");
            }
            await writer.WriteLineAsync();

            // Continuous metrics table
            await writer.WriteLineAsync("**Continuous Metrics (Median/Mean):**");
            await writer.WriteLineAsync();
            await writer.WriteLineAsync("| Time Control | NPS | EBF | Helper Depth | TT Hit Rate |");
            await writer.WriteLineAsync("|--------------|-----|-----|--------------|-------------|");

            if (stats.BulletMoves.Count > 0)
            {
                var nps = ComputeContinuousStats(stats.BulletMoves.Select(m => m.NPS).ToList());
                var ebf = ComputeContinuousStats(stats.BulletMoves.Select(m => m.EBF).ToList());
                var hd = ComputeContinuousStats(stats.BulletMoves.Select(m => m.HelperDepth).ToList());
                var tt = ComputeContinuousStats(stats.BulletMoves.Select(m => m.TTHitRate).ToList());
                await writer.WriteLineAsync($"| Bullet | {FormatNPS(nps.Median)}/{FormatNPS(nps.Mean)} | {ebf.Median:F1}/{ebf.Mean:F1} | {hd.Median:F1}/{hd.Mean:F1} | {tt.Median:F1}%/{tt.Mean:F1}% |");
            }

            if (stats.BlitzMoves.Count > 0)
            {
                var nps = ComputeContinuousStats(stats.BlitzMoves.Select(m => m.NPS).ToList());
                var ebf = ComputeContinuousStats(stats.BlitzMoves.Select(m => m.EBF).ToList());
                var hd = ComputeContinuousStats(stats.BlitzMoves.Select(m => m.HelperDepth).ToList());
                var tt = ComputeContinuousStats(stats.BlitzMoves.Select(m => m.TTHitRate).ToList());
                await writer.WriteLineAsync($"| Blitz | {FormatNPS(nps.Median)}/{FormatNPS(nps.Mean)} | {ebf.Median:F1}/{ebf.Mean:F1} | {hd.Median:F1}/{hd.Mean:F1} | {tt.Median:F1}%/{tt.Mean:F1}% |");
            }
            await writer.WriteLineAsync();

            // VCF triggers
            await writer.WriteLineAsync($"**VCF Triggers:** Bullet: {stats.BulletVCFCount}, Blitz: {stats.BlitzVCFCount}");
            await writer.WriteLineAsync();
        }
    }

    private static double GetMoveTypePercent(Dictionary<string, int> counts, string type, int total)
    {
        return counts.TryGetValue(type, out var count) ? (double)count / total * 100 : 0;
    }

    private static async Task WriteDiscreteMetricTable(StreamWriter writer, List<ParsedMatchup> results,
        string metricName, Func<ParsedMatchup, List<double>> selector, string suffix = " (Discrete - Mode meaningful)")
    {
        await writer.WriteLineAsync($"### {metricName}{suffix}");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Mode | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|------|--------|------|");
        foreach (var r in results)
        {
            var values = selector(r);
            var stats = ComputeDiscreteStats(values);
            await writer.WriteLineAsync($"| {r.HigherDifficulty} vs {r.LowerDifficulty} | {r.TimeControl} | {stats.Mode:F0} | {stats.Median:F1} | {stats.Mean:F1} |");
        }
        await writer.WriteLineAsync();
    }

    private static async Task WriteContinuousMetricTable(StreamWriter writer, List<ParsedMatchup> results,
        string metricName, Func<ParsedMatchup, List<double>> selector, bool formatAsNPS = false)
    {
        await writer.WriteLineAsync($"### {metricName} (Continuous - Median/Mean only)");
        await writer.WriteLineAsync();
        await writer.WriteLineAsync("| Matchup | Time | Median | Mean |");
        await writer.WriteLineAsync("|---------|------|--------|------|");
        foreach (var r in results)
        {
            var values = selector(r);
            var stats = ComputeContinuousStats(values);
            var medianStr = formatAsNPS ? FormatNPS(stats.Median) : $"{stats.Median:F1}";
            var meanStr = formatAsNPS ? FormatNPS(stats.Mean) : $"{stats.Mean:F1}";
            await writer.WriteLineAsync($"| {r.HigherDifficulty} vs {r.LowerDifficulty} | {r.TimeControl} | {medianStr} | {meanStr} |");
        }
        await writer.WriteLineAsync();
    }

    private static Dictionary<string, PerDifficultyAggregated> AggregatePerDifficulty(List<ParsedMatchup> results)
    {
        var dict = new Dictionary<string, PerDifficultyAggregated>();

        foreach (var r in results)
        {
            foreach (var move in r.Moves)
            {
                if (!dict.ContainsKey(move.Difficulty))
                    dict[move.Difficulty] = new PerDifficultyAggregated();

                var agg = dict[move.Difficulty];
                if (r.TimeControl == "bullet")
                    agg.BulletMoves.Add(move);
                else
                    agg.BlitzMoves.Add(move);
            }

            // Count VCF triggers per difficulty
            foreach (var vcf in r.VCFTriggers)
            {
                if (!dict.ContainsKey(vcf.Difficulty))
                    dict[vcf.Difficulty] = new PerDifficultyAggregated();

                if (r.TimeControl == "bullet")
                    dict[vcf.Difficulty].BulletVCFCount++;
                else
                    dict[vcf.Difficulty].BlitzVCFCount++;
            }
        }

        return dict;
    }

    private static (double Mode, double Median, double Mean) ComputeDiscreteStats(List<double> values)
    {
        if (values.Count == 0) return (0, 0, 0);

        var mode = values.Select(v => Math.Round(v)).GroupBy(v => v)
            .OrderByDescending(g => g.Count())
            .ThenBy(g => g.Key)
            .First().Key;

        var sorted = values.OrderBy(v => v).ToList();
        var mid = sorted.Count / 2;
        var median = sorted.Count % 2 == 0
            ? (sorted[mid - 1] + sorted[mid]) / 2.0
            : sorted[mid];

        return (mode, median, values.Average());
    }

    private static (double Median, double Mean) ComputeContinuousStats(List<double> values)
    {
        if (values.Count == 0) return (0, 0);

        var sorted = values.OrderBy(v => v).ToList();
        var mid = sorted.Count / 2;
        var median = sorted.Count % 2 == 0
            ? (sorted[mid - 1] + sorted[mid]) / 2.0
            : sorted[mid];

        return (median, values.Average());
    }

    private static string FormatNPS(double nps)
    {
        if (nps >= 1_000_000)
            return $"{nps / 1_000_000:F1}M";
        if (nps >= 1_000)
            return $"{nps / 1_000:F1}K";
        return $"{nps:F0}";
    }

    private class ParsedMatchup
    {
        public string FileName { get; set; } = "";
        public string TimeControl { get; set; } = "";
        public string HigherDifficulty { get; set; } = "";
        public string LowerDifficulty { get; set; } = "";
        public int HigherDifficultyWins { get; set; }
        public int LowerDifficultyWins { get; set; }
        public int Draws { get; set; }
        public List<ParsedMove> Moves { get; set; } = new();
        public List<int> GameMoveCounts { get; set; } = new();
        public List<VCFTriggerInfo> VCFTriggers { get; set; } = new();
        public Dictionary<string, int> MoveTypeCounts { get; set; } = new();
    }

    private class ParsedMove
    {
        public string Difficulty { get; set; } = "";
        public double NPS { get; set; }
        public double TTHitRate { get; set; }
        public double HelperDepth { get; set; }
        public double EBF { get; set; }
        public double FMC { get; set; }
        public int Depth { get; set; }
        public string MoveType { get; set; } = "";
    }

    private class VCFTriggerInfo
    {
        public int Game { get; set; }
        public int Move { get; set; }
        public string Difficulty { get; set; } = "";
    }

    private class PerDifficultyAggregated
    {
        public List<ParsedMove> BulletMoves { get; } = new();
        public List<ParsedMove> BlitzMoves { get; } = new();
        public int BulletVCFCount { get; set; }
        public int BlitzVCFCount { get; set; }
    }
}
