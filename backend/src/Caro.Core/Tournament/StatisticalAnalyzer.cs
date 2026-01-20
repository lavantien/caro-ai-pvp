using Caro.Core.Entities;

namespace Caro.Core.Tournament;

/// <summary>
/// Statistical analysis functions for AI strength validation.
/// Provides methods for calculating win rate significance, Elo differences,
/// and detecting color advantages in game results.
/// </summary>
public static class StatisticalAnalyzer
{
    /// <summary>
    /// Result of Sequential Probability Ratio Test
    /// </summary>
    public enum SPRTResult
    {
        /// <summary>Continue collecting data</summary>
        Continue,
        /// <summary>Accept H0: No significant difference</summary>
        H0,
        /// <summary>Accept H1: Significant difference detected</summary>
        H1
    }

    /// <summary>
    /// Calculate Likelihood of Superiority (LOS).
    /// Returns the probability that the first player is truly stronger than the second.
    /// LOS = 0.5 means equal strength, LOS > 0.95 means highly likely stronger.
    /// </summary>
    /// <param name="wins">Number of wins</param>
    /// <param name="losses">Number of losses</param>
    /// <param name="draws">Number of draws (treated as half win, half loss)</param>
    /// <returns>Probability between 0 and 1</returns>
    public static double CalculateLOS(int wins, int losses, int draws = 0)
    {
        // Treat draws as half wins
        var effectiveWins = wins + draws * 0.5;
        var effectiveLosses = losses + draws * 0.5;
        var totalGames = wins + losses + draws;

        if (totalGames == 0) return 0.5;

        // Use the binomial distribution-based LOS calculation
        // This is the standard method used in chess engine testing (CCRL/CEGT)
        // LOS = 1 / 2 * (1 + erf((wins - losses) / sqrt(2 * totalGames)))

        var numerator = effectiveWins - effectiveLosses;
        var denominator = Math.Sqrt(2.0 * totalGames);

        if (denominator == 0) return 0.5;

        var z = numerator / denominator;
        return 0.5 * (1.0 + Erf(z));
    }

    /// <summary>
    /// Calculate Elo difference with 95% confidence interval.
    /// Uses the standard logistic distribution model for Elo.
    /// </summary>
    /// <param name="wins">Number of wins</param>
    /// <param name="losses">Number of losses</param>
    /// <param name="draws">Number of draws</param>
    /// <returns>(Elo difference, lower CI, upper CI)</returns>
    public static (double eloDiff, double lowerCI, double upperCI) CalculateEloWithCI(
        int wins, int losses, int draws)
    {
        var totalGames = wins + losses + draws;
        if (totalGames == 0) return (0, -200, 200);

        // Calculate score (draws count as 0.5)
        var score = (wins + draws * 0.5) / totalGames;

        // Avoid edge cases
        score = Math.Clamp(score, 0.01, 0.99);

        // Convert score to Elo difference using inverse logistic distribution
        // Elo = -400 * log10(1/score - 1)
        var eloDiff = -400.0 * Math.Log10(1.0 / score - 1.0);

        // Calculate standard error using the delta method
        // Var(score) = score * (1 - score) / totalGames
        var scoreVariance = score * (1.0 - score) / totalGames;
        var scoreStdError = Math.Sqrt(scoreVariance);

        // Convert score CI to Elo CI using derivative at the point estimate
        // d(Elo)/d(score) = 400 / (score * (1 - score) * ln(10))
        var eloScale = 400.0 / (score * (1.0 - score) * Math.Log(10));
        var eloStdError = eloScale * scoreStdError;

        // 95% confidence interval = 1.96 * SE
        var margin = 1.96 * eloStdError;

        return (eloDiff, eloDiff - margin, eloDiff + margin);
    }

    /// <summary>
    /// Calculate binomial test p-value for win rate significance.
    /// Tests whether the observed win rate is significantly different from expected.
    /// </summary>
    /// <param name="wins">Number of wins</param>
    /// <param name="totalGames">Total number of games</param>
    /// <param name="expectedWinRate">Expected win rate under null hypothesis (default 0.5)</param>
    /// <returns>Two-tailed p-value</returns>
    public static double BinomialTestPValue(int wins, int totalGames, double expectedWinRate = 0.5)
    {
        if (totalGames == 0) return 1.0;

        var winRate = (double)wins / totalGames;
        var expected = expectedWinRate * totalGames;

        // Calculate standard error
        var stdError = Math.Sqrt(totalGames * expectedWinRate * (1.0 - expectedWinRate));

        if (stdError == 0) return 1.0;

        // Z-score for two-tailed test
        var z = Math.Abs(wins - expected) / stdError;

        // Convert Z to p-value using error function complement
        // p-value = 2 * (1 - Phi(|z|)) where Phi is standard normal CDF
        // Phi(z) = 0.5 * (1 + erf(z / sqrt(2)))
        var pValue = 2.0 * (1.0 - 0.5 * (1.0 + Erf(z / Math.Sqrt(2))));

        return Math.Clamp(pValue, 0.0, 1.0);
    }

