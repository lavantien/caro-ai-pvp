using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tournament;

/// <summary>
/// Statistical summary of a matchup between two AI difficulty levels.
/// Aggregates results from multiple games with color-specific tracking.
/// </summary>
public class MatchupStatistics
{
    /// <summary>
    /// The Red player's difficulty level
    /// </summary>
    public required AIDifficulty RedDifficulty { get; init; }

    /// <summary>
    /// The Blue player's difficulty level
    /// </summary>
    public required AIDifficulty BlueDifficulty { get; init; }

    /// <summary>
    /// Total number of games played
    /// </summary>
    public int TotalGames { get; set; }

    /// <summary>
    /// Number of games Red (the player, not the color) won
    /// </summary>
    public int RedPlayerWins { get; set; }

    /// <summary>
    /// Number of games Blue (the player, not the color) won
    /// </summary>
    public int BluePlayerWins { get; set; }

    /// <summary>
    /// Number of draws
    /// </summary>
    public int Draws { get; set; }

    /// <summary>
    /// Overall Red win rate (games where Red player won / total games)
    /// </summary>
    public double RedPlayerWinRate => TotalGames > 0 ? (double)RedPlayerWins / TotalGames : 0;

    // Color-specific performance tracking
    /// <summary>
    /// Games where Red difficulty won as Red player
    /// </summary>
    public int RedAsRed_Wins { get; set; }

    /// <summary>
    /// Games where Red difficulty won as Blue player
    /// </summary>
    public int RedAsBlue_Wins { get; set; }

    /// <summary>
    /// Games where Blue difficulty won as Red player
    /// </summary>
    public int BlueAsRed_Wins { get; set; }

    /// <summary>
    /// Games where Blue difficulty won as Blue player
    /// </summary>
    public int BlueAsBlue_Wins { get; set; }

    // Aggregate color statistics
    /// <summary>
    /// Total games where Red color won (regardless of which difficulty was playing)
    /// </summary>
    public int RedColorWins => RedAsRed_Wins + BlueAsRed_Wins;

    /// <summary>
    /// Total games where Blue color won (regardless of which difficulty was playing)
    /// </summary>
    public int BlueColorWins => RedAsBlue_Wins + BlueAsBlue_Wins;

    /// <summary>
    /// Red color win rate (proportion of games where Red color won)
    /// </summary>
    public double RedColorWinRate => TotalGames > 0 ? (double)RedColorWins / TotalGames : 0;

    /// <summary>
    /// Blue color win rate (proportion of games where Blue color won)
    /// </summary>
    public double BlueColorWinRate => TotalGames > 0 ? (double)BlueColorWins / TotalGames : 0;

    // Statistical metrics
    /// <summary>
    /// Likelihood of Superiority (LOS) - probability that Red difficulty is truly stronger than Blue
    /// LOS = 0.5 means equal strength, LOS > 0.95 means highly likely stronger
    /// </summary>
    public double LikelihoodOfSuperiority { get; set; }

    /// <summary>
    /// Estimated Elo difference from Red difficulty to Blue difficulty
    /// Positive means Red is stronger, negative means Blue is stronger
    /// </summary>
    public double EloDifference { get; set; }

    /// <summary>
    /// Lower bound of 95% confidence interval for Elo difference
    /// </summary>
    public double ConfidenceIntervalLower { get; set; }

    /// <summary>
    /// Upper bound of 95% confidence interval for Elo difference
    /// </summary>
    public double ConfidenceIntervalUpper { get; set; }

    /// <summary>
    /// Two-tailed p-value for the null hypothesis that both difficulties are equal
    /// P < 0.05 indicates statistically significant difference
    /// </summary>
    public double PValue { get; set; }

    /// <summary>
    /// Whether the result is statistically significant (p < 0.05)
    /// </summary>
    public bool IsStatisticallySignificant => PValue < 0.05;

    /// <summary>
    /// Whether Red difficulty has a color advantage (wins more when playing Red than expected)
    /// </summary>
    public bool RedHasColorAdvantage { get; set; }

    /// <summary>
    /// Whether there is any significant color advantage detected
    /// </summary>
    public bool HasColorAdvantage { get; set; }

    /// <summary>
    /// Effect size of color advantage (positive = Red favored, negative = Blue favored)
    /// Range: [-1, 1], where 0 means no advantage
    /// </summary>
    public double ColorAdvantageEffectSize { get; set; }

    /// <summary>
    /// Human-readable conclusion about the matchup
    /// </summary>
    public string Conclusion { get; set; } = string.Empty;

    /// <summary>
    /// Expected result based on difficulty levels
    /// </summary>
    public string ExpectedResult { get; set; } = string.Empty;

    /// <summary>
    /// Whether the test passed (higher difficulty won more games)
    /// </summary>
    public bool TestPassed { get; set; }

    /// <summary>
    /// Create a summary string for display
    /// </summary>
    public string ToSummaryString()
    {
        var higherDiff = RedDifficulty > BlueDifficulty ? RedDifficulty : BlueDifficulty;
        var lowerDiff = RedDifficulty < BlueDifficulty ? RedDifficulty : BlueDifficulty;
        var expectedWinner = RedDifficulty > BlueDifficulty ? "Red" : "Blue";

        var actualWinner = RedPlayerWins > BluePlayerWins ? "Red" : BluePlayerWins > RedPlayerWins ? "Blue" : "Draw";
        var result = actualWinner == expectedWinner ? "PASS" : "FAIL";

        return $"[{result}] {RedDifficulty} vs {BlueDifficulty}: " +
               $"{RedPlayerWins}-{Draws}-{BluePlayerWins} | " +
               $"Elo: {EloDifference:+0;-0} ({ConfidenceIntervalLower:+0;-0} to {ConfidenceIntervalUpper:+0;-0}) | " +
               $"LOS: {LikelihoodOfSuperiority:P1} | p: {PValue:F3}";
    }
}
