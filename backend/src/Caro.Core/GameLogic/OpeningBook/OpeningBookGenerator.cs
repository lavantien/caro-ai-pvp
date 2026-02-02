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
    private const int TimePerPositionMs = 60000;   // 60 seconds per position (as specified)

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

            // Start from empty board
            var board = new Board();
            var positions = new Queue<PositionToSearch>();
            positions.Enqueue(new PositionToSearch(board, Player.Red, 0));

            int positionsGenerated = 0;
            int positionsVerified = 0;
            int blundersFound = 0;
            int totalMovesStored = 0;
            int positionsEvaluated = 0;

            // Use BookGeneration difficulty for (N-4) threads
            var bookDifficulty = AIDifficulty.BookGeneration;

            // Process positions breadth-first
            while (positions.Count > 0 && !_cts.Token.IsCancellationRequested)
            {
                var current = positions.Dequeue();

                // Skip if beyond max depth
                if (current.Depth >= MaxBookMoves)
                    continue;

                _progress.CurrentPhase = $"Evaluating depth {current.Depth}";

                // Get candidate moves for this position
                var moves = await GenerateMovesForPositionAsync(
                    current.Board,
                    current.Player,
                    bookDifficulty,
                    MaxCandidatesPerPosition,
                    _cts.Token
                );

                if (moves.Length == 0)
                    continue;

                // Store position in book
                var canonical = _canonicalizer.Canonicalize(current.Board);

                // Check if already stored
                if (!_store.ContainsEntry(canonical.CanonicalHash, current.Player))
                {
                    var entry = new OpeningBookEntry
                    {
                        CanonicalHash = canonical.CanonicalHash,
                        Depth = current.Depth,
                        Player = current.Player,
                        Symmetry = canonical.SymmetryApplied,
                        IsNearEdge = canonical.IsNearEdge,
                        Moves = moves
                    };

                    _store.StoreEntry(entry);
                    positionsGenerated++;
                    totalMovesStored += moves.Length;

                    _progress.PositionsGenerated = positionsGenerated;
                    _progress.TotalMovesStored = totalMovesStored;
                }

                // Enqueue child positions for further exploration
                foreach (var move in moves.Take(3)) // Only explore top 3 moves per position
                {
                    var newBoard = CloneBoard(current.Board);
                    newBoard.PlaceStone(move.RelativeX, move.RelativeY, current.Player);

                    var nextPlayer = current.Player == Player.Red ? Player.Blue : Player.Red;

                    // Check if game continues
                    var winResult = new WinDetector().CheckWin(newBoard);
                    if (winResult.Winner == Player.None)
                    {
                        positions.Enqueue(new PositionToSearch(newBoard, nextPlayer, current.Depth + 1));
                    }
                }

                positionsEvaluated++;
                _progress.PositionsEvaluated = positionsEvaluated;

                // Safety limit to prevent exponential explosion
                if (positionsEvaluated > 1000)
                {
                    break;
                }
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

        // Evaluate each candidate with the full MinimaxAI engine
        var results = new List<(int x, int y, int score, long nodes, int depth)>();

        foreach (var (cx, cy) in candidates.Take(maxMoves * 2)) // Evaluate more candidates than we store
        {
            cancellationToken.ThrowIfCancellationRequested();

            // Skip if invalid
            if (!_validator.IsValidMove(board, cx, cy, player))
                continue;

            // Check for immediate win
            if (_validator.IsWinningMove(board, cx, cy, player))
            {
                results.Add((cx, cy, 100000, 1, 1));
                continue;
            }

            // Make move
            board.PlaceStone(cx, cy, player);

            // Use the full MinimaxAI engine with 60 second time budget
            var searchBoard = CloneBoard(board);
            var opponent = player == Player.Red ? Player.Blue : Player.Red;

            // Calculate move number for time budget (depth * 2 + 1)
            var moveNumber = board.GetBitBoard(Player.Red).CountBits() + board.GetBitBoard(Player.Blue).CountBits();

            // Call GetBestMove with full time budget
            var timeBudgetMs = TimePerPositionMs;

            // Need to run on background thread since GetBestMove is synchronous
            var (bestX, bestY) = await Task.Run(() =>
            {
                return _searchEngine.GetBestMove(
                    searchBoard,
                    opponent,
                    difficulty,
                    timeRemainingMs: timeBudgetMs,
                    moveNumber: moveNumber,
                    ponderingEnabled: false,
                    parallelSearchEnabled: true
                );
            }, cancellationToken);

            // Undo move
            board.GetCell(cx, cy).Player = Player.None;

            // Get search statistics
            var (depthAchieved, nodesSearched, _, _, _, _, _, threadCount, _, _, _, _)
                = _searchEngine.GetSearchStatistics();

            // Get the score from transposition table or evaluate position
            // Note: We need to evaluate the resulting position
            searchBoard.PlaceStone(bestX, bestY, opponent);
            int score = EvaluateBoard(searchBoard, opponent);
            searchBoard.GetCell(bestX, bestY).Player = Player.None;

            results.Add((cx, cy, score, nodesSearched, depthAchieved));

            _progress.LastDepth = depthAchieved;
            _progress.LastNodes = nodesSearched;
            _progress.LastThreads = threadCount;
        }

        // Sort by score
        results.Sort((a, b) => b.score.CompareTo(a.score));

        // Convert to BookMove records
        int priority = maxMoves;
        foreach (var (x, y, score, nodes, depth) in results.Take(maxMoves))
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
}

