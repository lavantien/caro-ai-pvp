using System.Collections.Concurrent;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// In-memory implementation of IOpeningBookStore for testing.
/// Thread-safe using ConcurrentDictionary for concurrent test scenarios.
/// </summary>
public sealed class MockOpeningBookStore : IOpeningBookStore
{
    private readonly ConcurrentDictionary<ulong, OpeningBookEntry> _entries;
    private readonly ConcurrentDictionary<string, string> _metadata;

    /// <summary>
    /// Create a new empty mock store.
    /// </summary>
    public MockOpeningBookStore()
    {
        _entries = new ConcurrentDictionary<ulong, OpeningBookEntry>();
        _metadata = new ConcurrentDictionary<string, string>();
    }

    /// <summary>
    /// Create a mock store pre-populated with entries.
    /// </summary>
    public MockOpeningBookStore(IEnumerable<OpeningBookEntry> initialEntries)
    {
        _entries = new ConcurrentDictionary<ulong, OpeningBookEntry>();
        _metadata = new ConcurrentDictionary<string, string>();

        foreach (var entry in initialEntries)
        {
            _entries[entry.CanonicalHash] = entry;
        }
    }

    /// <summary>
    /// Get the number of entries currently stored.
    /// </summary>
    public int Count => _entries.Count;

    /// <inheritdoc/>
    public OpeningBookEntry? GetEntry(ulong canonicalHash)
    {
        _entries.TryGetValue(canonicalHash, out var entry);
        return entry;
    }

    /// <inheritdoc/>
    public OpeningBookEntry? GetEntry(ulong canonicalHash, Player player)
    {
        _entries.TryGetValue(canonicalHash, out var entry);
        // Filter by player if entry exists
        if (entry != null && entry.Player == player)
            return entry;
        return null;
    }

    /// <inheritdoc/>
    public void StoreEntry(OpeningBookEntry entry)
    {
        _entries[entry.CanonicalHash] = entry;
    }

    /// <inheritdoc/>
    public void StoreEntriesBatch(IEnumerable<OpeningBookEntry> entries)
    {
        foreach (var entry in entries)
        {
            _entries[entry.CanonicalHash] = entry;
        }
    }

    /// <inheritdoc/>
    public bool ContainsEntry(ulong canonicalHash)
    {
        return _entries.ContainsKey(canonicalHash);
    }

    /// <inheritdoc/>
    public bool ContainsEntry(ulong canonicalHash, Player player)
    {
        if (!_entries.TryGetValue(canonicalHash, out var entry))
            return false;
        return entry.Player == player;
    }

    /// <inheritdoc/>
    public BookStatistics GetStatistics()
    {
        var entries = _entries.Values.ToArray();
        int maxDepth = entries.Length > 0 ? entries.Max(e => e.Depth) : 0;
        int totalMoves = entries.Sum(e => e.Moves.Length);

        // Build coverage by depth array
        int[] coverageByDepth = new int[maxDepth + 1];
        foreach (var entry in entries)
        {
            if (entry.Depth <= maxDepth)
                coverageByDepth[entry.Depth]++;
        }

        return new BookStatistics(
            TotalEntries: entries.Length,
            MaxDepth: maxDepth,
            CoverageByDepth: coverageByDepth,
            TotalMoves: totalMoves,
            GeneratedAt: _metadata.TryGetValue("GeneratedAt", out var dateStr) && DateTime.TryParse(dateStr, out var date) ? date : DateTime.UtcNow,
            Version: _metadata.TryGetValue("Version", out var version) ? version : "1"
        );
    }

    /// <inheritdoc/>
    public void Clear()
    {
        _entries.Clear();
    }

    /// <inheritdoc/>
    public void Initialize()
    {
        // No-op for in-memory store
    }

    /// <inheritdoc/>
    public void Flush()
    {
        // No-op for in-memory store
    }

    /// <inheritdoc/>
    public void SetMetadata(string key, string value)
    {
        _metadata[key] = value;
    }

    /// <inheritdoc/>
    public string? GetMetadata(string key)
    {
        _metadata.TryGetValue(key, out var value);
        return value;
    }

    /// <inheritdoc/>
    public OpeningBookEntry[] GetAllEntries()
    {
        return _entries.Values.ToArray();
    }
}
