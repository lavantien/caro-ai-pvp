using System.Collections.Concurrent;
using System.Text;
using System.Text.Json;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// File-based implementation of IStagingBookStore.
/// Eliminates SQLite lock contention by using per-worker buffer files.
///
/// Design principles:
/// - Each worker writes to their own buffer file (zero lock contention)
/// - Manifest tracks game index for fast pagination
/// - Committed files are immutable (read-only after commit)
/// - Atomic file rename for safe manifest updates
///
/// File structure:
/// /staging/
///   buffer/
///     worker_0.sgf
///     worker_1.sgf
///   committed/
///     batch_0001.sgf
///     batch_0002.sgf
///   manifest.json
/// </summary>
public sealed class FileStagingBookStore : IStagingBookStore
{
    private const int GamesPerBatch = 256;
    private readonly string _basePath;
    private readonly ILogger<FileStagingBookStore> _logger;
    private readonly object _manifestLock = new();
    private SgfManifest _manifest;
    private bool _isInitialized;
    private bool _isDisposed;

    // Per-worker buffers (indexed by thread ID for zero contention)
    private readonly ConcurrentDictionary<int, WorkerBuffer> _workerBuffers = new();

    public FileStagingBookStore(string basePath, ILogger<FileStagingBookStore> logger)
    {
        _basePath = basePath ?? throw new ArgumentNullException(nameof(basePath));
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
        _manifest = new SgfManifest();
    }

    public void Initialize()
    {
        if (_isInitialized) return;

        lock (_manifestLock)
        {
            if (_isInitialized) return;

            try
            {
                // Create directories
                var bufferDir = Path.Combine(_basePath, "buffer");
                var committedDir = Path.Combine(_basePath, "committed");

                if (!Directory.Exists(bufferDir))
                    Directory.CreateDirectory(bufferDir);
                if (!Directory.Exists(committedDir))
                    Directory.CreateDirectory(committedDir);

                // Load or create manifest
                var manifestPath = GetManifestPath();
                if (File.Exists(manifestPath))
                {
                    var json = File.ReadAllText(manifestPath);
                    _manifest = JsonSerializer.Deserialize<SgfManifest>(json)
                        ?? new SgfManifest();
                    _logger.LogInformation("Loaded manifest with {Count} batches", _manifest.Batches.Count);
                }
                else
                {
                    _manifest = new SgfManifest();
                    SaveManifest();
                    _logger.LogInformation("Created new manifest at {Path}", _basePath);
                }

                _isInitialized = true;
                _logger.LogInformation("File staging store initialized at {Path}", _basePath);
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to initialize file staging store");
                throw;
            }
        }
    }

    /// <summary>
    /// Record a completed game in SGF format.
    /// Each worker writes to its own buffer file for zero lock contention.
    /// </summary>
    public void RecordGame(SelfPlayGameRecord gameRecord)
    {
        EnsureInitialized();

        // Get or create worker buffer (use worker ID from game ID for simplicity)
        var workerId = (int)(gameRecord.GameId % Environment.ProcessorCount);
        var buffer = _workerBuffers.GetOrAdd(workerId, id => new WorkerBuffer(id, GamesPerBatch));

        // Add game to buffer (thread-safe)
        if (buffer.TryAddAndCheckFull(gameRecord))
        {
            // Buffer is full, commit it
            CommitBuffer(buffer);
            _workerBuffers.TryRemove(workerId, out _);
        }
    }

    /// <summary>
    /// Record a single move (legacy method - converts to game-level storage).
    /// </summary>
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

