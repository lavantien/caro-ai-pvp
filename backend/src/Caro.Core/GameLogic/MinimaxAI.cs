using System.Runtime.CompilerServices;
using System.Diagnostics;
using Caro.Core.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Pondering;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning and advanced optimizations
/// Optimizations: Transposition Table, Killer Heuristic, History Heuristic, Improved Move Ordering, Iterative Deepening, VCF Solver
/// For higher difficulties (D7+), uses parallel search (Lazy SMP) for multi-core speedup
/// Time management: Intelligent time allocation optimized for 7+5 time control
/// </summary>
public class MinimaxAI
{
    private readonly BoardEvaluator _evaluator = new();
    private readonly Random _random = new();
    private readonly TranspositionTable _transpositionTable = new();
    private readonly WinDetector _winDetector = new();
    private readonly ThreatSpaceSearch _vcfSolver = new();
    private readonly OpeningBook _openingBook = new();

    // Time management for 7+5 time control
    private readonly TimeManager _timeManager = new();

    // Parallel search for high difficulties (D7+)
    // Lazy SMP provides 4-8x speedup on multi-core systems
    private readonly ParallelMinimaxSearch _parallelSearch = new();

    // Search radius around existing stones (optimization)
    private const int SearchRadius = 2;

    // Killer heuristic: track best moves at each depth
    private const int MaxKillerMoves = 2;
    private readonly (int x, int y)[,] _killerMoves = new (int x, int y)[20, MaxKillerMoves]; // Max depth 20

    // History heuristic: track moves that cause cutoffs across all depths
    // Two tables: one for Red, one for Blue (each move can be good for different players)
    private readonly int[,] _historyRed = new int[15, 15];   // History scores for Red moves
    private readonly int[,] _historyBlue = new int[15, 15];  // History scores for Blue moves

    // Butterfly heuristic: track moves that cause beta cutoffs (complements history)
    private readonly int[,] _butterflyRed = new int[15, 15];
    private readonly int[,] _butterflyBlue = new int[15, 15];

    // Track transposition table hits for debugging
    private int _tableHits;
    private int _tableLookups;

    // Track search statistics for last move
    private long _nodesSearched;
    private int _depthAchieved;
    private int _vcfNodesSearched;
    private int _vcfDepthAchieved;
    private readonly Stopwatch _searchStopwatch = new();

    // Pondering (thinking on opponent's time)
    private readonly Ponderer _ponderer = new();
    private PV _lastPV = PV.Empty;
    private Board? _lastBoard;
    private Player _lastPlayer;

