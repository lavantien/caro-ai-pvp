using Microsoft.Data.Sqlite;

namespace Caro.Api.Logging;

/// <summary>
/// Service for persisting game logs to SQLite with FTS5 full-text search
/// </summary>
public class GameLogService : IAsyncDisposable
{
    private readonly SqliteConnection _connection;
    private readonly ILogger<GameLogService> _logger;
    private readonly SemaphoreSlim _writeLock = new(1, 1);

    private GameLogService(SqliteConnection connection, ILogger<GameLogService> logger)
    {
        _connection = connection;
        _logger = logger;
    }

    /// <summary>
    /// Initialize the log service and create database schema
    /// </summary>
    public static async Task<GameLogService> CreateAsync(string dbPath, ILogger<GameLogService> logger)
    {
        // Ensure directory exists
        var directory = Path.GetDirectoryName(dbPath);
        if (!string.IsNullOrEmpty(directory) && !Directory.Exists(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var connection = new SqliteConnection($"Data Source={dbPath}");
        await connection.OpenAsync();

        var service = new GameLogService(connection, logger);
        await service.InitializeSchemaAsync();

        logger.LogInformation("GameLogService initialized with database at {DbPath}", dbPath);
        return service;
    }

    /// <summary>
    /// Create the database tables including FTS5 virtual table
    /// </summary>
    private async Task InitializeSchemaAsync()
    {
        // Main game logs table with all fields for structured queries
        var createTableSql = """
            CREATE TABLE IF NOT EXISTS GameLogs (
                Id INTEGER PRIMARY KEY AUTOINCREMENT,
                Timestamp TEXT NOT NULL,
                Level TEXT NOT NULL,
                Source TEXT NOT NULL,
                Message TEXT NOT NULL,
                GameId TEXT,
                MoveNumber INTEGER,
                Player TEXT,
                X INTEGER,
                Y INTEGER,
                DepthAchieved INTEGER,
                NodesSearched INTEGER,
                NodesPerSecond REAL,
                TableHitRate REAL,
                VCFDepthAchieved INTEGER,
                VCFNodesSearched INTEGER,
                PonderingActive INTEGER,
                CreatedUtc TEXT NOT NULL DEFAULT (datetime('now'))
            );
        """;

        // FTS5 virtual table for full-text search
        var createFtsTableSql = """
            CREATE VIRTUAL TABLE IF NOT EXISTS GameLogsFts USING fts5(
                Timestamp UNINDEXED,
                Level UNINDEXED,
                Source,
                Message,
                GameId UNINDEXED,
                content='GameLogs',
                content_rowid='Id'
            );
        """;

        // Triggers to keep FTS table in sync with main table
        var createTriggersSql = """
            CREATE TRIGGER IF NOT EXISTS GameLogs_Insert AFTER INSERT ON GameLogs BEGIN
                INSERT INTO GameLogsFts(rowid, Timestamp, Level, Source, Message, GameId)
                VALUES (NEW.Id, NEW.Timestamp, NEW.Level, NEW.Source, NEW.Message, NEW.GameId);
            END;

            CREATE TRIGGER IF NOT EXISTS GameLogs_Delete AFTER DELETE ON GameLogs BEGIN
                INSERT INTO GameLogsFts(GameLogsFts, rowid, Timestamp, Level, Source, Message, GameId)
                VALUES ('delete', OLD.Id, OLD.Timestamp, OLD.Level, OLD.Source, OLD.Message, OLD.GameId);
            END;

            CREATE TRIGGER IF NOT EXISTS GameLogs_Update AFTER UPDATE ON GameLogs BEGIN
                INSERT INTO GameLogsFts(GameLogsFts, rowid, Timestamp, Level, Source, Message, GameId)
                VALUES ('delete', OLD.Id, OLD.Timestamp, OLD.Level, OLD.Source, OLD.Message, OLD.GameId);
                INSERT INTO GameLogsFts(rowid, Timestamp, Level, Source, Message, GameId)
                VALUES (NEW.Id, NEW.Timestamp, NEW.Level, NEW.Source, NEW.Message, NEW.GameId);
            END;
        """;

        // Indexes for common queries
        var createIndexesSql = """
            CREATE INDEX IF NOT EXISTS IX_GameLogs_Timestamp ON GameLogs(Timestamp DESC);
            CREATE INDEX IF NOT EXISTS IX_GameLogs_GameId ON GameLogs(GameId);
            CREATE INDEX IF NOT EXISTS IX_GameLogs_Level ON GameLogs(Level);
            CREATE INDEX IF NOT EXISTS IX_GameLogs_Source ON GameLogs(Source);
        """;

        await ExecuteSqlAsync(createTableSql);
        await ExecuteSqlAsync(createFtsTableSql);
        await ExecuteSqlAsync(createTriggersSql);
        await ExecuteSqlAsync(createIndexesSql);

        _logger.LogInformation("GameLogService database schema initialized");
    }

    /// <summary>
    /// Helper method to execute SQL without parameters
    /// </summary>
    private async Task ExecuteSqlAsync(string sql)
    {
        await using var command = _connection.CreateCommand();
        command.CommandText = sql;
        await command.ExecuteNonQueryAsync();
    }

    /// <summary>
    /// Log a game event asynchronously (fire-and-forget)
    /// </summary>
    public async Task LogAsync(GameLogEntry entry, CancellationToken cancellationToken = default)
    {
        await _writeLock.WaitAsync(cancellationToken);
        try
        {
            var sql = @"
                INSERT INTO GameLogs (
                    Timestamp, Level, Source, Message, GameId, MoveNumber,
                    Player, X, Y, DepthAchieved, NodesSearched, NodesPerSecond,
                    TableHitRate, VCFDepthAchieved, VCFNodesSearched, PonderingActive
                ) VALUES (
                    @Timestamp, @Level, @Source, @Message, @GameId, @MoveNumber,
                    @Player, @X, @Y, @DepthAchieved, @NodesSearched, @NodesPerSecond,
                    @TableHitRate, @VCFDepthAchieved, @VCFNodesSearched, @PonderingActive
                );
            ";

            await using var command = _connection.CreateCommand();
            command.CommandText = sql;

            command.Parameters.AddWithValue("@Timestamp", entry.Timestamp);
            command.Parameters.AddWithValue("@Level", entry.Level);
            command.Parameters.AddWithValue("@Source", entry.Source);
            command.Parameters.AddWithValue("@Message", entry.Message);
            command.Parameters.AddWithValue("@GameId", (object?)entry.GameId ?? DBNull.Value);
            command.Parameters.AddWithValue("@MoveNumber", entry.MoveNumber.HasValue ? (object)entry.MoveNumber.Value : DBNull.Value);
            command.Parameters.AddWithValue("@Player", (object?)entry.Player ?? DBNull.Value);
            command.Parameters.AddWithValue("@X", entry.X.HasValue ? (object)entry.X.Value : DBNull.Value);
            command.Parameters.AddWithValue("@Y", entry.Y.HasValue ? (object)entry.Y.Value : DBNull.Value);
            command.Parameters.AddWithValue("@DepthAchieved", entry.DepthAchieved.HasValue ? (object)entry.DepthAchieved.Value : DBNull.Value);
            command.Parameters.AddWithValue("@NodesSearched", entry.NodesSearched.HasValue ? (object)entry.NodesSearched.Value : DBNull.Value);
            command.Parameters.AddWithValue("@NodesPerSecond", entry.NodesPerSecond.HasValue ? (object)entry.NodesPerSecond.Value : DBNull.Value);
            command.Parameters.AddWithValue("@TableHitRate", entry.TableHitRate.HasValue ? (object)entry.TableHitRate.Value : DBNull.Value);
            command.Parameters.AddWithValue("@VCFDepthAchieved", entry.VCFDepthAchieved.HasValue ? (object)entry.VCFDepthAchieved.Value : DBNull.Value);
            command.Parameters.AddWithValue("@VCFNodesSearched", entry.VCFNodesSearched.HasValue ? (object)entry.VCFNodesSearched.Value : DBNull.Value);
            command.Parameters.AddWithValue("@PonderingActive", (entry.PonderingActive ?? false) ? 1 : 0);

            await command.ExecuteNonQueryAsync();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to write game log entry");
        }
        finally
        {
            _writeLock.Release();
        }
    }

    /// <summary>
    /// Search logs by full-text search query
    /// </summary>
    public async Task<List<GameLogEntry>> SearchAsync(string query, int limit = 100, CancellationToken cancellationToken = default)
    {
        var sql = @"
            SELECT
                Id, Timestamp, Level, Source, Message, GameId, MoveNumber,
                Player, X, Y, DepthAchieved, NodesSearched, NodesPerSecond,
                TableHitRate, VCFDepthAchieved, VCFNodesSearched, PonderingActive
            FROM GameLogs
            WHERE Id IN (
                SELECT rowid FROM GameLogsFts
                WHERE GameLogsFts MATCH @Query
                ORDER BY rank
                LIMIT @Limit
            )
            ORDER BY Timestamp DESC;
        ";

        var results = new List<GameLogEntry>();

        await using var command = _connection.CreateCommand();
        command.CommandText = sql;
        command.Parameters.AddWithValue("@Query", query);
        command.Parameters.AddWithValue("@Limit", limit);

        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            results.Add(new GameLogEntry
            {
                Id = reader.GetInt64(0),
                Timestamp = reader.GetString(1),
                Level = reader.GetString(2),
                Source = reader.GetString(3),
                Message = reader.GetString(4),
                GameId = reader.IsDBNull(5) ? null : reader.GetString(5),
                MoveNumber = reader.IsDBNull(6) ? null : (int?)(long)reader.GetInt64(6),
                Player = reader.IsDBNull(7) ? null : reader.GetString(7),
                X = reader.IsDBNull(8) ? null : (int?)(long)reader.GetInt64(8),
                Y = reader.IsDBNull(9) ? null : (int?)(long)reader.GetInt64(9),
                DepthAchieved = reader.IsDBNull(10) ? null : (int?)(long)reader.GetInt64(10),
                NodesSearched = reader.IsDBNull(11) ? null : (long?)(long)reader.GetInt64(11),
                NodesPerSecond = reader.IsDBNull(12) ? null : (double?)(double)reader.GetFloat(12),
                TableHitRate = reader.IsDBNull(13) ? null : (double?)(double)reader.GetFloat(13),
                VCFDepthAchieved = reader.IsDBNull(14) ? null : (int?)(long)reader.GetInt64(14),
                VCFNodesSearched = reader.IsDBNull(15) ? null : (long?)(long)reader.GetInt64(15),
                PonderingActive = reader.IsDBNull(16) ? null : reader.GetInt64(16) != 0
            });
        }

        return results;
    }

    /// <summary>
    /// Get recent logs with optional filtering
    /// </summary>
    public async Task<List<GameLogEntry>> GetRecentAsync(
        string? gameId = null,
        string? level = null,
        string? source = null,
        int limit = 100,
        CancellationToken cancellationToken = default)
    {
        var sql = @"
            SELECT
                Id, Timestamp, Level, Source, Message, GameId, MoveNumber,
                Player, X, Y, DepthAchieved, NodesSearched, NodesPerSecond,
                TableHitRate, VCFDepthAchieved, VCFNodesSearched, PonderingActive
            FROM GameLogs
            WHERE 1=1
        ";

        if (!string.IsNullOrEmpty(gameId))
            sql += " AND GameId = @GameId";
        if (!string.IsNullOrEmpty(level))
            sql += " AND Level = @Level";
        if (!string.IsNullOrEmpty(source))
            sql += " AND Source = @Source";

        sql += " ORDER BY Timestamp DESC LIMIT @Limit";

        var results = new List<GameLogEntry>();

        await using var command = _connection.CreateCommand();
        command.CommandText = sql;
        command.Parameters.AddWithValue("@GameId", (object?)gameId ?? DBNull.Value);
        command.Parameters.AddWithValue("@Level", (object?)level ?? DBNull.Value);
        command.Parameters.AddWithValue("@Source", (object?)source ?? DBNull.Value);
        command.Parameters.AddWithValue("@Limit", limit);

        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            results.Add(new GameLogEntry
            {
                Id = reader.GetInt64(0),
                Timestamp = reader.GetString(1),
                Level = reader.GetString(2),
                Source = reader.GetString(3),
                Message = reader.GetString(4),
                GameId = reader.IsDBNull(5) ? null : reader.GetString(5),
                MoveNumber = reader.IsDBNull(6) ? null : (int?)(long)reader.GetInt64(6),
                Player = reader.IsDBNull(7) ? null : reader.GetString(7),
                X = reader.IsDBNull(8) ? null : (int?)(long)reader.GetInt64(8),
                Y = reader.IsDBNull(9) ? null : (int?)(long)reader.GetInt64(9),
                DepthAchieved = reader.IsDBNull(10) ? null : (int?)(long)reader.GetInt64(10),
                NodesSearched = reader.IsDBNull(11) ? null : (long?)(long)reader.GetInt64(11),
                NodesPerSecond = reader.IsDBNull(12) ? null : (double?)(double)reader.GetFloat(12),
                TableHitRate = reader.IsDBNull(13) ? null : (double?)(double)reader.GetFloat(13),
                VCFDepthAchieved = reader.IsDBNull(14) ? null : (int?)(long)reader.GetInt64(14),
                VCFNodesSearched = reader.IsDBNull(15) ? null : (long?)(long)reader.GetInt64(15),
                PonderingActive = reader.IsDBNull(16) ? null : reader.GetInt64(16) != 0
            });
        }

        return results;
    }

