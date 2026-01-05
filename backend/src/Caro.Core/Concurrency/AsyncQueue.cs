using System.Threading.Channels;

namespace Caro.Core.Concurrency;

/// <summary>
/// Channel-based async queue for fire-and-forget operations with proper exception handling.
/// Provides backpressure and prevents unbounded memory growth.
/// Replaces fire-and-forget Task.Run patterns for better thread safety.
/// </summary>
/// <typeparam name="T">The type of items in the queue (must be non-nullable)</typeparam>
public sealed class AsyncQueue<T> : IDisposable where T : notnull
{
    private readonly Channel<T> _channel;
    private readonly CancellationTokenSource _cts;
    private readonly Task _processingTask;
    private readonly Func<T, ValueTask> _processAsync;
    private readonly string _queueName;
    private long _processedCount;
    private long _errorCount;
    private long _droppedCount;

    /// <summary>
    /// Gets the number of items processed successfully
    /// </summary>
    public long ProcessedCount => Interlocked.Read(ref _processedCount);

    /// <summary>
    /// Gets the number of processing errors encountered
    /// </summary>
    public long ErrorCount => Interlocked.Read(ref _errorCount);

    /// <summary>
    /// Gets the number of items dropped due to full queue
    /// </summary>
    public long DroppedCount => Interlocked.Read(ref _droppedCount);

    /// <summary>
    /// Creates a new AsyncQueue with bounded capacity
    /// </summary>
    /// <param name="processAsync">The async function to process each item</param>
    /// <param name="capacity">Maximum queue capacity (default: 100)</param>
    /// <param name="queueName">Optional name for diagnostics (default: "AsyncQueue")</param>
    /// <param name="dropOldest">Whether to drop oldest items when full (default: true)</param>
    public AsyncQueue(
        Func<T, ValueTask> processAsync,
        int capacity = 100,
        string queueName = "AsyncQueue",
        bool dropOldest = true)
    {
        _processAsync = processAsync ?? throw new ArgumentNullException(nameof(processAsync));
        _queueName = queueName;

        var options = new BoundedChannelOptions(capacity)
        {
            FullMode = dropOldest
                ? BoundedChannelFullMode.DropOldest
                : BoundedChannelFullMode.Wait
        };

        _channel = Channel.CreateBounded<T>(options);
        _cts = new CancellationTokenSource();
        _processingTask = ProcessAsync(_cts.Token);
    }

    /// <summary>
    /// Attempts to enqueue an item without blocking.
    /// Returns false if the queue is full and DropOldest is false.
    /// </summary>
    /// <param name="item">The item to enqueue</param>
    /// <returns>True if the item was enqueued, false if the queue is full</returns>
    public bool TryEnqueue(T item)
    {
        if (item == null)
            throw new ArgumentNullException(nameof(item));

        var result = _channel.Writer.TryWrite(item);

        if (!result)
        {
            Interlocked.Increment(ref _droppedCount);
        }

        return result;
    }

    /// <summary>
    /// Enqueues an item asynchronously, waiting if the queue is full.
    /// </summary>
    /// <param name="item">The item to enqueue</param>
    /// <param name="cancellationToken">Optional cancellation token</param>
    public async ValueTask EnqueueAsync(T item, CancellationToken cancellationToken = default)
    {
        if (item == null)
            throw new ArgumentNullException(nameof(item));

        try
        {
            await _channel.Writer.WriteAsync(item, cancellationToken);
        }
        catch (ChannelClosedException)
        {
            throw;
        }
    }

    /// <summary>
    /// Background processing loop that reads from the channel and processes items
    /// Uses ConfigureAwait(false) to avoid deadlocks when Dispose blocks on Wait()
    /// </summary>
    private async Task ProcessAsync(CancellationToken ct)
    {
        try
        {
            await foreach (var item in _channel.Reader.ReadAllAsync(ct).ConfigureAwait(false))
            {
                try
                {
                    await _processAsync(item).ConfigureAwait(false);
                    Interlocked.Increment(ref _processedCount);
                }
                catch (Exception)
                {
                    Interlocked.Increment(ref _errorCount);
                    // Continue processing other items despite error
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Expected during shutdown
        }
    }

    /// <summary>
    /// Gets statistics about the queue performance
    /// </summary>
    public (long processed, long errors, long dropped) GetStatistics()
    {
        return (
            Interlocked.Read(ref _processedCount),
            Interlocked.Read(ref _errorCount),
            Interlocked.Read(ref _droppedCount)
        );
    }

    /// <summary>
    /// Disposes the queue, waiting for processing to complete gracefully
    /// </summary>
    public void Dispose()
    {
        // Signal no more items will be added
        _channel.Writer.Complete();

        // Cancel processing
        _cts.Cancel();

        // Wait for processing to complete (with timeout)
        if (!_processingTask.Wait(TimeSpan.FromSeconds(5)))
        {
            // Timeout - force cleanup
        }

        _cts.Dispose();
    }
}
