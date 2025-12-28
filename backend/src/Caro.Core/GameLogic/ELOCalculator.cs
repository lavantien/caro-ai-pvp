namespace Caro.Core.GameLogic;

/// <summary>
/// Calculates ELO ratings for players using the standard ELO formula
/// </summary>
public class ELOCalculator
{
    private const int KFactor = 32;

    /// <summary>
    /// Calculate the new rating for a player after a game
    /// </summary>
    /// <param name="playerRating">Player's current rating</param>
    /// <param name="opponentRating">Opponent's rating</param>
    /// <param name="won">True if player won, false if lost</param>
    /// <param name="difficultyMultiplier">Optional multiplier for AI difficulty (default 1.0)</param>
    /// <returns>New rating for the player</returns>
    public int CalculateNewRating(
        int playerRating,
        int opponentRating,
        bool won,
        double difficultyMultiplier = 1.0)
    {
        var expectedScore = CalculateExpectedScore(playerRating, opponentRating);
        var actualScore = won ? 1.0 : 0.0;

        // Apply K-factor with difficulty multiplier
        var ratingChange = KFactor * difficultyMultiplier * (actualScore - expectedScore);

        return (int)Math.Round(playerRating + ratingChange);
    }

    /// <summary>
    /// Calculate the expected score for a player based on ratings
    /// </summary>
    /// <param name="playerRating">Player's rating</param>
    /// <param name="opponentRating">Opponent's rating</param>
    /// <returns>Expected score (0.0 to 1.0)</returns>
    public double CalculateExpectedScore(int playerRating, int opponentRating)
    {
        var ratingDifference = opponentRating - playerRating;
        return 1.0 / (1.0 + Math.Pow(10, ratingDifference / 400.0));
    }
}
