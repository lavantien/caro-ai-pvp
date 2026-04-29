namespace Caro.Core.Domain.Exceptions;

/// <summary>
/// Exception thrown when an AI search times out
/// </summary>
public sealed class TimeoutException : OperationCanceledException
{
    /// <summary>
    /// The time limit that was exceeded (milliseconds)
    /// </summary>
    public long TimeLimitMs { get; }

    /// <summary>
    /// The actual time elapsed (milliseconds)
    /// </summary>
    public long ElapsedMs { get; }

    /// <summary>
    /// The depth achieved before timeout
    /// </summary>
    public int DepthAchieved { get; }

    public TimeoutException(string message, long timeLimitMs, long elapsedMs, int depthAchieved)
        : base(message)
    {
        TimeLimitMs = timeLimitMs;
        ElapsedMs = elapsedMs;
        DepthAchieved = depthAchieved;
    }

    public TimeoutException(string message, long timeLimitMs, long elapsedMs, int depthAchieved, Exception innerException)
        : base(message, innerException)
    {
        TimeLimitMs = timeLimitMs;
        ElapsedMs = elapsedMs;
        DepthAchieved = depthAchieved;
    }
}
