namespace Caro.Core.GameLogic;

/// <summary>
/// Persistent worker pool for Lazy SMP parallel search.
/// Maintains dedicated threads that wait for search tasks, eliminating
/// ThreadPool injection delays (5-20ms per move in Bullet games).
///
/// Usage:
/// 1. Create pool once at engine initialization
/// 2. Call SearchAsync for each move
/// 3. Dispose when engine shuts down
/// </summary>
public sealed class PersistentWorkerPool : IDisposable
{
    private readonly Thread[] _workers;
    private readonly ManualResetEventSlim _workAvailable = new(false);
    private readonly ManualResetEventSlim _workComplete = new(false);
    private readonly object _lock = new();

    // Task parameters (set before signaling workers)
    private Func<int, (int x, int y, int score, int depth, long nodes)>? _taskFunc;
    private int _taskCount;
    private readonly (int x, int y, int score, int depth, long nodes)[] _results;
    private readonly Exception?[] _exceptions;
    private int _completedCount;
    private CancellationToken _cancellationToken;
    private bool _disposed;

    /// <summary>
    /// Create a pool of persistent worker threads.
    /// </summary>
    /// <param name="workerCount">Number of worker threads to create</param>
    public PersistentWorkerPool(int workerCount)
    {
        _workers = new Thread[workerCount];
        _results = new (int x, int y, int score, int depth, long nodes)[workerCount];
        _exceptions = new Exception?[workerCount];

        for (int i = 0; i < workerCount; i++)
        {
            _workers[i] = new Thread(WorkerLoop)
            {
                IsBackground = true,
                Priority = ThreadPriority.AboveNormal,
                Name = $"CaroWorker-{i}"
            };
            _workers[i].Start(i);
        }
    }

    /// <summary>
    /// Execute a search task on all workers in parallel.
    /// </summary>
    /// <param name="taskFunc">Function that takes threadId and returns search result</param>
    /// <param name="cancellationToken">Cancellation token for the search</param>
    /// <param name="timeoutMs">Maximum time to wait for completion</param>
    /// <returns>Array of results from all workers</returns>
    public (int x, int y, int score, int depth, long nodes)[] Search(
        Func<int, (int x, int y, int score, int depth, long nodes)> taskFunc,
        CancellationToken cancellationToken,
        int timeoutMs)
    {
        if (_disposed)
            throw new ObjectDisposedException(nameof(PersistentWorkerPool));

        lock (_lock)
        {
            // Reset state
            _taskFunc = taskFunc;
            _taskCount = _workers.Length;
            _completedCount = 0;
            _cancellationToken = cancellationToken;
            Array.Clear(_results, 0, _results.Length);
            Array.Clear(_exceptions, 0, _exceptions.Length);

            // Signal workers to start
            _workComplete.Reset();
            _workAvailable.Set();
        }

        // Wait for completion or timeout
        bool completed = _workComplete.Wait(timeoutMs, cancellationToken);

        lock (_lock)
        {
            // Signal workers to stop waiting for work
            _workAvailable.Reset();

            // If not all completed, return what we have
            return ((int x, int y, int score, int depth, long nodes)[])_results.Clone();
        }
    }

    private void WorkerLoop(object? state)
    {
        int threadId = (int)state!;

        while (!_disposed)
        {
            // Wait for work
            try
            {
                _workAvailable.Wait(_cancellationToken);
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (ObjectDisposedException)
            {
                break;
            }

            if (_disposed || _cancellationToken.IsCancellationRequested)
                break;

            // Execute task
            try
            {
                var taskFunc = _taskFunc;
                if (taskFunc != null)
                {
                    _results[threadId] = taskFunc(threadId);
                }
            }
            catch (OperationCanceledException)
            {
                // Expected when search is cancelled
            }
            catch (Exception ex)
            {
                _exceptions[threadId] = ex;
            }

            // Signal completion
            lock (_lock)
            {
                _completedCount++;
                if (_completedCount >= _taskCount)
                {
                    _workComplete.Set();
                }
            }
        }
    }

    /// <summary>
    /// Get the number of worker threads.
    /// </summary>
    public int WorkerCount => _workers.Length;

    /// <summary>
    /// Release all resources and stop worker threads.
    /// </summary>
    public void Dispose()
    {
        if (_disposed)
            return;

        _disposed = true;
        _workAvailable.Set(); // Wake up workers so they can exit
        _workComplete.Set();

        // Give workers time to exit gracefully
        foreach (var worker in _workers)
        {
            if (!worker.Join(100))
            {
                worker.Interrupt();
            }
        }

        _workAvailable.Dispose();
        _workComplete.Dispose();
    }
}
