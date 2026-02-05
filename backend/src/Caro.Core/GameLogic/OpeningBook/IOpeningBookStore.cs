using Caro.Core.Domain.Entities;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for opening book storage abstraction.
/// Enables testability with in-memory implementations and different storage backends.
/// </summary>
public interface IOpeningBookStore
{
    /// <summary>
    /// Get book entry for a canonical position hash.
    /// Returns null if position is not in the book.
    /// </summary>
    OpeningBookEntry? GetEntry(ulong canonicalHash);

    /// <summary>
    /// Get book entry for a canonical position hash and player.
    /// Returns null if position is not in the book.
    /// </summary>
    OpeningBookEntry? GetEntry(ulong canonicalHash, Player player);

    /// <summary>
    /// Store a new book entry.
    /// If an entry with the same canonical hash exists, it will be replaced.
    /// </summary>
    void StoreEntry(OpeningBookEntry entry);

    /// <summary>
    /// Check if book contains an entry for the given canonical hash.
    /// </summary>
    bool ContainsEntry(ulong canonicalHash);

    /// <summary>
    /// Check if book contains an entry for the given canonical hash and player.
    /// </summary>
    bool ContainsEntry(ulong canonicalHash, Player player);

    /// <summary>
    /// Get book statistics including total entries and coverage by depth.
    /// </summary>
    BookStatistics GetStatistics();

    /// <summary>
    /// Clear all entries from the book.
    /// Primarily used for testing.
    /// </summary>
    void Clear();

    /// <summary>
    /// Initialize the book storage.
    /// Called before any other operations.
    /// </summary>
    void Initialize();

    /// <summary>
    /// Flush any pending writes and close resources.
    /// </summary>
    void Flush();

    /// <summary>
    /// Set metadata value for the book.
    /// Used for storing version, generation date, etc.
    /// </summary>
    void SetMetadata(string key, string value);

    /// <summary>
    /// Get metadata value from the book.
    /// Returns null if key doesn't exist.
    /// </summary>
    string? GetMetadata(string key);

    /// <summary>
    /// Get all entries from the book.
    /// Used for response generation phase.
    /// </summary>
    OpeningBookEntry[] GetAllEntries();
}
