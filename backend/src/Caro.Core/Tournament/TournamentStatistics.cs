using Caro.Core.Entities;
using Caro.Core.GameLogic;
using System.Text;

namespace Caro.Core.Tournament;

/// <summary>
/// Analyzes tournament results and generates balance reports
/// </summary>
public class TournamentStatistics
{
    private readonly List<MatchResult> _results;

    public TournamentStatistics(List<MatchResult> results)
    {
        _results = results;
    }

    /// <summary>
    /// Generate a comprehensive balance report
    /// </summary>
    public string GenerateBalanceReport()
    {
        var sb = new StringBuilder();
        sb.AppendLine("=".PadRight(80, '='));
        sb.AppendLine("AI TOURNAMENT BALANCE REPORT");
        sb.AppendLine("=".PadRight(80, '='));
        sb.AppendLine();

        // Overall statistics
        var totalGames = _results.Count;
        var draws = _results.Count(r => r.IsDraw);
        var timeouts = _results.Count(r => r.EndedByTimeout);
        var redWins = _results.Count(r => r.Winner == Player.Red);
        var blueWins = _results.Count(r => r.Winner == Player.Blue);

        sb.AppendLine("OVERALL STATISTICS");
        sb.AppendLine("-".PadRight(80, '-'));
        sb.AppendLine($"Total Games: {totalGames}");
        sb.AppendLine($"Red Wins: {redWins} ({(double)redWins / totalGames * 100:F1}%)");
        sb.AppendLine($"Blue Wins: {blueWins} ({(double)blueWins / totalGames * 100:F1}%)");
        sb.AppendLine($"Draws: {draws} ({(double)draws / totalGames * 100:F1}%)");
        sb.AppendLine($"Timeouts: {timeouts} ({(double)timeouts / totalGames * 100:F1}%)");
        sb.AppendLine();

        // Win rate by difficulty pairing
        sb.AppendLine("WIN RATE BY DIFFICULTY PAIRING");
        sb.AppendLine("-".PadRight(80, '-'));
        GenerateWinRateTable(sb);
        sb.AppendLine();

        // Average game length by pairing
        sb.AppendLine("AVERAGE GAME LENGTH (MOVES)");
        sb.AppendLine("-".PadRight(80, '-'));
        GenerateGameLengthTable(sb);
        sb.AppendLine();

        // Average move time by difficulty
        sb.AppendLine("AVERAGE MOVE TIME (MS)");
        sb.AppendLine("-".PadRight(80, '-'));
        GenerateMoveTimeTable(sb);
        sb.AppendLine();

        // Balance analysis
        sb.AppendLine("BALANCE ANALYSIS");
        sb.AppendLine("-".PadRight(80, '-'));
        GenerateBalanceAnalysis(sb);
        sb.AppendLine();

        sb.AppendLine("=".PadRight(80, '='));

        return sb.ToString();
    }

    /// <summary>
    /// Generate ELO leaderboard with 8 individual bots (2 per difficulty)
    /// </summary>
    public string GenerateLeaderboard()
    {
        // Create all 8 bots with initial ELO of 600
        var allBots = AIBotFactory.GetAllTournamentBots();

        // Group bots by difficulty for easier access
        var botsByDifficulty = allBots.GroupBy(b => b.Difficulty)
            .ToDictionary(g => g.Key, g => g.ToList());

        // Track which bot index to use for each game (distribute games evenly)
        var botIndices = new Dictionary<AIDifficulty, int>
        {
            { AIDifficulty.Easy, 0 },
            { AIDifficulty.Medium, 0 },
            { AIDifficulty.Hard, 0 },
            { AIDifficulty.Expert, 0 }
        };

        // Process all games and update ELO for specific bots
        foreach (var result in _results)
        {
            if (result.IsDraw) continue;

            // Get winner bots for this difficulty
            var winnerBots = botsByDifficulty[result.WinnerDifficulty];
            var loserBots = botsByDifficulty[result.LoserDifficulty];

            // Select specific bot (round-robin through bots of same difficulty)
            var winnerBotIndex = botIndices[result.WinnerDifficulty] % winnerBots.Count;
            var loserBotIndex = botIndices[result.LoserDifficulty] % loserBots.Count;

            var winnerBot = winnerBots[winnerBotIndex];
            var loserBot = loserBots[loserBotIndex];

            // Update ELO for the specific bots that played
            ELOCalculator.UpdateELOs(winnerBot, loserBot, result.IsDraw);

            // Move to next bot for this difficulty (distribute games evenly)
            botIndices[result.WinnerDifficulty]++;
            botIndices[result.LoserDifficulty]++;
        }

        // Sort by ELO (descending)
        var sortedBots = allBots.OrderByDescending(b => b.ELO).ToList();

        var sb = new StringBuilder();
        sb.AppendLine("╔══════════════════════════════════════════════════════════════════════════════╗");
        sb.AppendLine("║                    AI TOURNAMENT LEADERBOARD                                  ║");
        sb.AppendLine("╚══════════════════════════════════════════════════════════════════════════════╝");
        sb.AppendLine();

        sb.AppendLine($"  {"Rank",-6} {"Bot Name",-20} {"Difficulty",-12} {"ELO",-8} {"W-L-D",-12} {"Win Rate",-10}");
        sb.AppendLine("  " + new string('=', 70));

        int rank = 1;
        foreach (var bot in sortedBots)
        {
            string medal = rank switch
            {
                1 => "🥇",
                2 => "🥈",
                3 => "🥉",
                _ => "  "
            };

            sb.AppendLine($"  {medal} {rank,-4} {bot.Name,-20} {bot.Difficulty,-12} {bot.ELO,-8} " +
                        $"{bot.Wins}-{bot.Losses}-{bot.Draws,-6} {bot.WinRate:F1}%");

            rank++;
        }

        sb.AppendLine();
        sb.AppendLine("  " + new string('-', 70));
        sb.AppendLine($"  Total Games: {_results.Count}");
        sb.AppendLine($"  ELO System: K=32, All bots started at 600 ELO");
        sb.AppendLine($"  8 Bots: 2 per difficulty (Alpha/Bravo variants)");
        sb.AppendLine();

        return sb.ToString();
    }

