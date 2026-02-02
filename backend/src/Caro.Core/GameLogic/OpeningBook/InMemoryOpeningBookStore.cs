using Caro.Core.Entities;
using System.Collections.Concurrent;

namespace Caro.Core.GameLogic;

/// <summary>
/// In-memory implementation of IOpeningBookStore for testing.
/// Thread-safe using ConcurrentDictionary.
/// </summary>
public sealed class InMemoryOpeningBookStore : IOpeningBookStore
{
    private readonly ConcurrentDictionary<ulong, OpeningBookEntry> _store = new();
    private readonly ConcurrentDictionary<(ulong Hash, Player Player), OpeningBookEntry> _storeByPlayer = new();
    private readonly ConcurrentDictionary<string, string> _metadata = new();
    private int _totalMovesStored;
    private DateTime _generatedAt = DateTime.UtcNow;
    private readonly string _version = "1.0.0";

    public OpeningBookEntry? GetEntry(ulong canonicalHash)
    {
        return _store.TryGetValue(canonicalHash, out var entry) ? entry : null;
    }

    public OpeningBookEntry? GetEntry(ulong canonicalHash, Player player)
    {
        return _storeByPlayer.TryGetValue((canonicalHash, player), out var entry) ? entry : null;
    }

    public void StoreEntry(OpeningBookEntry entry)
    {
        ArgumentNullException.ThrowIfNull(entry);

        _store.AddOrUpdate(entry.CanonicalHash, entry, (_, _) => entry);
        _storeByPlayer.AddOrUpdate(
            (entry.CanonicalHash, entry.Player),
            entry,
            (_, _) => entry);

        Interlocked.Add(ref _totalMovesStored, entry.Moves.Length);
    }

    public bool ContainsEntry(ulong canonicalHash)
    {
        return _store.ContainsKey(canonicalHash);
    }

    public bool ContainsEntry(ulong canonicalHash, Player player)
    {
        return _storeByPlayer.ContainsKey((canonicalHash, player));
    }

    public BookStatistics GetStatistics()
    {
        var coverageByDepth = new int[25]; // Cover up to depth 24
        int totalMoves = 0;

        foreach (var entry in _store.Values)
        {
            if (entry.Depth < coverageByDepth.Length)
            {
                coverageByDepth[entry.Depth]++;
            }
            totalMoves += entry.Moves.Length;
        }

        return new BookStatistics(
            TotalEntries: _store.Count,
            MaxDepth: coverageByDepth.Length - 1,
            CoverageByDepth: coverageByDepth,
            TotalMoves: totalMoves,
            GeneratedAt: _generatedAt,
            Version: _version
        );
    }

    public void Clear()
    {
        _store.Clear();
        _storeByPlayer.Clear();
        _totalMovesStored = 0;
    }

    public void Initialize()
    {
        // Nothing to initialize for in-memory store
    }

    public void Flush()
    {
        // Nothing to flush for in-memory store
    }

    public void SetMetadata(string key, string value)
    {
        _metadata.AddOrUpdate(key, value, (_, _) => value);
    }

    public string? GetMetadata(string key)
    {
        return _metadata.TryGetValue(key, out var value) ? value : null;
    }
}
