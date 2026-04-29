namespace Caro.Core.GameLogic;

/// <summary>
/// Persistent worker pool for Lazy SMP parallel search.
/// Maintains dedicated threads that wait for search tasks, eliminating
/// ThreadPool injection delays and OS thread creation overhead per move.
///
/// Usage:
/// 1. Create pool once at engine initialization
/// 2. Call Search for each move
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
    private volatile bool _disposed;

    /// <summary>
    /// Create a pool of persistent worker threads.
    /// </summary>
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
    /// Blocks until all workers complete or timeout expires.
    /// </summary>
    public (int x, int y, int score, int depth, long nodes)[] Search(
        Func<int, (int x, int y, int score, int depth, long nodes)> taskFunc,
        int timeoutMs)
    {
        if (_disposed)
            throw new ObjectDisposedException(nameof(PersistentWorkerPool));

        lock (_lock)
        {
            _taskFunc = taskFunc;
            _taskCount = _workers.Length;
            _completedCount = 0;
            Array.Clear(_results, 0, _results.Length);
            Array.Clear(_exceptions, 0, _exceptions.Length);

            _workComplete.Reset();
            _workAvailable.Set();
        }

        _workComplete.Wait(timeoutMs);

        lock (_lock)
        {
            _workAvailable.Reset();
            return ((int x, int y, int score, int depth, long nodes)[])_results.Clone();
        }
    }

    private void WorkerLoop(object? state)
    {
        int threadId = (int)state!;

        while (!_disposed)
        {
            try
            {
                _workAvailable.Wait();
            }
            catch (ObjectDisposedException)
            {
                break;
            }

            if (_disposed) break;

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
                // Search cancelled — expected, worker stays alive for next search
            }
            catch (Exception ex)
            {
                _exceptions[threadId] = ex;
            }

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
        if (_disposed) return;

        _disposed = true;
        _workAvailable.Set();
        _workComplete.Set();

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
