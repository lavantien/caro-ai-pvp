using System.Runtime.CompilerServices;
using System.Diagnostics;
using System.Threading.Channels;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.Tournament;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning and advanced optimizations
/// Optimizations: Transposition Table, Killer Heuristic, History Heuristic, Improved Move Ordering, Iterative Deepening, VCF Solver
/// Parallel Search: Lazy SMP for D7+ (VeryHard and above) with conservative thread count (processorCount/2)-1
/// Time management: Intelligent time allocation optimized for 7+5 time control
/// Pondering: Constant pondering for D7+ during opponent's turn
/// Stats: Publisher-subscriber pattern for real-time stats reporting
/// </summary>
public class MinimaxAI : IStatsPublisher
{
    private readonly BoardEvaluator _evaluator = new();
    private readonly TranspositionTable _transpositionTable;
    private readonly WinDetector _winDetector = new();
    private readonly ThreatDetector _threatDetector = new();
    private readonly ThreatSpaceSearch _vcfSolver = new();
    private readonly VCFSolver _inTreeVCFSolver;  // In-tree VCF solver for Lazy SMP
    private readonly OpeningBook? _openingBook;

    // Time management for 7+5 time control
    private readonly TimeManager _timeManager = new();

    // Adaptive time management using PID-like controller
    private readonly AdaptiveTimeManager _adaptiveTimeManager = new();

    // Track initial time for adaptive depth thresholds
    // -1 means "unknown, will infer from first move"
    private long _inferredInitialTimeMs = -1;

    // Track thread count used for last search (for diagnostics)
    private int _lastThreadCount = 1;

    // Track parallel diagnostics from last search
    private string? _lastParallelDiagnostics = null;

    // Parallel search for high difficulties (D7+)
    // Lazy SMP provides 4-8x speedup on multi-core systems
    private readonly ParallelMinimaxSearch _parallelSearch;

    // Search radius around existing stones (optimization)
    // Set to 7 to ensure safety checks detect all winning moves (5-in-a-row can have winning cells
    // up to 4 squares from existing stones, plus margin for complex patterns)
    private const int SearchRadius = 7;

    // Board size constant for array sizing and bounds checking
    private const int BoardSize = GameConstants.BoardSize;

    // Killer heuristic: track best moves at each depth
    // No depth cap - array sized for maximum practical depth (32x32 board = 1024 cells)
    private const int MaxKillerMoves = 2;
    private const int MaxKillerDepth = 512;  // Effectively unlimited for practical game play
    private readonly (int x, int y)[,] _killerMoves = new (int x, int y)[MaxKillerDepth, MaxKillerMoves];

    // History heuristic: track moves that cause cutoffs across all depths
    // Two tables: one for Red, one for Blue (each move can be good for different players)
    private readonly int[,] _historyRed = new int[BoardSize, BoardSize];   // History scores for Red moves
    private readonly int[,] _historyBlue = new int[BoardSize, BoardSize];  // History scores for Blue moves

    // Butterfly heuristic: track moves that cause beta cutoffs (complements history)
    private readonly int[,] _butterflyRed = new int[BoardSize, BoardSize];
    private readonly int[,] _butterflyBlue = new int[BoardSize, BoardSize];

    // Track transposition table hits for debugging
    private int _tableHits;
    private int _tableLookups;

    // Track search statistics for last move
    private long _nodesSearched;
    private int _depthAchieved;
    private int _vcfNodesSearched;
    private int _vcfDepthAchieved;
    private readonly Stopwatch _searchStopwatch = new();
    private long _lastAllocatedTimeMs;  // Track time allocated for last move
    private bool _lastPonderingEnabled;  // Track if pondering was enabled for last move
    private bool _bookUsed;  // True if last move came from opening book
    private MoveType _moveType;  // How the last move was determined
    private int _lastSearchScore;  // Score from last search (for book builder)

    // Time control for search timeout
    private long _searchHardBoundMs;
    // Check time more frequently to catch timeout earlier (power of 2 for efficient masking)
    // 4096 = check every ~4K nodes. At 1M nodes/sec, this checks every ~4ms
    // This is much more frequent than the old 100K interval which only checked every ~100ms
    private const int TimeCheckInterval = 16;  // Check time every 16 nodes (was 4096 - too slow for short time controls)
    private bool _searchStopped;

    // Pondering (thinking on opponent's time)
    private readonly Ponderer _ponderer = new();
    private PV _lastPV = PV.Empty;
    private Board? _lastBoard;
    private Player _lastPlayer;
    private AIDifficulty _lastDifficulty;

    // Stats publisher-subscriber pattern
    private static int _instanceCounter = 0;
    private readonly string _publisherId;
    private readonly Channel<MoveStatsEvent> _statsChannel;
    public Channel<MoveStatsEvent> StatsChannel => _statsChannel;
    public string PublisherId => _publisherId;

    // Optional logger for diagnostics
    private readonly ILogger<MinimaxAI> _logger;

    // Random source for tie-breaking and error rate simulation (injectable for deterministic tests)
    private readonly Random? _random;

    // Mutable SearchBoard for high-performance search (make/unmake pattern)
    // Reused across searches to avoid allocations
    private readonly SearchBoard _searchBoard = new();

    public MinimaxAI(int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null, OpeningBook? openingBook = null, Random? random = null)
    {
        _logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger<MinimaxAI>.Instance;
        _openingBook = openingBook;  // Can be null - engine will work without opening book
        _random = random;  // null means use Random.Shared (default behavior)
        _publisherId = Interlocked.Increment(ref _instanceCounter).ToString();
        _statsChannel = Channel.CreateUnbounded<MoveStatsEvent>();

        // Initialize with passed size parameter
        _transpositionTable = new TranspositionTable(ttSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(ttSizeMb);

        _inTreeVCFSolver = new VCFSolver(_vcfSolver);
    }

    // Helper methods for random operations (uses injected Random or Random.Shared)
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private double NextRandomDouble() => _random?.NextDouble() ?? Random.Shared.NextDouble();

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int NextRandomInt(int maxValue) => _random?.Next(maxValue) ?? Random.Shared.Next(maxValue);

    /// <summary>
    /// Get the best move for the AI player
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, bool ponderingEnabled = false, bool parallelSearchEnabled = false)
    {
        return GetBestMove(board, player, difficulty, timeRemainingMs: null, moveNumber: 0, ponderingEnabled: ponderingEnabled, parallelSearchEnabled: parallelSearchEnabled);
    }

