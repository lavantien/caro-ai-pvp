using System.Runtime.CompilerServices;

namespace Caro.Core.GameLogic;

/// <summary>
/// Contest manager for dynamic contempt factor calculation.
/// Contempt adjusts evaluation based on game state and opponent strength.
///
/// Positive contempt = play more aggressively (take risks)
/// Negative contempt = play more conservatively (simplify position)
///
/// Formula: contempt = baseContempt + positionAdjustment + difficultyAdjustment
/// Expected ELO gain: +5-20 through better playstyle adaptation
/// </summary>
public sealed class ContestManager
{
    private const int MinContempt = -200;
    private const int MaxContempt = 200;

    private readonly int _initialBaseContempt;
    private int _baseContempt;

    /// <summary>
    /// Create a new contest manager with specified base contempt.
    /// </summary>
    /// <param name="baseContempt">Base contempt value in centipawns (default 20)</param>
    public ContestManager(int baseContempt = 20)
    {
        _initialBaseContempt = baseContempt;
        _baseContempt = Math.Clamp(baseContempt, MinContempt / 2, MaxContempt / 2);
    }

    /// <summary>
    /// Calculate contempt factor based on current position and opponent difficulty.
    /// </summary>
    /// <param name="eval">Current evaluation score in centipawns (positive = winning)</param>
    /// <param name="estimatedDifficulty">Estimated opponent difficulty (0.0 = weak, 1.0 = strong)</param>
    /// <returns>Contempt value in centipawns, clamped to [-200, 200]</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int CalculateContempt(int eval, double estimatedDifficulty)
    {
        // Position adjustment: modify contempt based on evaluation
        // - Winning (positive eval): reduce contempt to simplify (can be negative)
        // - Equal (eval near 0): use base contempt for aggressive play
        // - Losing (negative eval): increase contempt to complicate and seek chances
        int positionAdjustment = CalculatePositionAdjustment(eval);

        // Difficulty adjustment: higher difficulty opponents get more contempt
        // This means we play more aggressively against stronger opponents
        int difficultyAdjustment = CalculateDifficultyAdjustment(estimatedDifficulty);

        int contempt = _baseContempt + positionAdjustment + difficultyAdjustment;

        return Math.Clamp(contempt, MinContempt, MaxContempt);
    }

    /// <summary>
    /// Calculate position-based contempt adjustment.
    /// Returns positive adjustment when losing (need more aggression)
    /// Returns negative adjustment when winning (play safer)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CalculatePositionAdjustment(int eval)
    {
        // Sigmoid-like transition based on evaluation
        // When losing badly (eval < -200): +50 to +150 contempt adjustment
        // When losing slightly (eval around -100): +20 to +50 contempt adjustment
        // When equal (eval around 0): 0 adjustment (use base contempt)
        // When winning slightly (eval around +100): -20 to -50 contempt adjustment
        // When winning badly (eval > +200): -50 to -150 contempt adjustment

        // The adjustment should be:
        // - Positive when losing (we want to increase contempt)
        // - Negative when winning (we want to decrease contempt)
        // This naturally comes from negating the eval: positive eval = negative adjustment

        int adjustment;
        if (eval < -200)
        {
            // Badly losing - significantly increase contempt
            adjustment = 50 + Math.Min(100, (-eval - 200) / 3);
        }
        else if (eval < -100)
        {
            // Slightly losing - moderately increase contempt
            adjustment = 25 + (-eval - 100) / 2;
        }
        else if (eval < 0)
        {
            // Small disadvantage - slight increase
            adjustment = (-eval) / 2; // 0 to 50
        }
        else if (eval < 100)
        {
            // Small advantage - slight decrease
            adjustment = -(eval / 2); // 0 to -50
        }
        else if (eval < 200)
        {
            // Slightly winning - moderately decrease contempt
            adjustment = -(25 + (eval - 100) / 2);
        }
        else
        {
            // Winning badly - significantly decrease contempt
            adjustment = -(50 + Math.Min(100, (eval - 200) / 3));
        }

        return adjustment;
    }

    /// <summary>
    /// Calculate difficulty-based contempt adjustment.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CalculateDifficultyAdjustment(double estimatedDifficulty)
    {
        // Difficulty is 0.0 to 1.0
        // Higher difficulty = more contempt (play aggressively)
        // Lower difficulty = less contempt (can play more conservatively)

        // Scale: difficulty 0.0 -> -20 adjustment
        //          difficulty 0.5 -> 0 adjustment
        //          difficulty 1.0 -> +40 adjustment

        return (int)((estimatedDifficulty - 0.5) * 60);
    }

    /// <summary>
    /// Get or set the base contempt value.
    /// </summary>
    public int BaseContempt
    {
        get => _baseContempt;
        set => _baseContempt = Math.Clamp(value, MinContempt / 2, MaxContempt / 2);
    }

    /// <summary>
    /// Reset the contest manager to initial state.
    /// </summary>
    public void Reset()
    {
        _baseContempt = _initialBaseContempt;
    }

    /// <summary>
    /// Get the minimum contempt value.
    /// </summary>
    public static int MinContemptValue => MinContempt;

    /// <summary>
    /// Get the maximum contempt value.
    /// </summary>
    public static int MaxContemptValue => MaxContempt;
}
