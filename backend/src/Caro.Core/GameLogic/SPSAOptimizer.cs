using System.Runtime.CompilerServices;

namespace Caro.Core.GameLogic;

/// <summary>
/// SPSA (Simultaneous Perturbation Stochastic Approximation) optimizer.
///
/// Gradient-free optimization algorithm that efficiently tunes high-dimensional
/// parameters using only 2 function evaluations per iteration regardless of
/// parameter count.
///
/// Algorithm:
/// 1. Generate random perturbation vector delta with +/-1 entries
/// 2. Evaluate objective at theta + c*delta and theta - c*delta
/// 3. Approximate gradient: g = (y_plus - y_minus) / (2 * c * delta)
/// 4. Update: theta_new = theta_old - a * g
///
/// Expected ELO gain: +20-40 through optimized evaluation weights.
/// </summary>
public sealed class SPSAOptimizer
{
    private readonly SPSAParameters _parameters;
    private readonly Random _random;
    private int _iteration;

    /// <summary>
    /// Create a new SPSA optimizer.
    /// </summary>
    /// <param name="parameters">SPSA configuration parameters</param>
    /// <param name="seed">Optional random seed for reproducibility</param>
    public SPSAOptimizer(SPSAParameters parameters, int? seed = null)
    {
        _parameters = parameters ?? throw new ArgumentNullException(nameof(parameters));
        _random = seed.HasValue ? new Random(seed.Value) : new Random();
        _iteration = 0;
    }

    /// <summary>
    /// Generate a random perturbation vector with +/-1 entries (Bernoulli distribution).
    /// </summary>
    /// <param name="dimension">Number of parameters</param>
    /// <returns>Perturbation vector delta with values +/-1</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public double[] GeneratePerturbation(int dimension)
    {
        if (dimension <= 0)
            throw new ArgumentOutOfRangeException(nameof(dimension), "Dimension must be positive");

        double[] delta = new double[dimension];
        for (int i = 0; i < dimension; i++)
        {
            // Bernoulli distribution: +1 or -1 with equal probability
            delta[i] = _random.Next(2) == 0 ? 1.0 : -1.0;
        }

        return delta;
    }

    /// <summary>
    /// Update parameters based on two objective evaluations.
    /// Increments the iteration counter as part of the update.
    /// </summary>
    /// <param name="theta">Current parameter values</param>
    /// <param name="delta">Perturbation vector from GeneratePerturbation()</param>
    /// <param name="y_plus">Objective evaluation at theta + c*delta</param>
    /// <param name="y_minus">Objective evaluation at theta - c*delta</param>
    /// <returns>Updated parameter values</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public double[] UpdateParameters(double[] theta, double[] delta, double y_plus, double y_minus)
    {
        if (theta == null)
            throw new ArgumentNullException(nameof(theta));
        if (delta == null)
            throw new ArgumentNullException(nameof(delta));
        if (theta.Length != delta.Length)
            throw new ArgumentException("Theta and delta must have same length");

        int n = theta.Length;
        double[] newTheta = new double[n];

        // Increment iteration for this update
        _iteration++;

        // Get current gain coefficients
        double ak = GetGainA(_iteration);
        double ck = GetGainC(_iteration);

        // SPSA gradient approximation:
        // g_k = (y_plus - y_minus) / (2 * c_k * delta)
        // Each component: g_i = (y_plus - y_minus) / (2 * c_k * delta_i)
        // Since delta_i = +/-1, g_i = (y_plus - y_minus) * delta_i / (2 * c_k)

        double gradientScale = (y_plus - y_minus) / (2.0 * ck);

        // Update each parameter
        for (int i = 0; i < n; i++)
        {
            // Gradient approximation for component i
            double gi = gradientScale * delta[i];

            // Update: theta_new = theta_old - a_k * g_k
            // Note: For minimization, subtract. For maximization, add.
            // We assume minimization (lower is better), so:
            newTheta[i] = theta[i] - ak * gi;

            // Clamp to bounds if specified
            if (_parameters.MinValues != null && i < _parameters.MinValues.Length)
                newTheta[i] = Math.Max(newTheta[i], _parameters.MinValues[i]);
            if (_parameters.MaxValues != null && i < _parameters.MaxValues.Length)
                newTheta[i] = Math.Min(newTheta[i], _parameters.MaxValues[i]);
        }

        return newTheta;
    }

    /// <summary>
    /// Calculate parameter update gain at iteration k.
    /// a_k = A / (k + A_decay)^alpha
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public double GetGainA(int k)
    {
        return _parameters.A / Math.Pow(k + _parameters.ADecay, _parameters.Alpha);
    }

    /// <summary>
    /// Calculate perturbation magnitude at iteration k.
    /// c_k = C / (k + 1)^gamma
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public double GetGainC(int k)
    {
        return _parameters.C / Math.Pow(k + 1, _parameters.Gamma);
    }

    /// <summary>
    /// Get the current iteration number.
    /// </summary>
    public int CurrentIteration => _iteration;

    /// <summary>
    /// Reset the optimizer to initial state.
    /// Useful for starting a new optimization run.
    /// </summary>
    public void Reset()
    {
        _iteration = 0;
    }

    /// <summary>
    /// Get the current perturbation coefficient for the iteration.
    /// </summary>
    public double CurrentC => GetGainC(_iteration);

    /// <summary>
    /// Get the current update gain coefficient for the iteration.
    /// </summary>
    public double CurrentA => GetGainA(_iteration);
}
