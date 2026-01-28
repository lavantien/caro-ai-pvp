using Caro.Core.Tournament;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.TournamentRunner;

class Program
{
    static async Task Main(string[] args)
    {
        var autoMode = args.Contains("--auto");
        var quickTestMode = args.Contains("--test");
        var validateStrengthMode = args.Contains("--validate-strength");
        var quickValidateMode = args.Contains("--quick-validate");
        var depthProfileMode = args.Contains("--profile-depths");
        var quickValidationMode = args.Contains("--quick-val");
        var baselineMode = args.Contains("--baseline");
        var detailedVerifyMode = args.Contains("--detailed-verify");

        // Detailed verification mode: Per-move diagnostics with time, threads, nodes, NPS, TT
        if (detailedVerifyMode)
        {
            var time = GetArgValue(args, "--time", 420);
            var inc = GetArgValue(args, "--inc", 5);
            var games = GetArgValue(args, "--games", 10);
            await DetailedVerificationRunner.RunAsync(time, inc, games);
            return;
        }

        // Baseline mode: Comprehensive test with all difficulties
        if (baselineMode)
        {
            var time = GetArgValue(args, "--time", 420);  // Default 7+5
            var inc = GetArgValue(args, "--inc", 5);
            var games = GetArgValue(args, "--games", 10);
            await BaselineRunner.RunAsync(time, inc, games);
            return;
        }

        // Quick validation mode
        if (quickValidationMode)
        {
            var time = GetArgValue(args, "--time", 180);
            var inc = GetArgValue(args, "--inc", 2);
            var games = GetArgValue(args, "--games", 10);
            await QuickValidationRunner.RunAsync(time, inc, games);
            return;
        }

        // Depth profiler mode
        if (depthProfileMode)
        {
            DepthProfiler.Run(args);
            return;
        }

        // AI Strength Validation mode
        if (validateStrengthMode)
        {
            var config = new AIStrengthTestRunner.TestConfig
            {
                GamesPerMatchup = GetArgValue(args, "--games", 25),
                InitialTimeSeconds = GetArgValue(args, "--time", 120),
                IncrementSeconds = GetArgValue(args, "--inc", 1),
                EnablePondering = !args.Contains("--no-pondering"),
                Verbose = args.Contains("--verbose")
            };
            await AIStrengthTestRunner.RunAllAsync(config);
            return;
        }

        // Quick validation mode (fewer games)
        if (quickValidateMode)
        {
            await AIStrengthTestRunner.RunQuickAsync();
            return;
        }

        // Quick test mode: Run isolated matchups to verify AI strength
        if (quickTestMode)
        {
            QuickTest.RunAllTests();
            return;
        }

        Console.WriteLine("╔═══════════════════════════════════════════════════════════════════╗");
        Console.WriteLine("║           CARO AI TOURNAMENT - AUTOMATED BATTLE SYSTEM              ║");
        Console.WriteLine("╚═══════════════════════════════════════════════════════════════════╝");
        Console.WriteLine();

        // Show help if requested
        if (args.Contains("--help") || args.Contains("-h"))
        {
            ShowHelp();
            return;
        }

        var engine = new TournamentEngine();

        // Define tournament matchups
        var matchups = new Dictionary<(AIDifficulty, AIDifficulty), int>();
        matchups[(AIDifficulty.Easy, AIDifficulty.Easy)] = 10;
        matchups[(AIDifficulty.Easy, AIDifficulty.Medium)] = 20;
        matchups[(AIDifficulty.Easy, AIDifficulty.Hard)] = 20;
        matchups[(AIDifficulty.Easy, AIDifficulty.Grandmaster)] = 20;

        matchups[(AIDifficulty.Medium, AIDifficulty.Medium)] = 10;
        matchups[(AIDifficulty.Medium, AIDifficulty.Hard)] = 20;
        matchups[(AIDifficulty.Medium, AIDifficulty.Grandmaster)] = 20;

        matchups[(AIDifficulty.Hard, AIDifficulty.Hard)] = 10;
        matchups[(AIDifficulty.Hard, AIDifficulty.Grandmaster)] = 20;

        matchups[(AIDifficulty.Grandmaster, AIDifficulty.Grandmaster)] = 10;

        // Debug: Print all matchups
        Console.WriteLine("DEBUG - Matchups configured:");
        foreach (var (key, count) in matchups)
        {
            Console.WriteLine($"  {key.Item1} vs {key.Item2}: {count} games");
        }
        Console.WriteLine();

        var totalGames = matchups.Values.Sum();
        Console.WriteLine($"Tournament Configuration:");
        Console.WriteLine($"  Total Matchups: {matchups.Count}");
        Console.WriteLine($"  Total Games: {totalGames}");
        Console.WriteLine($"  Estimated Time: ~{totalGames * 0.5:F0} minutes");
        Console.WriteLine();

        if (!autoMode)
        {
            Console.WriteLine("Press ENTER to start tournament...");
            Console.ReadLine();
        }
        else
        {
            Console.WriteLine("Auto-mode: Starting tournament...");
            Thread.Sleep(1000);
        }

        try { Console.Clear(); } catch { /* Ignore if no console */ }
        Console.WriteLine("🏁 TOURNAMENT STARTED");
        Console.WriteLine();

        var results = new List<MatchResult>();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        // Run tournament with progress reporting
        var progress = new Progress<TournamentProgress>(progress =>
        {
            // Clear current line and show progress
            var winnerText = progress.LatestResult.Winner == Player.Red ? "Red" :
                           progress.LatestResult.Winner == Player.Blue ? "Blue" : "Draw";
            var resultText = progress.LatestResult.IsDraw ? "" :
                            progress.LatestResult.Winner == Player.Red ? "wins" : "wins";

            var resultDisplay = progress.LatestResult.IsDraw ? "Draw" : $"{winnerText} {resultText}";

            Console.Write($"\r[{progress.CompletedGames}/{progress.TotalGames}] " +
                         $"{progress.ProgressPercent:F1}% - " +
                         $"{progress.CurrentMatch}: " +
                         $"{resultDisplay}");

            // Show detailed results every 10 games
            if (progress.CompletedGames % 10 == 0)
            {
                Console.WriteLine();
                if (progress.LatestResult.IsDraw)
                {
                    Console.WriteLine($"  → Draw - {progress.LatestResult.WinnerDifficulty} vs {progress.LatestResult.LoserDifficulty}");
                }
                else
                {
                    Console.WriteLine($"  → {progress.LatestResult.WinnerDifficulty} ({progress.LatestResult.Winner}) " +
                                   $"defeated {progress.LatestResult.LoserDifficulty} ({progress.LatestResult.Loser})");
                }
                Console.WriteLine($"     Moves: {progress.LatestResult.TotalMoves}, " +
                               $"Time: {progress.LatestResult.DurationMs / 1000.0:F1}s, " +
                               $"Avg Move: {progress.LatestResult.AverageMoveTimeMs:F1}ms");
            }
        });

        try
        {
            results = engine.RunTournament(matchups, progress);
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\n\n❌ ERROR: {ex.Message}");
            return;
        }

        stopwatch.Stop();

        Console.WriteLine();
        Console.WriteLine();
        Console.WriteLine("✓ Tournament Complete!");
        Console.WriteLine($"  Total Time: {stopwatch.Elapsed.TotalMinutes:F2} minutes");
        Console.WriteLine($"  Games Completed: {results.Count}/{totalGames}");
        Console.WriteLine();

        // Generate and display leaderboard
        var stats = new TournamentStatistics(results);
        var leaderboard = stats.GenerateLeaderboard();

        Console.WriteLine(leaderboard);

        // Generate and display balance report
        var report = stats.GenerateBalanceReport();
        Console.WriteLine(report);

        // Save both reports to files
        var timestamp = DateTime.Now.ToString("yyyyMMdd_HHmmss");
        var leaderboardFile = $"leaderboard_{timestamp}.txt";
        var reportFile = $"tournament_report_{timestamp}.txt";

        File.WriteAllText(leaderboardFile, leaderboard);
        File.WriteAllText(reportFile, report);

        Console.WriteLine();
        Console.WriteLine($"📄 Leaderboard saved to: {leaderboardFile}");
        Console.WriteLine($"📄 Balance report saved to: {reportFile}");
        Console.WriteLine();
        Console.WriteLine("Press ENTER to exit...");
        Console.ReadLine();
    }

