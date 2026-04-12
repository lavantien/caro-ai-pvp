namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized timeout and interval constants for time-sensitive operations.
/// </summary>
public static class TimeConstants
{
    // TimeMonitor
    public const int MonitorCheckIntervalMs = 10;
    public const int MonitorDisposalTimeoutMs = 50;

    // AsyncQueue
    public const int QueueDefaultCapacity = 100;
    public const int QueueDisposalTimeoutSeconds = 5;

    // UCIProtocol
    public const int DefaultHashSizeMb = 256;
    public const int UCICommandTimeoutSeconds = 30;

    // SearchLogger
    public const int MaxLogFileSizeMb = 100;
    public const int LogRotationHours = 24;
    public const int LogProcessingSleepMs = 1;
    public const int LogTimeoutSeconds = 5;

    // UCIMockClient
    public const int MockUCIInitTimeoutSeconds = 5;
    public const int MockBestMoveTimeoutMinutes = 5;
    public const int MockProcessPollIntervalMs = 100;
    public const int MockProcessExitTimeoutMs = 1000;

    // Ponderer
    public const int DefaultMaxPonderDepth = 20;
    public const int PonderStopTimeoutMs = 500;
    public const int PonderMinElapsedMs = 10;
    public const int PonderDisposalTimeoutMs = 50;

    // DFPNSearch / ThreatSpaceSearch
    public const uint DFPNInfinity = 1_000_000;
    public const int DefaultSearchDepth = 30;
    public const int DefaultTimeLimitMs = 1000;
    public const int MaxDefensesPerThreat = 10;
    public const int MaxCandidateMoves = 10;

    // HardBound buffer
    public const int HardBoundBufferMs = 200;
    public const int MinRemainingHardBoundMs = 50;
    public const int MinSoftBoundFallbackMs = 25;
}