    /// <summary>
    /// Get the best move for the AI player
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, bool ponderingEnabled = false)
    {
        return GetBestMove(board, player, difficulty, timeRemainingMs: null, moveNumber: 0, ponderingEnabled: ponderingEnabled);
    }

    /// <summary>
    /// Get the best move for the AI player with time awareness
    /// Dynamically adjusts search depth based on remaining time
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, long? timeRemainingMs, bool ponderingEnabled = false)
    {
        return GetBestMove(board, player, difficulty, timeRemainingMs, moveNumber: 0, ponderingEnabled: ponderingEnabled);
    }

    /// <summary>
    /// Get the best move for the AI player with full time awareness
    /// Dynamically adjusts search depth based on remaining time and game phase
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player to move</param>
    /// <param name="difficulty">AI difficulty level (D1-D11)</param>
    /// <param name="timeRemainingMs">Time remaining on clock in milliseconds (null for unlimited)</param>
    /// <param name="moveNumber">Current move number (1-indexed, 0 if unknown)</param>
    /// <param name="ponderingEnabled">Enable pondering (thinking on opponent's time)</param>
    /// <returns>Best move coordinates</returns>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, long? timeRemainingMs, int moveNumber, bool ponderingEnabled = false)
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
            // No valid candidates - board is empty or all filtered out, play center (if not filtered) or first available
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
            return (7, 7);
        }

        // Check for ponder hit - if opponent played predicted move, we can use cached result
        if (ponderingEnabled && _ponderer.IsPondering && _lastPV.IsEmpty == false)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var (ponderState, ponderResult) = _ponderer.HandleOpponentMove(-1, -1); // Dummy, just to get state

            // For now, we handle ponder hit by checking if board matches our pondered position
            // The actual ponder hit detection is done externally via TournamentEngine
        }

        // TODO: Opening book disabled temporarily - needs better threat detection to avoid interfering with tactical positions
        // var bookMove = _openingBook.GetBookMove(board, player, lastOpponentMove);
        // if (bookMove.HasValue)
        // {
        //     return bookMove.Value;
        // }

        // For Beginner difficulty, sometimes add randomness
        if (difficulty == AIDifficulty.Beginner && _random.Next(100) < 20)
        {
            // 20% chance to play random valid move
            var randomIndex = _random.Next(candidates.Count);
            return candidates[randomIndex];
        }

        // Calculate time allocation for 7+5 time control
        TimeAllocation timeAlloc;
        if (timeRemainingMs.HasValue)
        {
            timeAlloc = _timeManager.CalculateMoveTime(
                timeRemainingMs.Value,
                moveNumber,
                candidates.Count,
                board,
                player,
                difficulty
            );
        }
        else
        {
            timeAlloc = GetDefaultTimeAllocation(difficulty);
        }

        // Emergency mode - use TT move at D5+ if available
        if (timeAlloc.IsEmergency && difficulty >= AIDifficulty.Hard)
        {
            var ttMove = GetTranspositionTableMove(board, player, minDepth: 5);
            if (ttMove.HasValue)
            {
                Console.WriteLine("[AI] Emergency mode: Using TT move at D5+");
                return ttMove.Value;
            }
        }

        // CRITICAL DEFENSE: Check if opponent has immediate winning threat
        // This takes priority over everything - even VCF search
        // If opponent has four in a row (or open three), we must block immediately
        var criticalDefense = FindCriticalDefense(board, player);
        if (criticalDefense.HasValue)
        {
            var (blockX, blockY) = criticalDefense.Value;
            Console.WriteLine($"[AI DEFENSE] Critical threat detected! Blocking at ({blockX}, {blockY})");
            return (blockX, blockY);
        }

        // Try VCF (Victory by Continuous Four) search first for higher difficulties
        // VCF is much faster than full minimax for tactical positions
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(timeAlloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, timeLimitMs: vcfTimeLimit, maxDepth: 30);

            // Capture VCF statistics even if not a winning sequence
            _vcfDepthAchieved = vcfResult.DepthAchieved;
            _vcfNodesSearched = vcfResult.NodesSearched;

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                // VCF found a forced win sequence - use it immediately
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}, nodes: {vcfResult.NodesSearched}");
                return vcfResult.BestMove.Value;
            }
        }

        // DISABLED: Parallel search (Lazy SMP) has fundamental architectural issues
        // The current implementation with shared transposition table causes:
        // 1. Result aggregation picking wrong moves due to thread timing differences
        // 2. Transposition table corruption from concurrent writes
        // 3. AI strength inversion where higher difficulties lose to lower ones
        //
        // TODO: Re-implement using one of these approaches:
        // - Young Brothers Wait Concept (YBWC): Helper threads depend on master thread
        // - Root Parallelization: Different threads search different root moves
        // - Aspiration Windows with proper per-thread alpha/beta isolation
        //
        // For now, sequential search provides correct AI strength ordering.

        // Adjust depth based on time remaining
        var adjustedDepth = CalculateDepthForTime(baseDepth, timeAlloc, timeRemainingMs, candidates.Count);

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

        // Debug: Print depth usage for high difficulties to investigate balance
        if (difficulty == AIDifficulty.Normal || difficulty == AIDifficulty.Hard || difficulty == AIDifficulty.Master || difficulty == AIDifficulty.Grandmaster || difficulty == AIDifficulty.Legend || difficulty == AIDifficulty.Expert)
        {
            Console.WriteLine($"[AI DEBUG] {difficulty}: baseDepth={baseDepth}, adjustedDepth={adjustedDepth}, candidates={candidates.Count}, timeRemaining={(timeRemainingMs.HasValue ? $"{timeRemainingMs.Value / 1000.0:F1}s" : "N/A")}");
        }

        // Iterative deepening with time awareness
        var bestMove = candidates[0];
        var currentDepth = Math.Min(2, adjustedDepth); // Start with depth 2

        while (currentDepth <= adjustedDepth)
        {
            // Check time bounds using TimeAllocation
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= timeAlloc.HardBoundMs)
            {
                break; // Hard bound reached, must stop
            }

            // Soft bound check - can continue if position is unstable
            if (elapsed >= timeAlloc.SoftBoundMs)
            {
                // Continue only if this seems critical (e.g., close game)
                // For now, be conservative and stop
                break;
            }

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
                _depthAchieved = currentDepth; // Track deepest completed search
            }
            currentDepth++;
        }

        _searchStopwatch.Stop();

        // Print transposition table statistics for debugging
        if (difficulty == AIDifficulty.Hard || difficulty == AIDifficulty.Master || difficulty == AIDifficulty.Grandmaster || difficulty == AIDifficulty.Legend)
        {
            double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
            Console.WriteLine($"[AI TT] Hits: {_tableHits}/{_tableLookups} ({hitRate:F1}%)");
            var (used, usage) = _transpositionTable.GetStats();
            Console.WriteLine($"[AI TT] Table usage: {used} entries ({usage:F2}%)");
            var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
            var nps = elapsedMs > 0 ? _nodesSearched * 1000 / elapsedMs : 0;
            Console.WriteLine($"[AI STATS] Depth: {_depthAchieved}, Nodes: {_nodesSearched}, NPS: {nps:F0}");
        }

        // Store PV for pondering
        _lastPV = PV.FromSingleMove(bestMove.x, bestMove.y, baseDepth, 0);
        _lastBoard = board.Clone();
        _lastPlayer = player;

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

        return bestMove;
    }

    /// <summary>
    /// Calculate appropriate search depth based on time allocation and position complexity
    /// Uses percentage-based thresholds for 7+5 time control (420 seconds initial)
    /// Ensures full depth is reachable at the start of the game
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, long? timeRemainingMs, int candidateCount)
    {
        // Emergency mode - VERY aggressive depth reduction to avoid timeout
        // In time scramble, rely on VCF + TT move, not deep search
        if (timeAlloc.IsEmergency)
        {
            return Math.Max(1, baseDepth / 3); // D11 -> D3, D10 -> D3, etc.
        }

        // Per-move time allocation
        var softBoundSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Total time remaining (for 7+5, initial time is 420 seconds)
        // Default to 420s if not specified (assumes 7+5 time control)
        var totalTimeRemainingSeconds = timeRemainingMs.HasValue ? timeRemainingMs.Value / 1000.0 : 420.0;

        // For 7+5 time control (420s), use percentage-based thresholds:
        // - Full depth when > 50% time remaining (>210s)
        // - Depth-1 when 25-50% time remaining (105-210s)
        // - Depth-2 when 15-25% time remaining (63-105s)
        // - Aggressive reduction when < 15% time remaining (<63s)

        // Critical: less than 15% of initial time (63s for 7+5) or very tight per-move limit
        if (softBoundSeconds < 3 || totalTimeRemainingSeconds < 60)
        {
            return Math.Max(2, baseDepth / 2); // D11 -> D5, D10 -> D5
        }

        // Low: 15-25% of initial time (63-105s for 7+5) or moderate per-move limit
        if (softBoundSeconds < 6 || totalTimeRemainingSeconds < 105)
        {
            if (candidateCount > 25) // Complex position
            {
                return Math.Max(3, baseDepth - 3);
            }
            return Math.Max(3, baseDepth - 2); // D11 -> D9, D10 -> D8
        }

        // Moderate: 25-50% of initial time (105-210s for 7+5)
        if (totalTimeRemainingSeconds < 210)
        {
            if (candidateCount > 25) // Complex position with limited time
            {
                return Math.Max(4, baseDepth - 2);
            }
            return Math.Max(4, baseDepth - 1); // D11 -> D10, D10 -> D9
        }

        // Good time availability (>50% remaining): use full depth
        return baseDepth;
    }

    /// <summary>
    /// Calculate appropriate time limit for VCF search based on time allocation
    /// VCF is fast for tactical positions but we limit it to avoid wasting time
    /// </summary>
    private int CalculateVCFTimeLimit(TimeAllocation timeAlloc)
    {
        // Emergency mode - very quick VCF check
        if (timeAlloc.IsEmergency)
        {
            return 50; // Very quick check
        }

        // Use a fraction of the soft bound for VCF
        var vcfTime = Math.Max(50, timeAlloc.SoftBoundMs / 10);

        // Cap at reasonable values
        return (int)Math.Min(vcfTime, 500);
    }

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
            AIDifficulty.Beginner => baseTimeMs / 20,   // 5% of time
            AIDifficulty.Easy => baseTimeMs / 10,       // 10%
            AIDifficulty.Normal => baseTimeMs / 5,      // 20%
            AIDifficulty.Medium => baseTimeMs / 4,      // 25%
            AIDifficulty.Hard => baseTimeMs / 3,        // 33%
            AIDifficulty.Harder => (long)(baseTimeMs / 2.5),  // 40%
            _ => baseTimeMs / 2                          // 50% for high levels (D7+)
        };
    }

    /// <summary>
    /// Get default time allocation when no time limit is specified
    /// Provides reasonable time targets for each difficulty level
    /// </summary>
    private static TimeAllocation GetDefaultTimeAllocation(AIDifficulty difficulty) => difficulty switch
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
            if (x >= 0 && x < 15 && y >= 0 && y < 15)
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
    /// </summary>
    private (int x, int y)? FindCriticalDefense(Board board, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBoard = board.GetBitBoard(opponent);
        var occupied = board.GetBitBoard(player) | opponentBoard;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        // Use a HashSet to track positions we've already checked (avoid duplicates)
        var criticalMoves = new HashSet<(int x, int y)>();

        // Check each opponent stone for threats
        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!opponentBoard.GetBit(x, y))
                    continue;

                foreach (var (dx, dy) in directions)
                {
                    // Count consecutive opponent stones in this direction
                    var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBoard, x, y, dx, dy);

                    // Four in a row - CRITICAL! Must block immediately
                    if (count == 4)
                    {
                        var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBoard, occupied, x, y, dx, dy, count);

                        if (openEnds >= 1)
                        {
                            // Find the sequence start and end
                            var startX = x;
                            var startY = y;

                            // Find the start of the sequence
                            while (startX - dx >= 0 && startX - dx < BitBoard.Size &&
                                   startY - dy >= 0 && startY - dy < BitBoard.Size &&
                                   opponentBoard.GetBit(startX - dx, startY - dy))
                            {
                                startX -= dx;
                                startY -= dy;
                            }

                            var endX = startX + dx * 3;
                            var endY = startY + dy * 3;

                            // Check positive end (after the sequence)
                            if (endX + dx >= 0 && endX + dx < BitBoard.Size &&
                                endY + dy >= 0 && endY + dy < BitBoard.Size &&
                                !occupied.GetBit(endX + dx, endY + dy))
                            {
                                criticalMoves.Add((endX + dx, endY + dy));
                            }

                            // Check negative end (before the sequence)
                            if (startX - dx >= 0 && startX - dx < BitBoard.Size &&
                                startY - dy >= 0 && startY - dy < BitBoard.Size &&
                                !occupied.GetBit(startX - dx, startY - dy))
                            {
                                criticalMoves.Add((startX - dx, startY - dy));
                            }
                        }
                    }

                    // Open three (three in a row with both ends open) - also critical
                    // Opponent can create four in a row on their next turn
                    if (count == 3)
                    {
                        var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBoard, occupied, x, y, dx, dy, count);

                        if (openEnds == 2)
                        {
                            // Find the sequence start and end
                            var startX = x;
                            var startY = y;

                            while (startX - dx >= 0 && startX - dx < BitBoard.Size &&
                                   startY - dy >= 0 && startY - dy < BitBoard.Size &&
                                   opponentBoard.GetBit(startX - dx, startY - dy))
                            {
                                startX -= dx;
                                startY -= dy;
                            }

                            var endX = startX + dx * 2;
                            var endY = startY + dy * 2;

                            // Block one end - prioritize based on board position (center is better)
                            if (startX - dx >= 0 && startX - dx < BitBoard.Size &&
                                startY - dy >= 0 && startY - dy < BitBoard.Size &&
                                !occupied.GetBit(startX - dx, startY - dy))
                            {
                                criticalMoves.Add((startX - dx, startY - dy));
                            }

                            if (endX + dx >= 0 && endX + dx < BitBoard.Size &&
                                endY + dy >= 0 && endY + dy < BitBoard.Size &&
                                !occupied.GetBit(endX + dx, endY + dy))
                            {
                                criticalMoves.Add((endX + dx, endY + dy));
                            }
                        }
                    }
                }
            }
        }

        // Return the highest priority critical move
        // For four in a row, any blocking move works - return first found
        // For open three, prefer center positions
        if (criticalMoves.Count > 0)
        {
            // Prefer moves closer to center (7, 7)
            var bestMove = criticalMoves.First();
            var bestDistance = Math.Abs(bestMove.x - 7) + Math.Abs(bestMove.y - 7);

            foreach (var move in criticalMoves)
            {
                var distance = Math.Abs(move.x - 7) + Math.Abs(move.y - 7);
                if (distance < bestDistance)
                {
                    bestMove = move;
                    bestDistance = distance;
                }
            }

            return bestMove;
        }

        return null;
    }

    private (int x, int y) SearchWithDepth(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        // Aspiration window: try narrow search first, then wider if needed
        const int aspirationWindow = 50;  // Initial window size
        const int maxAspirationAttempts = 3;  // Max re-searches with wider windows

        var bestScore = int.MinValue;
        var bestMove = candidates[0];

        // Calculate board hash for transposition table
        var boardHash = _transpositionTable.CalculateHash(board);

        // First, do a quick search at depth-1 to get an estimate (if depth > 2)
        int estimatedScore = 0;
        if (depth > 2)
        {
            // Quick search with wide window to get estimate
            var searchAlpha = int.MinValue;
            var searchBeta = int.MaxValue;
            foreach (var (x, y) in candidates)
            {
                board.PlaceStone(x, y, player);
                var score = Minimax(board, depth - 2, searchAlpha, searchBeta, false, player, depth);
                board.GetCell(x, y).Player = Player.None;

                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }

                searchAlpha = Math.Max(searchAlpha, score);
                if (searchBeta <= searchAlpha)
                    break;
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
                    return cachedMove.Value;
            }

            // Reset best score for this attempt
            bestScore = int.MinValue;
            bestMove = candidates[0];

            // Order moves: try killer moves first, then by proximity to center, then cached move
            var orderedMoves = OrderMoves(candidates, depth, board, player, cachedMove);

            var aspirationFailed = false;
            foreach (var (x, y) in orderedMoves)
            {
                // Make move
                board.PlaceStone(x, y, player);

                // Evaluate using minimax
                var score = Minimax(board, depth - 1, alpha, beta, false, player, depth);

                // Undo move
                board.GetCell(x, y).Player = Player.None;

                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
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
            }

            // If aspiration didn't fail, we're done
            if (!aspirationFailed && bestScore > alpha && bestScore < beta)
            {
                // Store result in transposition table
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, estimatedScore - aspirationWindow, estimatedScore + aspirationWindow);
                return bestMove;
            }

            // Aspiration failed - widen window and try again
            alpha = int.MinValue;
            beta = int.MaxValue;

            // On final attempt, just return the best we found
            if (attempt == maxAspirationAttempts - 1)
            {
                // Store result with wide window
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, int.MinValue, int.MaxValue);
                return bestMove;
            }
        }

        return bestMove;
    }

    /// <summary>
    /// Order moves for better alpha-beta pruning
    /// Priority: TT cached move > Killer moves > History heuristic > Tactical patterns > Position heuristics
    /// Zero-allocation implementation using array-based sorting
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove = null)
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

            // Cached move from transposition table gets highest priority
            if (cachedMove.HasValue && x == cachedMove.Value.x && y == cachedMove.Value.y)
            {
                score += 2000;
            }

            // Killer moves get high priority
            for (int k = 0; k < MaxKillerMoves; k++)
            {
                if (_killerMoves[depth, k].x == x && _killerMoves[depth, k].y == y)
                {
                    score += 1000;
                    break;
                }
            }

            // Butterfly heuristic: moves that cause beta cutoffs
            var butterflyScore = player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y];
            score += Math.Min(300, butterflyScore / 100);

            // History heuristic: moves that caused cutoffs get priority
            var historyScore = GetHistoryScore(player, x, y);
            score += Math.Min(500, historyScore / 10);

            // Tactical pattern scoring
            score += EvaluateTacticalPattern(board, x, y, player);

            // Prefer center (7,7)
            var distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
            score += (14 - distanceToCenter) * 10;

            // Prefer moves near existing stones
            var nearby = 0;
            for (int dx = -1; dx <= 1; dx++)
            {
                for (int dy = -1; dy <= 1; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
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
            var count = 1; // Include the placed stone
            var openEnds = 0;

            // Check positive direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

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
                    break; // Blocked by opponent
                }
            }

            // Check negative direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

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
                    break; // Blocked by opponent
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

        // Check blocking value (how much this blocks opponent) - using BitBoard
        foreach (var (dx, dy) in directions)
        {
            var count = 1;
            var openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

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
    /// Clear all AI state between games to prevent cross-contamination
    /// This is critical when AI of different difficulties play in sequence
    /// </summary>
    public void ClearAllState()
    {
        ClearHistory();
        _transpositionTable.Clear();
        ResetPondering();

        // Clear killer moves
        for (int d = 0; d < 20; d++)
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
    }

    /// <summary>
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// </summary>
    private int Quiesce(Board board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
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

        // Search tactical moves
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search (depth stays at 0, but we track via rootDepth)
                var eval = Quiesce(board, alpha, beta, false, aiPlayer, rootDepth + 1);

                board.GetCell(x, y).Player = Player.None;

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
                board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(board, alpha, beta, true, aiPlayer, rootDepth + 1);

                board.GetCell(x, y).Player = Player.None;

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
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check horizontal
                var count = 1;
                for (int dy = 1; dy <= 4 && y + dy < 15; dy++)
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
                for (int dx = 1; dx <= 4 && x + dx < 15; dx++)
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
                for (int i = 1; i <= 4 && x + i < 15 && y + i < 15; i++)
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
                for (int i = 1; i <= 4 && x + i < 15 && y - i >= 0; i++)
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
            for (int x = 0; x < 15; x++)
            {
                for (int y = 0; y < 15; y++)
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
    /// Minimax algorithm with alpha-beta pruning and transposition table
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

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
                board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(board, depth - 2, alpha, beta, false, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
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
                    eval = Minimax(board, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    // If null window search beats alpha, re-search with full window
                    if (eval > alpha && eval < beta)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full beta
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                board.GetCell(x, y).Player = Player.None;

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
                board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(board, depth - 2, alpha, beta, true, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
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
                    eval = Minimax(board, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    // If null window search is below beta, re-search with full window
                    if (eval < beta && eval > alpha)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full alpha
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                board.GetCell(x, y).Player = Player.None;

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
    /// Get candidate moves (empty cells near existing stones)
    /// Zero-allocation implementation using stackalloc for tracking
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        const int boardSize = 15;
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
            candidates.Add((7, 7));
        }

        return candidates;
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
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (board.GetCell(x, y).Player == opponent)
                {
                    return (x, y);
                }
            }
        }

        return null;
    }

    // ========== AGGRESSIVE PRUNING TECHNIQUES FOR D10/D11 ==========

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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: creates 4+ or open 3
            if (count >= 4) return true; // Potential winning move
            if (count == 3)
            {
                // Check if both ends are open
                bool leftOpen = x - dx >= 0 && x - dx < 15 && y - dy >= 0 && y - dy < 15
                               && !occupied.GetBit(x - dx, y - dy);
                bool rightOpen = x + dx * 3 >= 0 && x + dx * 3 < 15 && y + dy * 3 >= 0 && y + dy * 3 < 15
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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count opponent stones in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: blocks opponent's 4 or open 3
            if (oppCount >= 4) return true; // Blocks winning threat
            if (oppCount == 3)
            {
                // Check if this blocks an open three
                var leftOpen = x - dx >= 0 && x - dx < 15 && y - dy >= 0 && y - dy < 15
                              && !occupied.GetBit(x - dx, y - dy);
                var rightOpen = x + dx * 3 >= 0 && x + dx * 3 < 15 && y + dy * 3 >= 0 && y + dy * 3 < 15
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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
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
    /// Stop any active pondering
    /// </summary>
    public void StopPondering()
    {
        _ponderer.StopPondering();
    }

    /// <summary>
    /// Reset pondering state (call when starting a new game)
    /// </summary>
    public void ResetPondering()
    {
        _ponderer.Reset();
        _lastPV = PV.Empty;
        _lastBoard = null;
        _lastPlayer = Player.None;
    }

    /// <summary>
    /// Get pondering statistics
    /// </summary>
    public string GetPonderingStatistics() => _ponderer.GetStatistics();

    /// <summary>
    /// Get search statistics for the last move
    /// </summary>
    public (int DepthAchieved, long NodesSearched, double NodesPerSecond, double TableHitRate, bool PonderingActive, int VCFDepthAchieved, long VCFNodesSearched) GetSearchStatistics()
    {
        double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
        var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
        double nps = elapsedMs > 0 ? (double)_nodesSearched * 1000 / elapsedMs : 0;
        return (_depthAchieved, _nodesSearched, nps, hitRate, _ponderer.IsPondering, _vcfDepthAchieved, _vcfNodesSearched);
    }

    #endregion
}
