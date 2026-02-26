namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// Centralized resolver for opening book database path.
/// All code should use this resolver to find the opening_book.db file at the repository root.
/// Uses multi-strategy path resolution for reliability across different execution contexts.
/// </summary>
public static class OpeningBookPathResolver
{
    private const string BookFileName = "opening_book.db";
    private const string RepoRootEnvVar = "CARO_REPO_ROOT";

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
    /// Finds the repository root using multi-strategy path resolution.
    /// Strategies (in order of priority):
    /// 1. Environment variable override (CARO_REPO_ROOT)
    /// 2. Assembly location (most reliable for compiled executables)
    /// 3. Current directory (legacy fallback)
    /// </summary>
    /// <returns>Absolute path to repository root, or null if not found</returns>
    public static string? TryFindRepoRoot()
    {
        // Strategy 1: Environment variable override (for containers/CI)
        var envPath = Environment.GetEnvironmentVariable(RepoRootEnvVar);
        if (!string.IsNullOrEmpty(envPath))
        {
            if (File.Exists(Path.Combine(envPath, BookFileName)))
                return envPath;
            if (IsRepoRoot(envPath))
                return envPath;
        }

        // Strategy 2: Start from assembly location (most reliable)
        var assemblyPath = typeof(OpeningBookPathResolver).Assembly.Location;
        if (!string.IsNullOrEmpty(assemblyPath))
        {
            var assemblyDir = Path.GetDirectoryName(assemblyPath);
            var result = SearchUpward(assemblyDir);
            if (result != null)
                return result;
        }

        // Strategy 3: Current directory (legacy fallback)
        return SearchUpward(Directory.GetCurrentDirectory());
    }

    /// <summary>
    /// Search upward from a starting directory for the repository root.
    /// </summary>
    private static string? SearchUpward(string? startDir)
    {
        var currentDir = startDir;

        while (currentDir != null)
        {
            // Check for opening_book.db first (fastest check)
            var bookPath = Path.Combine(currentDir, BookFileName);
            if (File.Exists(bookPath))
                return currentDir;

            // Check for repository markers
            if (IsRepoRoot(currentDir))
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
