namespace Caro.Core.GameLogic;

/// <summary>
/// Parameters for SPSA (Simultaneous Perturbation Stochastic Approximation) optimization.
/// SPSA is a gradient-free optimization algorithm that efficiently tunes high-dimensional parameters.
///
/// Expected ELO gain: +20-40 through optimized evaluation weights.
///
/// Reference: Spall, J. C. (1998). "An Overview of the Simultaneous Perturbation Method for Efficient Optimization"
/// </summary>
public sealed class SPSAParameters
{
    /// <summary>
    /// Alpha parameter: controls gain decay rate for parameter updates.
    /// Typical values: 0.602 for convergence, 0.101 for asymptotic efficiency.
    /// </summary>
    public double Alpha { get; }

    /// <summary>
    /// Gamma parameter: controls gain decay rate for perturbation.
    /// Typical value: 0.101 (1/6 of alpha for optimal efficiency).
    /// </summary>
    public double Gamma { get; }

    /// <summary>
    /// A: coefficient for parameter update gain (a_k = A / (k + A)^alpha).
    /// Larger A = more aggressive early exploration.
    /// </summary>
    public double A { get; }

    /// <summary>
    /// C: coefficient for perturbation magnitude (c_k = C / (k + 1)^gamma).
    /// Larger C = larger perturbations.
    /// </summary>
    public double C { get; }

    /// <summary>
    /// A_decay: denominator term for a_k calculation.
    /// Affects how quickly step size decreases.
    /// </summary>
    public double ADecay { get; }

    /// <summary>
    /// C_decay: denominator term for c_k calculation.
    /// Affects how quickly perturbation size decreases.
    /// </summary>
    public double CDecay { get; }

    /// <summary>
    /// Optional minimum values for parameter clamping.
    /// Null means no lower bound.
    /// </summary>
    public double[]? MinValues { get; }

    /// <summary>
    /// Optional maximum values for parameter clamping.
    /// Null means no upper bound.
    /// </summary>
    public double[]? MaxValues { get; }

    /// <summary>
    /// Create SPSA parameters with default values.
    /// </summary>
    public SPSAParameters(
        double alpha = 0.602,
        double gamma = 0.101,
        double a = 100.0,
        double c = 1.0,
        double a_decay = 100,
        double c_decay = 10,
        double[]? minValues = null,
        double[]? maxValues = null)
    {
        if (alpha <= 0 || alpha >= 1)
            throw new ArgumentOutOfRangeException(nameof(alpha), "Alpha must be in (0, 1)");
        if (gamma <= 0 || gamma >= 1)
            throw new ArgumentOutOfRangeException(nameof(gamma), "Gamma must be in (0, 1)");
        if (a <= 0)
            throw new ArgumentOutOfRangeException(nameof(a), "A must be positive");
        if (c <= 0)
            throw new ArgumentOutOfRangeException(nameof(c), "C must be positive");
        if (a_decay <= 0)
            throw new ArgumentOutOfRangeException(nameof(a_decay), "A_decay must be positive");
        if (c_decay <= 0)
            throw new ArgumentOutOfRangeException(nameof(c_decay), "C_decay must be positive");

        Alpha = alpha;
        Gamma = gamma;
        A = a;
        C = c;
        ADecay = a_decay;
        CDecay = c_decay;
        MinValues = minValues;
        MaxValues = maxValues;
    }

    /// <summary>
    /// Create default parameters for engine tuning.
    /// Optimized for ~100-500 iteration tuning runs.
    /// </summary>
    public static SPSAParameters Default => new(
        alpha: 0.602,
        gamma: 0.101,
        a: 1000.0,
        c: 10.0,
        a_decay: 500,
        c_decay: 50);

    /// <summary>
    /// Create aggressive parameters for fast convergence.
    /// Larger steps, faster decay - good for quick prototyping.
    /// </summary>
    public static SPSAParameters Aggressive => new(
        alpha: 0.602,
        gamma: 0.101,
        a: 2000.0,
        c: 20.0,
        a_decay: 200,
        c_decay: 20);

    /// <summary>
    /// Create conservative parameters for fine-tuning.
    /// Smaller steps, slower decay - good for final optimization.
    /// </summary>
    public static SPSAParameters Conservative => new(
        alpha: 0.602,
        gamma: 0.101,
        a: 500.0,
        c: 5.0,
        a_decay: 1000,
        c_decay: 100);
}