    private void GenerateWinRateTable(StringBuilder sb)
    {
        var difficulties = Enum.GetValues<AIDifficulty>();

        sb.AppendLine($"{"Red",-10} {"Blue",-10} {"Games",-8} {"Red Win%",-10} {"Blue Win%",-10} {"Draw%",-10}");
        sb.AppendLine(new string('-', 60));

        foreach (var redDiff in difficulties)
        {
            foreach (var blueDiff in difficulties)
            {
                var matchupResults = _results
                    .Where(r => !r.IsDraw &&
                               ((r.WinnerDifficulty == redDiff && r.LoserDifficulty == blueDiff && r.Winner == Player.Red) ||
                                (r.WinnerDifficulty == blueDiff && r.LoserDifficulty == redDiff && r.Winner == Player.Blue)))
                    .ToList();

                var allMatchups = _results
                    .Where(r => r.WinnerDifficulty == redDiff || r.LoserDifficulty == redDiff ||
                               r.WinnerDifficulty == blueDiff || r.LoserDifficulty == blueDiff)
                    .Where(r => (r.WinnerDifficulty == redDiff && r.LoserDifficulty == blueDiff) ||
                               (r.WinnerDifficulty == blueDiff && r.LoserDifficulty == redDiff))
                    .ToList();

                if (allMatchups.Count == 0) continue;

                var redWins = allMatchups.Count(r => r.Winner == Player.Red);
                var blueWins = allMatchups.Count(r => r.Winner == Player.Blue);
                var draws = allMatchups.Count(r => r.IsDraw);

                sb.AppendLine($"{redDiff,-10} {blueDiff,-10} {allMatchups.Count,-8} " +
                            $"{Pct(redWins, allMatchups.Count),-10} " +
                            $"{Pct(blueWins, allMatchups.Count),-10} " +
                            $"{Pct(draws, allMatchups.Count),-10}");
            }
        }
    }

    private void GenerateGameLengthTable(StringBuilder sb)
    {
        var difficulties = Enum.GetValues<AIDifficulty>();

        sb.AppendLine($"{"Red",-10} {"Blue",-10} {"Avg Moves",-12} {"Min",-6} {"Max",-6}");
        sb.AppendLine(new string('-', 50));

        foreach (var redDiff in difficulties)
        {
            foreach (var blueDiff in difficulties)
            {
                var games = _results
                    .Where(r => (r.WinnerDifficulty == redDiff && r.LoserDifficulty == blueDiff) ||
                               (r.WinnerDifficulty == blueDiff && r.LoserDifficulty == redDiff))
                    .Select(r => r.TotalMoves)
                    .ToList();

                if (games.Count == 0) continue;

                sb.AppendLine($"{redDiff,-10} {blueDiff,-10} " +
                            $"{games.Average():F1,-12} " +
                            $"{games.Min(),-6} " +
                            $"{games.Max(),-6}");
            }
        }
    }

    private void GenerateMoveTimeTable(StringBuilder sb)
    {
        var difficulties = Enum.GetValues<AIDifficulty>();

        sb.AppendLine($"{"Difficulty",-10} {"Avg Time (ms)",-15} {"Min",-10} {"Max",-10}");
        sb.AppendLine(new string('-', 50));

        foreach (var diff in difficulties)
        {
            var times = _results
                .Where(r => r.WinnerDifficulty == diff || r.LoserDifficulty == diff)
                .SelectMany(r => r.MoveTimesMs)
                .ToList();

            if (times.Count == 0) continue;

            sb.AppendLine($"{diff,-10} {times.Average():F2,-15} {times.Min(),-10} {times.Max(),-10}");
        }
    }

    private void GenerateBalanceAnalysis(StringBuilder sb)
    {
        var difficulties = Enum.GetValues<AIDifficulty>();

        foreach (var higherDiff in difficulties)
        {
            foreach (var lowerDiff in difficulties)
            {
                if (higherDiff <= lowerDiff) continue; // Only check higher vs lower

                var games = _results
                    .Where(r => (r.WinnerDifficulty == higherDiff && r.LoserDifficulty == lowerDiff) ||
                               (r.WinnerDifficulty == lowerDiff && r.LoserDifficulty == higherDiff))
                    .ToList();

                if (games.Count < 5) continue; // Need sample size

                var higherWins = games.Count(r => r.WinnerDifficulty == higherDiff);
                var winRate = (double)higherWins / games.Count;

                sb.Append($"{higherDiff} vs {lowerDiff}: ");
                sb.Append($"{higherDiff} wins {Pct(higherWins, games.Count)} ");

                if (winRate >= 0.7)
                {
                    sb.AppendLine("✓ (Good balance - stronger AI wins more often)");
                }
                else if (winRate >= 0.55)
                {
                    sb.AppendLine("~ (Acceptable - stronger AI has advantage)");
                }
                else if (winRate <= 0.3)
                {
                    sb.AppendLine($"⚠ WARNING: {lowerDiff} beating {higherDiff} too often!");
                }
                else
                {
                    sb.AppendLine($"⚠ WARNING: Difficulties too close (no clear advantage)");
                }
            }
        }
    }

    private static string Pct(int value, int total) => total > 0 ? $"{(double)value / total * 100:F1}%" : "N/A";
}
