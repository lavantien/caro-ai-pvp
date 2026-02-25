namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// Centralized resolver for opening book database path.
/// All code should use this resolver to find the opening_book.db file at the repository root.
/// </summary>
public static class OpeningBookPathResolver
{
    private const string BookFileName = "opening_book.db";

    /// <summary>
    /// Marker files/directories that indicate the repository root.
    /// Only use .git directory as it's the only reliable marker - .gitignore and README.md can exist in subdirectories.
    /// </summary>
    private static readonly string[] RepoRootMarkers = [".git"];

    /// <summary>
    /// Finds the opening book database at the repository root.
    /// Throws FileNotFoundException if not found.
    /// </summary>
    /// <returns>Absolute path to opening_book.db</returns>
    /// <exception cref="FileNotFoundException">When the opening book file cannot be found</exception>
    public static string FindOpeningBookPath()
    {
        var path = TryFindOpeningBookPath();
        if (path != null)
            return path;

        var repoRoot = TryFindRepoRoot();
        var searchedPath = repoRoot != null
            ? Path.Combine(repoRoot, BookFileName)
            : "repository root (not found)";

        throw new FileNotFoundException(
            $"Opening book database not found. Expected location: {searchedPath}. " +
            "Run 'dotnet run --project backend/src/Caro.BookBuilder' to generate it.");
    }

    /// <summary>
    /// Attempts to find the opening book database at the repository root.
    /// Returns null if not found (for optional book usage scenarios).
    /// </summary>
    /// <returns>Absolute path to opening_book.db, or null if not found</returns>
    public static string? TryFindOpeningBookPath()
    {
        var repoRoot = TryFindRepoRoot();
        if (repoRoot == null)
            return null;

        var bookPath = Path.Combine(repoRoot, BookFileName);
        return File.Exists(bookPath) ? bookPath : null;
    }

    /// <summary>
    /// Finds the repository root by searching upward for marker files/directories.
    /// </summary>
    /// <returns>Absolute path to repository root, or null if not found</returns>
    public static string? TryFindRepoRoot()
    {
        // Start from current directory and search upward
        var currentDir = Directory.GetCurrentDirectory();

        while (currentDir != null)
        {
            if (IsRepoRoot(currentDir))
                return currentDir;

            // Check if opening_book.db exists directly in this directory
            var bookPath = Path.Combine(currentDir, BookFileName);
            if (File.Exists(bookPath))
                return currentDir;

            currentDir = Directory.GetParent(currentDir)?.FullName;
        }

        return null;
    }

    /// <summary>
    /// Checks if a directory is the repository root by looking for marker files.
    /// </summary>
    private static bool IsRepoRoot(string directory)
    {
        return RepoRootMarkers.Any(marker =>
        {
            var path = Path.Combine(directory, marker);
            return File.Exists(path) || Directory.Exists(path);
        });
    }
}
