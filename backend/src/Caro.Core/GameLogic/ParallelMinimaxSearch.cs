using System.Collections.Concurrent;
using System.Diagnostics;
using Caro.Core.Entities;
using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.GameLogic;

/// <summary>
/// Result from parallel search including move and statistics
/// </summary>
public record ParallelSearchResult(int X, int Y, int DepthAchieved, long NodesSearched);

/// <summary>
/// Parallel Minimax search using Lazy SMP (Shared Memory Parallelism)
/// Multiple threads search independently with shared transposition table
/// Provides 4-8× speedup on multi-core systems
/// Time-aware iterative deepening with move stability detection
/// </summary>
public sealed class ParallelMinimaxSearch
{
    private readonly LockFreeTranspositionTable _transpositionTable;
    private readonly BoardEvaluator _evaluator;
    private readonly WinDetector _winDetector;
    private readonly ThreatSpaceSearch _vcfSolver;
    private readonly Random _random;
    private readonly int _maxThreads;

    // Search constants
    private const int SearchRadius = 2;
    private const int NullMoveMinDepth = 3;
    private const int NullMoveDepthReduction = 3;

    // Time management - shared flag for all threads to check
    private volatile bool _searchShouldStop;
    private Stopwatch? _searchStopwatch;
    private long _hardTimeBoundMs;

    // Per-thread data (not shared between threads)
    private sealed class ThreadData
    {
        public (int x, int y)[,] KillerMoves = new (int x, int y)[20, 2];
        public int[,] HistoryRed = new int[15, 15];
        public int[,] HistoryBlue = new int[15, 15];
        public int TableHits;
        public int TableLookups;
        public Random Random = new();

        public void Reset()
        {
            // Clear killer moves
            for (int i = 0; i < 20; i++)
            {
                KillerMoves[i, 0] = (-1, -1);
                KillerMoves[i, 1] = (-1, -1);
            }
        }
    }

    /// <summary>
    /// Create parallel search instance
    /// </summary>
    /// <param name="sizeMB">Transposition table size in MB</param>
    /// <param name="maxThreads">Maximum threads to use (default: processor count - 1)</param>
    public ParallelMinimaxSearch(int sizeMB = 256, int? maxThreads = null)
    {
        _transpositionTable = new LockFreeTranspositionTable(sizeMB);
        _evaluator = new BoardEvaluator();
        _winDetector = new WinDetector();
        _vcfSolver = new ThreatSpaceSearch();
        _random = new Random();
        _maxThreads = maxThreads ?? ThreadPoolConfig.GetOptimalThreadCount();

        // Configure thread pool for CPU-bound work
        ThreadPoolConfig.ConfigureForSearch();
    }

