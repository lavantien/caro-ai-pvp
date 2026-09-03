using Caro.Domain;

namespace Caro.Api;

public sealed class GameStore : IDisposable
{
    private readonly ReaderWriterLockSlim _lock = new();
    private readonly Dictionary<string, GameSession> _games = [];
    private readonly CaroConfig _config;

    public GameStore(CaroConfig? config = null)
    {
        _config = config ?? CaroConfig.Default;
    }

    public void Set(string id, GameSession session)
    {
        _lock.EnterWriteLock();
        try
        {
            _games[id] = session;
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    public bool TryGet(string id, out GameSession session)
    {
        _lock.EnterReadLock();
        try
        {
            return _games.TryGetValue(id, out session!);
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    public void Delete(string id)
    {
        _lock.EnterWriteLock();
        try
        {
            if (_games.TryGetValue(id, out GameSession? g))
            {
                g.DisposeAI();
                _games.Remove(id);
            }
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    public int Count()
    {
        _lock.EnterReadLock();
        try
        {
            return _games.Count;
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    public int ActiveGameCount()
    {
        _lock.EnterReadLock();
        try
        {
            int count = 0;
            foreach (GameSession g in _games.Values)
            {
                if (!g.IsGameOver())
                {
                    count++;
                }
            }
            return count;
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    public int CleanupCompleted()
    {
        _lock.EnterWriteLock();
        try
        {
            int removed = 0;
            DateTime now = DateTime.UtcNow;
            foreach (string id in _games.Keys.ToList())
            {
                GameSession g = _games[id];
                // Finished games go immediately; a live game (e.g. a long
                // think under a slow control) only goes after the
                // abandoned-game window, never the short idle sweep.
                if (g.IsGameOver() || now - g.LastActivityAt() > TimeSpan.FromMinutes(_config.AbandonedTimeoutMinutes))
                {
                    g.DisposeAI();
                    _games.Remove(id);
                    removed++;
                }
            }
            return removed;
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    public int CleanupAll()
    {
        _lock.EnterWriteLock();
        try
        {
            int count = _games.Count;
            foreach (string id in _games.Keys.ToList())
            {
                _games[id].DisposeAI();
                _games.Remove(id);
            }
            return count;
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    public void Dispose() => _lock.Dispose();
}
