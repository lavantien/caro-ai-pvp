using System.Threading.Channels;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Logging;

/// <summary>
/// Structured logging infrastructure for search operations.
/// Provides thread-safe async logging of search statistics for analysis and debugging.
///
/// Uses channels for producer/consumer pattern with file rotation.
/// </summary>
public sealed class SearchLogger : IDisposable
{
    private readonly Channel<SearchLogEntry> _logChannel;
    private readonly CancellationTokenSource _cts;
    private readonly Task _processingTask;
    private readonly string _logDirectory;
    private readonly string _logFilePrefix;
    private readonly DateTime _startDate;
    private readonly TimeSpan _rotationInterval;
    private long _currentFileSize;

    /// <summary>
    /// Maximum log file size before rotation (100 MB default).
    /// </summary>
    public const long MaxFileSizeBytes = 100 * 1024 * 1024;

    /// <summary>
    /// Create a new search logger.
    /// </summary>
    /// <param name="logDirectory">Directory for log files (default: backend/logs/)</param>
    /// <param name="filePrefix">Prefix for log files (default: "search-")</param>
    /// <param name="rotationInterval">Time-based rotation interval (default: 24 hours)</param>
    public SearchLogger(
        string? logDirectory = null,
        string filePrefix = "search-",
        TimeSpan? rotationInterval = null)
    {
        _logDirectory = logDirectory ?? Path.Combine(
            Directory.GetCurrentDirectory(), "..", "..", "logs");
        _logFilePrefix = filePrefix;
        _rotationInterval = rotationInterval ?? TimeSpan.FromHours(24);
        _startDate = DateTime.UtcNow;
        _currentFileSize = 0;
        _cts = new CancellationTokenSource();

        // Ensure log directory exists
        Directory.CreateDirectory(_logDirectory);

        // Create unbounded channel for logging
        _logChannel = Channel.CreateUnbounded<SearchLogEntry>(new UnboundedChannelOptions
        {
            SingleReader = true,
            SingleWriter = false
        });

        // Start background processing task
        _processingTask = ProcessLogEntriesAsync(_cts.Token);
    }

    /// <summary>
    /// Log a search entry.
    /// </summary>
    public void Log(SearchLogEntry entry)
    {
        if (entry == null)
            return;

        // Add timestamp if not set using with expression
        var entryWithTimestamp = entry.TimestampMs == 0
            ? entry with { TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() }
            : entry;

        // Try to write without blocking
        while (!_logChannel.Writer.TryWrite(entryWithTimestamp))
        {
            // Channel is full, wait a bit
            Thread.Sleep(1);
        }
    }

    /// <summary>
    /// Log a search completion event with performance stats.
    /// </summary>
    public void LogSearchCompletion(
        string playerId,
        Player player,
        int depth,
        long nodes,
        double nps,
        int ttHits,
        int ttProbes,
        long timeMs,
        int score,
        (int x, int y)? bestMove)
    {
        Log(new SearchLogEntry
        {
            PlayerId = playerId,
            Player = player,
            EntryType = SearchLogEntryType.SearchComplete,
            Depth = depth,
            Nodes = nodes,
            NodesPerSecond = nps,
            TTHits = ttHits,
            TTProbes = ttProbes,
            TTHitRate = ttProbes > 0 ? (double)ttHits / ttProbes : 0.0,
            TimeMs = timeMs,
            Score = score,
            BestMove = bestMove,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        });
    }

    /// <summary>
    /// Log an iteration during iterative deepening.
    /// </summary>
    public void LogIteration(
        string playerId,
        int depth,
        long nodes,
        int score,
        (int x, int y)? bestMove,
        long timeMs)
    {
        Log(new SearchLogEntry
        {
            PlayerId = playerId,
            EntryType = SearchLogEntryType.Iteration,
            Depth = depth,
            Nodes = nodes,
            Score = score,
            BestMove = bestMove,
            TimeMs = timeMs,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        });
    }

    /// <summary>
    /// Log a TT operation.
    /// </summary>
    public void LogTTOperation(
        string playerId,
        ulong hash,
        int depth,
        bool found,
        int? score,
        (int x, int y)? bestMove)
    {
        Log(new SearchLogEntry
        {
            PlayerId = playerId,
            EntryType = SearchLogEntryType.TTProbe,
            Hash = hash,
            Depth = depth,
            TTFound = found,
            Score = score ?? 0,
            BestMove = bestMove,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        });
    }

    /// <summary>
    /// Get current log file path.
    /// </summary>
    public string GetCurrentLogPath()
    {
        string dateStr = _startDate.ToString("yyyy-MM-dd");
        return Path.Combine(_logDirectory, $"{_logFilePrefix}{dateStr}.log");
    }

