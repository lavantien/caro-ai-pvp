using System.Diagnostics;

namespace Caro.Core.GameLogic;

/// <summary>
/// Timer-based time monitor that polls at fixed intervals to check time limits.
/// More accurate and less taxing than node-count-based checking.
/// Uses PeriodicTimer for efficient async polling.
/// </summary>
public sealed class TimeMonitor : IDisposable
{
    private readonly Stopwatch _stopwatch;
    private readonly CancellationTokenSource _cts;
    private readonly PeriodicTimer _timer;
    private readonly long _hardTimeBoundMs;
    private readonly long _softTimeBoundMs;
    private readonly Action? _onTimeUp;
    private readonly Task _monitorTask;
    private bool _disposed;
    private bool _timeUpTriggered;

    /// <summary>
    /// Time interval between checks (10ms - accurate enough for game timing)
    /// </summary>
    private static readonly TimeSpan CheckInterval = TimeSpan.FromMilliseconds(10);

    /// <summary>
    /// Create a new time monitor.
    /// </summary>
    /// <param name="hardTimeBoundMs">Hard time limit in milliseconds</param>
    /// <param name="softTimeBoundMs">Soft time limit for early termination (optional)</param>
    /// <param name="cts">CancellationTokenSource to cancel when time is up</param>
    /// <param name="onTimeUp">Optional callback when time is up</param>
    public TimeMonitor(
        long hardTimeBoundMs,
        CancellationTokenSource cts,
        long softTimeBoundMs = long.MaxValue,
        Action? onTimeUp = null)
    {
        _stopwatch = Stopwatch.StartNew();
        _cts = cts;
        _hardTimeBoundMs = hardTimeBoundMs;
        _softTimeBoundMs = softTimeBoundMs;
        _onTimeUp = onTimeUp;
        _timer = new PeriodicTimer(CheckInterval);
        _monitorTask = MonitorLoopAsync();
    }

    /// <summary>
    /// Get elapsed milliseconds since monitor started.
    /// </summary>
    public long ElapsedMs => _stopwatch.ElapsedMilliseconds;

    /// <summary>
    /// Get remaining time in milliseconds.
    /// </summary>
    public long RemainingMs => Math.Max(0, _hardTimeBoundMs - _stopwatch.ElapsedMilliseconds);

    /// <summary>
    /// Check if time is up.
    /// </summary>
    public bool IsTimeUp => _timeUpTriggered || _stopwatch.ElapsedMilliseconds >= _hardTimeBoundMs;

    /// <summary>
    /// Check if soft time bound has been reached.
    /// </summary>
    public bool IsSoftTimeReached => _stopwatch.ElapsedMilliseconds >= _softTimeBoundMs;

    /// <summary>
    /// Background task that monitors time and cancels when limit is reached.
    /// </summary>
    private async Task MonitorLoopAsync()
    {
        try
        {
            while (await _timer.WaitForNextTickAsync(_cts.Token).ConfigureAwait(false))
            {
                if (_stopwatch.ElapsedMilliseconds >= _hardTimeBoundMs)
                {
                    _timeUpTriggered = true;
                    _onTimeUp?.Invoke();
                    _cts.Cancel();
                    break;
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Expected when search completes before time is up
        }
        catch (ObjectDisposedException)
        {
            // Expected when disposed while timer is waiting
        }
    }

    /// <summary>
    /// Stop the monitor and release resources.
    /// </summary>
    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;

        _stopwatch.Stop();
        _timer.Dispose();

        // Cancel the monitor task if still running
        try
        {
            _cts.Cancel();
        }
        catch (ObjectDisposedException)
        {
            // Already disposed
        }

        // Wait for monitor task to complete (with timeout to avoid hanging)
        try
        {
            _monitorTask.Wait(TimeSpan.FromMilliseconds(50));
        }
        catch
        {
            // Ignore any exceptions during cleanup
        }
    }
}
