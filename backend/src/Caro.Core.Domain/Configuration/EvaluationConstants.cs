namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized AI evaluation scoring constants.
/// These values determine how the AI evaluates board positions.
/// </summary>
public static class EvaluationConstants
{
    /// <summary>
    /// Score for five stones in a row (winning position)
    /// </summary>
    public const int FiveInRowScore = 100_000;

    /// <summary>
    /// Score for an open four (four in a row with both ends open)
    /// </summary>
    public const int OpenFourScore = 10_000;

    /// <summary>
    /// Score for a closed four (four in a row with one end blocked)
    /// </summary>
    public const int ClosedFourScore = 1_000;

    /// <summary>
    /// Score for an open three (three in a row with both ends open)
    /// </summary>
    public const int OpenThreeScore = 1_000;

    /// <summary>
    /// Score for a closed three (three in a row with one end blocked)
    /// </summary>
    public const int ClosedThreeScore = 100;

    /// <summary>
    /// Score for an open two (two in a row with both ends open)
    /// </summary>
    public const int OpenTwoScore = 100;

    /// <summary>
    /// Bonus score for center control
    /// </summary>
    public const int CenterBonus = 50;

    /// <summary>
    /// Defense multiplier for asymmetric scoring (as numerator for integer math).
    /// In Caro, blocking opponent threats is MORE important than creating your own.
    /// </summary>
    public const int DefenseMultiplierNumerator = 3;

    /// <summary>
    /// Defense multiplier denominator (DefenseMultiplier = 3/2 = 1.5)
    /// </summary>
    public const int DefenseMultiplierDenominator = 2;
}