    /// <summary>
    /// Background task to process log entries and write to file.
    /// </summary>
    private async Task ProcessLogEntriesAsync(CancellationToken cancellationToken)
    {
        await foreach (var entry in _logChannel.Reader.ReadAllAsync(cancellationToken))
        {
            try
            {
                await WriteEntryAsync(entry);
            }
            catch when (cancellationToken.IsCancellationRequested)
            {
                break;
            }
            catch (Exception ex)
            {
                // Log to console as fallback
                Console.Error.WriteLine($"SearchLogger error: {ex.Message}");
            }
        }
    }

    /// <summary>
    /// Write a log entry to the file.
    /// </summary>
    private async Task WriteEntryAsync(SearchLogEntry entry)
    {
        string logPath = GetCurrentLogPath();

        // Check rotation
        if (_currentFileSize > MaxFileSizeBytes ||
            (DateTime.UtcNow - _startDate) > _rotationInterval)
        {
            await RotateLogAsync();
        }

        // Format: JSON line
        string json = System.Text.Json.JsonSerializer.Serialize(entry);
        string line = $"{json}\n";

        // Write to file
        using var writer = new StreamWriter(
            logPath,
            append: true,
            System.Text.Encoding.UTF8);
        await writer.WriteAsync(line);

        _currentFileSize += System.Text.Encoding.UTF8.GetByteCount(line);
    }

    /// <summary>
    /// Rotate log file.
    /// </summary>
    private async Task RotateLogAsync()
    {
        string oldPath = GetCurrentLogPath();
        string timestamp = DateTime.UtcNow.ToString("yyyyMMdd-HHmmss");
        string archivePath = Path.Combine(
            _logDirectory,
            $"{_logFilePrefix}{_startDate:yyyy-MM-dd}.{timestamp}.log");

        // Rename current file
        if (File.Exists(oldPath))
        {
            File.Move(oldPath, archivePath);
        }

        _currentFileSize = 0;
    }

    /// <summary>
    /// Flush all pending log entries to disk.
    /// Waits until all entries currently in the channel have been written.
    /// </summary>
    public async Task FlushAsync(TimeSpan? timeout = null)
    {
        timeout ??= TimeSpan.FromSeconds(5);

        // Mark the channel as complete so the reader will drain it
        _logChannel.Writer.Complete();

        try
        {
            // Wait for the processing task to finish draining the channel
            using var cts = new CancellationTokenSource(timeout.Value);
            await _processingTask.WaitAsync(cts.Token);
        }
        catch (TimeoutException)
        {
            // Timeout waiting for flush
        }
        catch (OperationCanceledException)
        {
            // Cancellation requested
        }
    }

    /// <summary>
    /// Dispose of the logger and flush remaining entries.
    /// </summary>
    public void Dispose()
    {
        _cts.Cancel();

        try
        {
            _processingTask.Wait(TimeSpan.FromSeconds(5));
        }
        catch (OperationCanceledException)
        {
            // Expected when cancellation token is signaled
        }
        catch (AggregateException ex) when (ex.InnerException is OperationCanceledException)
        {
            // Expected when cancellation token is signaled
        }
        finally
        {
            _cts.Dispose();
        }
    }
}

/// <summary>
/// Structured log entry for search operations.
/// </summary>
public sealed record SearchLogEntry
{
    public string PlayerId { get; init; } = "Unknown";
    public Player Player { get; init; } = Player.None;
    public SearchLogEntryType EntryType { get; init; } = SearchLogEntryType.Info;
    public int Depth { get; init; }
    public long Nodes { get; init; }
    public double NodesPerSecond { get; init; }
    public int TTHits { get; init; }
    public int TTProbes { get; init; }
    public double TTHitRate { get; init; }
    public long TimeMs { get; init; }
    public int Score { get; init; }
    public (int x, int y)? BestMove { get; init; }
    public ulong Hash { get; init; }
    public bool TTFound { get; init; }
    public string? Message { get; init; }
    public long TimestampMs { get; init; }
    public string? PrincipalVariation { get; init; }
}

/// <summary>
/// Types of search log entries.
/// </summary>
public enum SearchLogEntryType
{
    /// <summary>General information message</summary>
    Info,
    /// <summary>Search iteration completed</summary>
    Iteration,
    /// <summary>Entire search completed</summary>
    SearchComplete,
    /// <summary>Transposition table probe</summary>
    TTProbe,
    /// <summary>Transposition table store</summary>
    TTStore,
    /// <summary>Cutoff occurred</summary>
    Cutoff,
    /// <summary>Time management decision</summary>
    TimeDecision,
    /// <summary>Error or warning</summary>
    Error
}