    /// <summary>
    /// Get statistics about logged data
    /// </summary>
    public async Task<GameLogStats> GetStatsAsync(CancellationToken cancellationToken = default)
    {
        var sql = @"
            SELECT
                COUNT(*) as TotalEntries,
                COUNT(DISTINCT GameId) as TotalGames,
                COUNT(CASE WHEN Level = 'error' THEN 1 END) as ErrorCount,
                COUNT(CASE WHEN Level = 'warning' THEN 1 END) as WarningCount,
                MIN(CreatedUtc) as FirstEntry,
                MAX(CreatedUtc) as LastEntry
            FROM GameLogs;
        ";

        await using var command = _connection.CreateCommand();
        command.CommandText = sql;

        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        await reader.ReadAsync(cancellationToken);

        return new GameLogStats
        {
            TotalEntries = reader.GetInt32(0),
            TotalGames = reader.GetInt32(1),
            ErrorCount = reader.GetInt32(2),
            WarningCount = reader.GetInt32(3),
            FirstEntry = reader.IsDBNull(4) ? null : reader.GetString(4),
            LastEntry = reader.IsDBNull(5) ? null : reader.GetString(5)
        };
    }

    public async ValueTask DisposeAsync()
    {
        _writeLock.Dispose();
        await _connection.DisposeAsync();
    }
}

/// <summary>
/// Game log entry with full details
/// </summary>
public record GameLogEntry
{
    public long Id { get; init; }
    public required string Timestamp { get; init; }
    public required string Level { get; init; }
    public required string Source { get; init; }
    public required string Message { get; init; }
    public string? GameId { get; init; }
    public int? MoveNumber { get; init; }
    public string? Player { get; init; }
    public int? X { get; init; }
    public int? Y { get; init; }
    public int? DepthAchieved { get; init; }
    public long? NodesSearched { get; init; }
    public double? NodesPerSecond { get; init; }
    public double? TableHitRate { get; init; }
    public int? VCFDepthAchieved { get; init; }
    public long? VCFNodesSearched { get; init; }
    public bool? PonderingActive { get; init; }
}

/// <summary>
/// Statistics about game logs
/// </summary>
public record GameLogStats
{
    public int TotalEntries { get; init; }
    public int TotalGames { get; init; }
    public int ErrorCount { get; init; }
    public int WarningCount { get; init; }
    public string? FirstEntry { get; init; }
    public string? LastEntry { get; init; }
}
