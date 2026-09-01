using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class TimeMonitorTests
{
    [Fact]
    public void TimeMonitorExpiry()
    {
        TimeMonitor tm = new(50, CancellationToken.None);
        try
        {
            Thread.Sleep(100);
            Assert.True(tm.ShouldStop());
        }
        finally
        {
            tm.Stop();
        }
    }

    [Fact]
    public void TimeMonitorNotExpired()
    {
        TimeMonitor tm = new(5000, CancellationToken.None);
        try
        {
            Assert.False(tm.ShouldStop());
        }
        finally
        {
            tm.Stop();
        }
    }

    [Fact]
    public void TimeMonitorStop()
    {
        TimeMonitor tm = new(5000, CancellationToken.None);
        tm.Stop();

        Assert.True(tm.ShouldStop());
    }

    [Fact]
    public void TimeMonitorElapsedMs()
    {
        TimeMonitor tm = new(5000, CancellationToken.None);
        try
        {
            Thread.Sleep(50);
            long elapsed = tm.ElapsedMs();
            Assert.True(elapsed >= 40);
        }
        finally
        {
            tm.Stop();
        }
    }
}