        // This is a legacy method - for backward compatibility
        // In file-based storage, we only use RecordGame for complete games
        _logger.LogWarning("RecordMove is deprecated for file-based storage. Use RecordGame instead.");
    }

    /// <summary>
    /// Get all games for batch processing.
    /// Supports pagination via manifest index.
    /// </summary>
    public List<SelfPlayGameRecord> GetGames(int limit = 1000, int offset = 0)
    {
        EnsureInitialized();

        var games = new List<SelfPlayGameRecord>(limit);

        lock (_manifestLock)
        {
            var globalIndex = 0;

            foreach (var batch in _manifest.Batches.OrderBy(b => b.Index))
            {
                var batchPath = Path.Combine(_basePath, "committed", batch.FileName);
                if (!File.Exists(batchPath)) continue;

                var batchGames = LoadGamesFromSgf(batchPath);

                foreach (var game in batchGames)
                {
                    if (globalIndex >= offset && globalIndex < offset + limit)
                    {
                        games.Add(game);
                    }
                    globalIndex++;

                    if (globalIndex >= offset + limit)
                        return games;
                }
            }
        }

        return games;
    }

    /// <summary>
    /// Get games filtered by result.
    /// </summary>
    public List<SelfPlayGameRecord> GetGamesByResult(int result, int limit = 1000)
    {
        EnsureInitialized();

        var games = new List<SelfPlayGameRecord>(limit);

        lock (_manifestLock)
        {
            // Use result index from manifest
            var gameIndices = result == 1 ? _manifest.ResultIndex.RedWins
                : result == 0 ? _manifest.ResultIndex.Draws
                : _manifest.ResultIndex.BlueWins;

            if (gameIndices == null || gameIndices.Count == 0) return games;

            // Cache batch contents to avoid repeated file reads
            var batchCache = new Dictionary<string, List<SelfPlayGameRecord>>();

            foreach (var gameIndex in gameIndices)
            {
                if (games.Count >= limit) break;

                // Find the batch that contains this gameIndex
                var batch = _manifest.Batches.FirstOrDefault(b =>
                    b.Offset <= gameIndex && gameIndex < b.Offset + b.Count);

                if (batch == null) continue;

                // Load batch if not cached
                if (!batchCache.TryGetValue(batch.FileName, out var batchGames))
                {
                    var batchPath = Path.Combine(_basePath, "committed", batch.FileName);
                    if (!File.Exists(batchPath))
                    {
                        batchCache[batch.FileName] = new List<SelfPlayGameRecord>();
                        continue;
                    }

                    batchGames = LoadGamesFromSgf(batchPath);
                    batchCache[batch.FileName] = batchGames;
                }

                var localIndex = gameIndex - batch.Offset;
                if (localIndex >= 0 && localIndex < batchGames.Count)
                {
                    games.Add(batchGames[localIndex]);
                }
            }
        }

        return games;
    }

    /// <summary>
    /// Get positions for verification (reconstructed from SGF move sequences).
    /// </summary>
    public IEnumerable<StagingPosition> GetPositionsForVerification(int maxPly = 16)
    {
        EnsureInitialized();

        var canonicalizer = new PositionCanonicalizer();

        lock (_manifestLock)
        {
            foreach (var batch in _manifest.Batches)
            {
                var batchPath = Path.Combine(_basePath, "committed", batch.FileName);
                if (!File.Exists(batchPath)) continue;

                var games = LoadGamesFromSgf(batchPath);

                foreach (var game in games)
                {
                    // Reconstruct positions from SGF
                    foreach (var position in ReconstructPositions(game, maxPly, canonicalizer))
                    {
                        yield return position;
                    }
                }
            }
        }
    }

    /// <summary>
    /// Get aggregated statistics for all positions.
    /// </summary>
    public Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> GetPositionStatistics()
    {
        EnsureInitialized();
        var stats = new Dictionary<(ulong, ulong, Player), PositionStatistics>();

        // Aggregate statistics from positions
        foreach (var position in GetPositionsForVerification())
        {
            var key = (position.CanonicalHash, position.DirectHash, position.Player);
            if (!stats.TryGetValue(key, out var existing))
            {
                existing = new PositionStatistics
                {
                    PlayCount = 0,
                    WinCount = 0,
                    WinRate = 0,
                    AvgTimeBudgetMs = 0,
                    DrawCount = 0,
                    LossCount = 0
                };
                stats[key] = existing;
            }

            // Update statistics
            stats[key] = existing with
            {
                PlayCount = existing.PlayCount + 1,
                WinCount = existing.WinCount + (position.GameResult == 1 ? 1 : 0),
                LossCount = existing.LossCount + (position.GameResult == -1 ? 1 : 0),
                DrawCount = existing.DrawCount + (position.GameResult == 0 ? 1 : 0),
                WinRate = (double)(existing.WinCount + (position.GameResult == 1 ? 1 : 0)) / (existing.PlayCount + 1),
                AvgTimeBudgetMs = (existing.AvgTimeBudgetMs * existing.PlayCount + position.TimeBudgetMs) / (existing.PlayCount + 1)
            };
        }

        return stats;
    }

    /// <summary>
    /// Get all moves played for a specific position.
    /// </summary>
    public List<StagingMove> GetMovesForPosition(ulong canonicalHash, ulong directHash, Player player)
    {
        EnsureInitialized();
        var moves = new List<StagingMove>();
        var seenMoves = new HashSet<(int, int)>();

        foreach (var position in GetPositionsForVerification())
        {
            if (position.CanonicalHash == canonicalHash &&
                position.DirectHash == directHash &&
                position.Player == player)
            {
                var moveKey = (position.MoveX, position.MoveY);
                if (seenMoves.Contains(moveKey)) continue;

                seenMoves.Add(moveKey);
                moves.Add(new StagingMove
                {
                    MoveX = position.MoveX,
                    MoveY = position.MoveY,
                    Ply = position.Ply,
                    GameResult = position.GameResult,
                    PlayCount = 1,
                    WinRate = position.GameResult == 1 ? 1.0 : 0.0
                });
            }
        }

        return moves;
    }

    public void Flush()
    {
        EnsureInitialized();

        // Commit worker buffers in sorted order by worker ID for deterministic indexing
        foreach (var buffer in _workerBuffers.OrderBy(kv => kv.Key).Select(kv => kv.Value))
        {
            CommitBuffer(buffer);
        }
        _workerBuffers.Clear();

        SaveManifest();
        _logger.LogDebug("Flushed all buffers and saved manifest");
    }

    public void Clear()
    {
        EnsureInitialized();

        lock (_manifestLock)
        {
            // Delete all files
            try
            {
                var bufferDir = Path.Combine(_basePath, "buffer");
                var committedDir = Path.Combine(_basePath, "committed");

                if (Directory.Exists(bufferDir))
                    Directory.Delete(bufferDir, true);
                if (Directory.Exists(committedDir))
                    Directory.Delete(committedDir, true);

                _manifest = new SgfManifest();
                SaveManifest();

                _logger.LogInformation("Cleared all staging data");
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Failed to clear staging data");
                throw;
            }
        }
    }

    public long GetGameCount()
    {
        EnsureInitialized();
        return _manifest.TotalGames;
    }

    public long GetPositionCount()
    {
        EnsureInitialized();
        // Each game has ~maxPly positions
        return _manifest.TotalGames * 16L; // Rough estimate
    }

    public void Dispose()
    {
        if (_isDisposed) return;

        try
        {
            Flush();
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Error during dispose");
        }
        finally
        {
            _isDisposed = true;
        }
    }

    private void EnsureInitialized()
    {
        if (!_isInitialized)
        {
            Initialize();
        }
    }

    private string GetManifestPath()
    {
        return Path.Combine(_basePath, "manifest.json");
    }

    private void SaveManifest()
    {
        var manifestPath = GetManifestPath();
        var tempPath = manifestPath + ".tmp";
        var json = JsonSerializer.Serialize(_manifest, new JsonSerializerOptions
        {
            WriteIndented = false
        });

        // Atomic write: write to temp file, then rename with overwrite
        File.WriteAllText(tempPath, json);
        File.Move(tempPath, manifestPath, overwrite: true);
    }

    private void CommitBuffer(WorkerBuffer buffer)
    {
        if (buffer.GameCount == 0) return;

        // Get snapshot of games to commit
        var gamesToCommit = buffer.GetGamesSnapshot();
        if (gamesToCommit.Count == 0) return;

        lock (_manifestLock)
        {
            // Determine batch index
            var batchIndex = _manifest.Batches.Count;
            var fileName = $"batch_{batchIndex:D4}.sgf";

            // Write committed file
            var committedPath = Path.Combine(_basePath, "committed", fileName);
            var sgfContent = new StringBuilder();
            foreach (var game in gamesToCommit)
            {
                sgfContent.AppendLine(game.SgfMoves);
            }
            File.WriteAllText(committedPath, sgfContent.ToString());

            // Update manifest
            _manifest.Batches.Add(new SgfBatch
            {
                Index = batchIndex,
                FileName = fileName,
                Offset = _manifest.TotalGames,
                Count = gamesToCommit.Count
            });

            // Update result index
            for (int i = 0; i < gamesToCommit.Count; i++)
            {
                var game = gamesToCommit[i];
                var gameIndex = _manifest.TotalGames + i;

                if (game.Winner == Player.Red)
                    _manifest.ResultIndex.RedWins.Add(gameIndex);
                else if (game.Winner == Player.Blue)
                    _manifest.ResultIndex.BlueWins.Add(gameIndex);
                else
                    _manifest.ResultIndex.Draws.Add(gameIndex);
            }

            _manifest.TotalGames += gamesToCommit.Count;

            _logger.LogDebug("Committed batch {Index} with {Count} games", batchIndex, gamesToCommit.Count);
        }
    }

    private static List<SelfPlayGameRecord> LoadGamesFromSgf(string filePath)
    {
        var games = new List<SelfPlayGameRecord>();
        var content = File.ReadAllText(filePath);
        var lines = content.Split('\n', StringSplitOptions.RemoveEmptyEntries);

        foreach (var line in lines)
        {
            if (string.IsNullOrWhiteSpace(line)) continue;

            var game = ParseSgfGame(line);
            if (game != null)
                games.Add(game);
        }

        return games;
    }

    private static SelfPlayGameRecord? ParseSgfGame(string sgf)
    {
        if (string.IsNullOrWhiteSpace(sgf)) return null;

        try
        {
            // Parse SGF format: (;GM[1]SZ[19]RE[B]B[aa];W[bb];...;)
            var moves = SelfPlayGameRecord.FromSgf(sgf);
            var winner = Player.None;

            // Extract result from RE tag
            var reMatch = System.Text.RegularExpressions.Regex.Match(sgf, @"RE\[([BW])\]");
            if (reMatch.Success)
            {
                winner = reMatch.Groups[1].Value == "B" ? Player.Red : Player.Blue;
            }

            return new SelfPlayGameRecord
            {
                GameId = 0, // Will be set by index
                SgfMoves = sgf,
                Winner = winner,
                TotalMoves = moves.Count,
                MoveList = moves
            };
        }
        catch
        {
            return null;
        }
    }

    private static List<StagingPosition> ReconstructPositions(SelfPlayGameRecord game, int maxPly, PositionCanonicalizer canonicalizer)
    {
        var positions = new List<StagingPosition>();
        if (game.MoveList == null || game.MoveList.Count == 0) return positions;

        // Create board and replay moves
        var board = new Board();
        var gameResult = game.Winner == Player.Red ? 1 : game.Winner == Player.Blue ? -1 : 0;

        for (int ply = 0; ply < Math.Min(game.MoveList.Count, maxPly); ply++)
        {
            var (x, y) = game.MoveList[ply];
            var player = ply % 2 == 0 ? Player.Red : Player.Blue;
            var canonical = canonicalizer.Canonicalize(board);

            positions.Add(new StagingPosition
            {
                CanonicalHash = canonical.CanonicalHash,
                DirectHash = board.GetHash(),
                Player = player,
                Ply = ply,
                MoveX = x,
                MoveY = y,
                TimeBudgetMs = 0,
                GameResult = gameResult
            });

            board = board.PlaceStone(x, y, player);
        }

        return positions;
    }

    private sealed class WorkerBuffer
    {
        private readonly object _lock = new();
        public int WorkerId { get; }
        public int Capacity { get; }
        private readonly List<SelfPlayGameRecord> _games = new();
        public int GameCount
        {
            get { lock (_lock) return _games.Count; }
        }
        public bool IsFull => GameCount >= Capacity;

        public WorkerBuffer(int workerId, int capacity)
        {
            WorkerId = workerId;
            Capacity = capacity;
        }

        /// <summary>
        /// Add a game and return true if buffer became full (caller should commit).
        /// Thread-safe via locking.
        /// </summary>
        public bool TryAddAndCheckFull(SelfPlayGameRecord game)
        {
            lock (_lock)
            {
                _games.Add(game);
                return _games.Count >= Capacity;
            }
        }

        /// <summary>
        /// Get a snapshot of games for committing.
        /// </summary>
        public List<SelfPlayGameRecord> GetGamesSnapshot()
        {
            lock (_lock)
            {
                return new List<SelfPlayGameRecord>(_games);
            }
        }

        public string GetSgfContent()
        {
            lock (_lock)
            {
                var sb = new StringBuilder();
                foreach (var game in _games)
                {
                    sb.AppendLine(game.SgfMoves);
                }
                return sb.ToString();
            }
        }
    }
}

/// <summary>
/// Manifest for tracking committed batches and game indices.
/// </summary>
public sealed class SgfManifest
{
    public int Version { get; set; } = 1;
    public int TotalGames { get; set; } = 0;
    public List<SgfBatch> Batches { get; set; } = new();
    public ResultIndex ResultIndex { get; set; } = new();
}

public sealed class SgfBatch
{
    public int Index { get; set; }
    public required string FileName { get; init; }
    public int Offset { get; init; }
    public int Count { get; init; }
}

public sealed class ResultIndex
{
    public List<int> RedWins { get; set; } = new();
    public List<int> BlueWins { get; set; } = new();
    public List<int> Draws { get; set; } = new();
}