    /// <summary>
    /// Get the best move for the AI player with time awareness
    /// Dynamically adjusts search depth based on remaining time
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, long? timeRemainingMs, bool ponderingEnabled = false, bool parallelSearchEnabled = false)
    {
        return GetBestMove(board, player, difficulty, timeRemainingMs, moveNumber: 0, ponderingEnabled: ponderingEnabled, parallelSearchEnabled: parallelSearchEnabled);
    }

    /// <summary>
    /// Get the best move for the AI player with full time awareness
    /// Dynamically adjusts search depth based on remaining time and game phase
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player to move</param>
    /// <param name="difficulty">AI difficulty level (D1-D7)</param>
    /// <param name="timeRemainingMs">Time remaining on clock in milliseconds (null for unlimited)</param>
    /// <param name="moveNumber">Current move number (1-indexed, 0 if unknown)</param>
    /// <param name="ponderingEnabled">Enable pondering (thinking on opponent's time)</param>
    /// <param name="parallelSearchEnabled">Enable Lazy SMP parallel search</param>
    /// <returns>Best move coordinates</returns>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, long? timeRemainingMs, int moveNumber, bool ponderingEnabled = false, bool parallelSearchEnabled = false)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var baseDepth = AdaptiveDepthCalculator.GetDepth(difficulty, board);
        var candidates = GetCandidateMoves(board);

        // Initialize search statistics BEFORE any early returns
        // This ensures stats are clean even for instant moves (error rate, critical defense, VCF, etc.)
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _bookUsed = false;
        _moveType = MoveType.Normal;  // Default, will be overridden by early exits
        _searchStopwatch.Restart();

        // Reset thread count and parallel diagnostics for this difficulty
        _lastThreadCount = ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);
        _lastParallelDiagnostics = null;

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections away from first red stone
        // Rule: |x - firstX| >= 3 OR |y - firstY| >= 3 (outside 5x5 zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            // Find first red stone
            (int firstX, int firstY)? firstRed = null;
            for (int bx = 0; bx < board.BoardSize; bx++)
            {
                for (int by = 0; by < board.BoardSize; by++)
                {
                    if (board.GetCell(bx, by).Player == Player.Red)
                    {
                        firstRed = (bx, by);
                        break;
                    }
                }
                if (firstRed.HasValue)
                    break;
            }

            if (firstRed.HasValue)
            {
                var fx = firstRed.Value.firstX;
                var fy = firstRed.Value.firstY;
                candidates = candidates.Where(c =>
                {
                    int dx = System.Math.Abs(c.x - fx);
                    int dy = System.Math.Abs(c.y - fy);
                    return dx >= 3 || dy >= 3;  // Valid if at least 3 away in one direction
                }).ToList();
            }
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            // Play first available cell that satisfies open rule (if applicable)
            for (int x = 0; x < board.BoardSize; x++)
            {
                for (int y = 0; y < board.BoardSize; y++)
                {
                    if (board.GetCell(x, y).Player == Player.None)
                    {
                        // For move #3, check open rule
                        if (player == Player.Red && moveNumber == 3)
                        {
                            // Find first red stone and check distance
                            (int firstX, int firstY)? firstRed = null;
                            for (int fx = 0; fx < board.BoardSize; fx++)
                            {
                                for (int fy = 0; fy < board.BoardSize; fy++)
                                {
                                    if (board.GetCell(fx, fy).Player == Player.Red)
                                    {
                                        firstRed = (fx, fy);
                                        break;
                                    }
                                }
                                if (firstRed.HasValue)
                                    break;
                            }

                            if (firstRed.HasValue)
                            {
                                int dx = System.Math.Abs(x - firstRed.Value.firstX);
                                int dy = System.Math.Abs(y - firstRed.Value.firstY);
                                if (dx >= 3 || dy >= 3)
                                    return (x, y);
                            }
                            else
                            {
                                // No red stone yet (shouldn't happen on move #3), play anywhere
                                return (x, y);
                            }
                        }
                        else
                        {
                            return (x, y);
                        }
                    }
                }
            }
            // Fallback: play center
            int center = board.BoardSize / 2;
            return (center, center);
        }

        // PONDER HIT HANDLING
        // On ponder hit, the ponder search is already running with the correct position.
        // We should wait for it to complete (up to our time budget) and use the result.
        // On ponder miss, we fall through to normal search.
        if (ponderingEnabled && _ponderer.IsPondering && _lastPV.IsEmpty == false)
        {
            var lastOppMove = GetLastOpponentMove(board, player);
            if (lastOppMove.HasValue)
            {
                // Check if opponent played the predicted move
                var (ponderState, _) = _ponderer.HandleOpponentMove(lastOppMove.Value.x, lastOppMove.Value.y);

                if (ponderState == PonderState.PonderHit)
                {
                    // PONDER HIT - opponent played expected move!
                    // The ponder search was running during opponent's turn (free precomputation).
                    // Use the result immediately - don't wait or count ponder time.
                    var ponderResult = _ponderer.GetPonderHitResult();

                    if (ponderResult.BestMove.HasValue && ponderResult.Depth > 0)
                    {
                        var ponderMove = ponderResult.BestMove.Value;
                        // Validate the ponder move is still valid on current board
                        if (board.GetCell(ponderMove.x, ponderMove.y).IsEmpty)
                        {
                            _depthAchieved = ponderResult.Depth;
                            _nodesSearched = ponderResult.NodesSearched;
                            _lastAllocatedTimeMs = ponderResult.TimeSpentMs;
                            _moveType = MoveType.Normal;
                            Console.WriteLine($"[PONDER HIT] Used ponder result: depth={ponderResult.Depth}, nodes={ponderResult.NodesSearched:N0}, time={ponderResult.TimeSpentMs}ms");
                            return ponderMove;
                        }
                    }
                }
                // Ponder miss - fall through to normal search
                // The ponder search was already stopped by HandleOpponentMove on miss
            }
        }

        // Opening book for Easy, Medium, Hard, Grandmaster, and Experimental difficulties
        // Depth-filtered by difficulty (from AIDifficultyConfig):
        // - Easy: 4 plies, Medium: 6 plies, Hard: 10 plies
        // - Grandmaster: 14 plies, Experimental: unlimited
        // BOOK MOVE VALIDATION: Always validate book moves with a quick search (D3-D5)
        // to prevent book errors from causing strength inversions
        var lastOpponentMove = GetLastOpponentMove(board, player);
        var bookMove = _openingBook?.GetBookMove(board, player, difficulty, lastOpponentMove);
        if (bookMove.HasValue)
        {
            // DEFENSIVE: Verify the book move is actually valid before returning
            if (!board.GetCell(bookMove.Value.x, bookMove.Value.y).IsEmpty)
            {
                // Book returned an invalid move - this should not happen
                // Fall through to normal search instead
                Console.WriteLine($"[AI ERROR] Book returned occupied cell ({bookMove.Value.x},{bookMove.Value.y}) - falling through to search");
            }
            else
            {
                // Validate book move with quick search (Grandmaster+ only for performance)
                // This prevents book errors from causing losses against weaker opponents
                if (difficulty >= AIDifficulty.Hard)
                {
                    var validationResult = ValidateBookMove(board, player, bookMove.Value, difficulty);
                    if (validationResult.IsAcceptable)
                    {
                        // Book move passed validation - use it
                        _depthAchieved = validationResult.ValidationDepth;
                        _nodesSearched = validationResult.NodesSearched;
                        _lastAllocatedTimeMs = validationResult.TimeMs;
                        _bookUsed = true;
                        _moveType = MoveType.BookValidated;
                        return bookMove.Value;
                    }
                    // Book move failed validation - fall through to full search
                    // This prevents bad book moves from causing losses
                }
                else
                {
                    // Lower difficulties use book moves directly without validation
                    _depthAchieved = 0;
                    _nodesSearched = 0;
                    _lastAllocatedTimeMs = 0;
                    _bookUsed = true;
                    _moveType = MoveType.Book;
                    return bookMove.Value;
                }
            }
        }

        // CRITICAL OPTIMIZATION: Check for immediate winning moves BEFORE any expensive operations
        // This ensures we never waste time searching when a win is available in one move
        // Applies to ALL difficulties - winning is winning
        foreach (var (cx, cy) in candidates)
        {
            if (_threatDetector.IsWinningMove(board, cx, cy, player))
            {
                _depthAchieved = 1;
                _nodesSearched = 1;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ImmediateWin;
                return (cx, cy);
            }
        }

        // CRITICAL DEFENSE: Check for opponent's immediate winning moves
        // Must scan full board since blocking square may be far from existing stones
        // This is O(n²) but necessary to prevent instant losses
        var oppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var opponentWinningSquares = new List<(int x, int y)>();

        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (_threatDetector.IsWinningMove(board, x, y, oppPlayer))
                    {
                        opponentWinningSquares.Add((x, y));
                    }
                }
            }
        }

        // If opponent has immediate winning moves, we must block
        if (opponentWinningSquares.Count > 0)
        {
            // Single threat: block it directly
            if (opponentWinningSquares.Count == 1)
            {
                _depthAchieved = 1;
                _nodesSearched = 1;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ImmediateBlock;
                return opponentWinningSquares[0];
            }

            // Multiple threats: find a square that blocks ALL of them
            // Optimization: Only check if OTHER winning squares are still threats
            // No need to scan entire board - if we block one winning square,
            // the only remaining threats would be the other known winning squares
            foreach (var (bx, by) in opponentWinningSquares)
            {
                // Simulate placing our stone at this candidate block position
                var testBoard = board.PlaceStone(bx, by, player);

                // Check if any OTHER winning squares are still threats after this block
                bool stillHasThreat = false;
                foreach (var (wx, wy) in opponentWinningSquares)
                {
                    if (wx == bx && wy == by)
                        continue; // This square is now occupied by our block

                    // Check if this other winning square is still a threat
                    if (_threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                    {
                        stillHasThreat = true;
                        break;
                    }
                }

                if (!stillHasThreat)
                {
                    // This block successfully eliminates ALL opponent threats
                    _depthAchieved = 1;
                    _nodesSearched = opponentWinningSquares.Count;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ImmediateBlock;
                    return (bx, by);
                }
            }

            // No single block can save us - multiple independent threats exist (e.g., open four)
            // Block the first winning square anyway. This is NOT futile because:
            // 1. It delays the opponent's win, giving us time to counter-attack
            // 2. The opponent might not have time to play the other winning square
            // 3. We might create our own threat that forces a response
            // Falling through to normal search risks playing a non-blocking move.
            _depthAchieved = 1;
            _nodesSearched = opponentWinningSquares.Count;
            _lastAllocatedTimeMs = 0;
            _moveType = MoveType.ImmediateBlock;
            return opponentWinningSquares[0];
        }

        // PROACTIVE DEFENSE: Check for opponent's open threes (3 in a row with both ends open)
        // An open three becomes an open four on the next move, which has 2 winning squares.
        // We should block open threes BEFORE they become open fours.
        // This is critical for Caro rules where sandwiched wins are blocked.
        // CHANGED: Don't immediately block - instead add to blocking candidates for search evaluation
        // This allows the AI to consider whether its own threats might be more urgent
        var openThreeBlocks = FindOpenThreeBlocks(board, oppPlayer);

        // If there are open threes but NO immediate winning threats, add them to candidates
        // This ensures the search considers blocking open threes
        if (openThreeBlocks.Count > 0)
        {
            // Add open three blocks to candidates if not already present
            foreach (var block in openThreeBlocks)
            {
                if (!candidates.Contains(block))
                {
                    candidates.Insert(0, block); // Insert at beginning for high priority
                }
            }
        }

        // Error rate simulation: Lower difficulties make random/suboptimal moves
        // Uses AdaptiveDepthCalculator.GetErrorRate() for consistent error rates
        // - Braindead: 10%, all other difficulties: 0% (optimal play)
        // IMPORTANT: Error moves are TRUE random - selected from ALL legal moves, not tactical moves
        var errorRate = AdaptiveDepthCalculator.GetErrorRate(difficulty);
        var randomValue = NextRandomDouble();
        if (errorRate > 0 && randomValue < errorRate)
        {
            // Get ALL legal moves (every empty cell), not just tactical candidates
            var allLegalMoves = GetAllLegalMoves(board);
            if (allLegalMoves.Count > 0)
            {
                if (System.Diagnostics.Debugger.IsAttached)
                    Console.WriteLine($"[ERROR RATE] {difficulty} playing random move (rolled {randomValue:F2} < {errorRate})");
                // Play a random valid move instead of searching
                // Report minimal stats to indicate instant move (not D0 which looks like a bug)
                _depthAchieved = 1;
                _nodesSearched = 1;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ErrorRate;
                var randomIndex = NextRandomInt(allLegalMoves.Count);
                return allLegalMoves[randomIndex];
            }
        }

        // Calculate time allocation for chess-clock time control
        // Infer initial time and increment from the remaining time
        // This works for any time control: 3+2, 7+5, 15+10, etc.
        TimeAllocation timeAlloc;
        // CRITICAL FIX: For BookGeneration, use direct time allocation without AdaptiveTimeManager
        // The adaptive manager is designed for tournament play and under-allocates for long time budgets
        if (timeRemainingMs.HasValue && difficulty != AIDifficulty.BookGeneration)
        {
            // Infer initial time from first few moves
            var inferredInitialMs = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : timeRemainingMs.Value;
            var initialTimeSeconds = (int)(inferredInitialMs / 1000);

            // Estimate increment based on common time control ratios
            // 3+2: 2/180 = 1.1%, 7+5: 5/420 = 1.2%, 15+10: 10/900 = 1.1%
            var incrementSeconds = Math.Max(2, (int)Math.Round(initialTimeSeconds / 90.0));

            // Use AdaptiveTimeManager with PID-like controller for better time management
            // Automatically adjusts to any time control without hardcoded multipliers
            timeAlloc = _adaptiveTimeManager.CalculateMoveTime(
                timeRemainingMs.Value,
                moveNumber,
                candidates.Count,
                board,
                player,
                difficulty,
                initialTimeSeconds,
                incrementSeconds
            );
        }
        else
        {
            // For BookGeneration with timeRemainingMs, create a direct time allocation
            timeAlloc = (difficulty == AIDifficulty.BookGeneration && timeRemainingMs.HasValue)
                ? GetDefaultTimeAllocation(difficulty) with
                {
                    SoftBoundMs = Math.Max(50, timeRemainingMs.Value - Math.Min(1000, timeRemainingMs.Value / 10)),
                    HardBoundMs = timeRemainingMs.Value,
                    OptimalTimeMs = (long)(timeRemainingMs.Value * 0.8)
                }
                : GetDefaultTimeAllocation(difficulty);
        }

        // CRITICAL DEFENSE: Check for opponent threats BEFORE any early returns
        // This ensures we don't skip blocking in emergency mode
        // Note: oppPlayer is already defined above
        // CRITICAL FIX: Include BrokenFour threats - they create double attacks that are as dangerous as open fours
        var threats = _threatDetector.DetectThreats(board, oppPlayer)
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenFour)
            .ToList();

        bool hasOpponentThreats = threats.Count > 0;
        bool hasOpenFour = false;  // Open four: 4 stones with 2 blocking squares

        List<(int x, int y)> blockingSquares = new();
        List<(int x, int y)> priorityBlockingSquares = new();  // For open fours

        if (hasOpponentThreats)
        {
            var straightFourCount = threats.Count(t => t.Type == ThreatType.StraightFour);
            var straightThreeCount = threats.Count(t => t.Type == ThreatType.StraightThree);
            var brokenFourCount = threats.Count(t => t.Type == ThreatType.BrokenFour);

            blockingSquares = threats
                .SelectMany(t => t.GainSquares)
                .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                .ToList();

            // Check for open four (StraightFour with exactly 2 blocking squares)
            // This is a critical threat that requires special handling
            foreach (var threat in threats.Where(t => t.Type == ThreatType.StraightFour))
            {
                if (threat.GainSquares.Count >= 2)
                {
                    hasOpenFour = true;
                    // For open fours, prioritize blocking squares that also prevent other threats
                    foreach (var square in threat.GainSquares)
                    {
                        if (board.GetCell(square.x, square.y).IsEmpty)
                            priorityBlockingSquares.Add(square);
                    }
                }
            }

            // CRITICAL FIX: BrokenFour also indicates critical threat (double attack potential)
            if (brokenFourCount > 0)
            {
                hasOpenFour = true;  // Treat as critically as open four
            }

            _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) Opponent has {StraightFourCount} StraightFour, {StraightThreeCount} StraightThree, {BrokenFourCount} BrokenFour threat(s), blocking squares: {BlockingSquares}{OpenFourSuffix}",
                difficulty, player, straightFourCount, straightThreeCount, brokenFourCount,
                string.Join(", ", blockingSquares.Select(g => $"({g.x},{g.y})")),
                hasOpenFour ? " [CRITICAL THREAT DETECTED]" : "");
        }

        // Emergency mode - use TT move at D3+ (Medium+) if available
        // BUT: If opponent has threats, blocking takes priority
        if (timeAlloc.IsEmergency && difficulty >= AIDifficulty.Medium && !hasOpponentThreats)
        {
            var ttMove = GetTranspositionTableMove(board, player, minDepth: 5);
            if (ttMove.HasValue)
            {
                //Console.WriteLine("[AI] Emergency mode: Using TT move at D5+");
                _depthAchieved = 5;
                _nodesSearched = 1;
                return ttMove.Value;
            }
        }

        // CRITICAL DEFENSE: Filter candidates to only blocking moves when opponent has threats
        if (hasOpponentThreats)
        {
            // CRITICAL FIX: For open fours, reserve minimum time to respond properly
            // An open four is a game-ending threat that requires proper calculation
            if (hasOpenFour)
            {
                const long minCriticalResponseTimeMs = 3000;  // Minimum 3 seconds for critical responses

                if (timeAlloc.SoftBoundMs < minCriticalResponseTimeMs)
                {
                    _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) CRITICAL: Open four detected - reserving minimum time ({MinCriticalResponseTimeMs}ms)",
                        difficulty, player, minCriticalResponseTimeMs);
                    timeAlloc = new TimeAllocation
                    {
                        SoftBoundMs = Math.Max(minCriticalResponseTimeMs, timeAlloc.SoftBoundMs),
                        HardBoundMs = Math.Max(minCriticalResponseTimeMs * 13 / 10, timeAlloc.HardBoundMs),
                        OptimalTimeMs = Math.Max(minCriticalResponseTimeMs * 8 / 10, timeAlloc.OptimalTimeMs),
                        IsEmergency = false,
                        Phase = timeAlloc.Phase,
                        ComplexityMultiplier = timeAlloc.ComplexityMultiplier
                    };
                }
            }

            // For open fours with 2+ blocking squares, use ALL blocking squares as candidates
            // The search will choose the best one based on evaluation
            var blockingSet = new HashSet<(int x, int y)>(blockingSquares);
            var filteredCandidates = candidates.Where(c => blockingSet.Contains(c)).ToList();

            if (filteredCandidates.Count > 0)
            {
                candidates = filteredCandidates;
                _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) Filtered to {CandidateCount} blocking move(s)",
                    difficulty, player, candidates.Count);
            }
            else
            {
                // Fallback: use the blocking squares directly as candidates
                candidates = blockingSquares;
                _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) Using blocking squares directly as candidates",
                    difficulty, player);
            }

            // CRITICAL FIX: For open fours (StraightFour with 2+ blocking squares),
            // we're in a lost position if we can't win immediately. Log this for debugging.
            if (hasOpenFour)
            {
                _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) WARNING: Open four detected - opponent can win in 2 moves",
                    difficulty, player);

                // Check if we have counter-threats
                var ourThreats = _threatDetector.DetectThreats(board, player);
                var ourStraightFours = ourThreats.Count(t => t.Type == ThreatType.StraightFour);
                var ourStraightThrees = ourThreats.Count(t => t.Type == ThreatType.StraightThree);

                if (ourStraightFours > 0)
                {
                    _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) We have {OurStraightFours} StraightFour threat(s) - counter-attack instead of just blocking",
                        difficulty, player, ourStraightFours);
                }
                else if (ourStraightThrees > 1)
                {
                    _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) We have {OurStraightThrees} StraightThree threat(s) - creating counter-play",
                        difficulty, player, ourStraightThrees);
                }
            }
        }
        // VCF Defense was causing Grandmaster+ to play too reactively, blocking opponent threats
        // instead of developing its own position. The evaluation function's defense
        // multiplier (2.2x for opponent threats) should be sufficient for defense.
        // Grandmaster's advantage comes from offensive VCF, not defensive VCF detection.
        // if (difficulty >= AIDifficulty.Grandmaster)  // Only D5
        // {
        //     var vcfDefense = FindVCFDefense(board, player, timeAlloc, difficulty);
        //     if (vcfDefense.HasValue)
        //     {
        //         var (defenseX, defenseY) = vcfDefense.Value;
        //         Console.WriteLine($"[AI VCF] Opponent VCF detected, blocking at ({defenseX}, {defenseY})");
        //         return (defenseX, defenseY);
        //     }
        // }

        // Try VCF (Victory by Continuous Four) search - ONLY Grandmaster has this!
        // VCF finds forced win sequences through continuous four threats.
        // By restricting VCF to only Grandmaster, we ensure it's a unique differentiator.
        // Use centralized config to check VCF support for this difficulty.
        // CRITICAL FIX: Skip VCF for BookGeneration - full search is sufficient and VCF consumes time budget
        var settings = AIDifficultyConfig.Instance.GetSettings(difficulty);
        if (settings.VCFEnabled && difficulty != AIDifficulty.BookGeneration)
        {
            var (vcfTimeLimit, vcfMaxDepth) = CalculateVCFTimeLimit(timeAlloc, difficulty);

            // VCF-FIRST MODE: In emergency, use up to 80% of hard bound for VCF
            // CRITICAL: Even in emergency, VCF time scales with difficulty!
            // This prevents emergency mode from making all AIs equal
            if (timeAlloc.IsEmergency)
            {
                var emergencyVcfCap = GetEmergencyVCFCap(difficulty);
                vcfTimeLimit = (int)Math.Min(timeAlloc.HardBoundMs * 0.8, emergencyVcfCap);
            }

            var vcfResult = _vcfSolver.SolveVCF(board, player, timeLimitMs: vcfTimeLimit, maxDepth: vcfMaxDepth);

            // Capture VCF statistics even if not a winning sequence
            _vcfDepthAchieved = vcfResult.DepthAchieved;
            _vcfNodesSearched = vcfResult.NodesSearched;

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                // VCF found a forced win sequence - use it immediately
                return vcfResult.BestMove.Value;
            }

            // VCF-FIRST MODE: In emergency mode, if VCF didn't find a win, check opponent threats
            // CRITICAL: Don't skip blocking even in emergency mode
            if (timeAlloc.IsEmergency)
            {
                // If opponent has threats, MUST block - don't use TT fallback
                if (hasOpponentThreats && candidates.Count > 0)
                {
                    // Candidates are already filtered to blocking squares from earlier threat detection
                    _depthAchieved = 1;
                    _nodesSearched = 1;
                    return candidates[0];
                }

                // No opponent threats - safe to use TT move
                var ttMove = GetTranspositionTableMove(board, player, minDepth: 3);
                if (ttMove.HasValue)
                {
                    _depthAchieved = 3;
                    _nodesSearched = 1;
                    return ttMove.Value;
                }

                // Last resort: return the first candidate (usually the center or near existing stones)
                _depthAchieved = 1;
                _nodesSearched = 1;
                return candidates[0];
            }
        }

        // Get time multiplier for this difficulty (applies to both parallel and sequential search)
        // From AIDifficultyConfig: Braindead: 5%, Easy: 20%, Medium: 50%, Hard: 75%, Grandmaster: 100%
        double timeMultiplier = AdaptiveDepthCalculator.GetTimeMultiplier(difficulty);

        // NPS is learned from actual search performance - no hardcoded targets

        // PARALLEL SEARCH: Use Lazy SMP when enabled
        // TournamentEngine already checks the config, so we just respect the flag here
        // Thread counts are fetched from config via ThreadPoolConfig
        if (parallelSearchEnabled)
        {
            int threadCount = ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);
            _lastThreadCount = threadCount;
            _tableHits = 0;
            _tableLookups = 0;
            //Console.WriteLine($"[AI] Using parallel search (Lazy SMP) for {difficulty} with {threadCount} threads");

            // CRITICAL: Apply time multiplier to time allocation for parallel search
            // Lower difficulties should use proportionally less time
            // BUT: Ensure minimum time for at least one search iteration (50ms)
            // Without this, Easy with 20% multiplier can end up with 1ms which is too tight
            const long minSearchTimeMs = 50;
            var adjustedTimeAlloc = new TimeAllocation
            {
                SoftBoundMs = Math.Max(minSearchTimeMs, (long)(timeAlloc.SoftBoundMs * timeMultiplier)),
                HardBoundMs = Math.Max(minSearchTimeMs, (long)(timeAlloc.HardBoundMs * timeMultiplier)),
                OptimalTimeMs = Math.Max(minSearchTimeMs / 2, (long)(timeAlloc.OptimalTimeMs * timeMultiplier)),
                IsEmergency = timeAlloc.IsEmergency,
                Phase = timeAlloc.Phase
            };

            var parallelResult = _parallelSearch.GetBestMoveWithStats(
                board,
                player,
                difficulty,
                timeRemainingMs: timeRemainingMs,
                timeAlloc: adjustedTimeAlloc,
                moveNumber: moveNumber,
                fixedThreadCount: threadCount,
                candidates: candidates);

            // DEFENSIVE: Validate the returned move is actually a valid, empty cell
            // NOTE: Move validation against candidates is already done in SearchLazySMP
            // GetBestMoveWithStats may filter candidates for blocking moves, so checking
            // against the original candidates list here would be a false positive
            var cell = board.GetCell(parallelResult.X, parallelResult.Y);
            if (!cell.IsEmpty)
            {
                Console.WriteLine($"[AI ERROR] Parallel search returned occupied cell ({parallelResult.X},{parallelResult.Y}) at move {moveNumber} - cell player: {cell.Player}");
                // Fall back to first empty candidate
                var fallbackMove = candidates.FirstOrDefault(c => board.GetCell(c.x, c.y).IsEmpty, candidates[0]);
                parallelResult = new ParallelSearchResult(fallbackMove.x, fallbackMove.y, 1, 1, 0, null, parallelResult.AllocatedTimeMs, 0, 0);
            }

            // Update statistics from parallel search
            _depthAchieved = parallelResult.DepthAchieved;
            _nodesSearched = parallelResult.NodesSearched;
            _lastParallelDiagnostics = parallelResult.ParallelDiagnostics;
            _lastAllocatedTimeMs = parallelResult.AllocatedTimeMs;
            _lastPonderingEnabled = ponderingEnabled;
            _tableHits = parallelResult.TableHits;
            _tableLookups = parallelResult.TableLookups;
            _lastSearchScore = parallelResult.Score;

            // Store PV and board for pondering prediction
            _lastPV = PV.FromSingleMove(parallelResult.X, parallelResult.Y, _depthAchieved, 0);
            _lastBoard = board;

            // Start pondering for opponent's response
            if (ponderingEnabled)
            {
                var opponent = player == Player.Red ? Player.Blue : Player.Red;
                var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
                var ponderTimeMs = CalculatePonderTime(timeRemainingMs, difficulty);

                if (ponderTimeMs > 0)
                {
                    _ponderer.StartPondering(
                        board,
                        opponent,
                        predictedOpponentMove,
                        player,
                        difficulty,
                        ponderTimeMs
                    );
                }
            }

            //Console.WriteLine($"[AI PARALLEL] Move: ({parallelResult.X}, {parallelResult.Y}), Depth: {_depthAchieved}, Nodes: {_nodesSearched:N0}, Threads: {parallelResult.ThreadCount}");
            return (parallelResult.X, parallelResult.Y);
        }

        // TIME-BUDGET-BASED SEARCH: No hardcoded depths, scales with machine capability
        // Each difficulty uses a time multiplier: Braindead 1%, Easy 10%, Medium 30%, Hard 70%, Grandmaster 100%
        // Faster machines reach deeper depths naturally, slower machines stop earlier
        // This ensures strength ordering regardless of server performance

        // Track thread count for diagnostics (even if using sequential search)
        _lastThreadCount = ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);
        _lastParallelDiagnostics = null; // No parallel search in this path
        _lastPonderingEnabled = ponderingEnabled;

        // NPS is learned from actual search performance - no hardcoded targets

        // Apply time multiplier to the soft bound - lower difficulties use less time
        // BUT: Ensure minimum time for at least one search iteration (50ms)
        const long minSequentialSearchTimeMs = 50;
        long adjustedSoftBoundMs = Math.Max(minSequentialSearchTimeMs, (long)(timeAlloc.SoftBoundMs * timeMultiplier));

        // CRITICAL FIX: Also apply time multiplier to hard bound!
        // Without this, lower difficulties search until the original hard bound,
        // which defeats the purpose of the time multiplier.
        long adjustedHardBoundMs = Math.Max(adjustedSoftBoundMs, (long)(timeAlloc.HardBoundMs * timeMultiplier));

        (int x, int y) bestMove;
        int depthAchieved;
        long nodesSearched;

        // Initialize transposition table for this search
        _transpositionTable.IncrementAge();
        _tableHits = 0;
        _tableLookups = 0;

        // Initialize search statistics
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _searchStopwatch.Restart();

        // Initialize time control for search timeout
        _searchHardBoundMs = adjustedHardBoundMs;
        _lastAllocatedTimeMs = adjustedHardBoundMs;
        _searchStopped = false;

        // ITERATIVE DEEPENING: Search depth 1, 2, 3... until time runs out
        // PURE TIME-BASED: No depth target - different machines reach different depths naturally
        // Always return best move from deepest completed iteration
        bestMove = candidates[0];
        int currentDepth = 1; // Start from depth 1

        while (true)  // Time-based only - depth is incidental
        {
            // Depth cap for BookGeneration to prevent indefinite search
            if (difficulty == AIDifficulty.BookGeneration && currentDepth > 6)
            {
                break;
            }

            // Check time bounds using TimeAllocation
            var elapsed = _searchStopwatch.ElapsedMilliseconds;

            // Hard bound check - must stop
            if (elapsed >= _searchHardBoundMs)
            {
                break;
            }

            // Soft bound check with time multiplier applied
            // Lower difficulties hit soft bound earlier due to multiplier
            if (elapsed >= adjustedSoftBoundMs)
            {
                // Check if we should continue for one more iteration
                // Only continue if we have significant time left and next iteration won't exceed hard bound
                double remainingSeconds = (_searchHardBoundMs - elapsed) / 1000.0;
                double estimatedNextTime = elapsed / 1000.0 * 2.5; // EBF ~2.5
                if (remainingSeconds < estimatedNextTime * 0.8)
                {
                    break;
                }
            }

            // Reset stopped flag for this depth
            _searchStopped = false;

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
                _lastSearchScore = result.score;  // Track score for book builder
                _depthAchieved = currentDepth; // Track deepest completed search
            }

            // If search was stopped due to timeout, don't continue to next depth
            if (_searchStopped)
            {
                break;
            }

            currentDepth++;
        }

        _searchStopwatch.Stop();
        depthAchieved = _depthAchieved;
        nodesSearched = _nodesSearched;

        // Report time used to adaptive time manager for feedback loop
        if (timeRemainingMs.HasValue)
        {
            var actualTimeMs = _searchStopwatch.ElapsedMilliseconds;
            bool timedOut = actualTimeMs >= timeAlloc.HardBoundMs;
            _adaptiveTimeManager.ReportTimeUsed(actualTimeMs, timeAlloc.SoftBoundMs, timedOut);
        }

        // Print transposition table statistics for debugging
        if (difficulty == AIDifficulty.Hard || difficulty == AIDifficulty.Grandmaster)
        {
            double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
            var (used, usage) = _transpositionTable.GetStats();
            var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
            var nps = elapsedMs > 0 ? nodesSearched * 1000 / elapsedMs : 0;
        }

        // Store PV for pondering
        _lastPV = PV.FromSingleMove(bestMove.x, bestMove.y, baseDepth, 0);
        _lastBoard = board;
        _lastPlayer = player;
        _lastDifficulty = difficulty;

        // Start pondering for opponent's response
        if (ponderingEnabled)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
            var ponderTimeMs = CalculatePonderTime(timeRemainingMs, difficulty);

            if (ponderTimeMs > 0)
            {
                _ponderer.StartPondering(
                    board,
                    opponent,
                    predictedOpponentMove,
                    player,  // Pondering for us (next to move after opponent)
                    difficulty,
                    ponderTimeMs
                );
            }
        }

        // Publish search stats to channel
        PublishSearchStats(player, StatsType.MainSearch, _searchStopwatch.ElapsedMilliseconds);

        return bestMove;
    }

    /// <summary>
    /// Calculate appropriate search depth based on time allocation.
    ///
    /// PURE TIME-BASED DEPTH:
    /// - Search runs until time expires via iterative deepening
    /// - NO NPS estimation (unreliable across different machines)
    /// - NO artificial depth floors or reductions
    /// - Higher difficulties get more time allocation, naturally reaching deeper
    /// - Different machines reach different depths based on hardware capability
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, long? timeRemainingMs, int candidateCount)
    {
        // Infer initial time from the move number and remaining time (for emergency detection)
        if (timeRemainingMs.HasValue && timeAlloc.Phase == GamePhase.Opening)
        {
            if (_inferredInitialTimeMs < 0)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
            else if (Math.Abs(timeRemainingMs.Value - _inferredInitialTimeMs) > _inferredInitialTimeMs * 0.3)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
        }

        // Emergency mode: minimum depth to avoid timeout
        if (timeAlloc.IsEmergency)
        {
            return 1;
        }

        // Return high max depth and let iterative deepening stop when time runs out
        // The search naturally completes as many depths as possible within the time budget
        // Different machines will reach different depths - this is expected and correct
        return 64;
    }

    /// <summary>
    /// Calculate appropriate time limit for VCF search based on time allocation and difficulty
    ///
    /// CRITICAL: VCF must scale with difficulty for proper AI strength ordering!
    /// Higher difficulties should have:
    /// - More time for VCF search (find deeper tactical sequences)
    /// - Higher maxDepth (look further ahead for forcing moves)
    /// - Better defensive VCF (find moves that break opponent's VCF)
    ///
    /// Without proper scaling, VCF becomes an "equalizer" where lower difficulties
    /// can beat higher ones through tactical brilliance alone.
    /// </summary>
    private (int timeLimitMs, int maxDepth) CalculateVCFTimeLimit(TimeAllocation timeAlloc, AIDifficulty difficulty)
    {
        // Emergency mode - very quick VCF check for all difficulties
        if (timeAlloc.IsEmergency)
        {
            return (50, 15); // Very quick check, shallow depth
        }

        // Base VCF time from soft bound
        var baseVcfTime = Math.Max(50, timeAlloc.SoftBoundMs / 10);

        // Difficulty-based multipliers for VCF time and depth
        // Higher difficulties spend MORE time on VCF because it's their primary tactical weapon
        var (timeMultiplier, depthBonus) = difficulty switch
        {
            AIDifficulty.Grandmaster => (2.5, 10),   // D5: 2.5x time, depth 40
            AIDifficulty.Hard => (1.5, 5),            // D4: 1.5x time, depth 35
            AIDifficulty.Medium => (1.0, 0),          // D3: 1x time, depth 30
            _ => (0.5, 0)                             // D2 and below: 0.5x time
        };

        var vcfTime = (int)(baseVcfTime * timeMultiplier);
        var maxDepth = 30 + depthBonus;

        // Scale caps based on difficulty
        var maxCap = difficulty switch
        {
            AIDifficulty.Grandmaster => 2000,  // D5: up to 2 seconds for VCF
            AIDifficulty.Hard => 1000,         // D4: up to 1 second
            _ => 500                           // D3 and below: up to 500ms
        };

        var finalVcfTime = (int)Math.Clamp(vcfTime, 50, maxCap);

        return (finalVcfTime, maxDepth);
    }

    // REMOVED: GetCriticalDefenseLevel - no longer used, threats handled by evaluation function

    /// <summary>
    /// <summary>
    /// Get emergency VCF time cap based on difficulty
    /// CRITICAL: In time scramble, use the available increment time for VCF
    /// Higher difficulties get more VCF time to find tactical solutions
    /// </summary>
    private static int GetEmergencyVCFCap(AIDifficulty difficulty) => difficulty switch
    {
        AIDifficulty.Grandmaster => 2500,  // D5: up to 2.5s (50% of 5s increment)
        AIDifficulty.Hard => 2000,         // D4: up to 2s (40% of 5s increment)
        AIDifficulty.Medium => 1500,       // D3: up to 1.5s (30% of 5s increment)
        _ => 1000                          // D2 and below: up to 1s
    };

    /// <summary>
    /// Calculate pondering time based on remaining time and difficulty
    /// Pondering uses a portion of the opponent's thinking time
    /// </summary>
    private long CalculatePonderTime(long? timeRemainingMs, AIDifficulty difficulty)
    {
        // Proportional time allocation based on difficulty
        var baseTimeMs = timeRemainingMs ?? 5000;

        return difficulty switch
        {
            AIDifficulty.Braindead => baseTimeMs / 20,   // 5% of time
            AIDifficulty.Easy => baseTimeMs / 10,         // 10%
            AIDifficulty.Medium => baseTimeMs / 3,        // 33% (pondering enabled)
            AIDifficulty.Hard => baseTimeMs / 2,          // 50% (pondering enabled)
            AIDifficulty.Grandmaster => baseTimeMs / 2,   // 50% (pondering enabled)
            _ => baseTimeMs / 20                          // Default: minimal
        };
    }

    /// <summary>
    /// Get default time allocation when no time limit is specified
    /// Provides reasonable time targets for each difficulty level
    /// </summary>
    private static TimeAllocation GetDefaultTimeAllocation(AIDifficulty difficulty) => difficulty switch
    {
        AIDifficulty.Braindead => new() { SoftBoundMs = 50, HardBoundMs = 200, OptimalTimeMs = 40, IsEmergency = false },
        AIDifficulty.Easy => new() { SoftBoundMs = 200, HardBoundMs = 1000, OptimalTimeMs = 160, IsEmergency = false },
        AIDifficulty.Medium => new() { SoftBoundMs = 1000, HardBoundMs = 3000, OptimalTimeMs = 800, IsEmergency = false },
        AIDifficulty.Hard => new() { SoftBoundMs = 3000, HardBoundMs = 10000, OptimalTimeMs = 2400, IsEmergency = false },
        AIDifficulty.Grandmaster => new() { SoftBoundMs = 5000, HardBoundMs = 20000, OptimalTimeMs = 4000, IsEmergency = false },
        _ => TimeAllocation.Default
    };

    /// <summary>
    /// Get a move from the transposition table if available at sufficient depth
    /// Used for emergency mode when time is very low
    /// </summary>
    private (int x, int y)? GetTranspositionTableMove(Board board, Player player, int minDepth)
    {
        var boardHash = _transpositionTable.CalculateHash(board);

        // Try to get the best move from TT with a wide search
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(
            boardHash, minDepth, int.MinValue, int.MaxValue);

        if (found && cachedMove.HasValue)
        {
            // Verify the move is valid
            var (x, y) = cachedMove.Value;
            if (x >= 0 && x < BoardSize && y >= 0 && y < BoardSize)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty)
                {
                    _tableHits++;
                    return cachedMove;
                }
            }
        }

        return null;
    }

    /// <summary>
    /// Check if opponent has an immediate winning move that must be blocked
    /// This is a critical defensive check that runs before any search
    /// Returns the blocking position if found, null otherwise
    // REMOVED: FindCriticalDefense - no longer used, threats handled by evaluation function

    /// <summary>
    /// Check if opponent can VCF (Victory by Continuous Four) and find blocking move
    /// This is essential for Grandmaster+ to prevent losing to VCF attacks
    ///
    /// OPTIMIZED: Uses fast threat detection + immediate defensive move selection
    /// - VCF check time scales with difficulty (higher difficulties get more time)
    /// - Quick check: if opponent has no threats, skip VCF check entirely
    /// - Single VCF check (not nested) to detect opponent threats
    /// - Return first valid defensive move without re-checking VCF for each one
    /// - Skip VCF defense in emergency mode (prioritize speed over accuracy)
    /// </summary>
    private (int x, int y)? FindVCFDefense(Board board, Player player, TimeAllocation timeAlloc, AIDifficulty difficulty)
    {
        // Skip VCF defense in emergency mode - we need to move quickly
        // In emergency, the VCF-first mode already handles defensive prioritization
        if (timeAlloc.IsEmergency)
        {
            return null;
        }

        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Quick check: if opponent has very few threats, no need for VCF defense
        // This is a fast check that avoids expensive VCF search in non-tactical positions
        var opponentThreats = _vcfSolver.GetThreatMoves(board, opponent);
        if (opponentThreats.Count < 2)
        {
            // Opponent has less than 2 threat moves - not a VCF danger
            return null;
        }

        // Use scaled VCF time based on difficulty for defensive checking
        // Higher difficulties get more time to find defensive moves
        var (vcfCheckTime, vcfMaxDepth) = CalculateVCFTimeLimit(timeAlloc, difficulty);

        // For defensive VCF, use 50% of the offensive VCF time (we need to be efficient)
        vcfCheckTime = vcfCheckTime / 2;

        var opponentVCFResult = _vcfSolver.SolveVCF(board, opponent, timeLimitMs: vcfCheckTime, maxDepth: vcfMaxDepth);

        // If opponent can VCF, we need to find a defensive move
        if (opponentVCFResult.IsSolved && opponentVCFResult.IsWin)
        {
            // Get defensive moves - these are moves that block opponent's threats
            var defenses = _vcfSolver.GetDefenseMoves(board, opponent, player);

            if (defenses.Count > 0)
            {
                // OPTIMIZATION: Return first valid defensive move without re-checking
                // The old implementation did a nested VCF check for each defense move,
                // which was O(defenses × VCF_time) = 10 × 500ms = 5+ seconds overhead
                // The new approach is O(1) = just validate and return first valid move

                foreach (var defense in defenses)
                {
                    // Validate move is on board and empty
                    if (defense.x >= 0 && defense.x < board.BoardSize &&
                        defense.y >= 0 && defense.y < board.BoardSize &&
                        board.GetCell(defense.x, defense.y).IsEmpty)
                    {
                        return defense;
                    }
                }

                // Fallback: use first defensive move even if not currently empty
                // (shouldn't happen, but handle gracefully)
                var fallback = defenses[0];
                if (fallback.x >= 0 && fallback.x < board.BoardSize &&
                    fallback.y >= 0 && fallback.y < board.BoardSize)
                {
                    return fallback;
                }
            }
        }

        // CRITICAL FIX: Check for opponent immediate win even when VCF returns IsSolved=false
        // The VCF solver returns IsSolved=false when opponent has an immediate one-move win
        // We must detect this and defend against it by scanning all empty squares
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty)
                {
                    var testBoard = board.PlaceStone(x, y, opponent);
                    var winResult = _winDetector.CheckWin(testBoard);

                    if (winResult.HasWinner && winResult.Winner == opponent)
                    {
                        _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) Opponent has immediate win at ({X}, {Y}) - blocking!",
                            difficulty, player, x, y);
                        return (x, y);
                    }
                }
            }
        }

        return null;
    }

    private (int x, int y, int score) SearchWithDepth(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        // Aspiration window: try narrow search first, then wider if needed
        const int aspirationWindow = 50;  // Initial window size
        const int maxAspirationAttempts = 3;  // Max re-searches with wider windows

        var bestScore = int.MinValue;
        var bestMove = candidates[0];
        int bestTiebreaker = 0;  // Track tiebreaker score

        // Calculate board hash for transposition table
        var boardHash = _transpositionTable.CalculateHash(board);

        // Initialize SearchBoard from immutable Board for high-performance search
        _searchBoard.CopyFrom(new SearchBoard(board));

        // First, do a quick search at depth-1 to get an estimate (if depth > 2)
        int estimatedScore = 0;
        if (depth > 2)
        {
            // Quick search with wide window to get estimate
            var searchAlpha = int.MinValue;
            var searchBeta = int.MaxValue;

            // Pre-score candidates for tiebreaking (use position heuristics)
            var candidateScores = ScoreCandidatesForTiebreak(candidates, board, player, depth);

            int idx = 0;
            foreach (var (x, y) in candidates)
            {
                // CRITICAL: Check time before evaluating each move
                if (_searchStopwatch.ElapsedMilliseconds >= _searchHardBoundMs)
                {
                    _searchStopped = true;
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Make move on SearchBoard (in-place, zero allocation)
                var undo = _searchBoard.MakeMove(x, y, player);
                var score = MinimaxCore(_searchBoard, depth - 2, searchAlpha, searchBeta, false, player, depth);
                _searchBoard.UnmakeMove(undo);

                // If search was stopped during Minimax, return current best
                if (_searchStopped)
                {
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Tie-breaking: higher score wins, or equal score with better tiebreaker
                if (score > bestScore || (score == bestScore && candidateScores[idx] > bestTiebreaker))
                {
                    bestScore = score;
                    bestMove = (x, y);
                    bestTiebreaker = candidateScores[idx];
                }

                searchAlpha = Math.Max(searchAlpha, score);
                if (searchBeta <= searchAlpha)
                    break;
                idx++;
            }
            estimatedScore = bestScore;
        }

        // Now search with aspiration window
        var alpha = estimatedScore - aspirationWindow;
        var beta = estimatedScore + aspirationWindow;

        for (int attempt = 0; attempt < maxAspirationAttempts; attempt++)
        {
            // Check transposition table with current window
            _tableLookups++;
            var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
            if (found)
            {
                _tableHits++;
                if (cachedMove.HasValue)
                    return (cachedMove.Value.x, cachedMove.Value.y, cachedScore);
            }

            // Reset best score for this attempt
            bestScore = int.MinValue;
            bestMove = candidates[0];
            bestTiebreaker = 0;

            // Order moves: Hash > Emergency > Threats > Killers > History > Positional
            var orderedMoves = OrderMoves(candidates, depth, board, player, cachedMove);

            // Pre-score ordered moves for tiebreaking
            var orderedTiebreakScores = ScoreCandidatesForTiebreak(orderedMoves, board, player, depth);

            var aspirationFailed = false;
            int orderedIdx = 0;
            foreach (var (x, y) in orderedMoves)
            {
                // CRITICAL: Check time before evaluating each move
                // This catches timeout during long candidate loops
                if (_searchStopwatch.ElapsedMilliseconds >= _searchHardBoundMs)
                {
                    _searchStopped = true;
                    return (bestMove.x, bestMove.y, bestScore);  // Return best move found so far
                }

                // Make move on SearchBoard (in-place, zero allocation)
                var undo = _searchBoard.MakeMove(x, y, player);

                // Evaluate using MinimaxCore
                var score = MinimaxCore(_searchBoard, depth - 1, alpha, beta, false, player, depth);

                // Unmake move (restore board state)
                _searchBoard.UnmakeMove(undo);

                // If search was stopped during Minimax, return current best
                if (_searchStopped)
                {
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Tie-breaking: higher score wins, or equal score with better tiebreaker + small random
                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                    bestTiebreaker = orderedTiebreakScores[orderedIdx];
                }
                else if (score == bestScore)
                {
                    var currentTiebreaker = orderedTiebreakScores[orderedIdx];
                    var randomBonus = NextRandomInt(100);  // Small random factor (0-99)

                    // Prefer better tiebreaker score, or add randomness
                    if (currentTiebreaker + randomBonus > bestTiebreaker)
                    {
                        bestMove = (x, y);
                        bestTiebreaker = currentTiebreaker + randomBonus;
                    }
                }

                alpha = Math.Max(alpha, score);
                if (beta <= alpha)
                {
                    // Beta cutoff - record killer move and history
                    RecordKillerMove(depth, x, y);
                    RecordHistoryMove(player, x, y, depth);
                    break;
                }

                // Check if score exceeds beta (aspiration window too low)
                if (score >= beta)
                {
                    aspirationFailed = true;
                    break;
                }
                orderedIdx++;
            }

            // If aspiration didn't fail, we're done
            if (!aspirationFailed && bestScore > alpha && bestScore < beta)
            {
                // Store result in transposition table
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, estimatedScore - aspirationWindow, estimatedScore + aspirationWindow);
                return (bestMove.x, bestMove.y, bestScore);
            }

            // Aspiration failed - widen window and try again
            alpha = int.MinValue;
            beta = int.MaxValue;

            // On final attempt, just return the best we found
            if (attempt == maxAspirationAttempts - 1)
            {
                // Store result with wide window
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, int.MinValue, int.MaxValue);
                return (bestMove.x, bestMove.y, bestScore);
            }
        }

        return (bestMove.x, bestMove.y, bestScore);
    }

    /// <summary>
    /// Score candidates for tie-breaking when minimax scores are equal.
    /// Uses position heuristics similar to OrderMoves but without full sorting.
    /// Higher score = more desirable move.
    /// </summary>
    private int[] ScoreCandidatesForTiebreak(List<(int x, int y)> candidates, Board board, Player player, int depth)
    {
        int count = candidates.Count;
        var scores = new int[count];
        const int butterflySize = BoardSize;  // Must match array declaration

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // Bounds check - skip invalid coordinates
            if (x < 0 || x >= butterflySize || y < 0 || y >= butterflySize)
            {
                scores[i] = int.MinValue;  // Penalize invalid coordinates heavily
                continue;
            }

            // Killer moves get high priority
            // Bounds check for depth parameter
            if (depth >= 0 && depth < MaxKillerDepth)
            {
                for (int k = 0; k < MaxKillerMoves; k++)
                {
                    if (_killerMoves[depth, k].x == x && _killerMoves[depth, k].y == y)
                    {
                        score += 1000;
                        break;
                    }
                }
            }

            // Butterfly heuristic
            var butterflyScore = player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y];
            score += Math.Min(300, butterflyScore / 100);

            // History heuristic
            var historyScore = GetHistoryScore(player, x, y);
            score += Math.Min(500, historyScore / 10);

            // Tactical pattern scoring
            score += EvaluateTacticalPattern(board, x, y, player);

            // Prefer center (16,16) for 32x32 board
            var distanceToCenter = Math.Abs(x - 16) + Math.Abs(y - 16);
            score += (31 - distanceToCenter) * 10;

            // Prefer moves near existing stones
            var nearby = 0;
            for (int dx = -1; dx <= 1; dx++)
            {
                for (int dy = -1; dy <= 1; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                    {
                        var cell = board.GetCell(nx, ny);
                        if (cell.Player != Player.None)
                            nearby += 5;
                    }
                }
            }
            score += nearby;

            scores[i] = score;
        }

        return scores;
    }

    /// <summary>
    /// Order moves for better alpha-beta pruning
    /// Priority (optimized for Lazy SMP):
    /// 1. Hash Move (TT Move) - UNCONDITIONAL #1 for thread work sharing
    /// 2. Emergency Defense - blocks opponent's immediate threats (Open 4/Double 3)
    /// 3. Winning Threats - creates own threats (Open 4, Double 3)
    /// 4. Killer Moves - caused cutoffs at sibling nodes
    /// 5. History/Butterfly Heuristic - general statistical sorting
    /// 6. Positional Heuristics - center proximity, nearby stones
    /// Zero-allocation implementation using array-based sorting
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? ttMove = null)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        // Score array on stack (zero allocation)
        Span<int> scores = stackalloc int[count];

        // Score each move
        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // PRIORITY #1: Hash Move (TT Move) - UNCONDITIONAL #1 for Lazy SMP
            // This is CRITICAL: In Lazy SMP, TT is the primary communication between threads.
            // Searching the TT move first maximizes work reuse from other threads.
            if (ttMove.HasValue && x == ttMove.Value.x && y == ttMove.Value.y)
            {
                score = 10000;  // Highest priority - above all else
            }
            else
            {
                // PRIORITY #2: Emergency Defense - blocks opponent's immediate winning threats
                // These are moves we MUST play to avoid losing (Open 4, Double 3 blocks)
                if (IsEmergencyDefense(board, x, y, player))
                {
                    score += 5000;
                }

                // PRIORITY #3: Winning Threats (attacking) - creates own forcing moves
                // EvaluateTacticalPattern returns high scores for Open 4, Double 3, etc.
                score += EvaluateTacticalPattern(board, x, y, player);

                // PRIORITY #4: Killer Moves - caused cutoffs at sibling nodes
                // Bounds check for depth parameter
                if (depth >= 0 && depth < MaxKillerDepth)
                {
                    for (int k = 0; k < MaxKillerMoves; k++)
                    {
                        if (_killerMoves[depth, k].x == x && _killerMoves[depth, k].y == y)
                        {
                            score += 1000;
                            break;
                        }
                    }
                }

                // PRIORITY #5: History/Butterfly Heuristic - general statistical sorting
                // Bounds check for butterfly tables
                const int butterflySize = BoardSize;
                var butterflyScore = (x >= 0 && x < butterflySize && y >= 0 && y < butterflySize)
                    ? (player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y])
                    : 0;
                score += Math.Min(300, butterflyScore / 100);

                var historyScore = GetHistoryScore(player, x, y);
                score += Math.Min(500, historyScore / 10);

                // PRIORITY #6: Positional Heuristics - center proximity, nearby stones
                // Prefer center for 32x32 board
                var distanceToCenter = Math.Abs(x - GameConstants.CenterPosition) + Math.Abs(y - GameConstants.CenterPosition);
                score += ((GameConstants.BoardSize - 2) - distanceToCenter) * 10;

                // Prefer moves near existing stones
                var nearby = 0;
                for (int dx = -1; dx <= 1; dx++)
                {
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        if (dx == 0 && dy == 0) continue;
                        var nx = x + dx;
                        var ny = y + dy;
                        if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                        {
                            var cell = board.GetCell(nx, ny);
                            if (cell.Player != Player.None)
                                nearby += 5;
                        }
                    }
                }
                score += nearby;
            }

            scores[i] = score;
        }

        // Simple insertion sort (fast for small arrays, no allocations)
        for (int i = 1; i < count; i++)
        {
            var keyMove = candidates[i];
            var keyScore = scores[i];
            int j = i - 1;

            while (j >= 0 && scores[j] < keyScore)
            {
                candidates[j + 1] = candidates[j];
                scores[j + 1] = scores[j];
                j--;
            }

            candidates[j + 1] = keyMove;
            scores[j + 1] = keyScore;
        }

        return candidates;
    }

    /// <summary>
    /// Evaluate tactical importance of a move by detecting patterns
    /// Returns high scores for winning moves, threats, and blocks
    /// Optimized using BitBoard operations
    /// </summary>
    private int EvaluateTacticalPattern(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var score = 0;

        // Check all 4 directions for patterns
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count consecutive stones in both directions (for player)
            var count = 1;
            var openEnds = 0;

            // Check positive direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Check negative direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score based on pattern
            if (count >= 5)
            {
                score += 10000; // Winning move
            }
            else if (count == 4)
            {
                if (openEnds >= 1)
                    score += 5000; // Open 4 (unstoppable threat)
                else
                    score += 200; // Closed 4
            }
            else if (count == 3)
            {
                if (openEnds == 2)
                    score += 500; // Open 3 (very strong)
                else if (openEnds == 1)
                    score += 100; // Semi-open 3
                else
                    score += 20; // Closed 3
            }
            else if (count == 2)
            {
                if (openEnds == 2)
                    score += 50; // Open 2
            }
        }

        // Check blocking value (how much this blocks opponent)
        foreach (var (dx, dy) in directions)
        {
            var count = 1;
            var openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score blocking value
            if (count >= 4)
            {
                if (openEnds >= 1)
                    score += 4000; // Must block (opponent has winning threat)
            }
            else if (count == 3)
            {
                if (openEnds == 2)
                    score += 300; // Block open 3
                else if (openEnds == 1)
                    score += 80; // Block semi-open 3
            }
        }

        return score;
    }

    /// <summary>
    /// Check if a move is emergency defense (must block immediate threat)
    /// Returns true if this move blocks opponent's open-4 or double-open-3 threats.
    /// This is priority #2 in move ordering (after Hash Move, before general threats).
    /// Zero-allocation, very fast - runs at every node.
    /// </summary>
    private bool IsEmergencyDefense(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var playerBitBoard = board.GetBitBoard(player);
        var occupied = playerBitBoard | opponentBitBoard;

        // Temporarily place stone to check if it blocks threats
        playerBitBoard.SetBit(x, y, true);

        // Check all 4 directions for blocking patterns
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count opponent consecutive stones if we DON'T block
            var count = 1;
            var openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Emergency if blocking open-4 (4 with open end)
            if (count == 4 && openEnds >= 1)
            {
                playerBitBoard.SetBit(x, y, false);  // Undo before returning
                return true;
            }
        }

        playerBitBoard.SetBit(x, y, false);  // Undo
        return false;
    }

    private void RecordKillerMove(int depth, int x, int y)
    {
        // Shift existing killer moves
        for (int i = MaxKillerMoves - 1; i > 0; i--)
        {
            _killerMoves[depth, i] = _killerMoves[depth, i - 1];
        }
        _killerMoves[depth, 0] = (x, y);
    }

    /// <summary>
    /// Record a move that caused a cutoff in the history table
    /// Higher depth = more significant = larger bonus
    /// </summary>
    private void RecordHistoryMove(Player player, int x, int y, int depth)
    {
        // Depth-based bonus: deeper cutoffs are more significant
        var bonus = depth * depth;
        var butterflyBonus = depth * depth * 2; // Butterfly gets higher weight for beta cutoffs

        if (player == Player.Red)
        {
            _historyRed[x, y] += bonus;
            _butterflyRed[x, y] += butterflyBonus;
        }
        else
        {
            _historyBlue[x, y] += bonus;
            _butterflyBlue[x, y] += butterflyBonus;
        }
    }

    /// <summary>
    /// Get the history score for a move
    /// </summary>
    private int GetHistoryScore(Player player, int x, int y)
    {
        return player == Player.Red ? _historyRed[x, y] : _historyBlue[x, y];
    }

    /// <summary>
    /// Clear history tables (call at start of new game)
    /// </summary>
    public void ClearHistory()
    {
        Array.Clear(_historyRed, 0, _historyRed.Length);
        Array.Clear(_historyBlue, 0, _historyBlue.Length);
        Array.Clear(_butterflyRed, 0, _butterflyRed.Length);
        Array.Clear(_butterflyBlue, 0, _butterflyBlue.Length);
    }

    /// <summary>
    /// Clear search state for new position while preserving transposition table.
    /// Use for opening book generation where TT memoization across positions is beneficial.
    /// Clears: history tables, killer moves, pondering state.
    /// Preserves: transposition table entries (memoization), adaptive time state.
    /// </summary>
    public void ClearSearchState()
    {
        ClearHistory();
        ResetPondering();

        // Clear killer moves (position-specific move ordering)
        for (int d = 0; d < MaxKillerDepth; d++)
        {
            for (int k = 0; k < MaxKillerMoves; k++)
            {
                _killerMoves[d, k] = (0, 0);
            }
        }

        // Note: Transposition table is NOT cleared - this preserves memoization
        // TT entries will be aged out naturally via the depth-age replacement strategy
    }

    /// <summary>
    /// Clear all AI state between games to prevent cross-contamination
    /// This is critical when AI of different difficulties play in sequence
    /// </summary>
    public void ClearAllState()
    {
        ClearHistory();
        _transpositionTable.Clear();
        ResetPondering();

        // Reset adaptive time manager state
        _adaptiveTimeManager.Reset();

        // Reset inferred initial time for adaptive thresholds
        // -1 means "unknown, will infer from first move"
        _inferredInitialTimeMs = -1;

        // Clear killer moves
        for (int d = 0; d < MaxKillerDepth; d++)
        {
            for (int k = 0; k < MaxKillerMoves; k++)
            {
                _killerMoves[d, k] = (0, 0);
            }
        }

        // Reset statistics
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _tableHits = 0;
        _tableLookups = 0;

        // Clear parallel search state
        _parallelSearch.Clear();

        // Reset PV prediction state for pondering
        _lastPV = PV.Empty;
        _lastBoard = null;
    }

    /// <summary>
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// </summary>
    private int Quiesce(Board board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Time control: check frequently (every 16 nodes) to avoid timeout
        // Use a different offset to stagger checks between Minimax and Quiesce
        if ((_nodesSearched & 15) == 8)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                // Return current bound to avoid corrupting alpha-beta
                return isMaximizing ? alpha : beta;
            }
        }

        // Get stand-pat score (static evaluation)
        var standPat = _evaluator.Evaluate(board, aiPlayer);

        // Beta cutoff (stand-pat is good enough for maximizing player)
        if (isMaximizing && standPat >= beta)
            return beta;

        // Alpha cutoff (stand-pat is good enough for minimizing player)
        if (!isMaximizing && standPat <= alpha)
            return alpha;

        // Update bounds for search
        if (isMaximizing)
            alpha = Math.Max(alpha, standPat);
        else
            beta = Math.Min(beta, standPat);

        // Check for terminal states in quiescence
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        // Generate tactical moves (only near existing stones)
        var tacticalMoves = GetCandidateMoves(board);

        // Limit quiescence search depth to avoid explosion
        const int maxQuiescenceDepth = 4;  // Search up to 4 ply beyond depth 0
        if (rootDepth - 0 > maxQuiescenceDepth)
        {
            return standPat;  // Stop quiescing, return static eval
        }

        // If no tactical moves, return static evaluation
        if (tacticalMoves.Count == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order tactical moves for better pruning
        var orderedMoves = OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

        // Search tactical moves (only empty cells)
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells (can happen during quiescence recursion)
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search (depth stays at 0, but we track via rootDepth)
                var eval = Quiesce(qBoard, alpha, beta, false, aiPlayer, rootDepth + 1);

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);

                if (beta <= alpha)
                    return beta;  // Beta cutoff
            }
            return maxEval;
        }
        else
        {
            var minEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells (can happen during quiescence recursion)
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(qBoard, alpha, beta, true, aiPlayer, rootDepth + 1);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;  // Alpha cutoff
            }
            return minEval;
        }
    }

    /// <summary>
    /// Check if position is tactical (has threats) - should not use reduced depth
    /// Tactical positions have: 3+ in a row, or multiple threats nearby
    /// </summary>
    private bool IsTacticalPosition(Board board)
    {
        // Check for 3+ in a row
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check horizontal
                var count = 1;
                for (int dy = 1; dy <= 4 && y + dy < BoardSize; dy++)
                {
                    if (board.GetCell(x, y + dy).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check vertical
                count = 1;
                for (int dx = 1; dx <= 4 && x + dx < BoardSize; dx++)
                {
                    if (board.GetCell(x + dx, y).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check diagonal (down-right)
                count = 1;
                for (int i = 1; i <= 4 && x + i < BoardSize && y + i < BoardSize; i++)
                {
                    if (board.GetCell(x + i, y + i).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check diagonal (down-left)
                count = 1;
                for (int i = 1; i <= 4 && x + i < BoardSize && y - i >= 0; i++)
                {
                    if (board.GetCell(x + i, y - i).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;
            }
        }

        return false;  // Not tactical
    }

    /// <summary>
    /// Check if a specific move is tactical (creates threats or blocks opponent)
    /// Used for LMR - tactical moves should not use reduced depth
    /// </summary>
    private bool IsTacticalMove(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check if this move creates threat for player
            var playerCount = 1;
            var playerOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) playerCount++;
                else if (!occupied.GetBit(nx, ny)) { playerOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) playerCount++;
                else if (!occupied.GetBit(nx, ny)) { playerOpenEnds++; break; }
                else break;
            }

            // Creating 3+ with open ends is tactical
            if (playerCount >= 3 && playerOpenEnds >= 1)
                return true;
            if (playerCount >= 4)
                return true;

            // Check if this move blocks opponent threat
            var oppCount = 1;
            var oppOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) { oppOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) { oppOpenEnds++; break; }
                else break;
            }

            // Blocking 3+ with open ends is tactical (must block)
            if (oppCount >= 3 && oppOpenEnds >= 1)
                return true;
            if (oppCount >= 4)
                return true;
        }

        return false;  // Not a tactical move
    }

    // Null-move pruning constants
    private const int NullMoveDepthReduction = 3;  // Search depth-R for null move verification
    private const int NullMoveMinDepth = 3;        // Don't use null-move at shallow depths

    /// <summary>
    /// Verify if null-move is safe (avoid zugzwang positions)
    /// In Caro, null-move is generally safe except in very tight tactical positions
    /// </summary>
    private bool IsNullMoveSafe(Board board, Player player)
    {
        var playerBitBoard = board.GetBitBoard(player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        // Check if position is "quiet" enough for null-move
        // Count stones on board - if too few, null-move is risky
        int totalStones = playerBitBoard.CountBits() + opponentBitBoard.CountBits();
        if (totalStones < 10) return false;  // Early game, too volatile

        // Check for immediate threats (4-in-row, open 3s)
        // If there are threats, null-move is unsafe (might miss tactical sequences)
        foreach (var (dx, dy) in new[] { (1, 0), (0, 1), (1, 1), (1, -1) })
        {
            for (int x = 0; x < BoardSize; x++)
            {
                for (int y = 0; y < BoardSize; y++)
                {
                    if (!opponentBitBoard.GetBit(x, y)) continue;

                    var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBitBoard, x, y, dx, dy);
                    var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBitBoard, occupied, x, y, dx, dy, count);

                    // Opponent has 4-in-row or open 3 - too dangerous for null-move
                    if (count == 4 && openEnds > 0) return false;
                    if (count == 3 && openEnds == 2) return false;
                }
            }
        }

        return true;
    }

    /// <summary>
    /// Verify if null-move is safe for SearchBoard (high-performance path).
    /// </summary>
    private bool IsNullMoveSafe(SearchBoard board, Player player)
    {
        var playerBitBoard = board.GetBitBoard(player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        // Count stones on board
        int totalStones = playerBitBoard.CountBits() + opponentBitBoard.CountBits();
        if (totalStones < 10) return false;

        // Check for immediate threats
        foreach (var (dx, dy) in new[] { (1, 0), (0, 1), (1, 1), (1, -1) })
        {
            for (int x = 0; x < BoardSize; x++)
            {
                for (int y = 0; y < BoardSize; y++)
                {
                    if (!opponentBitBoard.GetBit(x, y)) continue;

                    var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBitBoard, x, y, dx, dy);
                    var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBitBoard, occupied, x, y, dx, dy, count);

                    if (count == 4 && openEnds > 0) return false;
                    if (count == 3 && openEnds == 2) return false;
                }
            }
        }

        return true;
    }

    /// <summary>
    /// Core minimax algorithm using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int MinimaxCore(SearchBoard board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

        // Time control: check periodically (every N nodes) to avoid timeout
        if ((_nodesSearched & (TimeCheckInterval - 1)) == 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            return QuiesceCore(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING
        var isNullMoveEligible = (beta - alpha) <= 1;
        if (depth >= NullMoveMinDepth && isNullMoveEligible && IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            int nullMoveDepth = depth - NullMoveDepthReduction;
            if (nullMoveDepth > 0)
            {
                int nullMoveScore = MinimaxCore(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);
                if (nullMoveScore >= beta)
                {
                    return beta;
                }
            }
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0;
        }

        // Transposition table lookup using SearchBoard hash
        var boardHash = board.GetHash();
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
        if (found)
        {
            _tableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order moves
        var orderedMoves = OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = 4;
        const int pvsEnabledDepth = 2;

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, false, aiPlayer, rootDepth);
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    if (eval > alpha && eval < beta)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval > maxEval)
                {
                    maxEval = eval;
                }

                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, true, aiPlayer, rootDepth);
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    if (eval < beta && eval > alpha)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval < minEval)
                {
                    minEval = eval;
                }

                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = minEval;
        }

        // Store result in transposition table
        _transpositionTable.Store(boardHash, depth, score, null, alpha, beta);

        return score;
    }

    /// <summary>
    /// Quiescence search using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int QuiesceCore(SearchBoard board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Time control
        if ((_nodesSearched & 15) == 8)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Get stand-pat score using SearchBoard evaluator
        var standPat = _evaluator.Evaluate(board, aiPlayer);

        // Beta cutoff
        if (isMaximizing && standPat >= beta)
            return beta;

        // Alpha cutoff
        if (!isMaximizing && standPat <= alpha)
            return alpha;

        // Update bounds
        if (isMaximizing)
            alpha = Math.Max(alpha, standPat);
        else
            beta = Math.Min(beta, standPat);

        // Check for terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        // Generate tactical moves
        var tacticalMoves = GetCandidateMoves(board);

        // Limit quiescence depth
        const int maxQuiescenceDepth = 4;
        if (rootDepth > maxQuiescenceDepth)
        {
            return standPat;
        }

        if (tacticalMoves.Count == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order tactical moves
        var orderedMoves = OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, false, aiPlayer, rootDepth + 1);
                board.UnmakeMove(undo);

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);

                if (beta <= alpha)
                    return beta;
            }
            return maxEval;
        }
        else
        {
            var minEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, true, aiPlayer, rootDepth + 1);
                board.UnmakeMove(undo);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;
            }
            return minEval;
        }
    }

    /// <summary>
    /// Minimax algorithm with alpha-beta pruning and transposition table
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

        // Time control: check periodically (every N nodes) to avoid timeout
        if ((_nodesSearched & (TimeCheckInterval - 1)) == 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                // Return current bound to avoid corrupting alpha-beta
                return isMaximizing ? alpha : beta;
            }
        }

        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING: Skip a move to verify position is already good
        // Only apply in non-PV nodes with sufficient depth and safe position
        var isNullMoveEligible = (beta - alpha) <= 1;  // Not a PV node (narrow window)
        if (depth >= NullMoveMinDepth && isNullMoveEligible && IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            // Make null move: skip turn, search with reduced depth
            // The reduced depth search is done from opponent's perspective (flipped min/max)
            int nullMoveDepth = depth - NullMoveDepthReduction;

            if (nullMoveDepth > 0)
            {
                // Search with null move (flip min/max because we skipped a turn)
                int nullMoveScore = Minimax(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);

                // If null move fails high (score >= beta), the position is so good
                // that even giving opponent a free move doesn't help them
                if (nullMoveScore >= beta)
                {
                    // Beta cutoff: position is good enough, skip searching remaining moves
                    return beta;
                }
            }
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        // Transposition table lookup
        var boardHash = _transpositionTable.CalculateHash(board);
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
        if (found)
        {
            _tableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // IN-TREE VCF CHECK: Check for forcing sequences before move generation
        // VCF runs at all nodes; only time budget limits it (no depth caps)
        // Percentage-based threshold: VCF runs MORE in time scramble (low time remaining)
        var remainingTime = _searchHardBoundMs - _searchStopwatch.ElapsedMilliseconds;
        var initialTime = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : _searchHardBoundMs;
        var timeRemainingPercent = (double)remainingTime / initialTime;

        // Time scramble: < 10% time remaining - VCF is critical (find quick wins)
        // Normal time: use 5% of initial time as threshold
        var vcfThresholdMs = timeRemainingPercent < 0.1
            ? 1  // Always run in time scramble (minimal threshold)
            : initialTime * 0.05;  // 5% of initial time in normal case

        if (remainingTime > vcfThresholdMs)
        {
            var vcfResult = _inTreeVCFSolver.CheckNodeVCF(board, currentPlayer, depth, alpha, remainingTime);
            if (vcfResult != null && vcfResult.Type != VCFResultType.NoVCF)
            {
                // VCF found - return immediately with appropriate score
                if (vcfResult.Type == VCFResultType.WinningSequence)
                {
                    _nodesSearched += vcfResult.NodesSearched;
                    return vcfResult.Score;
                }
                // For losing sequences, we could filter candidates, but for now
                // let the normal search handle it with proper alpha-beta bounds
            }
        }

        // Order moves for better pruning (use cached move if available)
        var orderedMoves = OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = 4;  // First 4 moves at full depth
        const int pvsEnabledDepth = 2;  // Enable PVS at depth 2+

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                var newBoard = board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(newBoard, depth - 2, alpha, beta, false, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (alpha, alpha+1)
                    eval = Minimax(newBoard, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    // If null window search beats alpha, re-search with full window
                    if (eval > alpha && eval < beta)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full beta
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                if (eval > maxEval)
                {
                    maxEval = eval;
                }

                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                {
                    // Beta cutoff - record killer move and history
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break; // Alpha cutoff
                }

                moveIndex++;
            }
            score = maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                var newBoard = board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(newBoard, depth - 2, alpha, beta, true, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (beta-1, beta)
                    eval = Minimax(newBoard, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    // If null window search is below beta, re-search with full window
                    if (eval < beta && eval > alpha)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full alpha
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                if (eval < minEval)
                {
                    minEval = eval;
                }

                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                {
                    // Alpha cutoff - record killer move and history
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break; // Beta cutoff
                }

                moveIndex++;
            }
            score = minEval;
        }

        // Store result in transposition table
        _transpositionTable.Store(boardHash, depth, score, null, alpha, beta);

        return score;
    }

    /// <summary>
    /// Validate a book move with a quick search to ensure it's not a blunder.
    /// This prevents bad book moves from causing strength inversions.
    /// Returns (IsAcceptable, ValidationDepth, NodesSearched, TimeMs)
    /// </summary>
    private (bool IsAcceptable, int ValidationDepth, long NodesSearched, long TimeMs) ValidateBookMove(
        Board board, Player player, (int x, int y) bookMove, AIDifficulty difficulty)
    {
        var validationSw = System.Diagnostics.Stopwatch.StartNew();

        // Quick validation search depth based on difficulty
        int validationDepth = difficulty switch
        {
            AIDifficulty.Grandmaster => 5,
            AIDifficulty.Experimental => 5,
            AIDifficulty.Hard => 4,
            _ => 3
        };

        // Make the book move and evaluate the resulting position
        var boardAfterMove = board.PlaceStone(bookMove.x, bookMove.y, player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Evaluate position from opponent's perspective (after our move)
        // A good book move should leave us with a reasonable position
        int evaluation = _evaluator.Evaluate(boardAfterMove, player);

        // Quick minimax search to validate the move
        int searchScore = QuickValidateSearch(boardAfterMove, opponent, validationDepth, int.MinValue + 1, int.MaxValue - 1);

        // Negate score because we evaluated from opponent's perspective
        int ourScore = -searchScore;

        validationSw.Stop();

        // Accept the book move if:
        // 1. The evaluation is not terrible (>= -100 centipawns)
        // 2. Or we're in a clearly winning/losing position anyway
        bool isAcceptable = ourScore >= -100 || Math.Abs(evaluation) > 500;

        return (isAcceptable, validationDepth, _nodesSearched, validationSw.ElapsedMilliseconds);
    }

    /// <summary>
    /// Quick validation search for book moves - simplified minimax without full features
    /// </summary>
    private int QuickValidateSearch(Board board, Player player, int depth, int alpha, int beta)
    {
        if (depth <= 0)
        {
            return _evaluator.Evaluate(board, player);
        }

        // Check for win
        var winResult = _winDetector.CheckWin(board);
        if (winResult.HasWinner)
        {
            // Return high score for win
            return winResult.Winner == player ? 100000 : -100000;
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
            return 0; // Draw

        int bestScore = int.MinValue + 1;
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        foreach (var (x, y) in candidates.Take(20)) // Limit candidates for speed
        {
            var newBoard = board.PlaceStone(x, y, player);
            int score = -QuickValidateSearch(newBoard, opponent, depth - 1, -beta, -alpha);

            if (score > bestScore)
                bestScore = score;
            if (score > alpha)
                alpha = score;
            if (alpha >= beta)
                break;
        }

        return bestScore;
    }

    /// <summary>
    /// Get candidate moves (empty cells near existing stones)
    /// Zero-allocation implementation using stackalloc for tracking
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        const int boardSize = BoardSize;
        const int cellCount = boardSize * boardSize;

        // Use stackalloc for considered tracking (zero allocation)
        Span<bool> considered = stackalloc bool[cellCount];

        // Pre-allocate with reasonable capacity to avoid resizing
        var candidates = new List<(int x, int y)>(64);

        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player != Player.None)
                {
                    // Check neighboring cells
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                            {
                                int idx = nx * boardSize + ny;
                                if (!considered[idx])
                                {
                                    considered[idx] = true;
                                    if (board.GetCell(nx, ny).Player == Player.None)
                                    {
                                        candidates.Add((nx, ny));
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // If no candidates (empty board), return center
        if (candidates.Count == 0)
        {
            int center = boardSize / 2;
            candidates.Add((center, center));
        }

        return candidates;
    }

    /// <summary>
    /// Get candidate moves for SearchBoard (high-performance path).
    /// Returns empty cells within SearchRadius of any existing stone.
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(SearchBoard board)
    {
        const int boardSize = BoardSize;
        const int cellCount = boardSize * boardSize;

        // Use stackalloc for considered tracking (zero allocation)
        Span<bool> considered = stackalloc bool[cellCount];

        // Pre-allocate with reasonable capacity to avoid resizing
        var candidates = new List<(int x, int y)>(64);

        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (!board.IsEmpty(x, y))
                {
                    // Check neighboring cells
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                            {
                                int idx = nx * boardSize + ny;
                                if (!considered[idx])
                                {
                                    considered[idx] = true;
                                    if (board.IsEmpty(nx, ny))
                                    {
                                        candidates.Add((nx, ny));
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // If no candidates (empty board), return center
        if (candidates.Count == 0)
        {
            int center = boardSize / 2;
            candidates.Add((center, center));
        }

        return candidates;
    }

    /// <summary>
    /// Order moves for SearchBoard (high-performance path).
    /// Uses same heuristics as Board version but optimized for SearchBoard.
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, SearchBoard board, Player player, (int x, int y)? ttMove = null)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        // Score array on stack (zero allocation)
        Span<int> scores = stackalloc int[count];

        // Score each move
        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // PRIORITY #1: Hash Move (TT Move)
            if (ttMove.HasValue && x == ttMove.Value.x && y == ttMove.Value.y)
            {
                score = 10000;
            }
            else
            {
                // PRIORITY #2: Emergency Defense
                if (IsEmergencyDefense(board, x, y, player))
                {
                    score += 5000;
                }

                // PRIORITY #3: Winning Threats
                score += EvaluateTacticalPattern(board, x, y, player);

                // PRIORITY #4: Killer Moves
                if (depth >= 0 && depth < MaxKillerDepth)
                {
                    for (int k = 0; k < MaxKillerMoves; k++)
                    {
                        if (_killerMoves[depth, k].x == x && _killerMoves[depth, k].y == y)
                        {
                            score += 1000;
                            break;
                        }
                    }
                }

                // PRIORITY #5: History/Butterfly Heuristic
                const int butterflySize = BoardSize;
                var butterflyScore = (x >= 0 && x < butterflySize && y >= 0 && y < butterflySize)
                    ? (player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y])
                    : 0;
                score += Math.Min(300, butterflyScore / 100);

                var historyScore = GetHistoryScore(player, x, y);
                score += Math.Min(500, historyScore / 10);

                // PRIORITY #6: Positional Heuristics
                var distanceToCenter = Math.Abs(x - GameConstants.CenterPosition) + Math.Abs(y - GameConstants.CenterPosition);
                score += ((GameConstants.BoardSize - 2) - distanceToCenter) * 10;

                // Prefer moves near existing stones
                var nearby = 0;
                for (int dx = -1; dx <= 1; dx++)
                {
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        if (dx == 0 && dy == 0) continue;
                        var nx = x + dx;
                        var ny = y + dy;
                        if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                        {
                            if (!board.IsEmpty(nx, ny))
                                nearby += 5;
                        }
                    }
                }
                score += nearby;
            }

            scores[i] = score;
        }

        // Simple insertion sort (fast for small arrays, no allocations)
        for (int i = 1; i < count; i++)
        {
            var keyMove = candidates[i];
            var keyScore = scores[i];
            int j = i - 1;

            while (j >= 0 && scores[j] < keyScore)
            {
                candidates[j + 1] = candidates[j];
                scores[j + 1] = scores[j];
                j--;
            }

            candidates[j + 1] = keyMove;
            scores[j + 1] = keyScore;
        }

        return candidates;
    }

    /// <summary>
    /// Check if there's a winner using SearchBoard (high-performance path).
    /// Uses bitboard-based 5-in-a-row detection.
    /// </summary>
    private Player? CheckWinner(SearchBoard board)
    {
        if (board.HasWin(Player.Red))
            return Player.Red;
        if (board.HasWin(Player.Blue))
            return Player.Blue;
        return null;
    }

    /// <summary>
    /// Check if position is tactical using SearchBoard.
    /// </summary>
    private bool IsTacticalPosition(SearchBoard board)
    {
        var redBits = board.GetBitBoard(Player.Red);
        var blueBits = board.GetBitBoard(Player.Blue);

        // Quick check using bitboard operations
        // Check for 3+ in a row in any direction for either player
        return HasThreeInRow(redBits) || HasThreeInRow(blueBits);
    }

    /// <summary>
    /// Check if a BitBoard has 3+ consecutive stones in any direction.
    /// </summary>
    private bool HasThreeInRow(BitBoard bits)
    {
        // Check horizontal: shift right 3 times and AND
        var h1 = bits;
        var h2 = h1.ShiftRight();
        var h3 = h2.ShiftRight();
        if ((h1 & h2 & h3).IsEmpty == false)
            return true;

        // Check vertical: shift down 3 times and AND
        var v1 = bits;
        var v2 = v1.ShiftDown();
        var v3 = v2.ShiftDown();
        if ((v1 & v2 & v3).IsEmpty == false)
            return true;

        // Check diagonal \
        var d1 = bits;
        var d2 = d1.ShiftDownRight();
        var d3 = d2.ShiftDownRight();
        if ((d1 & d2 & d3).IsEmpty == false)
            return true;

        // Check diagonal /
        var a1 = bits;
        var a2 = a1.ShiftDownLeft();
        var a3 = a2.ShiftDownLeft();
        if ((a1 & a2 & a3).IsEmpty == false)
            return true;

        return false;
    }

    /// <summary>
    /// Emergency defense check for SearchBoard.
    /// Returns true if this move blocks opponent's immediate winning threat.
    /// </summary>
    private bool IsEmergencyDefense(SearchBoard board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Check if opponent would win by playing at (x, y)
        if (board.IsWinningMove(x, y, opponent))
            return true;

        // Check for double threats (multiple open 3s or open 4s)
        var opponentBits = board.GetBitBoard(opponent);
        var threatCount = 0;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };
        foreach (var (dx, dy) in directions)
        {
            var count = 1;
            var openEnds = 0;

            // Check positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBits.GetBit(nx, ny)) count++;
                else if (board.IsEmpty(nx, ny)) { openEnds++; break; }
                else break;
            }

            // Check negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBits.GetBit(nx, ny)) count++;
                else if (board.IsEmpty(nx, ny)) { openEnds++; break; }
                else break;
            }

            // Open 4 or open 3 is a threat
            if (count >= 4 && openEnds >= 1) threatCount++;
            else if (count >= 3 && openEnds >= 2) threatCount++;
        }

        return threatCount >= 2;
    }

    /// <summary>
    /// Evaluate tactical pattern for SearchBoard.
    /// Uses bitboard operations for efficiency.
    /// </summary>
    private int EvaluateTacticalPattern(SearchBoard board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var score = 0;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count consecutive stones in both directions (for player)
            var count = 1;
            var openEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score based on pattern
            if (count >= 5) score += 100000;  // Winning move
            else if (count == 4 && openEnds >= 1) score += 10000;  // Open 4
            else if (count == 3 && openEnds == 2) score += 5000;   // Open 3 (double threat)
            else if (count == 3 && openEnds == 1) score += 500;    // Half-open 3
            else if (count == 2 && openEnds == 2) score += 100;    // Open 2

            // Also check blocking value (opponent patterns)
            var oppCount = 1;
            var oppOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    oppCount++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    oppOpenEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    oppCount++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    oppOpenEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Blocking is slightly less valuable than attacking
            if (oppCount >= 5) score += 90000;  // Block win
            else if (oppCount == 4 && oppOpenEnds >= 1) score += 9000;  // Block open 4
            else if (oppCount == 3 && oppOpenEnds == 2) score += 4000;   // Block open 3
        }

        return score;
    }

    /// <summary>
    /// Get ALL legal moves (every empty cell on the board).
    /// Used for error rate simulation - true random moves, not tactical moves.
    /// </summary>
    private List<(int x, int y)> GetAllLegalMoves(Board board)
    {
        var legalMoves = new List<(int x, int y)>(64);

        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    legalMoves.Add((x, y));
                }
            }
        }

        return legalMoves;
    }

    /// <summary>
    /// PROACTIVE DEFENSE: Find squares that block opponent's open threes.
    /// An open three is 3 stones in a row with BOTH ends open (not blocked).
    /// Open threes become open fours on the next move, which are unblockable.
    /// We should block open threes BEFORE they become open fours.
    /// </summary>
    private List<(int x, int y)> FindOpenThreeBlocks(Board board, Player opponent)
    {
        var blocks = new List<(int x, int y)>();
        var directions = new (int dx, int dy)[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        // Scan for open threes in all 4 directions
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player != opponent)
                    continue;

                foreach (var (dx, dy) in directions)
                {
                    // Check if this stone is the START of a 3-in-a-row
                    int prevX = x - dx;
                    int prevY = y - dy;

                    // Skip if not the start (previous cell is also opponent's stone)
                    if (prevX >= 0 && prevX < BoardSize && prevY >= 0 && prevY < BoardSize)
                    {
                        if (board.GetCell(prevX, prevY).Player == opponent)
                            continue;
                    }

                    // Count consecutive opponent stones
                    int count = 0;
                    int currX = x, currY = y;
                    while (currX >= 0 && currX < BoardSize && currY >= 0 && currY < BoardSize &&
                           board.GetCell(currX, currY).Player == opponent)
                    {
                        count++;
                        currX += dx;
                        currY += dy;
                    }

                    // Only interested in exactly 3 consecutive stones
                    if (count != 3)
                        continue;

                    // Check if both ends are open (empty)
                    int endX = currX;
                    int endY = currY;
                    bool endOpen = endX >= 0 && endX < BoardSize && endY >= 0 && endY < BoardSize &&
                                   board.GetCell(endX, endY).Player == Player.None;

                    int startX = x - dx;
                    int startY = y - dy;
                    bool startOpen = startX >= 0 && startX < BoardSize && startY >= 0 && startY < BoardSize &&
                                     board.GetCell(startX, startY).Player == Player.None;

                    // Open three: 3 in a row with both ends open
                    if (startOpen && endOpen)
                    {
                        // Block one end - prefer the end that prevents open four
                        // Add both ends as potential blocks
                        if (!blocks.Contains((startX, startY)))
                            blocks.Add((startX, startY));
                        if (!blocks.Contains((endX, endY)))
                            blocks.Add((endX, endY));
                    }
                }
            }
        }

        return blocks;
    }

    /// <summary>
    /// Check if there's a winner on the board using WinDetector
    /// This ensures Caro rules are enforced: exact 5-in-a-row, no sandwiched wins, no overlines
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner ? result.Winner : null;
    }

    /// <summary>
    /// Get the last move made by the opponent
    /// Used by opening book for intelligent responses
    /// </summary>
    private (int x, int y)? GetLastOpponentMove(Board board, Player currentPlayer)
    {
        var opponent = currentPlayer == Player.Red ? Player.Blue : Player.Red;

        // Find the most recent opponent move by checking all occupied cells
        // We'll return any occupied opponent cell (for opening book, this is sufficient)
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == opponent)
                {
                    return (x, y);
                }
            }
        }

        return null;
    }

    // ========== AGGRESSIVE PRUNING TECHNIQUES FOR Grandmaster+ ==========

    // Futility pruning constants
    private const int FutilityMarginBase = 300;      // Base margin for futility pruning
    private const int FutilityMarginPerDepth = 100;  // Additional margin per depth remaining
    private const int FutilityMinDepth = 3;          // Don't use futility at shallow depths

    /// <summary>
    /// Check if a move at (x, y) creates or blocks critical threats
    /// These moves should NEVER be pruned as they're tactically significant
    /// </summary>
    private bool IsCriticalMove(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check if this move creates threats for current player
            var count = 1; // Include the placed stone

            // Count in positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: creates 4+ or open 3
            if (count >= 4) return true; // Potential winning move
            if (count == 3)
            {
                // Check if both ends are open
                bool leftOpen = x - dx >= 0 && x - dx < BoardSize && y - dy >= 0 && y - dy < BoardSize
                               && !occupied.GetBit(x - dx, y - dy);
                bool rightOpen = x + dx * 3 >= 0 && x + dx * 3 < BoardSize && y + dy * 3 >= 0 && y + dy * 3 < BoardSize
                                && !occupied.GetBit(x + dx * 3, y + dy * 3);
                if (leftOpen && rightOpen) return true; // Creates open three
            }

            // Check if this move blocks opponent threats
            var oppCount = 1;

            // Count opponent stones in positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count opponent stones in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: blocks opponent's 4 or open 3
            if (oppCount >= 4) return true; // Blocks winning threat
            if (oppCount == 3)
            {
                // Check if this blocks an open three
                var leftOpen = x - dx >= 0 && x - dx < BoardSize && y - dy >= 0 && y - dy < BoardSize
                              && !occupied.GetBit(x - dx, y - dy);
                var rightOpen = x + dx * 3 >= 0 && x + dx * 3 < BoardSize && y + dy * 3 >= 0 && y + dy * 3 < BoardSize
                               && !occupied.GetBit(x + dx * 3, y + dy * 3);
                if (leftOpen && rightOpen) return true; // Blocks open three
            }
        }

        return false;
    }

    /// <summary>
    /// Estimate the maximum possible gain from a move at (x, y)
    /// Used for futility pruning - if max gain < alpha - margin, skip search
    /// </summary>
    private int EstimateMaxGain(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        int maxGain = 0;
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count consecutive stones after placing this stone
            var count = 1;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Score based on potential
            if (count >= 5) maxGain += 100000;
            else if (count == 4) maxGain += 10000;
            else if (count == 3) maxGain += 1000;
            else if (count == 2) maxGain += 100;
            else if (count == 1) maxGain += 10;
        }

        // Add blocking value
        foreach (var (dx, dy) in directions)
        {
            var count = 1;

            // Positive direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            if (count >= 4) maxGain += 10000;
            else if (count == 3) maxGain += 1000;
        }

        return maxGain;
    }

    /// <summary>
    /// Check if futility pruning is safe for this position
    /// Returns false if the position is tactical or has high uncertainty
    /// </summary>
    private bool IsFutilitySafe(Board board, int depth, int alpha, int beta)
    {
        // Don't use futility in PV nodes
        if (beta - alpha > 1) return false;

        // Don't use futility at shallow depths
        if (depth < FutilityMinDepth) return false;

        // Don't use futility if position is tactical
        if (IsTacticalPosition(board)) return false;

        return true;
    }

    /// <summary>
    /// Calculate LMR reduction based on move index and depth
    /// More aggressive reduction for later moves at higher depths
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int CalculateLMRReduction(int depth, int moveIndex, bool isCriticalMove)
    {
        // Critical moves (threats/blocks) get no reduction
        if (isCriticalMove) return 0;

        // Base reduction: floor(depth/3) + floor(moveIndex/3)
        int reduction = depth / 3 + moveIndex / 3;

        // Cap reduction at depth-2 (always search at least 2 ply)
        return Math.Min(reduction, depth - 2);
    }

    /// <summary>
    /// ProbCut: Probabilistic cutoff for deep searches
    /// Try a shallow search first; if it shows clear cutoff, skip deep search
    /// </summary>
    private bool TryProbCut(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Only use ProbCut at depth 5+ when we have a narrow window
        if (depth < 5 || (beta - alpha) > 100) return false;

        // Try shallow search at reduced depth
        int probCutDepth = depth / 2;
        int probCutBeta = beta + 200; // More optimistic threshold

        var score = Minimax(board, probCutDepth, probCutBeta - 1, probCutBeta, isMaximizing, aiPlayer, rootDepth);

        // If shallow search already exceeds beta, we're likely to cutoff
        return score >= probCutBeta;
    }

    #region Pondering Support

    /// <summary>
    /// Get the ponderer instance for external access (e.g., TournamentEngine)
    /// </summary>
    public Ponderer GetPonderer() => _ponderer;

    /// <summary>
    /// Get the last calculated Principal Variation
    /// </summary>
    public PV GetLastPV() => _lastPV;

    /// <summary>
    /// Stop any active pondering and publish ponder stats with explicit player color
    /// </summary>
    public void StopPondering(Player forPlayer)
    {
        _ponderer.StopPondering();
        PublishPonderStats(forPlayer);
    }

    /// <summary>
    /// Start pondering immediately (at start of opponent's turn, without waiting for prediction)
    /// </summary>
    public void StartPonderingNow(Board board, Player currentPlayerToMove, AIDifficulty difficulty, Player thisAIColor)
    {
        // The AI owning this method will ponder during currentPlayerToMove's turn
        // We want to analyze the position where currentPlayerToMove is to move
        var ponderTimeMs = CalculatePonderTime(null, difficulty);
        if (ponderTimeMs > 0)
        {
            // Ponder the position where the current player is to move
            // thisAIColor explicitly tells this AI which color it is playing as
            _ponderer.StartPondering(board, currentPlayerToMove, null, thisAIColor, difficulty, ponderTimeMs);
        }
    }

    /// <summary>
    /// Start pondering after making a move (for opponent's response)
    /// </summary>
    /// <summary>
    /// Start pondering after making a move (for opponent's response)
    /// This is a stateless version - all parameters passed explicitly
    /// </summary>
    public void StartPonderingAfterMove(Board board, Player opponentToMove, Player thisAIColor, AIDifficulty difficulty, PV? lastPV = null)
    {
        var predictedOpponentMove = lastPV?.GetPredictedOpponentMove() ?? _lastPV.GetPredictedOpponentMove();

        var ponderTimeMs = CalculatePonderTime(null, difficulty);
        if (ponderTimeMs > 0)
        {
            _ponderer.StartPondering(board, opponentToMove, predictedOpponentMove, thisAIColor, difficulty, ponderTimeMs);
        }
    }

    /// <summary>
    /// Reset pondering state (call when starting a new game)
    /// </summary>
    public void ResetPondering()
    {
        _ponderer.Reset();
        _lastPV = PV.Empty;
        _lastBoard = null;
    }

    /// <summary>
    /// Get pondering statistics
    /// </summary>
    public string GetPonderingStatistics() => _ponderer.GetStatistics();

    /// <summary>
    /// Get last ponder result statistics (nodes searched during opponent's turn)
    /// Returns (depth, nodesSearched, nodesPerSecond, timeSpentMs)
    /// </summary>
    public (int Depth, long NodesSearched, double NodesPerSecond, long TimeSpentMs) GetLastPonderStats(Player forPlayer)
    {
        var ponderResult = _ponderer.GetCurrentResult();
        var depth = ponderResult.Depth;
        var nodesSearched = ponderResult.NodesSearched;
        var timeSpentMs = ponderResult.TimeSpentMs;
        var nps = timeSpentMs > 0 ? (double)nodesSearched * 1000 / timeSpentMs : 0;

        return (depth, nodesSearched, nps, timeSpentMs);
    }

    /// <summary>
    /// Get search statistics for the last move
    /// </summary>
    public (int DepthAchieved, long NodesSearched, double NodesPerSecond, double TableHitRate, bool PonderingActive, int VCFDepthAchieved, long VCFNodesSearched, int ThreadCount, string? ParallelDiagnostics, double MasterTTPercent, double HelperAvgDepth, long AllocatedTimeMs, bool BookUsed, MoveType MoveType, int SearchScore) GetSearchStatistics()
    {
        double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
        var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
        double nps = elapsedMs > 0 ? (double)_nodesSearched * 1000 / elapsedMs : 0;

        // Parse % from master and helper avg depth from diagnostics string
        double masterTTPercent = 0;
        double helperAvgDepth = 0;

        if (!string.IsNullOrEmpty(_lastParallelDiagnostics))
        {
            // Parse "% from master" from TT part
            var ttMatch = System.Text.RegularExpressions.Regex.Match(_lastParallelDiagnostics, @"(\d+\.?\d*)% from master");
            if (ttMatch.Success && double.TryParse(ttMatch.Groups[1].Value, out var ttPercent))
            {
                masterTTPercent = ttPercent;
            }

            // Parse "avg=X.X" from Depths part
            var avgMatch = System.Text.RegularExpressions.Regex.Match(_lastParallelDiagnostics, @"avg=([\d\.]+)");
            if (avgMatch.Success && double.TryParse(avgMatch.Groups[1].Value, out var avgDepth))
            {
                helperAvgDepth = avgDepth;
            }
        }

        return (_depthAchieved, _nodesSearched, nps, hitRate, _lastPonderingEnabled, _vcfDepthAchieved, _vcfNodesSearched, _lastThreadCount, _lastParallelDiagnostics, masterTTPercent, helperAvgDepth, _lastAllocatedTimeMs, _bookUsed, _moveType, _lastSearchScore);
    }

    /// <summary>
    /// Publish search statistics to the stats channel
    /// Called automatically after each search
    /// </summary>
    public void PublishSearchStats(Player player, StatsType statsType, long moveTimeMs)
    {
        var (depthAchieved, nodesSearched, nps, hitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched, threadCount, _, masterTTPercent, helperAvgDepth, allocatedTimeMs, bookUsed, moveType, _) = GetSearchStatistics();

        var statsEvent = new MoveStatsEvent
        {
            PublisherId = _publisherId,
            Player = player,
            Type = statsType,
            DepthAchieved = depthAchieved,
            NodesSearched = nodesSearched,
            NodesPerSecond = nps,
            TableHitRate = hitRate,
            PonderingActive = ponderingActive,
            VCFDepthAchieved = vcfDepthAchieved,
            VCFNodesSearched = vcfNodesSearched,
            ThreadCount = threadCount,
            MoveTimeMs = moveTimeMs,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            MasterTTPercent = masterTTPercent,
            HelperAvgDepth = helperAvgDepth,
            AllocatedTimeMs = allocatedTimeMs,
            MoveType = moveType
        };

        _statsChannel.Writer.TryWrite(statsEvent);
    }

    /// <summary>
    /// Publish pondering statistics to the stats channel
    /// </summary>
    public void PublishPonderStats(Player player)
    {
        var (depth, nodesSearched, nps, timeSpentMs) = GetLastPonderStats(player);

        if (nodesSearched == 0 && timeSpentMs == 0)
            return;

        var statsEvent = new MoveStatsEvent
        {
            PublisherId = _publisherId,
            Player = player,
            Type = StatsType.Pondering,
            DepthAchieved = depth,
            NodesSearched = nodesSearched,
            NodesPerSecond = nps,
            TableHitRate = 0,
            PonderingActive = true,
            VCFDepthAchieved = 0,
            VCFNodesSearched = 0,
            ThreadCount = 0,
            MoveTimeMs = timeSpentMs,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        };

        _statsChannel.Writer.TryWrite(statsEvent);
    }

    #endregion
}
