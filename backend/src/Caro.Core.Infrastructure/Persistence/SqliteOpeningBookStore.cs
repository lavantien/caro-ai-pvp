using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// SQLite-backed implementation of the opening book store.
/// Provides persistent storage for precomputed opening positions.
/// </summary>
public sealed class SqliteOpeningBookStore : IOpeningBookStore, IDisposable
{
    private const int CurrentVersion = 1;
    private const string TableName = "OpeningBook";

    private readonly string _connectionString;
    private readonly ILogger<SqliteOpeningBookStore> _logger;
    private readonly object _lock = new();
    private SqliteConnection? _connection;
    private bool _isInitialized;

    public SqliteOpeningBookStore(string databasePath, ILogger<SqliteOpeningBookStore> logger)
    {
        _connectionString = $"Data Source={databasePath}";
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
                    CREATE TABLE IF NOT EXISTS {TableName} (
                        CanonicalHash INTEGER PRIMARY KEY NOT NULL,
                        Depth INTEGER NOT NULL,
                        Player INTEGER NOT NULL,
                        Symmetry INTEGER NOT NULL,
                        IsNearEdge INTEGER NOT NULL,
                        MovesData TEXT NOT NULL,
                        TotalMoves INTEGER NOT NULL,
                        CreatedAt TEXT NOT NULL
                    );

                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Depth ON {TableName}(Depth);
                    CREATE INDEX IF NOT EXISTS idx_{TableName}_Player ON {TableName}(Player);
                    
                    CREATE TABLE IF NOT EXISTS BookMetadata (
                        Key TEXT PRIMARY KEY NOT NULL,
                        Value TEXT NOT NULL
                    );
                ";
                command.ExecuteNonQuery();

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

    public OpeningBookEntry? GetEntry(ulong canonicalHash)
    {
        EnsureInitialized();

        try
        {
            using var command = Connection.CreateCommand();
            command.CommandText = $@"
                SELECT CanonicalHash, Depth, Player, Symmetry, IsNearEdge, MovesData
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
                SELECT CanonicalHash, Depth, Player, Symmetry, IsNearEdge, MovesData
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
                    (CanonicalHash, Depth, Player, Symmetry, IsNearEdge, MovesData, TotalMoves, CreatedAt)
                    VALUES ($hash, $depth, $player, $symmetry, $nearEdge, $moves, $totalMoves, $createdAt);
                ";
                command.Parameters.AddWithValue("$hash", (long)entry.CanonicalHash);
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
                _logger.LogError(ex, "Failed to store entry for hash {Hash}", entry.CanonicalHash);
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

    public BookStatistics GetStatistics()
    {
        EnsureInitialized();

        try
        {
            var coverageByDepth = new int[25]; // 0-24 plies

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
            return new BookStatistics(0, 0, new int[25], 0, DateTime.UtcNow, CurrentVersion.ToString());
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

    private static OpeningBookEntry ReadEntry(SqliteDataReader reader)
    {
        var hash = (ulong)(long)reader.GetInt64(0);
        var depth = reader.GetInt32(1);
        var player = (Player)reader.GetInt32(2);
        var symmetry = (SymmetryType)reader.GetInt32(3);
        var isNearEdge = reader.GetInt32(4) != 0;
        var movesJson = reader.GetString(5);

        var moves = System.Text.Json.JsonSerializer.Deserialize<BookMove[]>(movesJson)
            ?? Array.Empty<BookMove>();

        return new OpeningBookEntry
        {
            CanonicalHash = hash,
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
        _connection?.Dispose();
        _connection = null;
    }
}
