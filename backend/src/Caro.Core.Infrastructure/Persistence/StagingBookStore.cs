using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// SQLite-backed implementation of the staging book store.
/// Provides temporary storage for raw self-play data before verification.
///
/// Design principles:
/// - Store games in SGF format (one row per game) for efficient Phase 1 writes
/// - All buffer sizes are powers of 2 (default: 4096 = 2^12)
/// - Thread-safe for concurrent self-play recording
/// - Separate database from main opening_book.db
/// - Phase 2 reconstructs positions by replaying SGF move sequences
/// </summary>
public sealed class StagingBookStore : IStagingBookStore
{
    // All buffer sizes are powers of 2 for optimal performance
    private const int DefaultBufferSize = 4096;  // 2^12 games before commit
    private const int WriteBufferSize = 64;      // 2^6 entries before flush
    private const int GameBufferSize = 256;      // 2^8 games before flush

    private const string PositionsTable = "staging_positions";
    private const string GamesTable = "selfplay_games";

    private readonly string _connectionString;
    private readonly ILogger<StagingBookStore> _logger;
    private readonly object _lock = new();
    private readonly int _bufferSize;
    private SqliteConnection? _connection;
    private bool _isInitialized;
    private bool _isDisposed;

    // Buffers for batch inserts (powers of 2)
    private readonly List<StagingMoveRecord> _writeBuffer = new(WriteBufferSize);
    private readonly List<SelfPlayGameRecord> _gameBuffer = new(GameBufferSize);
    private long _currentGameId;
    private int _gamesInBuffer;

    public StagingBookStore(
        string databasePath,
        ILogger<StagingBookStore> logger,
        int bufferSize = DefaultBufferSize)
    {
        // Validate buffer size is power of 2
        if (bufferSize <= 0 || (bufferSize & (bufferSize - 1)) != 0)
        {
            throw new ArgumentException($"Buffer size must be a power of 2, got {bufferSize}", nameof(bufferSize));
        }

        _connectionString = $"Data Source={databasePath};Pooling=false";
        _logger = logger;
        _bufferSize = bufferSize;
    }