    /// <summary>
    /// Get best move using parallel search (Lazy SMP)
    /// </summary>
    public (int x, int y) GetBestMove(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var baseDepth = (int)difficulty;
        var candidates = GetCandidateMoves(board);

        // Apply Open Rule: Red's second move (move #3) cannot be in center 3x3 zone
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => !(c.x >= 6 && c.x <= 8 && c.y >= 6 && c.y <= 8)).ToList();
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            if (player == Player.Red && moveNumber == 3)
            {
                // Open rule applies - find first valid cell outside center 3x3
                for (int x = 0; x < 15; x++)
                {
                    for (int y = 0; y < 15; y++)
                    {
                        if (board.GetCell(x, y).Player == Player.None && !(x >= 6 && x <= 8 && y >= 6 && y <= 8))
                            return (x, y);
                    }
                }
            }
            return (7, 7); // Center move
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // Try VCF first for higher difficulties
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}");
                return vcfResult.BestMove.Value;
            }
        }

        // Check for opponent's immediate threats that must be blocked
        // This includes 4-in-row and other forced win patterns
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreatMoves = GetOpponentThreatMoves(board, opponent);
        if (opponentThreatMoves.Count > 0)
        {
            // Must block the opponent's immediate threat
            Console.WriteLine($"[AI] Blocking opponent threat at ({opponentThreatMoves[0].x}, {opponentThreatMoves[0].y})");
            return opponentThreatMoves[0];
        }

        // Adjust depth based on time allocation
        var adjustedDepth = CalculateDepthForTime(baseDepth, alloc, candidates.Count);

        // For Beginner difficulty, add randomness
        if (difficulty == AIDifficulty.Beginner && _random.Next(100) < 20)
        {
            return candidates[_random.Next(candidates.Count)];
        }

        // Single-threaded for low depths (overhead not worth it)
        if (adjustedDepth <= 3)
        {
            return SearchSingleThreaded(board, player, adjustedDepth, candidates);
        }

        // Multi-threaded Lazy SMP for deeper searches
        var parallelResult = SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc);
        return (parallelResult.X, parallelResult.Y);
    }

    /// <summary>
    /// Get best move using parallel search with full statistics reporting
    /// Returns move coordinates along with depth achieved and nodes searched
    /// </summary>
    public ParallelSearchResult GetBestMoveWithStats(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var baseDepth = (int)difficulty;
        var candidates = GetCandidateMoves(board);

        // Apply Open Rule: Red's second move (move #3) cannot be in center 3x3 zone
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => !(c.x >= 6 && c.x <= 8 && c.y >= 6 && c.y <= 8)).ToList();
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - return center with minimal stats
            return new ParallelSearchResult(7, 7, 0, 0);
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // Try VCF first for higher difficulties
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}");
                return new ParallelSearchResult(vcfResult.BestMove.Value.x, vcfResult.BestMove.Value.y,
                    vcfResult.DepthAchieved, vcfResult.NodesSearched);
            }
        }

        // Check for opponent's immediate threats that must be blocked
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreatMoves = GetOpponentThreatMoves(board, opponent);
        if (opponentThreatMoves.Count > 0)
        {
            // Must block the opponent's immediate threat
            Console.WriteLine($"[AI] Blocking opponent threat at ({opponentThreatMoves[0].x}, {opponentThreatMoves[0].y})");
            return new ParallelSearchResult(opponentThreatMoves[0].x, opponentThreatMoves[0].y, 1, candidates.Count);
        }

        // Adjust depth based on time allocation
        var adjustedDepth = CalculateDepthForTime(baseDepth, alloc, candidates.Count);

        // For Beginner difficulty, add randomness
        if (difficulty == AIDifficulty.Beginner && _random.Next(100) < 20)
        {
            var randomMove = candidates[_random.Next(candidates.Count)];
            return new ParallelSearchResult(randomMove.x, randomMove.y, 1, candidates.Count);
        }

        // Single-threaded for low depths (overhead not worth it)
        if (adjustedDepth <= 3)
        {
            var (x, y) = SearchSingleThreaded(board, player, adjustedDepth, candidates);
            // Estimate nodes for single-threaded search
            long estimatedNodes = EstimateNodes(adjustedDepth, candidates.Count);
            return new ParallelSearchResult(x, y, adjustedDepth, estimatedNodes);
        }

        // Multi-threaded Lazy SMP for deeper searches
        return SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc);
    }

    /// <summary>
    /// Single-threaded search (fallback for low depths)
    /// </summary>
    private (int x, int y) SearchSingleThreaded(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        var threadData = new ThreadData();
        _transpositionTable.IncrementAge();

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        foreach (var (x, y) in candidates)
        {
            board.PlaceStone(x, y, player);
            var score = Minimax(board, depth - 1, int.MinValue, int.MaxValue, false, player, depth, threadData);
            board.GetCell(x, y).Player = Player.None;

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }
        }

        return bestMove;
    }

    /// <summary>
    /// Lazy SMP: Multiple threads search independently with shared TT
    /// Each thread has slight variation to explore different parts of tree
    /// Time-aware iterative deepening with move stability detection
    /// </summary>
    private ParallelSearchResult SearchLazySMP(
        Board board,
        Player player,
        int depth,
        List<(int x, int y)> candidates,
        AIDifficulty difficulty,
        TimeAllocation timeAlloc)
    {
        _transpositionTable.IncrementAge();

        // Set up time management
        _searchShouldStop = false;
        _searchStopwatch = Stopwatch.StartNew();
        _hardTimeBoundMs = timeAlloc.HardBoundMs;

        // Number of threads based on depth and available cores
        int threadCount = Math.Min(_maxThreads, Math.Max(2, depth / 2));
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes)>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[threadCount];
        var candidatesArray = new List<(int x, int y)>[threadCount];
        for (int i = 0; i < threadCount; i++)
        {
            boardsArray[i] = board.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel searches with time-aware iterative deepening
        var tasks = new List<Task>();
        for (int i = 0; i < threadCount; i++)
        {
            int threadId = i;
            var task = Task.Run(() =>
            {
                var threadData = new ThreadData { Random = new Random(threadId) };

                // Variation: slightly different aspiration window per thread
                int aspirationOffset = threadId * 25 - (threadCount * 12);

                var result = SearchWithIterationTimeAware(
                    boardsArray[threadId], player, depth, candidatesArray[threadId],
                    threadData, aspirationOffset, timeAlloc, difficulty);

                results.Add(result);
            });
            tasks.Add(task);
        }

        // Wait for all threads
        Task.WaitAll(tasks.ToArray());

        _searchStopwatch.Stop();

        // Return best result (prefer deeper results if scores are similar)
        var bestResult = results.OrderByDescending(r => r.score)
                                .ThenByDescending(r => r.depth)
                                .First();

        // Aggregate total nodes searched across all threads
        long totalNodes = results.Sum(r => r.nodes);

        Console.WriteLine($"[TIME] Depth: {bestResult.depth}, Time: {_searchStopwatch.ElapsedMilliseconds}ms/" +
                         $"{timeAlloc.SoftBoundMs}ms, Phase: {timeAlloc.Phase}, " +
                         $"Complexity: {timeAlloc.ComplexityMultiplier:F2}, Nodes: {totalNodes:N0}");

        return new ParallelSearchResult(bestResult.x, bestResult.y, bestResult.depth, totalNodes);
    }

    /// <summary>
    /// Iterative deepening search for a single thread with time awareness
    /// Implements move stability detection for early termination
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchWithIterationTimeAware(
        Board board,
        Player player,
        int targetDepth,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        int aspirationOffset,
        TimeAllocation timeAlloc,
        AIDifficulty difficulty)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        long nodesSearched = 0;
        int stableCount = 0; // Number of consecutive depths with same best move

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check if search should stop (time exceeded by another thread)
            if (_searchShouldStop)
            {
                break;
            }

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Check hard bound (absolute maximum) - set stop flag for other threads
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchShouldStop = true;
                break;
            }

            // Check soft bound with stability consideration
            if (elapsed >= timeAlloc.SoftBoundMs)
            {
                // If move is stable (same for 2+ consecutive depths), we can stop early
                if (stableCount >= 2)
                {
                    break;
                }
            }

            // Check optimal time - if move is very stable, exit
            if (elapsed >= timeAlloc.OptimalTimeMs && stableCount >= 3)
            {
                break;
            }

            int alpha = int.MinValue + aspirationOffset;
            int beta = int.MaxValue + aspirationOffset;

            // Aspiration window based on previous score
            if (currentDepth > 2)
            {
                alpha = bestScore - 50 + aspirationOffset;
                beta = bestScore + 50 + aspirationOffset;
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta);

            // Estimate nodes searched at this depth (rough approximation)
            // Each depth roughly multiplies nodes by branching factor
            long depthNodes = EstimateNodes(currentDepth, candidates.Count);
            nodesSearched += depthNodes;

            // Check move stability
            if (result.x == bestMove.Item1 && result.y == bestMove.Item2)
            {
                stableCount++;
            }
            else
            {
                stableCount = 1; // Reset but count this as one
                bestMove = (result.x, result.y);
            }

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            bestDepth = currentDepth;

            // Early exit on winning move
            if (result.score >= 100000)
            {
                break;
            }
        }

        return (bestMove.x, bestMove.y, bestScore, bestDepth, nodesSearched);
    }

    /// <summary>
    /// Estimate nodes searched for a given depth and candidate count
    /// This is a rough approximation for statistics reporting
    /// </summary>
    private long EstimateNodes(int depth, int candidateCount)
    {
        // Approximate: candidate_count * (branching_factor^(depth-1))
        // Where branching factor is capped at a reasonable value
        long nodes = candidateCount;
        int branching = Math.Min(candidateCount, 25); // Cap effective branching
        for (int i = 1; i < depth && i < 6; i++) // Limit estimation depth
        {
            nodes *= branching;
        }
        return nodes;
    }

    /// <summary>
    /// Root search with aspiration window
    /// </summary>
    private (int x, int y, int score) SearchRoot(
        Board board, Player player, int depth, List<(int x, int y)> candidates,
        ThreadData threadData, int alpha, int beta)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        var orderedMoves = OrderMoves(candidates, depth, board, player, null, threadData);

        foreach (var (x, y) in orderedMoves)
        {
            board.PlaceStone(x, y, player);
            var score = Minimax(board, depth - 1, alpha, beta, false, player, depth, threadData);
            board.GetCell(x, y).Player = Player.None;

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }

            alpha = Math.Max(alpha, score);
            if (beta <= alpha)
            {
                RecordKillerMove(threadData, depth, x, y);
                break;
            }
        }

        // Store in shared TT
        _transpositionTable.Store(board.Hash, (sbyte)depth, (short)bestScore, bestMove, alpha, beta);

        return (bestMove.x, bestMove.y, bestScore);
    }

    /// <summary>
    /// Minimax with alpha-beta pruning (thread-safe via per-thread data)
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth, ThreadData threadData)
    {
        // Time management check - check at every node for early termination
        // This ensures we respect time limits even during deep searches
        if (_searchShouldStop)
        {
            // Return a neutral score to allow the search to unwind
            return 0;
        }

        // Periodic time check at the start of each node (every 1000 nodes approximately)
        // This adds minimal overhead while ensuring we don't exceed time bounds
        if (_searchStopwatch != null && _hardTimeBoundMs > 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchShouldStop = true;
                return 0;
            }
        }

        // Terminal check
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            return Evaluate(board, aiPlayer);
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        // TT lookup
        var boardHash = board.Hash;
        threadData.TableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, (sbyte)depth, alpha, beta);
        if (found)
        {
            threadData.TableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);
        var orderedMoves = OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove, threadData);

        int bestScore = isMaximizing ? int.MinValue : int.MaxValue;
        (int x, int y)? bestMove = null;

        foreach (var (x, y) in orderedMoves)
        {
            board.PlaceStone(x, y, currentPlayer);
            var score = Minimax(board, depth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData);
            board.GetCell(x, y).Player = Player.None;

            // Check if search was stopped during recursion
            if (_searchShouldStop)
            {
                return bestScore; // Return best we found so far
            }

            if (isMaximizing)
            {
                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }
                alpha = Math.Max(alpha, score);
            }
            else
            {
                if (score < bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }
                beta = Math.Min(beta, score);
            }

            if (beta <= alpha)
            {
                // Cutoff
                if (depth >= 2 && depth < 20)
                {
                    RecordKillerMove(threadData, rootDepth - depth, x, y);
                }
                RecordHistoryMove(threadData, currentPlayer, x, y, depth);
                break;
            }
        }

        // Store result
        if (bestMove.HasValue)
        {
            _ = (bestScore <= alpha)
                ? LockFreeTranspositionTable.EntryFlag.UpperBound
                : (bestScore >= beta ? LockFreeTranspositionTable.EntryFlag.LowerBound : LockFreeTranspositionTable.EntryFlag.Exact);

            _transpositionTable.Store(boardHash, (sbyte)depth, (short)bestScore,
                (sbyte)bestMove.Value.x, (sbyte)bestMove.Value.y, alpha, beta);
        }

        return bestScore;
    }

    /// <summary>
    /// Order moves by priority (TT move > Killer > History > Position)
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove, ThreadData threadData)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        Span<int> scores = stackalloc int[count];
        var historyTable = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            int score = 0;

            // TT cached move (highest priority)
            if (cachedMove.HasValue && cachedMove.Value == (x, y))
                score += 2000;

            // Killer moves
            if (depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y)) score += 1000;
                if (threadData.KillerMoves[depth, 1] == (x, y)) score += 900;
            }

            // History heuristic
            score += Math.Min(historyTable[x, y], 500);

            // Center preference
            int centerDist = Math.Abs(x - 7) + Math.Abs(y - 7);
            score += (14 - centerDist) * 10;

            // Proximity to existing stones
            score += GetProximityScore(x, y, board);

            scores[i] = score;
        }

        // Simple insertion sort (zero allocation)
        return InsertionSort(candidates, scores);
    }

    /// <summary>
    /// Insertion sort with zero allocations
    /// </summary>
    private List<(int x, int y)> InsertionSort(List<(int x, int y)> moves, Span<int> scores)
    {
        for (int i = 1; i < moves.Count; i++)
        {
            int j = i;
            while (j > 0 && scores[j] > scores[j - 1])
            {
                // Swap moves
                var tempMove = moves[j];
                moves[j] = moves[j - 1];
                moves[j - 1] = tempMove;

                // Swap scores
                int tempScore = scores[j];
                scores[j] = scores[j - 1];
                scores[j - 1] = tempScore;

                j--;
            }
        }
        return moves;
    }

    /// <summary>
    /// Get candidate moves near existing stones
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        var candidates = new List<(int x, int y)>(64);
        var considered = new bool[15, 15];

        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        var occupied = playerBitBoard | opponentBitBoard;

        // Find all cells within SearchRadius of existing stones
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (occupied.GetBit(x, y))
                {
                    // Add neighbors as candidates
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            int nx = x + dx;
                            int ny = y + dy;
                            if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15 &&
                                !occupied.GetBit(nx, ny) && !considered[nx, ny])
                            {
                                candidates.Add((nx, ny));
                                considered[nx, ny] = true;
                            }
                        }
                    }
                }
            }
        }

        return candidates;
    }

    /// <summary>
    /// Calculate proximity score (prefer moves near existing stones)
    /// </summary>
    private int GetProximityScore(int x, int y, Board board)
    {
        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        int score = 0;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                int nx = x + dx;
                int ny = y + dy;
                if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
                {
                    if (playerBitBoard.GetBit(nx, ny)) score += 3;
                    if (opponentBitBoard.GetBit(nx, ny)) score += 2;
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Record killer move
    /// </summary>
    private void RecordKillerMove(ThreadData threadData, int depth, int x, int y)
    {
        if (depth >= 0 && depth < 20)
        {
            // Shift existing killers
            threadData.KillerMoves[depth, 1] = threadData.KillerMoves[depth, 0];
            threadData.KillerMoves[depth, 0] = (x, y);
        }
    }

    /// <summary>
    /// Record history move
    /// </summary>
    private void RecordHistoryMove(ThreadData threadData, Player player, int x, int y, int depth)
    {
        var table = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        table[x, y] += depth * depth;
    }

    /// <summary>
    /// Check for winner
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner ? result.Winner : null;
    }

    /// <summary>
    /// Evaluate board position
    /// </summary>
    private int Evaluate(Board board, Player player)
    {
        return _evaluator.EvaluateOptimized(board, player, AIDifficulty.Grandmaster);
    }

    /// <summary>
    /// Calculate search depth based on time allocation
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, int candidateCount)
    {
        // Emergency mode - reduce depth significantly
        if (timeAlloc.IsEmergency)
        {
            return Math.Max(1, baseDepth - 3);
        }

        // Adjust based on time available
        var softBoundSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Very tight time (< 2s)
        if (softBoundSeconds < 2)
        {
            return Math.Max(1, baseDepth - 2);
        }

        // Tight time (< 5s)
        if (softBoundSeconds < 5)
        {
            return Math.Max(2, baseDepth - 1);
        }

        // Moderate time (< 10s)
        if (softBoundSeconds < 10)
        {
            if (candidateCount > 20) // Complex position
            {
                return Math.Max(2, baseDepth - 1);
            }
            return baseDepth;
        }

        // Good time availability: use full depth
        return baseDepth;
    }

    /// <summary>
    /// Calculate VCF time limit based on time allocation
    /// </summary>
    private int CalculateVCFTimeLimit(TimeAllocation timeAlloc)
    {
        // Emergency mode - very quick VCF check
        if (timeAlloc.IsEmergency)
        {
            return 50;
        }

        // Use a fraction of the soft bound for VCF
        var vcfTime = Math.Max(50, timeAlloc.SoftBoundMs / 10);

        // Cap at reasonable values
        return (int)Math.Min(vcfTime, 500);
    }

    /// <summary>
    /// Get default time allocation when no time limit is specified
    /// </summary>
    private static TimeAllocation GetDefaultTimeAllocation(AIDifficulty difficulty, long? timeRemainingMs)
    {
        // If time remaining is provided but no TimeAllocation, create a simple one
        if (timeRemainingMs.HasValue)
        {
            var timeLeft = timeRemainingMs.Value;
            long softBound = Math.Max(500, timeLeft / 40); // Distribute over 40 moves
            long hardBound = Math.Min(softBound * 2, timeLeft - 1000);

            return new TimeAllocation
            {
                SoftBoundMs = softBound,
                HardBoundMs = hardBound,
                OptimalTimeMs = softBound * 8 / 10,
                IsEmergency = timeLeft < 10000,
                Phase = GamePhase.EarlyMid,
                ComplexityMultiplier = 1.0
            };
        }

        // No time info - use difficulty defaults
        return difficulty switch
        {
            AIDifficulty.Beginner => new() { SoftBoundMs = 100, HardBoundMs = 500, OptimalTimeMs = 80, IsEmergency = false },
            AIDifficulty.Easy => new() { SoftBoundMs = 200, HardBoundMs = 1000, OptimalTimeMs = 160, IsEmergency = false },
            AIDifficulty.Normal => new() { SoftBoundMs = 500, HardBoundMs = 2000, OptimalTimeMs = 400, IsEmergency = false },
            AIDifficulty.Medium => new() { SoftBoundMs = 1000, HardBoundMs = 3000, OptimalTimeMs = 800, IsEmergency = false },
            AIDifficulty.Hard => new() { SoftBoundMs = 2000, HardBoundMs = 5000, OptimalTimeMs = 1600, IsEmergency = false },
            AIDifficulty.Harder => new() { SoftBoundMs = 3000, HardBoundMs = 8000, OptimalTimeMs = 2400, IsEmergency = false },
            AIDifficulty.VeryHard => new() { SoftBoundMs = 5000, HardBoundMs = 15000, OptimalTimeMs = 4000, IsEmergency = false },
            AIDifficulty.Expert => new() { SoftBoundMs = 8000, HardBoundMs = 20000, OptimalTimeMs = 6400, IsEmergency = false },
            AIDifficulty.Master => new() { SoftBoundMs = 10000, HardBoundMs = 30000, OptimalTimeMs = 8000, IsEmergency = false },
            AIDifficulty.Grandmaster => new() { SoftBoundMs = 12000, HardBoundMs = 40000, OptimalTimeMs = 9600, IsEmergency = false },
            AIDifficulty.Legend => new() { SoftBoundMs = 15000, HardBoundMs = 60000, OptimalTimeMs = 12000, IsEmergency = false },
            _ => TimeAllocation.Default
        };
    }

    /// <summary>
    /// Clear transposition table
    /// </summary>
    public void Clear()
    {
        _transpositionTable.Clear();
    }

    /// <summary>
    /// Get statistics
    /// </summary>
    public (int used, double usagePercent, int hitCount, int lookupCount, double hitRate) GetStats()
    {
        return _transpositionTable.GetStats();
    }

    /// <summary>
    /// Find opponent's immediate threat moves that must be blocked
    /// Includes 4-in-row completions and other forced win patterns
    /// </summary>
    private List<(int x, int y)> GetOpponentThreatMoves(Board board, Player opponent)
    {
        var threats = new List<(int x, int y)>();

        // Check each empty cell to see if it completes a winning line for opponent
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                // Temporarily place opponent's stone
                board.PlaceStone(x, y, opponent);
                bool isWinningMove = _winDetector.CheckWin(board).HasWinner;
                board.GetCell(x, y).Player = Player.None;

                if (isWinningMove)
                {
                    threats.Add((x, y));
                }
            }
        }

        return threats;
    }

    #region Pondering Support

    /// <summary>
    /// Pondering variant of Lazy SMP - uses reduced thread count to avoid starving main search
    /// Searches with predicted opponent move already made on the board
    /// Results are stored in the shared transposition table for main search benefit
    /// </summary>
    /// <param name="board">Board with predicted opponent move already made</param>
    /// <param name="player">Player to move (us, after opponent's predicted move)</param>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="maxPonderTimeMs">Maximum time to spend pondering</param>
    /// <param name="cancellationToken">Token to cancel pondering</param>
    /// <param name="progressCallback">Optional callback for progress updates</param>
    /// <returns>Best move found, depth reached, score, and nodes searched</returns>
    public ((int x, int y)? bestMove, int depth, int score, long nodesSearched) PonderLazySMP(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long maxPonderTimeMs,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback = null)
    {
        if (player == Player.None)
            return (null, 0, 0, 0);

        var targetDepth = (int)difficulty;
        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
            return (null, 0, 0, 0);

        // Use reduced thread count for pondering (half of max, minimum 1)
        // This prevents pondering from starving the main search when it starts
        int ponderThreadCount = Math.Max(1, _maxThreads / 2);

        // Cap thread count based on depth to avoid overhead
        ponderThreadCount = Math.Min(ponderThreadCount, Math.Max(2, targetDepth / 2));

        _transpositionTable.IncrementAge();

        // Set up time management for pondering
        _searchShouldStop = false;
        _searchStopwatch = Stopwatch.StartNew();
        _hardTimeBoundMs = maxPonderTimeMs;

        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes)>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[ponderThreadCount];
        var candidatesArray = new List<(int x, int y)>[ponderThreadCount];
        for (int i = 0; i < ponderThreadCount; i++)
        {
            boardsArray[i] = board.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Create pondering-specific time allocation
        var ponderTimeAlloc = new TimeAllocation
        {
            SoftBoundMs = maxPonderTimeMs * 3 / 4,  // 75% for soft bound
            HardBoundMs = maxPonderTimeMs,
            OptimalTimeMs = maxPonderTimeMs / 2,
            IsEmergency = false,
            Phase = GamePhase.EarlyMid,
            ComplexityMultiplier = 1.0
        };

        // Launch parallel pondering searches
        var tasks = new List<Task>();
        for (int i = 0; i < ponderThreadCount; i++)
        {
            int threadId = i;
            var task = Task.Run(() =>
            {
                var threadData = new ThreadData { Random = new Random(threadId) };

                // Variation: slightly different aspiration window per thread
                int aspirationOffset = threadId * 25 - (ponderThreadCount * 12);

                var result = SearchPonderIteration(
                    boardsArray[threadId],
                    player,
                    targetDepth,
                    candidatesArray[threadId],
                    threadData,
                    aspirationOffset,
                    ponderTimeAlloc,
                    cancellationToken,
                    progressCallback);

                results.Add(result);
            }, cancellationToken);
            tasks.Add(task);
        }

        // Wait for all threads or cancellation
        try
        {
            Task.WaitAll(tasks.ToArray());
        }
        catch (AggregateException)
        {
            // Cancellation occurred - this is expected during pondering
        }

        _searchStopwatch.Stop();
        _searchShouldStop = true;

        // Find best result (prefer deeper results if scores are similar)
        var bestResult = results.OrderByDescending(r => r.score)
                                .ThenByDescending(r => r.depth)
                                .FirstOrDefault();

        if (bestResult.depth == 0)
            return (null, 0, 0, 0);

        return ((bestResult.x, bestResult.y), bestResult.depth, bestResult.score, bestResult.nodes);
    }

    /// <summary>
    /// Iterative deepening search for pondering thread with cancellation support
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchPonderIteration(
        Board board,
        Player player,
        int targetDepth,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        int aspirationOffset,
        TimeAllocation timeAlloc,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        long nodesSearched = 0;

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check cancellation
            if (cancellationToken.IsCancellationRequested || _searchShouldStop)
                break;

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Check hard bound
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchShouldStop = true;
                break;
            }

            // Check soft bound
            if (elapsed >= timeAlloc.SoftBoundMs)
                break;

            int alpha = int.MinValue + aspirationOffset;
            int beta = int.MaxValue + aspirationOffset;

            // Aspiration window based on previous score
            if (currentDepth > 2)
            {
                alpha = bestScore - 50 + aspirationOffset;
                beta = bestScore + 50 + aspirationOffset;
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta);
            nodesSearched += CountNodes(currentDepth, candidates.Count);

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            bestDepth = currentDepth;

            // Report progress
            progressCallback?.Invoke((bestMove.x, bestMove.y, bestDepth, bestScore));

            // Early exit on winning move
            if (result.score >= 100000)
                break;
        }

        return (bestMove.x, bestMove.y, bestScore, bestDepth, nodesSearched);
    }

    /// <summary>
    /// Estimate node count for a search at given depth
    /// </summary>
    private long CountNodes(int depth, int branchingFactor)
    {
        // Rough estimation: branching_factor ^ depth
        // This is approximate but sufficient for progress reporting
        long nodes = 1;
        for (int i = 0; i < depth && i < 6; i++) // Cap at depth 6 for estimation
        {
            nodes *= Math.Min(branchingFactor, 30); // Cap branching at 30
        }
        return nodes;
    }

    /// <summary>
    /// Get the shared transposition table (for ponderer access)
    /// </summary>
    public LockFreeTranspositionTable GetTranspositionTable() => _transpositionTable;

    /// <summary>
    /// Stop any ongoing search (used when pondering needs to stop)
    /// </summary>
    public void StopSearch() => _searchShouldStop = true;

    /// <summary>
    /// Check if search is currently running
    /// </summary>
    public bool IsSearching => _searchStopwatch != null && _searchStopwatch.IsRunning;

    #endregion
}
