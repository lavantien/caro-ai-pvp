using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using System.Text;

namespace Caro.TournamentRunner.ReportGenerators;

/// <summary>
/// Generates HTML reports for AI strength validation test results.
/// Creates visually appealing, interactive reports with statistical summaries.
/// </summary>
public static class HtmlReportGenerator
{
    /// <summary>
    /// Generates a complete HTML report from a collection of matchup statistics.
    /// </summary>
    public static string GenerateReport(
        IReadOnlyList<MatchupStatistics> results,
        string title = "AI Strength Validation Report")
    {
        var sb = new StringBuilder();
        var timestamp = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss");

        sb.AppendLine("<!DOCTYPE html>");
        sb.AppendLine("<html lang='en'>");
        sb.AppendLine("<head>");
        sb.AppendLine("    <meta charset='UTF-8'>");
        sb.AppendLine("    <meta name='viewport' content='width=device-width, initial-scale=1.0'>");
        sb.AppendLine($"    <title>{title}</title>");
        sb.AppendLine(GetCssStyles());
        sb.AppendLine("</head>");
        sb.AppendLine("<body>");
        sb.AppendLine("    <div class='container'>");
        sb.AppendLine(GenerateHeader(title, timestamp));
        sb.AppendLine(GenerateSummarySection(results));
        sb.AppendLine(GeneratePhase1Results(results));
        sb.AppendLine(GeneratePhase2Results(results));
        sb.AppendLine(GeneratePhase3ColorAdvantage(results));
        sb.AppendLine(GenerateEloRanking(results));
        sb.AppendLine(GenerateFooter());
        sb.AppendLine("    </div>");
        sb.AppendLine("</body>");
        sb.AppendLine("</html>");

        return sb.ToString();
    }

    private static string GetCssStyles()
    {
        return @"    <style>
        :root {
            --primary: #2563eb;
            --success: #16a34a;
            --danger: #dc2626;
            --warning: #ca8a04;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 20px;
        }

        .container { max-width: 1200px; margin: 0 auto; }

        h1 { font-size: 1.75rem; margin-bottom: 0.5rem; color: var(--text); }
        h2 { font-size: 1.25rem; margin: 1.5rem 0 1rem; color: var(--text); border-bottom: 2px solid var(--border); padding-bottom: 0.5rem; }
        h3 { font-size: 1rem; margin: 1rem 0 0.5rem; color: var(--text-muted); }

        .card {
            background: var(--card-bg);
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }

        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin: 1rem 0;
        }

        .stat-box {
            background: var(--bg);
            padding: 1rem;
            border-radius: 6px;
            text-align: center;
        }

        .stat-value { font-size: 2rem; font-weight: bold; color: var(--primary); }
        .stat-label { font-size: 0.875rem; color: var(--text-muted); }

