using Caro.Core.Tournament;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.TournamentRunner;

class Program
{
    static void Main(string[] args)
    {
        var autoMode = args.Contains("--auto");
        var quickTestMode = args.Contains("--test");

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

        var engine = new TournamentEngine();

        // Define tournament matchups
        var matchups = new Dictionary<(AIDifficulty, AIDifficulty), int>();
        matchups[(AIDifficulty.Easy, AIDifficulty.Easy)] = 10;
        matchups[(AIDifficulty.Easy, AIDifficulty.Medium)] = 20;
        matchups[(AIDifficulty.Easy, AIDifficulty.Hard)] = 20;
        matchups[(AIDifficulty.Easy, AIDifficulty.Expert)] = 20;

        matchups[(AIDifficulty.Medium, AIDifficulty.Medium)] = 10;
        matchups[(AIDifficulty.Medium, AIDifficulty.Hard)] = 20;
        matchups[(AIDifficulty.Medium, AIDifficulty.Expert)] = 20;

        matchups[(AIDifficulty.Hard, AIDifficulty.Hard)] = 10;
        matchups[(AIDifficulty.Hard, AIDifficulty.Expert)] = 20;

        matchups[(AIDifficulty.Expert, AIDifficulty.Expert)] = 10;

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
}
