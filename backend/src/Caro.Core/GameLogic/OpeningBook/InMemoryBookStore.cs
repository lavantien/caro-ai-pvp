using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Adapter that wraps InMemoryOpeningBook to implement IOpeningBookStore.
/// Allows the in-memory book to be used with existing OpeningBookLookupService.
/// Provides nanosecond access times for book lookups after initial load.
/// </summary>
public sealed class InMemoryBookStore : IOpeningBookStore
{
    private readonly InMemoryOpeningBook _inMemoryBook;
    private readonly IOpeningBookStore _persistentStore;
    private readonly IPositionCanonicalizer _canonicalizer;

    /// <summary>
    /// Get the number of positions in the book.
    /// </summary>
    public int Count => _inMemoryBook.Count;

    /// <summary>
    /// Create a new in-memory book store.
    /// Loads all entries from the persistent store into memory.
    /// </summary>
    public InMemoryBookStore(
        IOpeningBookStore persistentStore,
        IPositionCanonicalizer canonicalizer,
        Microsoft.Extensions.Logging.ILoggerFactory? loggerFactory = null)
    {
        _persistentStore = persistentStore ?? throw new ArgumentNullException(nameof(persistentStore));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _inMemoryBook = new InMemoryOpeningBook(persistentStore, canonicalizer, loggerFactory);
    }

    /// <inheritdoc/>
    public OpeningBookEntry? GetEntry(ulong canonicalHash)
    {
        // For this adapter, we need to scan since we don't have board context
        // This is less efficient but maintains interface compatibility
        return _persistentStore.GetEntry(canonicalHash);
    }

    /// <inheritdoc/>
    public OpeningBookEntry? GetEntry(ulong canonicalHash, Player player)
    {
        // Delegate to persistent store for exact hash lookup
        return _persistentStore.GetEntry(canonicalHash, player);
    }

    /// <inheritdoc/>
    public OpeningBookEntry? GetEntry(ulong canonicalHash, ulong directHash, Player player)
    {
        // Delegate to persistent store for exact lookup
        return _persistentStore.GetEntry(canonicalHash, directHash, player);
    }

    /// <inheritdoc/>
    public OpeningBookEntry[] GetAllEntriesForCanonicalHash(ulong canonicalHash, Player player)
    {
        return _persistentStore.GetAllEntriesForCanonicalHash(canonicalHash, player);
    }

    /// <summary>
    /// Fast lookup using board position directly.
    /// This is the preferred method for in-memory book access.
    /// </summary>
    public BookMove[]? Lookup(Board board, Player player)
    {
        return _inMemoryBook.Lookup(board, player);
    }

    /// <summary>
    /// Check if position is in book using board directly.
    /// </summary>
    public bool Contains(Board board, Player player)
    {
        return _inMemoryBook.Contains(board, player);
    }

    /// <summary>
    /// Get the best move for a position.
    /// </summary>
    public BookMove? GetBestMove(Board board, Player player)
    {
        return _inMemoryBook.GetBestMove(board, player);
    }

    /// <inheritdoc/>
    public void StoreEntry(OpeningBookEntry entry)
    {
        // Store in both persistent and in-memory
        _persistentStore.StoreEntry(entry);
        _inMemoryBook.AddEntry(entry);
    }

    /// <inheritdoc/>
    public void StoreEntriesBatch(IEnumerable<OpeningBookEntry> entries)
    {
        _persistentStore.StoreEntriesBatch(entries);
        foreach (var entry in entries)
        {
            _inMemoryBook.AddEntry(entry);
        }
    }

    /// <inheritdoc/>
    public bool ContainsEntry(ulong canonicalHash)
    {
        return _persistentStore.ContainsEntry(canonicalHash);
    }

    /// <inheritdoc/>
    public bool ContainsEntry(ulong canonicalHash, Player player)
    {
        return _persistentStore.ContainsEntry(canonicalHash, player);
    }

    /// <inheritdoc/>
    public bool ContainsEntry(ulong canonicalHash, ulong directHash, Player player)
    {
        return _persistentStore.ContainsEntry(canonicalHash, directHash, player);
    }

    /// <inheritdoc/>
    public BookStatistics GetStatistics()
    {
        return _persistentStore.GetStatistics();
    }

    /// <summary>
    /// Get in-memory book statistics with additional detail.
    /// </summary>
    public InMemoryBookStatistics GetInMemoryStatistics()
    {
        return _inMemoryBook.GetStatistics();
    }

    /// <inheritdoc/>
    public void Clear()
    {
        _persistentStore.Clear();
        _inMemoryBook.Dispose();
    }

    /// <inheritdoc/>
    public void Initialize()
    {
        _persistentStore.Initialize();
    }

    /// <inheritdoc/>
    public void Flush()
    {
        _persistentStore.Flush();
    }

    /// <inheritdoc/>
    public void SetMetadata(string key, string value)
    {
        _persistentStore.SetMetadata(key, value);
    }

    /// <inheritdoc/>
    public string? GetMetadata(string key)
    {
        return _persistentStore.GetMetadata(key);
    }

    /// <inheritdoc/>
    public OpeningBookEntry[] GetAllEntries()
    {
        return _persistentStore.GetAllEntries();
    }

    /// <inheritdoc/>
    public OpeningBookEntry[] GetEntriesAtDepth(int depth, int offset, int limit)
    {
        return _persistentStore.GetEntriesAtDepth(depth, offset, limit);
    }

    /// <inheritdoc/>
    public int GetEntryCountAtDepth(int depth)
    {
        return _persistentStore.GetEntryCountAtDepth(depth);
    }

    /// <inheritdoc/>
    public void SaveProgress(BookGenerationResumeState progress)
    {
        _persistentStore.SaveProgress(progress);
    }

    /// <inheritdoc/>
    public BookGenerationResumeState? LoadProgress()
    {
        return _persistentStore.LoadProgress();
    }

    /// <inheritdoc/>
    public void ClearProgress()
    {
        _persistentStore.ClearProgress();
    }
}
