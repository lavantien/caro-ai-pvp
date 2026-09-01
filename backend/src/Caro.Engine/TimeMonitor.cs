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
        _watchTask = Task.Factory.StartNew(() => WatchLoop(token), token,
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
        _watchTask.Wait(500);
        _cts.Dispose();
        _linked.Dispose();
    }
}
