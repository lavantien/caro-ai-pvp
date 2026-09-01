using System.Diagnostics;

namespace Caro.Engine;

/// <summary>
/// Per-search watchdog: a dedicated task ticks roughly every 10ms against
/// the hard bound; the search polls ShouldStop and the node counter.
/// </summary>
public sealed class TimeMonitor : IDisposable
{
    private readonly long _hardBoundMs;
    private readonly long _startTimeStamp;
    private readonly CancellationTokenSource _cts;
    private readonly CancellationTokenSource _linked;
    private readonly Task _watchTask;
    private int _stopped;
    private long _nodes;
    private readonly object _stopGate = new();

    public TimeMonitor(long hardBoundMs, CancellationToken externalToken)
    {
        _hardBoundMs = hardBoundMs;
        _startTimeStamp = Stopwatch.GetTimestamp();
        _cts = new CancellationTokenSource();
        _linked = CancellationTokenSource.CreateLinkedTokenSource(externalToken, _cts.Token);
        CancellationToken token = _linked.Token;
        // The token is passed to the loop, not to StartNew: a task created
        // with an already-cancelled token would transition to Canceled and
        // make the Dispose join throw instead of returning promptly.
        _watchTask = Task.Factory.StartNew(() => WatchLoop(token), CancellationToken.None,
            TaskCreationOptions.LongRunning, TaskScheduler.Default);
    }

    private void WatchLoop(CancellationToken token)
    {
        while (!token.IsCancellationRequested)
        {
            Thread.Sleep(10);
            if (ElapsedMs() >= _hardBoundMs)
            {
                Stop();
                return;
            }
        }
    }

    public long ElapsedMs() => (long)Stopwatch.GetElapsedTime(_startTimeStamp).TotalMilliseconds;

    public bool ShouldStop()
    {
        if (Volatile.Read(ref _stopped) != 0)
        {
            return true;
        }
        if (ElapsedMs() >= _hardBoundMs)
        {
            return true;
        }
        if (_linked.IsCancellationRequested)
        {
            Stop();
            return true;
        }
        return false;
    }

    public void Stop()
    {
        lock (_stopGate)
        {
            if (Interlocked.CompareExchange(ref _stopped, 1, 0) == 0)
            {
                _cts.Cancel();
            }
        }
    }

    public void AddNode() => Interlocked.Increment(ref _nodes);

    public long NodesCount => Volatile.Read(ref _nodes);

    public CancellationToken Token => _linked.Token;

    public void Dispose()
    {
        Stop();
        try
        {
            _watchTask.Wait(500);
        }
        catch (AggregateException)
        {
            // The watch loop may have observed cancellation first; the join
            // only exists to bound the thread's lifetime.
        }
        _cts.Dispose();
        _linked.Dispose();
    }
}
