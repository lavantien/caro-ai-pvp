using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// SQLite-backed implementation of the opening book store.
/// Provides persistent storage for precomputed opening positions.
/// Uses compound key (CanonicalHash, DirectHash, Player) to avoid hash collision issues.
/// </summary>
public sealed class SqliteOpeningBookStore : IOpeningBookStore, IDisposable
{
    private const int CurrentVersion = 2;  // Bumped for DirectHash support
    private const string TableName = "OpeningBook";

    private readonly string _connectionString;
    private readonly ILogger<SqliteOpeningBookStore> _logger;
    private readonly object _lock = new();
    private readonly bool _readOnly;
    private SqliteConnection? _connection;
    private bool _isInitialized;

    public SqliteOpeningBookStore(string databasePath, ILogger<SqliteOpeningBookStore> logger, bool readOnly = true)
    {
        _connectionString = $"Data Source={databasePath}";
        if (readOnly)
        {
            _connectionString += ";Mode=ReadOnly";
        }
        _readOnly = readOnly;
        _logger = logger;
    }

    private SqliteConnection Connection
    {
        get
        {
            if (_connection == null)
            {
                _connection = new SqliteConnection(_connectionString);
                _connection.Open();
            }
            return _connection;
        }
    }

    public void Initialize()
    {
        if (_isInitialized) return;

        lock (_lock)
        {
            if (_isInitialized) return;

            // Read-only mode: database is already generated, skip table creation
            if (_readOnly)
            {
                _isInitialized = true;
                _logger.LogDebug("Opening book opened in read-only mode");
                return;
            }

            try
            {
                // Ensure directory exists
                var builder = new SqliteConnectionStringBuilder(_connectionString);
                var filePath = builder.DataSource;
                var directory = Path.GetDirectoryName(filePath);
                if (!string.IsNullOrEmpty(directory) && !Directory.Exists(directory))
                {
                    Directory.CreateDirectory(directory);
                }

                using var command = Connection.CreateCommand();
                command.CommandText = $@"
                    PRAGMA journal_mode=WAL;
                    PRAGMA synchronous=NORMAL;
                    PRAGMA cache_size=-64000;
                    PRAGMA busy_timeout=5000;

                    CREATE TABLE IF NOT EXISTS {TableName} (
                        CanonicalHash INTEGER NOT NULL,
                        DirectHash INTEGER NOT NULL,
                        Depth INTEGER NOT NULL,
                        Player INTEGER NOT NULL,
                        Symmetry INTEGER NOT NULL,
                        IsNearEdge INTEGER NOT NULL,
                        MovesData TEXT NOT NULL,
                        TotalMoves INTEGER NOT NULL,
                        CreatedAt TEXT NOT NULL,
                        PRIMARY KEY (CanonicalHash, DirectHash, Player)
                    );

                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Depth ON {TableName}(Depth);
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Player ON {TableName}(Player);
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_CanonicalHash ON {TableName}(CanonicalHash, Player);

                    CREATE TABLE IF NOT EXISTS BookMetadata (
                        Key TEXT PRIMARY KEY NOT NULL,
                        Value TEXT NOT NULL
                    );
                ";
                command.ExecuteNonQuery();

                // Migrate old schema (without DirectHash) if needed
                MigrateIfNeeded();

                _isInitialized = true;
                _logger.LogInformation("Opening book initialized at {Path}", builder.DataSource);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to initialize opening book");
                throw;
            }
        }
    }

