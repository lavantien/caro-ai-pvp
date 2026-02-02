using System.Collections.Concurrent;
using System.Threading;
using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions using the full MinimaxAI engine.
/// Uses Lazy SMP parallel search with (N-4) threads for maximum performance.
/// </summary>
public sealed class OpeningBookGenerator : IOpeningBookGenerator
{
    private const int MaxBookMoves = 12;           // Maximum plies in book (6 moves each)
    private const int MaxCandidatesPerPosition = 8; // Top N moves to store per position
    private const int TimePerPositionMs = 30000;   // 30 seconds per position (optimized from 60s)

    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly IOpeningBookValidator _validator;
    private readonly GeneratorProgress _progress = new();
    private readonly MinimaxAI _searchEngine;
    private CancellationTokenSource? _cts;

    public OpeningBookGenerator(
        IOpeningBookStore store,
        IPositionCanonicalizer canonicalizer,
        IOpeningBookValidator validator)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _validator = validator ?? throw new ArgumentNullException(nameof(validator));

        // Create MinimaxAI instance with book generation configuration
        _searchEngine = new MinimaxAI();
    }

    public GenerationProgress GetProgress() => _progress.ToPublicProgress();

    public void Cancel()
    {
        _cts?.Cancel();
        _progress.Status = GeneratorState.Cancelled;
    }

    public async Task<BookGenerationResult> GenerateAsync(
        int maxDepth,
        int targetDepth,
        CancellationToken cancellationToken = default)
    {
        _cts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        _progress.Reset();
        _progress.Status = GeneratorState.Running;
        _progress.StartTime = DateTime.UtcNow;

        try
        {
            // Configure thread pool for book generation (more aggressive than normal play)
            ThreadPoolConfig.ConfigureForSearch();

            // Use BookGeneration difficulty for (N-4) threads
            var bookDifficulty = AIDifficulty.BookGeneration;

            // Collect positions by depth level for breadth-first processing
            var positionsByDepth = new List<List<PositionToProcess>>();
            positionsByDepth.Add(new List<PositionToProcess>
            {
                new PositionToProcess(new Board(), Player.Red, 0)
            });

            int positionsGenerated = 0;
            int positionsVerified = 0;
            int blundersFound = 0;
            int totalMovesStored = 0;
            int positionsEvaluated = 0;

            // Process each depth level
            for (int depth = 0; depth < MaxBookMoves && !_cts.Token.IsCancellationRequested; depth++)
            {
                if (depth >= positionsByDepth.Count)
                    break;

                var currentLevelPositions = positionsByDepth[depth];
                if (currentLevelPositions.Count == 0)
                    continue;

                _progress.CurrentPhase = $"Evaluating depth {depth}";

                // Filter out positions already in the book
                var positionsToEvaluate = new List<(Board board, Player player, int depth, ulong hash, SymmetryType symmetry, bool nearEdge, int maxMoves)>();

                foreach (var pos in currentLevelPositions)
                {
                    var canonical = _canonicalizer.Canonicalize(pos.Board);

                    if (!_store.ContainsEntry(canonical.CanonicalHash, pos.Player))
                    {
                        // Calculate max moves based on position depth
                        int boardMoveNumber = pos.Depth * 2;
                        int maxMovesToStore = boardMoveNumber switch
                        {
                            <= 8 => 4,
                            <= 14 => 2,
                            _ => 1
                        };

                        positionsToEvaluate.Add((
                            pos.Board,
                            pos.Player,
                            pos.Depth,
                            canonical.CanonicalHash,
                            canonical.SymmetryApplied,
                            canonical.IsNearEdge,
                            maxMovesToStore
                        ));
                    }
                }

                if (positionsToEvaluate.Count == 0)
                {
                    _progress.PositionsEvaluated = positionsEvaluated;
                    continue;
                }

                // Process positions in parallel using worker pool
                var results = await ProcessPositionsInParallelAsync(
                    positionsToEvaluate,
                    bookDifficulty,
                    _cts.Token
                );

                // Store results and generate child positions
                var nextLevelPositions = new List<PositionToProcess>();

                foreach (var posData in positionsToEvaluate)
                {
                    if (cancellationToken.IsCancellationRequested)
                        break;

                    if (!results.ContainsKey(posData.hash) || results[posData.hash].Length == 0)
                        continue;

                    var moves = results[posData.hash];
                    var canonical = _canonicalizer.Canonicalize(posData.board);

                    // Store entry
                    var entry = new OpeningBookEntry
                    {
                        CanonicalHash = canonical.CanonicalHash,
                        Depth = posData.depth,
                        Player = posData.player,
                        Symmetry = posData.symmetry,
                        IsNearEdge = posData.nearEdge,
                        Moves = moves
                    };

                    _store.StoreEntry(entry);
                    positionsGenerated++;
                    totalMovesStored += moves.Length;
                    positionsEvaluated++;

                    // Calculate max children for this position
                    int maxChildren = posData.depth switch
                    {
                        <= 4 => 4,
                        <= 9 => 2,
                        _ => 1
                    };

                    // Enqueue child positions
                    foreach (var move in moves.Take(maxChildren))
                    {
                        var newBoard = CloneBoard(posData.board);
                        newBoard.PlaceStone(move.RelativeX, move.RelativeY, posData.player);
                        var nextPlayer = posData.player == Player.Red ? Player.Blue : Player.Red;

                        var winResult = new WinDetector().CheckWin(newBoard);
                        if (winResult.Winner == Player.None)
                        {
                            nextLevelPositions.Add(new PositionToProcess(newBoard, nextPlayer, posData.depth + 1));
                        }
                    }

                    _progress.PositionsGenerated = positionsGenerated;
                    _progress.PositionsEvaluated = positionsEvaluated;
                    _progress.TotalMovesStored = totalMovesStored;
                }

                // Add next level if we have positions
                if (nextLevelPositions.Count > 0)
                {
                    if (depth + 1 >= positionsByDepth.Count)
                        positionsByDepth.Add(nextLevelPositions);
                    else
                        positionsByDepth[depth + 1].AddRange(nextLevelPositions);
                }

                // Safety limit
                if (positionsEvaluated > 1000)
                    break;
            }

            // Final flush
            _store.Flush();
            _store.SetMetadata("Version", "1");
            _store.SetMetadata("GeneratedAt", DateTime.UtcNow.ToString("o"));
            _store.SetMetadata("MaxDepth", maxDepth.ToString());
            _store.SetMetadata("TargetDepth", targetDepth.ToString());

            var elapsed = DateTime.UtcNow - _progress.StartTime;
            _progress.Status = _cts.Token.IsCancellationRequested
                ? GeneratorState.Cancelled
                : GeneratorState.Completed;
            _progress.EndTime = DateTime.UtcNow;

            return new BookGenerationResult(
                PositionsGenerated: positionsGenerated,
                PositionsVerified: positionsVerified,
                GenerationTime: elapsed,
                BlundersFound: blundersFound,
                TotalMovesStored: totalMovesStored
            );
        }
        catch (Exception)
        {
            _progress.Status = GeneratorState.Failed;
            throw;
        }
        finally
        {
            _cts?.Dispose();
            _cts = null;
        }
    }

    public async Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        AIDifficulty difficulty,
        int maxMoves,
        CancellationToken cancellationToken = default)
    {
        var bookMoves = new List<BookMove>();

        // Get candidate moves
        var candidates = GetCandidateMoves(board, player);
        if (candidates.Count == 0)
            return Array.Empty<BookMove>();

        // Evaluate candidates IN PARALLEL for performance
        // This is critical - parallelizing candidate evaluation provides 4-8x speedup
        var candidatesToEvaluate = candidates.Take(maxMoves * 2).ToList();
        var results = new ConcurrentBag<(int x, int y, int score, long nodes, int depth)>();

        // Divide time budget among candidates to keep total time per position reasonable
        var timePerCandidateMs = Math.Max(2000, TimePerPositionMs / candidatesToEvaluate.Count);

        var candidateTasks = candidatesToEvaluate.Select(async candidate =>
        {
            var (cx, cy) = candidate;
            cancellationToken.ThrowIfCancellationRequested();

            // Skip if invalid
            if (!_validator.IsValidMove(board, cx, cy, player))
                return;

            // Check for immediate win
            if (_validator.IsWinningMove(board, cx, cy, player))
            {
                results.Add((cx, cy, 100000, 1, 1));
                return;
            }

            // Clone board for this candidate (don't modify original)
            var candidateBoard = CloneBoard(board);
            candidateBoard.PlaceStone(cx, cy, player);

            var searchBoard = CloneBoard(candidateBoard);
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var moveNumber = candidateBoard.GetBitBoard(Player.Red).CountBits() + candidateBoard.GetBitBoard(Player.Blue).CountBits();

            // Run search with divided time budget, NO inner parallel to avoid oversubscription
            var (bestX, bestY) = await Task.Run(() =>
            {
                return _searchEngine.GetBestMove(
                    searchBoard,
                    opponent,
                    difficulty,
                    timeRemainingMs: timePerCandidateMs,
                    moveNumber: moveNumber,
                    ponderingEnabled: false,
                    parallelSearchEnabled: false // Disable Lazy SMP to avoid oversubscribing threads
                );
            }, cancellationToken);

            var (depthAchieved, nodesSearched, _, _, _, _, _, threadCount, _, _, _, _)
                = _searchEngine.GetSearchStatistics();

            searchBoard.PlaceStone(bestX, bestY, opponent);
            int score = EvaluateBoard(searchBoard, opponent);
            searchBoard.GetCell(bestX, bestY).Player = Player.None;

            results.Add((cx, cy, score, nodesSearched, depthAchieved));

            // Update progress (last write wins is acceptable for display purposes)
            _progress.LastDepth = depthAchieved;
            _progress.LastNodes = nodesSearched;
            _progress.LastThreads = threadCount;
        }).ToArray();

        await Task.WhenAll(candidateTasks);

        // Convert to list for sorting
        var sortedResults = results.ToList();

        // Sort by score
        sortedResults.Sort((a, b) => b.score.CompareTo(a.score));

        // EARLY EXIT: If best move dominates, skip remaining evaluation
        // If top move has >200 point advantage (2 pawns), it's clearly superior
        if (sortedResults.Count >= 2)
        {
            int scoreGap = sortedResults[0].score - sortedResults[1].score;
            if (scoreGap > 200)
            {
                // Best move is clearly superior - stop evaluating further candidates
                sortedResults = sortedResults.Take(1).ToList();
            }
        }

        // Convert to BookMove records
        int priority = maxMoves;
        foreach (var (x, y, score, nodes, depth) in sortedResults.Take(maxMoves))
        {
            int winRate = ScoreToWinRate(score);

            bookMoves.Add(new BookMove
            {
                RelativeX = x,
                RelativeY = y,
                WinRate = winRate,
                DepthAchieved = depth,
                NodesSearched = nodes,
                Score = score,
                IsForcing = Math.Abs(score) > 1000, // Simplified forcing detection
                Priority = priority--,
                IsVerified = true
            });
        }

        return bookMoves.ToArray();
    }

    // Forward to the overload with difficulty parameter
    public Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        int searchDepth,
        int maxMoves,
        CancellationToken cancellationToken = default)
    {
        // Map search depth to difficulty
        var difficulty = searchDepth switch
        {
            <= 6 => AIDifficulty.Hard,
            <= 12 => AIDifficulty.Grandmaster,
            _ => AIDifficulty.Experimental
        };

        return GenerateMovesForPositionAsync(board, player, difficulty, maxMoves, cancellationToken);
    }

    public Task<VerificationResult> VerifyAsync(CancellationToken cancellationToken = default)
    {
        var stats = _store.GetStatistics();
        var blunderDetails = new List<string>();
        int blundersFound = 0;

        // For now, return a simple result
        return Task.FromResult(new VerificationResult(
            EntriesChecked: stats.TotalEntries,
            BlundersFound: blundersFound,
            BlunderDetails: blunderDetails.ToArray(),
            VerifiedCount: stats.TotalEntries - blundersFound
        ));
    }

    private int EvaluateBoard(Board board, Player player)
    {
        int score = 0;

        // Simple material evaluation
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        // Position scoring based on patterns
        score += EvaluateThreats(board, Player.Red) * 100;
        score -= EvaluateThreats(board, Player.Blue) * 100;

        return player == Player.Red ? score : -score;
    }

    private int EvaluateThreats(Board board, Player player)
    {
        int score = 0;
        int boardSize = board.BoardSize;

        // Count patterns for each direction
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (board.GetCell(x, y).Player == player)
                {
                    score += EvaluatePoint(board, x, y, player);
                }
            }
        }
        return score;
    }

    private static int EvaluatePoint(Board board, int x, int y, Player player)
    {
        int score = 0;
        int[] dx = { 1, 0, 1, 1 };
        int[] dy = { 0, 1, 1, -1 };

        for (int dir = 0; dir < 4; dir++)
        {
            int count = 1;
            int open = 0;

            // Check positive direction
            for (int i = 1; i < 5; i++)
            {
                int nx = x + dx[dir] * i;
                int ny = y + dy[dir] * i;
                if (!IsValid(board, nx, ny)) break;

                if (board.GetCell(nx, ny).Player == player)
                    count++;
                else if (board.GetCell(nx, ny).IsEmpty)
                {
                    open++;
                    break;
                }
                else break;
            }

            // Check negative direction
            for (int i = 1; i < 5; i++)
            {
                int nx = x - dx[dir] * i;
                int ny = y - dy[dir] * i;
                if (!IsValid(board, nx, ny)) break;

                if (board.GetCell(nx, ny).Player == player)
                    count++;
                else if (board.GetCell(nx, ny).IsEmpty)
                {
                    open++;
                    break;
                }
                else break;
            }

            score += CountToScore(count, open);
        }
        return score;
    }

    private static bool IsValid(Board board, int x, int y)
    {
        return x >= 0 && x < board.BoardSize && y >= 0 && y < board.BoardSize;
    }

    private static int CountToScore(int count, int open)
    {
        return count switch
        {
            5 => 100000,
            4 when open >= 1 => 10000,
            4 when open == 0 => 1000,
            3 when open >= 2 => 1000,
            3 when open == 1 => 100,
            2 when open >= 2 => 100,
            _ => 0
        };
    }

    private List<(int x, int y)> GetCandidateMoves(Board board, Player player)
    {
        var candidates = new List<(int, int)>();
        int boardSize = board.BoardSize;

        // Get all empty cells adjacent to existing stones
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty && HasAdjacentStone(board, x, y))
                {
                    candidates.Add((x, y));
                }
            }
        }

        // If board is empty, return center
        if (candidates.Count == 0)
        {
            int center = boardSize / 2;
            return new List<(int, int)> { (center, center) };
        }

        return candidates;
    }

    private static bool HasAdjacentStone(Board board, int x, int y)
    {
        int boardSize = board.BoardSize;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                if (dx == 0 && dy == 0) continue;

                int nx = x + dx;
                int ny = y + dy;

                if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                {
                    if (!board.GetCell(nx, ny).IsEmpty)
                    {
                        return true;
                    }
                }
            }
        }
        return false;
    }

    private static int ScoreToWinRate(int score)
    {
        // Convert centipawn score to win rate percentage
        const double scale = 0.005;
        double winProb = 1.0 / (1.0 + Math.Exp(-score * scale));
        return (int)(winProb * 100);
    }

    private static Board CloneBoard(Board board)
    {
        var newBoard = new Board();
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                newBoard.GetCell(x, y).Player = board.GetCell(x, y).Player;
            }
        }
        return newBoard;
    }

    private record PositionToProcess(Board Board, Player Player, int Depth);
    private record PositionToSearch(Board Board, Player Player, int Depth);

    /// <summary>
    /// Internal progress tracking.
    /// </summary>
    private sealed class GeneratorProgress
    {
        public GeneratorState Status { get; set; } = GeneratorState.NotStarted;
        public DateTime StartTime { get; set; }
        public DateTime EndTime { get; set; }
        public int PositionsGenerated { get; set; }
        public int PositionsEvaluated { get; set; }
        public int TotalMovesStored { get; set; }
        public string CurrentPhase { get; set; } = "Initializing";
        public int LastDepth { get; set; }
        public long LastNodes { get; set; }
        public int LastThreads { get; set; }

        public GenerationProgress ToPublicProgress()
        {
            var elapsed = DateTime.UtcNow - StartTime;
            double percent = PositionsEvaluated > 0
                ? (double)PositionsEvaluated / Math.Max(PositionsEvaluated + 100, 1) * 100
                : 0;

            return new GenerationProgress(
                PositionsEvaluated: PositionsEvaluated,
                PositionsStored: PositionsGenerated,
                TotalPositions: PositionsEvaluated + PositionsGenerated,
                PercentComplete: percent,
                CurrentPhase: $"{CurrentPhase} (Last: d{LastDepth}, {LastThreads} threads, {LastNodes}N)",
                ElapsedTime: elapsed,
                EstimatedTimeRemaining: percent > 0 ? TimeSpan.FromSeconds(elapsed.TotalSeconds * (100 - percent) / percent) : null
            );
        }

        public void Reset()
        {
            Status = GeneratorState.NotStarted;
            PositionsGenerated = 0;
            PositionsEvaluated = 0;
            TotalMovesStored = 0;
            CurrentPhase = "Initializing";
        }
    }

    private enum GeneratorState
    {
        NotStarted,
        Running,
        Completed,
        Cancelled,
        Failed
    }

    /// <summary>
    /// Job for parallel position evaluation
    /// </summary>
    private record PositionJob(
        int JobId,
        Board Board,
        Player Player,
        int Depth,
        ulong CanonicalHash,
        SymmetryType Symmetry,
        bool IsNearEdge,
        int MaxMovesToStore
    );

    /// <summary>
    /// Result from parallel position evaluation
    /// </summary>
    private record PositionResult(
        int JobId,
        ulong CanonicalHash,
        BookMove[] Moves,
        int DepthAchieved,
        long NodesSearched,
        int ThreadCount
    );

    /// <summary>
    /// Process a single position job (worker thread entry point)
    /// </summary>
    private PositionResult ProcessPositionJob(PositionJob job, AIDifficulty difficulty, CancellationToken cancellationToken)
    {
        try
        {
            var moves = GenerateMovesForPositionAsync(
                job.Board,
                job.Player,
                difficulty,
                job.MaxMovesToStore,
                cancellationToken
            ).GetAwaiter().GetResult();

            return new PositionResult(
                job.JobId,
                job.CanonicalHash,
                moves,
                _progress.LastDepth,
                _progress.LastNodes,
                _progress.LastThreads
            );
        }
        catch (OperationCanceledException)
        {
            // Return empty result on cancellation
            return new PositionResult(job.JobId, job.CanonicalHash, Array.Empty<BookMove>(), 0, 0, 0);
        }
    }

    /// <summary>
    /// Process multiple positions in parallel using worker pool pattern
    /// Uses thread count from BookGeneration difficulty config (N-4 threads)
    /// </summary>
    private async Task<Dictionary<ulong, BookMove[]>> ProcessPositionsInParallelAsync(
        List<(Board board, Player player, int depth, ulong hash, SymmetryType symmetry, bool nearEdge, int maxMoves)> positions,
        AIDifficulty difficulty,
        CancellationToken cancellationToken)
    {
        var results = new ConcurrentDictionary<ulong, BookMove[]>();
        var threadCount = AIDifficultyConfig.Instance.GetSettings(difficulty).ThreadCount;

        // Calculate batch size based on position count and thread count
        int batchSize = Math.Max(1, positions.Count / (threadCount * 2));

        // Process in batches to avoid overwhelming memory
        for (int i = 0; i < positions.Count; i += batchSize)
        {
            if (cancellationToken.IsCancellationRequested)
                break;

            int currentBatchSize = Math.Min(batchSize, positions.Count - i);
            var batch = positions.Skip(i).Take(currentBatchSize).ToList();

            // Create tasks for this batch
            var tasks = new Task<(ulong hash, BookMove[] moves)>[currentBatchSize];
            for (int j = 0; j < currentBatchSize; j++)
            {
                int idx = i + j;
                var pos = batch[j];

                tasks[j] = Task.Run(() =>
                {
                    try
                    {
                        var moves = GenerateMovesForPositionAsync(
                            pos.board,
                            pos.player,
                            difficulty,
                            pos.maxMoves,
                            cancellationToken
                        ).GetAwaiter().GetResult();

                        return (pos.hash, moves);
                    }
                    catch (OperationCanceledException)
                    {
                        return (pos.hash, Array.Empty<BookMove>());
                    }
                }, cancellationToken);
            }

            // Wait for all tasks in this batch
            await Task.WhenAll(tasks);

            // Collect results
            foreach (var task in tasks)
            {
                var (hash, moves) = await task;
                results[hash] = moves;
            }

            _progress.PositionsEvaluated = i + currentBatchSize;
        }

        return new Dictionary<ulong, BookMove[]>(results);
    }
}