    /// <summary>
    /// Detect color advantage using paired game analysis.
    /// Analyzes whether Red or Blue has a significant advantage independent of player strength.
    /// </summary>
    /// <param name="results">List of (isRedPlayer, winner) tuples</param>
    /// <returns>(hasAdvantage, effectSize, pValue)
    ///     - hasAdvantage: true if significant color advantage detected (p < 0.05)
    ///     - effectSize: Positive = Red advantage, Negative = Blue advantage, range [-1, 1]
    ///     - pValue: Statistical significance
    /// </returns>
    public static (bool hasAdvantage, double effectSize, double pValue) DetectColorAdvantage(
        List<(bool isRed, Player winner)> results)
    {
        if (results.Count == 0) return (false, 0, 1.0);

        // Count wins by color assignment
        var redAsRedWins = results.Count(r => r.isRed && r.winner == Player.Red);
        var redAsBlueWins = results.Count(r => !r.isRed && r.winner == Player.Blue);
        var blueAsRedWins = results.Count(r => r.isRed && r.winner == Player.Blue);
        var blueAsBlueWins = results.Count(r => !r.isRed && r.winner == Player.Red);

        // Total games where each color won
        var redWins = redAsRedWins + redAsBlueWins;
        var blueWins = blueAsRedWins + blueAsBlueWins;
        var totalGames = results.Count;

        // Calculate effect size: proportion of games Red won minus expected 0.5
        // Positive = Red advantage, Negative = Blue advantage
        var effectSize = totalGames > 0 ? ((double)redWins / totalGames) - 0.5 : 0;

        // Two-tailed binomial test against null hypothesis of 50% Red wins
        var pValue = BinomialTestPValue(redWins, totalGames, 0.5);

        // Significant advantage if p < 0.05
        var hasAdvantage = pValue < 0.05;

        return (hasAdvantage, effectSize, pValue);
    }

    /// <summary>
    /// Sequential Probability Ratio Test (SPRT) for early termination.
    /// Used to efficiently determine if there's a significant difference between two AIs.
    /// </summary>
    /// <param name="wins">Number of wins</param>
    /// <param name="losses">Number of losses</param>
    /// <param name="draws">Number of draws (counted as half win)</param>
    /// <param name="elo0">Elo difference for H0 (default 0)</param>
    /// <param name="elo1">Elo difference for H1 (default 50)</param>
    /// <param name="alpha">Type I error rate (default 0.05)</param>
    /// <param name="beta">Type II error rate (default 0.05)</param>
    /// <returns>SPRTResult indicating whether to continue or accept H0/H1</returns>
    public static SPRTResult SPRT(
        int wins, int losses, int draws,
        double elo0 = 0, double elo1 = 50,
        double alpha = 0.05, double beta = 0.05)
    {
        // Treat draws as half wins
        var effectiveWins = wins + draws * 0.5;
        var effectiveLosses = losses + draws * 0.5;
        var totalGames = wins + losses + draws;

        if (totalGames < 10) return SPRTResult.Continue;  // Need minimum sample size

        // Calculate current score
        var score = totalGames > 0 ? effectiveWins / totalGames : 0.5;

        // Expected scores under H0 and H1 using logistic distribution
        // P(win) = 1 / (1 + 10^(-elo/400))
        var p0 = 1.0 / (1.0 + Math.Pow(10.0, -elo0 / 400.0));
        var p1 = 1.0 / (1.0 + Math.Pow(10.0, -elo1 / 400.0));

        // Avoid log(0)
        p0 = Math.Clamp(p0, 0.001, 0.999);
        p1 = Math.Clamp(p1, 0.001, 0.999);
        score = Math.Clamp(score, 0.001, 0.999);

        // Log likelihood ratio
        // LLR = n * [score * log(p1/p0) + (1-score) * log((1-p1)/(1-p0))]
        var logRatio = score * Math.Log(p1 / p0) + (1.0 - score) * Math.Log((1.0 - p1) / (1.0 - p0));
        var llr = totalGames * logRatio;

        // Decision boundaries
        // Accept H1 if LLR > log((1-beta)/alpha)
        // Accept H0 if LLR < log(beta/(1-alpha))
        var upperBound = Math.Log((1.0 - beta) / alpha);
        var lowerBound = Math.Log(beta / (1.0 - alpha));

        if (llr > upperBound) return SPRTResult.H1;
        if (llr < lowerBound) return SPRTResult.H0;
        return SPRTResult.Continue;
    }

    /// <summary>
    /// Error function (Gauss error function).
    /// Used for calculating cumulative normal distribution.
    /// erf(x) = 2/sqrt(pi) * integral from 0 to x of e^(-t^2) dt
    /// </summary>
    /// <remarks>
    /// Approximation using Abramowitz and Stegun formula 7.1.26.
    /// Maximum error: 3 × 10^(-7)
    /// </remarks>
    private static double Erf(double x)
    {
        // Constants for approximation
        const double a1 = 0.254829592;
        const double a2 = -0.284496736;
        const double a3 = 1.421413741;
        const double a4 = -1.453152027;
        const double a5 = 1.061405429;
        const double p = 0.3275911;

        var sign = x < 0 ? -1.0 : 1.0;
        x = Math.Abs(x);

        var t = 1.0 / (1.0 + p * x);
        var y = 1.0 - (((((a5 * t + a4) * t) + a3) * t + a2) * t + a1) * t * Math.Exp(-x * x);

        return sign * y;
    }
}