    private static int GetArgValue(string[] args, string key, int defaultValue)
    {
        for (int i = 0; i < args.Length - 1; i++)
        {
            if (args[i] == key && int.TryParse(args[i + 1], out int value))
            {
                return value;
            }
        }
        return defaultValue;
    }

    private static void ShowHelp()
    {
        Console.WriteLine("Caro AI Tournament Runner - Usage:");
        Console.WriteLine();
        Console.WriteLine("Options:");
        Console.WriteLine("  --baseline          Run comprehensive baseline (all difficulties, detailed log)");
        Console.WriteLine("  --detailed-verify   Run detailed verification (per-move diagnostics)");
        Console.WriteLine("  --test              Run quick strength tests (legacy)");
        Console.WriteLine("  --validate-strength Run full AI strength validation suite");
        Console.WriteLine("  --quick-validate    Run quick AI strength validation (10 games/matchup)");
        Console.WriteLine("  --profile-depths    Profile search depths at different time controls");
        Console.WriteLine("  --auto              Run tournament in auto mode (no prompts)");
        Console.WriteLine();
        Console.WriteLine("Validation options (with --baseline, --detailed-verify, --quick-val, or --validate-strength):");
        Console.WriteLine("  --games <n>         Number of games per matchup (default: 10)");
        Console.WriteLine("  --time <n>          Initial time in seconds (default: 420 for baseline/detailed-verify)");
        Console.WriteLine("  --inc <n>           Increment time in seconds (default: 5 for baseline/detailed-verify)");
        Console.WriteLine();
        Console.WriteLine("Examples:");
        Console.WriteLine("  dotnet run -- --detailed-verify");
        Console.WriteLine("  dotnet run -- --detailed-verify --games 5 --time 180 --inc 2");
        Console.WriteLine("  dotnet run -- --baseline");
        Console.WriteLine("  dotnet run -- --baseline --games 5 --time 180 --inc 2");
        Console.WriteLine("  dotnet run -- --validate-strength");
        Console.WriteLine("  dotnet run -- --quick-validate");
    }
}