    /// <summary>
    /// Create read-only instance for verification phase.
    /// </summary>
    public StagingBookStore(
        string databasePath,
        ILogger<StagingBookStore> logger)
        : this(databasePath, logger, DefaultBufferSize)
    {
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
                    PRAGMA journal_mode=WAL;
                    PRAGMA synchronous=NORMAL;
                    PRAGMA cache_size=-64000;
                    PRAGMA busy_timeout=5000;

                    -- Position-level table (for backward compatibility and verification)
                    CREATE TABLE IF NOT EXISTS {PositionsTable} (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        canonical_hash INTEGER NOT NULL,
                        direct_hash INTEGER NOT NULL,
                        player INTEGER NOT NULL,
                        ply INTEGER NOT NULL,
                        move_x INTEGER NOT NULL,
                        move_y INTEGER NOT NULL,
                        game_result INTEGER NOT NULL,
                        game_id INTEGER NOT NULL,
                        time_budget_ms INTEGER NOT NULL,
                        created_at TEXT DEFAULT CURRENT_TIMESTAMP
                    );

                    CREATE INDEX IF NOT EXISTS idx_staging_hash ON {PositionsTable}(canonical_hash, direct_hash);
                    CREATE INDEX IF NOT EXISTS idx_staging_ply ON {PositionsTable}(ply);
                    CREATE INDEX IF NOT EXISTS idx_staging_game ON {PositionsTable}(game_id);
                    CREATE INDEX IF NOT EXISTS idx_staging_position ON {PositionsTable}(canonical_hash, direct_hash, player);

                    -- Game-level table (PRIMARY storage for Phase 1)
                    -- One row per game with SGF format moves
                    CREATE TABLE IF NOT EXISTS {GamesTable} (
                        game_id INTEGER PRIMARY KEY AUTOINCREMENT,
                        sgf_moves TEXT NOT NULL,
                        winner INTEGER NOT NULL,
                        total_moves INTEGER NOT NULL,
                        time_control TEXT,
                        temperature REAL NOT NULL DEFAULT 1.0,
                        difficulty INTEGER NOT NULL DEFAULT 5,
                        created_at TEXT DEFAULT CURRENT_TIMESTAMP
                    );

                    CREATE INDEX IF NOT EXISTS idx_games_winner ON {GamesTable}(winner);
                    CREATE INDEX IF NOT EXISTS idx_games_created ON {GamesTable}(created_at);
                ";
                command.ExecuteNonQuery();

                // Get current max game id for continuation
                using var maxIdCmd = Connection.CreateCommand();
                maxIdCmd.CommandText = $"SELECT COALESCE(MAX(game_id), 0) FROM {GamesTable};";
                _currentGameId = Convert.ToInt64(maxIdCmd.ExecuteScalar());

                _isInitialized = true;
                _logger.LogInformation("Staging book initialized at {Path}", builder.DataSource);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to initialize staging book");
                throw;
            }
        }
    }

    /// <summary>
    /// Record a completed game in SGF format (PRIMARY method).
    /// One row per game - efficient for Phase 1 writes.
    /// </summary>
    public void RecordGame(SelfPlayGameRecord gameRecord)
    {
        EnsureInitialized();

        lock (_lock)
        {
            _gameBuffer.Add(gameRecord);
            _gamesInBuffer++;

            // Flush when game buffer reaches capacity
            if (_gameBuffer.Count >= GameBufferSize)
            {
                FlushGameBuffer();
            }

            // Also flush based on total games count
            if (_gamesInBuffer >= _bufferSize)
            {
                Flush();
            }
        }
    }

    public void RecordMove(
        ulong canonicalHash,
        ulong directHash,
        Player player,
        int ply,
        int moveX,
        int moveY,
        int gameResult,
        long gameId,
        int timeBudgetMs)
    {
        EnsureInitialized();

        lock (_lock)
        {
            var record = new StagingMoveRecord(
                canonicalHash,
                directHash,
                player,
                ply,
                moveX,
                moveY,
                gameResult,
                gameId,
                timeBudgetMs
            );

            _writeBuffer.Add(record);

            // Flush when write buffer reaches capacity
            if (_writeBuffer.Count >= WriteBufferSize)
            {
                FlushWriteBuffer();
            }
        }
    }

    /// <summary>
    /// Get all games for batch processing.
    /// </summary>
    public List<SelfPlayGameRecord> GetGames(int limit = 1000, int offset = 0)
    {
        EnsureInitialized();

        var games = new List<SelfPlayGameRecord>();

        using var command = Connection.CreateCommand();
        command.CommandText = $@"
            SELECT game_id, sgf_moves, winner, total_moves, time_control, temperature, difficulty, created_at
            FROM {GamesTable}
            ORDER BY game_id
            LIMIT $limit OFFSET $offset;
        ";
        command.Parameters.AddWithValue("$limit", limit);
        command.Parameters.AddWithValue("$offset", offset);

        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            games.Add(new SelfPlayGameRecord
            {
                GameId = reader.GetInt64(0),
                SgfMoves = reader.GetString(1),
                Winner = (Player)reader.GetInt32(2),
                TotalMoves = reader.GetInt32(3),
                TimeControl = reader.IsDBNull(4) ? null : reader.GetString(4),
                Temperature = reader.IsDBNull(5) ? 1.0 : reader.GetDouble(5),
                Difficulty = reader.IsDBNull(6) ? AIDifficulty.Grandmaster : (AIDifficulty)reader.GetInt32(6),
                CreatedAt = reader.IsDBNull(7) ? DateTime.UtcNow : DateTime.Parse(reader.GetString(7)),
                MoveList = SelfPlayGameRecord.FromSgf(reader.GetString(1))
            });
        }

        return games;
    }

    /// <summary>
    /// Get games filtered by result.
    /// </summary>
    public List<SelfPlayGameRecord> GetGamesByResult(int result, int limit = 1000)
    {
        EnsureInitialized();

        var games = new List<SelfPlayGameRecord>();

        using var command = Connection.CreateCommand();
        command.CommandText = $@"
            SELECT game_id, sgf_moves, winner, total_moves, time_control, temperature, difficulty, created_at
            FROM {GamesTable}
            WHERE winner = $result
            ORDER BY game_id
            LIMIT $limit;
        ";
        command.Parameters.AddWithValue("$result", result);
        command.Parameters.AddWithValue("$limit", limit);

        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            games.Add(new SelfPlayGameRecord
            {
                GameId = reader.GetInt64(0),
                SgfMoves = reader.GetString(1),
                Winner = (Player)reader.GetInt32(2),
                TotalMoves = reader.GetInt32(3),
                TimeControl = reader.IsDBNull(4) ? null : reader.GetString(4),
                Temperature = reader.IsDBNull(5) ? 1.0 : reader.GetDouble(5),
                Difficulty = reader.IsDBNull(6) ? AIDifficulty.Grandmaster : (AIDifficulty)reader.GetInt32(6),
                CreatedAt = reader.IsDBNull(7) ? DateTime.UtcNow : DateTime.Parse(reader.GetString(7)),
                MoveList = SelfPlayGameRecord.FromSgf(reader.GetString(1))
            });
        }

        return games;
    }

    public IEnumerable<StagingPosition> GetPositionsForVerification(int maxPly = 16)
    {
        EnsureInitialized();

        var positions = new List<StagingPosition>();

        using var command = Connection.CreateCommand();
        command.CommandText = $@"
            SELECT DISTINCT canonical_hash, direct_hash, player, ply, move_x, move_y, time_budget_ms, game_result
            FROM {PositionsTable}
            WHERE ply <= $maxPly
            ORDER BY ply, canonical_hash, direct_hash;
        ";
        command.Parameters.AddWithValue("$maxPly", maxPly);

        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            positions.Add(new StagingPosition
            {
                CanonicalHash = (ulong)reader.GetInt64(0),
                DirectHash = (ulong)reader.GetInt64(1),
                Player = (Player)reader.GetInt32(2),
                Ply = reader.GetInt32(3),
                MoveX = reader.GetInt32(4),
                MoveY = reader.GetInt32(5),
                TimeBudgetMs = reader.GetInt32(6),
                GameResult = reader.GetInt32(7)
            });
        }

        return positions;
    }

    public Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> GetPositionStatistics()
    {
        EnsureInitialized();

        var stats = new Dictionary<(ulong, ulong, Player), PositionStatistics>();

        using var command = Connection.CreateCommand();
        command.CommandText = $@"
            SELECT
                canonical_hash,
                direct_hash,
                player,
                COUNT(*) as play_count,
                SUM(CASE WHEN game_result = 1 THEN 1 ELSE 0 END) as win_count,
                SUM(CASE WHEN game_result = 0 THEN 1 ELSE 0 END) as draw_count,
                SUM(CASE WHEN game_result = -1 THEN 1 ELSE 0 END) as loss_count,
                AVG(time_budget_ms) as avg_time_budget
            FROM {PositionsTable}
            GROUP BY canonical_hash, direct_hash, player;
        ";

        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            var canonicalHash = (ulong)reader.GetInt64(0);
            var directHash = (ulong)reader.GetInt64(1);
            var player = (Player)reader.GetInt32(2);
            var playCount = reader.GetInt32(3);
            var winCount = reader.GetInt32(4);
            var drawCount = reader.GetInt32(5);
            var lossCount = reader.GetInt32(6);
            var avgTimeBudget = reader.IsDBNull(7) ? 0 : (int)reader.GetDouble(7);

            var winRate = playCount > 0 ? (double)winCount / playCount : 0;

            stats[(canonicalHash, directHash, player)] = new PositionStatistics
            {
                PlayCount = playCount,
                WinCount = winCount,
                WinRate = winRate,
                AvgTimeBudgetMs = avgTimeBudget,
                DrawCount = drawCount,
                LossCount = lossCount
            };
        }

        return stats;
    }

    public List<StagingMove> GetMovesForPosition(ulong canonicalHash, ulong directHash, Player player)
    {
        EnsureInitialized();

        var moves = new List<StagingMove>();

        using var command = Connection.CreateCommand();
        command.CommandText = $@"
            SELECT
                move_x,
                move_y,
                ply,
                game_result,
                COUNT(*) as play_count,
                AVG(CASE WHEN game_result = 1 THEN 1.0 ELSE 0.0 END) as win_rate
            FROM {PositionsTable}
            WHERE canonical_hash = $canonicalHash
              AND direct_hash = $directHash
              AND player = $player
            GROUP BY move_x, move_y, ply
            ORDER BY play_count DESC, win_rate DESC;
        ";
        command.Parameters.AddWithValue("$canonicalHash", (long)canonicalHash);
        command.Parameters.AddWithValue("$directHash", (long)directHash);
        command.Parameters.AddWithValue("$player", (int)player);

        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            moves.Add(new StagingMove
            {
                MoveX = reader.GetInt32(0),
                MoveY = reader.GetInt32(1),
                Ply = reader.GetInt32(2),
                GameResult = reader.GetInt32(3),
                PlayCount = reader.GetInt32(4),
                WinRate = reader.GetDouble(5)
            });
        }

        return moves;
    }

    public void Flush()
    {
        EnsureInitialized();

        lock (_lock)
        {
            FlushWriteBuffer();
            FlushGameBuffer();
            _gamesInBuffer = 0;

            // Execute checkpoint to optimize WAL
            using var command = Connection.CreateCommand();
            command.CommandText = "PRAGMA wal_checkpoint(PASSIVE);";
            command.ExecuteNonQuery();
        }
    }

    public void Clear()
    {
        EnsureInitialized();

        lock (_lock)
        {
            try
            {
                _writeBuffer.Clear();
                _gameBuffer.Clear();

                using var command = Connection.CreateCommand();
                command.CommandText = $@"
                    DELETE FROM {PositionsTable};
                    DELETE FROM {GamesTable};
                    DROP INDEX IF EXISTS idx_staging_hash;
                    DROP INDEX IF EXISTS idx_staging_ply;
                    DROP INDEX IF EXISTS idx_staging_game;
                    DROP INDEX IF EXISTS idx_staging_position;
                    DROP INDEX IF EXISTS idx_games_winner;
                    DROP INDEX IF EXISTS idx_games_created;
                    CREATE INDEX IF NOT EXISTS idx_staging_hash ON {PositionsTable}(canonical_hash, direct_hash);
                    CREATE INDEX IF NOT EXISTS idx_staging_ply ON {PositionsTable}(ply);
                    CREATE INDEX IF NOT EXISTS idx_staging_game ON {PositionsTable}(game_id);
                    CREATE INDEX IF NOT EXISTS idx_staging_position ON {PositionsTable}(canonical_hash, direct_hash, player);
                    CREATE INDEX IF NOT EXISTS idx_games_winner ON {GamesTable}(winner);
                    CREATE INDEX IF NOT EXISTS idx_games_created ON {GamesTable}(created_at);
                ";
                command.ExecuteNonQuery();

                _currentGameId = 0;
                _gamesInBuffer = 0;
                _logger.LogInformation("Staging book cleared");
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to clear staging book");
                throw;
            }
        }
    }

    public long GetPositionCount()
    {
        EnsureInitialized();

        using var command = Connection.CreateCommand();
        command.CommandText = $"SELECT COUNT(*) FROM {PositionsTable};";
        return Convert.ToInt64(command.ExecuteScalar());
    }

    public long GetGameCount()
    {
        EnsureInitialized();

        using var command = Connection.CreateCommand();
        command.CommandText = $"SELECT COUNT(*) FROM {GamesTable};";
        return Convert.ToInt64(command.ExecuteScalar());
    }

    private void FlushWriteBuffer()
    {
        if (_writeBuffer.Count == 0) return;

        using var transaction = Connection.BeginTransaction();
        bool committed = false;

        try
        {
            using var command = Connection.CreateCommand();
            command.Transaction = transaction;
            command.CommandText = $@"
                INSERT INTO {PositionsTable}
                (canonical_hash, direct_hash, player, ply, move_x, move_y, game_result, game_id, time_budget_ms)
                VALUES
                ($canonicalHash, $directHash, $player, $ply, $moveX, $moveY, $gameResult, $gameId, $timeBudgetMs);
            ";

            foreach (var record in _writeBuffer)
            {
                command.Parameters.Clear();
                command.Parameters.AddWithValue("$canonicalHash", (long)record.CanonicalHash);
                command.Parameters.AddWithValue("$directHash", (long)record.DirectHash);
                command.Parameters.AddWithValue("$player", (int)record.Player);
                command.Parameters.AddWithValue("$ply", record.Ply);
                command.Parameters.AddWithValue("$moveX", record.MoveX);
                command.Parameters.AddWithValue("$moveY", record.MoveY);
                command.Parameters.AddWithValue("$gameResult", record.GameResult);
                command.Parameters.AddWithValue("$gameId", record.GameId);
                command.Parameters.AddWithValue("$timeBudgetMs", record.TimeBudgetMs);

                command.ExecuteNonQuery();
            }

            transaction.Commit();
            committed = true;

            int count = _writeBuffer.Count;
            _writeBuffer.Clear();
            _logger.LogDebug("Flushed {Count} staging records", count);
        }
        catch (Exception ex)
        {
            if (!committed)
            {
                try
                {
                    transaction.Rollback();
                }
                catch (Exception rollbackEx)
                {
                    _logger.LogWarning(rollbackEx, "Failed to rollback transaction after error");
                }
            }
            _logger.LogError(ex, "Failed to flush write buffer");
            throw;
        }
    }

    private void FlushGameBuffer()
    {
        if (_gameBuffer.Count == 0) return;

        using var transaction = Connection.BeginTransaction();
        bool committed = false;

        try
        {
            using var command = Connection.CreateCommand();
            command.Transaction = transaction;
            command.CommandText = $@"
                INSERT INTO {GamesTable}
                (sgf_moves, winner, total_moves, time_control, temperature, difficulty)
                VALUES
                ($sgfMoves, $winner, $totalMoves, $timeControl, $temperature, $difficulty);
            ";

            foreach (var game in _gameBuffer)
            {
                command.Parameters.Clear();
                command.Parameters.AddWithValue("$sgfMoves", game.SgfMoves);
                command.Parameters.AddWithValue("$winner", (int)game.Winner);
                command.Parameters.AddWithValue("$totalMoves", game.TotalMoves);
                command.Parameters.AddWithValue("$timeControl", game.TimeControl ?? (object)DBNull.Value);
                command.Parameters.AddWithValue("$temperature", game.Temperature);
                command.Parameters.AddWithValue("$difficulty", (int)game.Difficulty);

                command.ExecuteNonQuery();
            }

            transaction.Commit();
            committed = true;

            int count = _gameBuffer.Count;
            _gameBuffer.Clear();
            _logger.LogDebug("Flushed {Count} game records", count);
        }
        catch (Exception ex)
        {
            if (!committed)
            {
                try
                {
                    transaction.Rollback();
                }
                catch (Exception rollbackEx)
                {
                    _logger.LogWarning(rollbackEx, "Failed to rollback transaction after error");
                }
            }
            _logger.LogError(ex, "Failed to flush game buffer");
            throw;
        }
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
        if (_isDisposed) return;

        try
        {
            lock (_lock)
            {
                // Flush any remaining records
                if (_writeBuffer.Count > 0)
                {
                    FlushWriteBuffer();
                }
                if (_gameBuffer.Count > 0)
                {
                    FlushGameBuffer();
                }

                if (_connection != null)
                {
                    // Checkpoint and truncate WAL before closing
                    try
                    {
                        using var cmd = _connection.CreateCommand();
                        cmd.CommandText = "PRAGMA wal_checkpoint(TRUNCATE)";
                        cmd.ExecuteNonQuery();
                    }
                    catch
                    {
                        // Ignore checkpoint errors
                    }

                    if (_connection.State == System.Data.ConnectionState.Open)
                    {
                        _connection.Close();
                    }
                    _connection.Dispose();
                    _connection = null;
                }
            }
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Error during dispose");
        }
        finally
        {
            _connection = null;
            _isDisposed = true;
        }
    }

    /// <summary>
    /// Internal record for buffering position writes.
    /// </summary>
    private sealed record StagingMoveRecord(
        ulong CanonicalHash,
        ulong DirectHash,
        Player Player,
        int Ply,
        int MoveX,
        int MoveY,
        int GameResult,
        long GameId,
        int TimeBudgetMs
    );
}