        table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid var(--border); }
        th { background: var(--bg); font-weight: 600; }
        tr:hover { background: var(--bg); }

        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
        }

        .badge-pass { background: #dcfce7; color: var(--success); }
        .badge-fail { background: #fee2e2; color: var(--danger); }
        .badge-skip { background: #fef9c3; color: var(--warning); }

        .elo-positive { color: var(--success); font-weight: 600; }
        .elo-negative { color: var(--danger); font-weight: 600; }
        .elo-neutral { color: var(--text-muted); }

        .progress-bar {
            width: 100%;
            height: 8px;
            background: var(--border);
            border-radius: 4px;
            overflow: hidden;
        }

        .progress-fill {
            height: 100%;
            background: var(--primary);
            transition: width 0.3s ease;
        }

        .progress-fill.success { background: var(--success); }
        .progress-fill.danger { background: var(--danger); }

        .footer {
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
            text-align: center;
            color: var(--text-muted);
            font-size: 0.875rem;
        }

        @media (max-width: 768px) {
            .summary-grid { grid-template-columns: 1fr; }
            table { font-size: 0.875rem; }
            th, td { padding: 0.5rem; }
        }
    </style>";
    }

    private static string GenerateHeader(string title, string timestamp)
    {
        return $@"    <div class='card'>
        <h1>{title}</h1>
        <p style='color: var(--text-muted);'>Generated: {timestamp}</p>
    </div>";
    }

    private static string GenerateSummarySection(IReadOnlyList<MatchupStatistics> results)
    {
        var totalTests = results.Count;
        var passedTests = results.Count(r => r.TestPassed);
        var failedTests = totalTests - passedTests;
        var totalGames = results.Sum(r => r.TotalGames);
        var passRate = totalTests > 0 ? (double)passedTests / totalTests * 100 : 0;

        return $@"    <div class='card'>
        <h2>Summary</h2>
        <div class='summary-grid'>
            <div class='stat-box'>
                <div class='stat-value'>{totalTests}</div>
                <div class='stat-label'>Total Tests</div>
            </div>
            <div class='stat-box'>
                <div class='stat-value' style='color: var(--success);'>{passedTests}</div>
                <div class='stat-label'>Passed</div>
            </div>
            <div class='stat-box'>
                <div class='stat-value' style='color: var(--danger);'>{failedTests}</div>
                <div class='stat-label'>Failed</div>
            </div>
            <div class='stat-box'>
                <div class='stat-value'>{totalGames}</div>
                <div class='stat-label'>Total Games</div>
            </div>
        </div>
        <div class='progress-bar'>
            <div class='progress-fill {(passRate >= 80 ? "success" : "danger")}' style='width: {passRate:F1}%'></div>
        </div>
        <p style='text-align: center; margin-top: 0.5rem; font-size: 0.875rem;'>Pass Rate: {passRate:F1}%</p>
    </div>";
    }

    private static string GeneratePhase1Results(IReadOnlyList<MatchupStatistics> results)
    {
        var adjacentTests = results
            .Where(r => r.RedDifficulty != r.BlueDifficulty)
            .OrderBy(r => Math.Abs((int)r.RedDifficulty - (int)r.BlueDifficulty))
            .ThenByDescending(r => (int)r.RedDifficulty)
            .ToList();

        if (!adjacentTests.Any())
            return string.Empty;

        var rows = new StringBuilder();
        foreach (var stat in adjacentTests)
        {
            var badgeClass = stat.TestPassed ? "badge-pass" : "badge-fail";
            var badgeText = stat.TestPassed ? "PASS" : "FAIL";
            var eloClass = stat.EloDifference > 0 ? "elo-positive" : stat.EloDifference < 0 ? "elo-negative" : "elo-neutral";

            rows.AppendLine($"            <tr>");
            rows.AppendLine($"                <td>{stat.RedDifficulty} vs {stat.BlueDifficulty}</td>");
            rows.AppendLine($"                <td>{stat.RedPlayerWins}-{stat.Draws}-{stat.BluePlayerWins}</td>");
            rows.AppendLine($"                <td class='{eloClass}'>{stat.EloDifference:+0;-0}</td>");
            rows.AppendLine($"                <td>({stat.ConfidenceIntervalLower:+0} to {stat.ConfidenceIntervalUpper:+0})</td>");
            rows.AppendLine($"                <td>{stat.LikelihoodOfSuperiority:P1}</td>");
            rows.AppendLine($"                <td>{stat.PValue:F3}</td>");
            rows.AppendLine($"                <td><span class='badge {badgeClass}'>{badgeText}</span></td>");
            rows.AppendLine($"            </tr>");
        }

        return $@"    <div class='card'>
        <h2>Phase 1: Adjacent Difficulty Testing</h2>
        <p style='color: var(--text-muted); margin-bottom: 1rem;'>Testing neighboring difficulty levels to detect strength inversions.</p>
        <table>
            <thead>
                <tr>
                    <th>Matchup</th>
                    <th>Result (W-D-L)</th>
                    <th>Elo Diff</th>
                    <th>95% CI</th>
                    <th>LOS</th>
                    <th>P-Value</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
{rows}            </tbody>
        </table>
    </div>";
    }

    private static string GeneratePhase2Results(IReadOnlyList<MatchupStatistics> results)
    {
        // Cross-level tests: gaps of 3+ difficulty levels
        var crossLevelTests = results
            .Where(r => Math.Abs((int)r.RedDifficulty - (int)r.BlueDifficulty) >= 3)
            .OrderByDescending(r => Math.Abs((int)r.RedDifficulty - (int)r.BlueDifficulty))
            .ToList();

        if (!crossLevelTests.Any())
            return string.Empty;

        var rows = new StringBuilder();
        foreach (var stat in crossLevelTests)
        {
            var badgeClass = stat.TestPassed ? "badge-pass" : "badge-fail";
            var badgeText = stat.TestPassed ? "PASS" : "FAIL";
            var gap = Math.Abs((int)stat.RedDifficulty - (int)stat.BlueDifficulty);

            rows.AppendLine($"            <tr>");
            rows.AppendLine($"                <td>{stat.RedDifficulty} vs {stat.BlueDifficulty}</td>");
            rows.AppendLine($"                <td>{gap} levels</td>");
            rows.AppendLine($"                <td>{stat.RedPlayerWins}-{stat.Draws}-{stat.BluePlayerWins}</td>");
            rows.AppendLine($"                <td>{stat.EloDifference:+0}</td>");
            rows.AppendLine($"                <td>{stat.LikelihoodOfSuperiority:P1}</td>");
            rows.AppendLine($"                <td><span class='badge {badgeClass}'>{badgeText}</span></td>");
            rows.AppendLine($"            </tr>");
        }

        return $@"    <div class='card'>
        <h2>Phase 2: Cross-Level Testing</h2>
        <p style='color: var(--text-muted); margin-bottom: 1rem;'>Testing large difficulty gaps (3+ levels).</p>
        <table>
            <thead>
                <tr>
                    <th>Matchup</th>
                    <th>Gap</th>
                    <th>Result</th>
                    <th>Elo Diff</th>
                    <th>LOS</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
{rows}            </tbody>
        </table>
    </div>";
    }

    private static string GeneratePhase3ColorAdvantage(IReadOnlyList<MatchupStatistics> results)
    {
        // Symmetric tests: same difficulty vs itself
        var symmetricTests = results
            .Where(r => r.RedDifficulty == r.BlueDifficulty)
            .OrderByDescending(r => (int)r.RedDifficulty)
            .ToList();

        if (!symmetricTests.Any())
            return string.Empty;

        var rows = new StringBuilder();
        foreach (var stat in symmetricTests)
        {
            var redWinRate = stat.RedColorWinRate;
            var isBalanced = redWinRate >= 0.40 && redWinRate <= 0.60;
            var badgeClass = isBalanced ? "badge-pass" : "badge-fail";
            var badgeText = isBalanced ? "BALANCED" : "BIASED";

            rows.AppendLine($"            <tr>");
            rows.AppendLine($"                <td>{stat.RedDifficulty}</td>");
            rows.AppendLine($"                <td>{stat.TotalGames}</td>");
            rows.AppendLine($"                <td>{stat.RedColorWins}-{stat.Draws}-{stat.BlueColorWins}</td>");
            rows.AppendLine($"                <td>{stat.RedColorWinRate:P1}</td>");
            rows.AppendLine($"                <td>{stat.BlueColorWinRate:P1}</td>");
            rows.AppendLine($"                <td>{stat.ColorAdvantageEffectSize:+0.000;-0.000}</td>");
            rows.AppendLine($"                <td><span class='badge {badgeClass}'>{badgeText}</span></td>");
            rows.AppendLine($"            </tr>");
        }

        return $@"    <div class='card'>
        <h2>Phase 3: Color Advantage Detection</h2>
        <p style='color: var(--text-muted); margin-bottom: 1rem;'>Symmetric tests to detect color bias (Red vs Blue with equal difficulty).</p>
        <table>
            <thead>
                <tr>
                    <th>Difficulty</th>
                    <th>Games</th>
                    <th>Result (R-D-B)</th>
                    <th>Red Win Rate</th>
                    <th>Blue Win Rate</th>
                    <th>Effect Size</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
{rows}            </tbody>
        </table>
    </div>";
    }

    private static string GenerateEloRanking(IReadOnlyList<MatchupStatistics> results)
    {
        // Calculate Elo scores for each difficulty
        var eloScores = new Dictionary<AIDifficulty, double>();
        var gamesPlayed = new Dictionary<AIDifficulty, int>();

        foreach (var diff in Enum.GetValues<AIDifficulty>())
        {
            eloScores[diff] = 0;
            gamesPlayed[diff] = 0;
        }

        foreach (var stat in results)
        {
            if (stat.RedDifficulty == stat.BlueDifficulty)
                continue;

            // Add Elo contribution (positive means red won more)
            eloScores[stat.RedDifficulty] += stat.EloDifference / 2;
            eloScores[stat.BlueDifficulty] -= stat.EloDifference / 2;
            gamesPlayed[stat.RedDifficulty] += stat.TotalGames;
            gamesPlayed[stat.BlueDifficulty] += stat.TotalGames;
        }

        var sorted = eloScores
            .OrderByDescending(kvp => kvp.Value)
            .ToList();

        var rows = new StringBuilder();
        int rank = 1;
        foreach (var (diff, elo) in sorted)
        {
            var games = gamesPlayed[diff];
            var eloClass = elo >= 0 ? "elo-positive" : "elo-negative";
            rows.AppendLine($"            <tr>");
            rows.AppendLine($"                <td>{rank}</td>");
            rows.AppendLine($"                <td><strong>{diff}</strong></td>");
            rows.AppendLine($"                <td class='{eloClass}'>{elo:+0;-0}</td>");
            rows.AppendLine($"                <td>{games}</td>");
            rows.AppendLine($"            </tr>");
            rank++;
        }

        return $@"    <div class='card'>
        <h2>Elo Ranking (All Difficulties)</h2>
        <p style='color: var(--text-muted); margin-bottom: 1rem;'>Relative Elo scores based on all matchups. Higher values indicate stronger performance.</p>
        <table>
            <thead>
                <tr>
                    <th>Rank</th>
                    <th>Difficulty</th>
                    <th>Elo Score</th>
                    <th>Games Played</th>
                </tr>
            </thead>
            <tbody>
{rows}            </tbody>
        </table>
    </div>";
    }

    private static string GenerateFooter()
    {
        return $@"    <div class='footer'>
        <p>AI Strength Validation Test Suite | Caro AI PvP</p>
        <p>This report was generated automatically. Results are based on statistical analysis of AI vs AI games.</p>
    </div>";
    }

    /// <summary>
    /// Saves the HTML report to a file.
    /// </summary>
    public static void SaveToFile(string html, string outputPath)
    {
        var directory = Path.GetDirectoryName(outputPath);
        if (!string.IsNullOrEmpty(directory) && !Directory.Exists(directory))
        {
            Directory.CreateDirectory(directory);
        }
        File.WriteAllText(outputPath, html);
    }
}
