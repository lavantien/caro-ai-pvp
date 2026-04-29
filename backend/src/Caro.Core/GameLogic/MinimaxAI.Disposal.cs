namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    private int _disposed;

    public void Dispose()
    {
        if (Interlocked.CompareExchange(ref _disposed, 1, 0) != 0) return;
        _ponderer.Dispose();
        _parallelSearch.Dispose();
        _statsChannel.Writer.TryComplete();
    }
}
