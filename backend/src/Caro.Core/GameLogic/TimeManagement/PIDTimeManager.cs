using System.Runtime.CompilerServices;

namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// PID-based time manager for dynamic time allocation in AI search.
/// Uses Proportional-Integral-Derivative control to adjust time based on how
/// well the engine is staying on target time budget.
/// 
/// Formula: adjustment = Kp*error + Ki*integral + Kd*derivative
/// Where:
/// - error = remainingTime - (targetTime * movesRemaining)
/// - integral = accumulated error over time
/// - derivative = rate of change of error
/// 
/// Expected ELO gain: +20-50 through better time management
/// </summary>
public sealed class PIDTimeManager
{
    private const double DefaultKp = 1.0;
    private const double DefaultKi = 0.1;
    private const double DefaultKd = 0.5;

    private readonly double _kp;
    private readonly double _ki;
    private readonly double _kd;
    private readonly double _targetTimeMs;

    private double _integral;
    private double _lastError;
    private bool _initialized;

    /// <summary>
    /// Create a new PID time manager with tunable parameters.
    /// </summary>
    /// <param name="targetTimeMs">Target time per move in milliseconds</param>
    /// <param name="remainingMoves">Estimated number of moves remaining in game</param>
    /// <param name="kp">Proportional gain (default 1.0)</param>
    /// <param name="ki">Integral gain (default 0.1)</param>
    /// <param name="kd">Derivative gain (default 0.5)</param>
    public PIDTimeManager(
        double targetTimeMs,
        int remainingMoves,
        double kp = DefaultKp,
        double ki = DefaultKi,
        double kd = DefaultKd)
    {
        if (targetTimeMs <= 0)
            throw new ArgumentOutOfRangeException(nameof(targetTimeMs), "Target time must be positive");
        if (remainingMoves <= 0)
            throw new ArgumentOutOfRangeException(nameof(remainingMoves), "Remaining moves must be positive");

        _kp = kp;
        _ki = ki;
        _kd = kd;
        _targetTimeMs = targetTimeMs;
        _integral = 0;
        _lastError = 0;
        _initialized = false;
    }

    /// <summary>
    /// Calculate time adjustment based on current time state.
    /// Positive value = use more time (behind schedule)
    /// Negative value = use less time (ahead of schedule)
    /// </summary>
    /// <param name="remainingTimeMs">Total remaining time in milliseconds</param>
    /// <param name="movesRemaining">Estimated moves remaining</param>
    /// <returns>Time adjustment in milliseconds (can be negative)</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public double CalculateTimeAdjustment(double remainingTimeMs, int movesRemaining)
    {
        if (movesRemaining <= 0)
            return 0;

        // Calculate expected time for remaining moves
        double expectedTime = _targetTimeMs * movesRemaining;

        // Error: positive = behind (need more time), negative = ahead (can slow down)
        // When we have LESS time than expected, we're behind and need positive adjustment
        double error = expectedTime - remainingTimeMs;

        // Clamp error to reasonable bounds to prevent extreme adjustments
        error = Math.Clamp(error, -expectedTime * 2, expectedTime * 2);

        // Update integral with clamping to prevent windup
        _integral += error;
        _integral = Math.Clamp(_integral, -_targetTimeMs * 10, _targetTimeMs * 10);

        // Calculate derivative (rate of change)
        double derivative = _initialized ? error - _lastError : 0;
        _lastError = error;
        _initialized = true;

        // PID formula
        double adjustment = (_kp * error) + (_ki * _integral) + (_kd * derivative);

        // Clamp adjustment to reasonable bounds (max 200% of target time)
        return Math.Clamp(adjustment, -_targetTimeMs * 2, _targetTimeMs * 2);
    }

    /// <summary>
    /// Reset the PID controller state.
    /// Call this at the start of a new game.
    /// </summary>
    public void Reset()
    {
        _integral = 0;
        _lastError = 0;
        _initialized = false;
    }

    /// <summary>
    /// Get the target time per move in milliseconds.
    /// </summary>
    public double TargetTimeMs => _targetTimeMs;

    /// <summary>
    /// Get the current accumulated integral term (for diagnostics).
    /// </summary>
    public double IntegralTerm => _integral;

    /// <summary>
    /// Get the last calculated error (for diagnostics).
    /// </summary>
    public double LastError => _lastError;
}