    /// <summary>
    /// Migrate from old schema (without DirectHash) to new schema.
    /// Old entries without DirectHash will be dropped since they can't be used reliably.
    /// </summary>
    private void MigrateIfNeeded()
    {
        try
        {
            // Check if DirectHash column exists
            using var checkCmd = Connection.CreateCommand();
            checkCmd.CommandText = $"PRAGMA table_info({TableName});";
            using var reader = checkCmd.ExecuteReader();
            bool hasDirectHash = false;
            while (reader.Read())
            {
                var columnName = reader.GetString(1);
                if (columnName.Equals("DirectHash", StringComparison.OrdinalIgnoreCase))
                {
                    hasDirectHash = true;
                    break;
                }
            }

            if (!hasDirectHash)
            {
                _logger.LogInformation("Migrating opening book schema to include DirectHash column...");
                // Need to recreate the table with the new schema
                using var migrateCmd = Connection.CreateCommand();
                migrateCmd.CommandText = $@"
                    -- Create new table with correct schema
                    CREATE TABLE IF NOT EXISTS {TableName}_new (
                        CanonicalHash INTEGER NOT NULL,
                        DirectHash INTEGER NOT NULL,
                        Depth INTEGER NOT NULL,
                        Player INTEGER NOT NULL,
                        Symmetry INTEGER NOT NULL,
                        IsNearEdge INTEGER NOT NULL,
                        MovesData TEXT NOT NULL,
                        TotalMoves INTEGER NOT NULL,
                        CreatedAt TEXT NOT NULL,
                        PRIMARY KEY (CanonicalHash, DirectHash, Player)
                    );

                    -- Drop old table (entries will be lost but they're unreliable without DirectHash)
                    DROP TABLE IF EXISTS {TableName};

                    -- Rename new table
                    ALTER TABLE {TableName}_new RENAME TO {TableName};

                    -- Recreate indexes
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Depth ON {TableName}(Depth);
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Player ON {TableName}(Player);
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_CanonicalHash ON {TableName}(CanonicalHash, Player);
                ";
                migrateCmd.ExecuteNonQuery();
                _logger.LogInformation("Migration complete. Old entries dropped (unreliable without DirectHash).");
            }
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Schema migration check failed, assuming new schema");
        }
    }

    public OpeningBookEntry? GetEntry(ulong canonicalHash)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData
                FROM {TableName}
                WHERE CanonicalHash = $hash
                LIMIT 1;
            ";
            command.Parameters.AddWithValue("$hash", (long)canonicalHash);

            using var reader = command.ExecuteReader();
            if (reader.Read())
            {
                return ReadEntry(reader);
            }
            return null;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get entry for hash {Hash}", canonicalHash);
            return null;
        }
    }

    public OpeningBookEntry? GetEntry(ulong canonicalHash, Player player)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData
                FROM {TableName}
                WHERE CanonicalHash = $hash AND Player = $player
                LIMIT 1;
            ";
            command.Parameters.AddWithValue("$hash", (long)canonicalHash);
            command.Parameters.AddWithValue("$player", (int)player);

            using var reader = command.ExecuteReader();
            if (reader.Read())
            {
                return ReadEntry(reader);
            }
            return null;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get entry for hash {Hash} and player {Player}", canonicalHash, player);
            return null;
        }
    }

    public OpeningBookEntry? GetEntry(ulong canonicalHash, ulong directHash, Player player)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData
                FROM {TableName}
                WHERE CanonicalHash = $canonicalHash AND DirectHash = $directHash AND Player = $player
                LIMIT 1;
            ";
            command.Parameters.AddWithValue("$canonicalHash", (long)canonicalHash);
            command.Parameters.AddWithValue("$directHash", (long)directHash);
            command.Parameters.AddWithValue("$player", (int)player);

            using var reader = command.ExecuteReader();
            if (reader.Read())
            {
                return ReadEntry(reader);
            }
            return null;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get entry for canonical hash {CanonicalHash}, direct hash {DirectHash}, and player {Player}",
                canonicalHash, directHash, player);
            return null;
        }
    }

    public OpeningBookEntry[] GetAllEntriesForCanonicalHash(ulong canonicalHash, Player player)
    {
        EnsureInitialized();

        try
        {
            var entries = new List<OpeningBookEntry>();

            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData
                FROM {TableName}
                WHERE CanonicalHash = $hash AND Player = $player;
            ";
            command.Parameters.AddWithValue("$hash", (long)canonicalHash);
            command.Parameters.AddWithValue("$player", (int)player);

            using var reader = command.ExecuteReader();
            while (reader.Read())
            {
                entries.Add(ReadEntry(reader));
            }

            return entries.ToArray();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get all entries for canonical hash {Hash} and player {Player}",
                canonicalHash, player);
            return Array.Empty<OpeningBookEntry>();
        }
    }

    public void StoreEntry(OpeningBookEntry entry)
    {
        EnsureInitialized();

        lock (_lock)
        {
            try
            {
                var movesJson = System.Text.Json.JsonSerializer.Serialize(entry.Moves);

                using var command = Connection.CreateCommand();
                command.CommandText = $@"
                    INSERT OR REPLACE INTO {TableName}
                    (CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData, TotalMoves, CreatedAt)
                    VALUES ($canonicalHash, $directHash, $depth, $player, $symmetry, $nearEdge, $moves, $totalMoves, $createdAt);
                ";
                command.Parameters.AddWithValue("$canonicalHash", (long)entry.CanonicalHash);
                command.Parameters.AddWithValue("$directHash", (long)entry.DirectHash);
                command.Parameters.AddWithValue("$depth", entry.Depth);
                command.Parameters.AddWithValue("$player", (int)entry.Player);
                command.Parameters.AddWithValue("$symmetry", (int)entry.Symmetry);
                command.Parameters.AddWithValue("$nearEdge", entry.IsNearEdge ? 1 : 0);
                command.Parameters.AddWithValue("$moves", movesJson);
                command.Parameters.AddWithValue("$totalMoves", entry.Moves.Length);
                command.Parameters.AddWithValue("$createdAt", DateTime.UtcNow.ToString("o"));

                command.ExecuteNonQuery();
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to store entry for canonical hash {CanonicalHash}, direct hash {DirectHash}",
                    entry.CanonicalHash, entry.DirectHash);
                throw;
            }
        }
    }

    public void StoreEntriesBatch(IEnumerable<OpeningBookEntry> entries)
    {
        EnsureInitialized();

        lock (_lock)
        {
            using var transaction = Connection.BeginTransaction();
            bool committed = false;

            try
            {
                using var command = Connection.CreateCommand();
                command.Transaction = transaction;
                command.CommandText = $@"
                    INSERT OR REPLACE INTO {TableName}
                    (CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData, TotalMoves, CreatedAt)
                    VALUES ($canonicalHash, $directHash, $depth, $player, $symmetry, $nearEdge, $moves, $totalMoves, $createdAt);
                ";

                int count = 0;
                foreach (var entry in entries)
                {
                    command.Parameters.Clear();
                    command.Parameters.AddWithValue("$canonicalHash", (long)entry.CanonicalHash);
                    command.Parameters.AddWithValue("$directHash", (long)entry.DirectHash);
                    command.Parameters.AddWithValue("$depth", entry.Depth);
                    command.Parameters.AddWithValue("$player", (int)entry.Player);
                    command.Parameters.AddWithValue("$symmetry", (int)entry.Symmetry);
                    command.Parameters.AddWithValue("$nearEdge", entry.IsNearEdge ? 1 : 0);
                    command.Parameters.AddWithValue("$moves", System.Text.Json.JsonSerializer.Serialize(entry.Moves));
                    command.Parameters.AddWithValue("$totalMoves", entry.Moves.Length);
                    command.Parameters.AddWithValue("$createdAt", DateTime.UtcNow.ToString("o"));

                    command.ExecuteNonQuery();
                    count++;
                }

                transaction.Commit();
                committed = true;
                _logger.LogDebug("Batch stored {Count} entries in transaction", count);
            }
            catch (Exception ex)
            {
                // Only rollback if commit was not successful
                if (!committed)
                {
                    try
                    {
                        transaction.Rollback();
                    }
                    catch (Exception rollbackEx)
                    {
                        // Log but don't throw - original exception is more important
                        _logger.LogWarning(rollbackEx, "Failed to rollback transaction after error");
                    }
                }
                _logger.LogError(ex, "Failed to store batch entries");
                throw;
            }
        }
    }

    public bool ContainsEntry(ulong canonicalHash)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $"SELECT 1 FROM {TableName} WHERE CanonicalHash = $hash LIMIT 1;";
            command.Parameters.AddWithValue("$hash", (long)canonicalHash);

            var result = command.ExecuteScalar();
            return result != null && result != DBNull.Value;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to check entry existence for hash {Hash}", canonicalHash);
            return false;
        }
    }

    public bool ContainsEntry(ulong canonicalHash, Player player)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $"SELECT 1 FROM {TableName} WHERE CanonicalHash = $hash AND Player = $player LIMIT 1;";
            command.Parameters.AddWithValue("$hash", (long)canonicalHash);
            command.Parameters.AddWithValue("$player", (int)player);

            var result = command.ExecuteScalar();
            return result != null && result != DBNull.Value;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to check entry existence for hash {Hash} and player {Player}", canonicalHash, player);
            return false;
        }
    }

    public bool ContainsEntry(ulong canonicalHash, ulong directHash, Player player)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $"SELECT 1 FROM {TableName} WHERE CanonicalHash = $canonicalHash AND DirectHash = $directHash AND Player = $player LIMIT 1;";
            command.Parameters.AddWithValue("$canonicalHash", (long)canonicalHash);
            command.Parameters.AddWithValue("$directHash", (long)directHash);
            command.Parameters.AddWithValue("$player", (int)player);

            var result = command.ExecuteScalar();
            return result != null && result != DBNull.Value;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to check entry existence for canonical hash {CanonicalHash}, direct hash {DirectHash}, and player {Player}",
                canonicalHash, directHash, player);
            return false;
        }
    }

    public BookStatistics GetStatistics()
    {
        EnsureInitialized();

        try
        {
            var coverageByDepth = new int[41]; // 0-40 plies

            using (var command = Connection.CreateCommand())
            {
                command.CommandText = $@"
                    SELECT
                        COUNT(*) as TotalEntries,
                        COALESCE(MAX(Depth), 0) as MaxDepth,
                        COALESCE(SUM(TotalMoves), 0) as TotalMoves
                    FROM {TableName};
                ";

                using var reader = command.ExecuteReader();
                if (reader.Read())
                {
                    var totalEntries = reader.GetInt32(0);
                    var maxDepth = reader.GetInt32(1);
                    var totalMoves = reader.GetInt32(2);

                    // Get coverage by depth
                    reader.Close();

                    command.CommandText = $"SELECT Depth, COUNT(*) FROM {TableName} GROUP BY Depth;";
                    using var depthReader = command.ExecuteReader();
                    while (depthReader.Read())
                    {
                        var depth = depthReader.GetInt32(0);
                        var count = depthReader.GetInt32(1);
                        if (depth < coverageByDepth.Length)
                        {
                            coverageByDepth[depth] = count;
                        }
                    }

                    // Get or set version
                    var version = GetMetadata("Version") ?? CurrentVersion.ToString();
                    var generatedAtStr = GetMetadata("GeneratedAt");
                    var generatedAt = !string.IsNullOrEmpty(generatedAtStr)
                        ? DateTime.Parse(generatedAtStr)
                        : DateTime.UtcNow;

                    return new BookStatistics(
                        TotalEntries: totalEntries,
                        MaxDepth: maxDepth,
                        CoverageByDepth: coverageByDepth,
                        TotalMoves: totalMoves,
                        GeneratedAt: generatedAt,
                        Version: version
                    );
                }
            }

            return new BookStatistics(0, 0, coverageByDepth, 0, DateTime.UtcNow, CurrentVersion.ToString());
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get statistics");
            return new BookStatistics(0, 0, new int[41], 0, DateTime.UtcNow, CurrentVersion.ToString());
        }
    }

    public void Clear()
    {
        EnsureInitialized();

        lock (_lock)
        {
            try
            {
                using var command = Connection.CreateCommand();
                command.CommandText = $"DELETE FROM {TableName}; DELETE FROM BookMetadata;";
                command.ExecuteNonQuery();
                _logger.LogInformation("Opening book cleared");
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to clear opening book");
                throw;
            }
        }
    }

    public void Flush()
    {
        try
        {
            if (_connection != null)
            {
                // Execute VACUUM to optimize the database
                using var command = _connection.CreateCommand();
                command.CommandText = "VACUUM;";
                command.ExecuteNonQuery();
            }
            _logger.LogDebug("Opening book flushed");
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Failed to flush opening book");
        }
    }

    public void SetMetadata(string key, string value)
    {
        EnsureInitialized();

        using var command = Connection.CreateCommand();
        command.CommandText = "INSERT OR REPLACE INTO BookMetadata (Key, Value) VALUES ($key, $value);";
        command.Parameters.AddWithValue("$key", key);
        command.Parameters.AddWithValue("$value", value);
        command.ExecuteNonQuery();
    }

    public string? GetMetadata(string key)
    {
        EnsureInitialized();

        using var command = Connection.CreateCommand();
        command.CommandText = "SELECT Value FROM BookMetadata WHERE Key = $key;";
        command.Parameters.AddWithValue("$key", key);

        var result = command.ExecuteScalar();
        return result as string;
    }

    public OpeningBookEntry[] GetAllEntries()
    {
        EnsureInitialized();

        try
        {
            var entries = new List<OpeningBookEntry>();

            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, DirectHash, Depth, Player, Symmetry, IsNearEdge, MovesData
                FROM {TableName};
            ";

            using var reader = command.ExecuteReader();
            while (reader.Read())
            {
                entries.Add(ReadEntry(reader));
            }

            return entries.ToArray();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to get all entries");
            return Array.Empty<OpeningBookEntry>();
        }
    }

    private static OpeningBookEntry ReadEntry(SqliteDataReader reader)
    {
        var canonicalHash = (ulong)(long)reader.GetInt64(0);
        var directHash = (ulong)(long)reader.GetInt64(1);
        var depth = reader.GetInt32(2);
        var player = (Player)reader.GetInt32(3);
        var symmetry = (SymmetryType)reader.GetInt32(4);
        var isNearEdge = reader.GetInt32(5) != 0;
        var movesJson = reader.GetString(6);

        var moves = System.Text.Json.JsonSerializer.Deserialize<BookMove[]>(movesJson)
            ?? Array.Empty<BookMove>();

        return new OpeningBookEntry
        {
            CanonicalHash = canonicalHash,
            DirectHash = directHash,
            Depth = depth,
            Player = player,
            Symmetry = symmetry,
            IsNearEdge = isNearEdge,
            Moves = moves
        };
    }

    private void EnsureInitialized()
    {
        if (!_isInitialized)
        {
            Initialize();
        }
    }

    public void Dispose()
    {
        try
        {
            if (_connection != null)
            {
                if (_connection.State == System.Data.ConnectionState.Open)
                {
                    _connection.Close();
                }
                _connection.Dispose();
            }
        }
        catch
        {
            // Suppress dispose errors - connection may be in an invalid state due to cancellation
        }
        finally
        {
            _connection = null;
        }
    }
}
