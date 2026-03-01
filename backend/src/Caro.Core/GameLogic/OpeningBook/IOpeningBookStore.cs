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
    /// Get book entry for exact position (canonical hash + direct hash + player).
    /// This is the preferred method for lookup as it avoids hash collision issues.
    /// Returns null if position is not in the book.
    /// </summary>
    OpeningBookEntry? GetEntry(ulong canonicalHash, ulong directHash, Player player);

    /// <summary>
    /// Get all entries matching a canonical hash and player.
    /// Used when checking symmetric equivalents during lookup.
    /// </summary>
    OpeningBookEntry[] GetAllEntriesForCanonicalHash(ulong canonicalHash, Player player);

    /// <summary>
    /// Store a new book entry.
    /// If an entry with the same canonical hash and direct hash exists, it will be replaced.
    /// </summary>
    void StoreEntry(OpeningBookEntry entry);

    /// <summary>
    /// Store multiple book entries in a single transaction for better performance.
    /// Reduces database lock contention during book generation.
    /// </summary>
    void StoreEntriesBatch(IEnumerable<OpeningBookEntry> entries);

    /// <summary>
    /// Check if book contains an entry for the given canonical hash.
    /// </summary>
    bool ContainsEntry(ulong canonicalHash);

    /// <summary>
    /// Check if book contains an entry for the given canonical hash and player.
    /// </summary>
    bool ContainsEntry(ulong canonicalHash, Player player);

    /// <summary>
    /// Check if book contains an entry for the exact position (canonical hash + direct hash + player).
    /// This is the preferred method for checking existence as it avoids hash collision issues.
    /// </summary>
    bool ContainsEntry(ulong canonicalHash, ulong directHash, Player player);

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

    /// <summary>
    /// Get entries at a specific depth for batch streaming.
    /// Used for memory-efficient iteration during book generation.
    /// </summary>
    /// <param name="depth">The ply depth to retrieve entries for</param>
    /// <param name="offset">Number of entries to skip (for pagination)</param>
    /// <param name="limit">Maximum number of entries to return</param>
    /// <returns>Array of entries at the specified depth</returns>
    OpeningBookEntry[] GetEntriesAtDepth(int depth, int offset, int limit);

    /// <summary>
    /// Get the total count of entries at a specific depth.
    /// Used for progress tracking during batch processing.
    /// </summary>
    /// <param name="depth">The ply depth to count entries for</param>
    /// <returns>Number of entries at the specified depth</returns>
    int GetEntryCountAtDepth(int depth);

    /// <summary>
    /// Save generation progress for resume functionality.
    /// Allows interrupted book generation to continue from where it left off.
    /// </summary>
    /// <param name="progress">The progress state to save</param>
    void SaveProgress(BookGenerationResumeState progress);

    /// <summary>
    /// Load generation progress for resume functionality.
    /// Returns null if no progress has been saved.
    /// </summary>
    /// <returns>The saved progress state, or null if not found</returns>
    BookGenerationResumeState? LoadProgress();

    /// <summary>
    /// Clear saved generation progress.
    /// Called after successful book generation completion.
    /// </summary>
    void ClearProgress();
}

/// <summary>
/// Represents the resume state of book generation for continuation after interruption.
/// </summary>
public sealed record BookGenerationResumeState
{
    /// <summary>
    /// Current depth being processed (ply number).
    /// </summary>
    public required int CurrentDepth { get; init; }

    /// <summary>
    /// Current batch index within the depth (0-based).
    /// </summary>
    public required int CurrentBatchIndex { get; init; }

    /// <summary>
    /// Current generation phase.
    /// </summary>
    public required GenerationPhase Phase { get; init; }

    /// <summary>
    /// When this progress was last updated.
    /// </summary>
    public required DateTime LastUpdatedAt { get; init; }

    /// <summary>
    /// Total positions processed so far.
    /// </summary>
    public required int TotalPositionsProcessed { get; init; }
}

/// <summary>
/// Phases of opening book generation.
/// </summary>
public enum GenerationPhase
{
    /// <summary>VCF/VCT tactical solving (ply 0-8)</summary>
    VcfSolving,

    /// <summary>Deep search evaluation (ply 8-16)</summary>
    DeepSearch,

    /// <summary>Self-play game generation (ply 16+)</summary>
    SelfPlay
}
