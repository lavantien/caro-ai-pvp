using System.Runtime.CompilerServices;
using System.Diagnostics;
using System.Threading.Channels;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.GameLogic.Search;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning and advanced optimizations
/// Optimizations: Transposition Table, Killer Heuristic, History Heuristic, Improved Move Ordering, Iterative Deepening, VCF Solver
/// Parallel Search: Lazy SMP with conservative thread count (processorCount/2)-1
/// Time management: Intelligent time allocation optimized for 7+5 time control
/// Pondering: Constant pondering during opponent's turn
/// Stats: Publisher-subscriber pattern for real-time stats reporting
/// </summary>
public class MinimaxAI : IStatsPublisher
{
    private readonly BoardEvaluator _evaluator = new();
    private TranspositionTable _transpositionTable;
    private readonly WinDetector _winDetector = new();
    private readonly ThreatDetector _threatDetector = new();
    private readonly ThreatSpaceSearch _vcfSolver = new();
    private readonly VCFSolver _inTreeVCFSolver;  // In-tree VCF solver for Lazy SMP

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

    // Parallel search (Lazy SMP provides 4-8x speedup on multi-core systems)
    private ParallelMinimaxSearch _parallelSearch;

    // Search radius around existing stones from centralized config
    private const int MaxSearchRadius = SearchConstants.MaxSearchRadius;

    // Board size constant for array sizing and bounds checking
    private const int BoardSize = GameConstants.BoardSize;

    // Search heuristics: killer moves, history tables, butterfly tables
    private readonly SearchHeuristics _heuristics = new();
    private readonly MoveOrderer _moveOrderer;

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
    private MoveType _moveType;  // How the last move was determined
    private int _lastSearchScore;  // Score from last search
    private double _lastFmcPercent;  // First Move Cutoff % from last search
    private double _lastEbf;  // Effective Branching Factor from last search

    // Time control for search timeout
    private long _searchHardBoundMs;
    // Check time at regular intervals (power of 2 for efficient masking)
    // 16 = check every ~16 nodes. At 1M nodes/sec, this checks every ~16us
    // Frequent checking allows fine-grained timeout control
    private const int TimeCheckInterval = SearchConstants.TimeCheckInterval;
    private bool _searchStopped;

    // Pondering (thinking on opponent's time)
    private readonly Ponderer _ponderer = new();
    private PV _lastPV = PV.Empty;
    private Board? _lastBoard;
    private Player _lastPlayer;

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

    // Optional parameter provider for SPSA tuning (null = use default constants)
    private readonly IEvaluationParameterProvider? _parameterProvider;

    // Mutable SearchBoard for high-performance search (make/unmake pattern)
    // Reused across searches to avoid allocations
    private readonly SearchBoard _searchBoard = new();

    public MinimaxAI(int ttSizeMb = TimeConstants.DefaultHashSizeMb, ILogger<MinimaxAI>? logger = null, Random? random = null, IEvaluationParameterProvider? parameterProvider = null)
    {
        _logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger<MinimaxAI>.Instance;
        _random = random;  // null means use Random.Shared (default behavior)
        _parameterProvider = parameterProvider;  // null = use default evaluation constants
        _publisherId = Interlocked.Increment(ref _instanceCounter).ToString();
        _statsChannel = Channel.CreateUnbounded<MoveStatsEvent>();

        // Initialize with passed size parameter
        _transpositionTable = new TranspositionTable(ttSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(ttSizeMb);

        _inTreeVCFSolver = new VCFSolver(_vcfSolver);
        _moveOrderer = new MoveOrderer(_heuristics);
    }

    /// <summary>
    /// Evaluate board position using custom parameters if provider is set.
    /// Falls back to default evaluation if no parameter provider is configured.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int EvaluateBoard(SearchBoard board, Player player)
    {
        if (_parameterProvider != null)
        {
            return _evaluator.Evaluate(board, player, _parameterProvider.GetParameters());
        }
        return _evaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Evaluate immutable Board position using custom parameters if provider is set.
    /// Falls back to default evaluation if no parameter provider is configured.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int EvaluateBoard(Board board, Player player)
    {
        if (_parameterProvider != null)
        {
            return _evaluator.Evaluate(board, player, _parameterProvider.GetParameters());
        }
        return _evaluator.Evaluate(board, player);
    }

    // Helper methods for random operations (uses injected Random or Random.Shared)
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private double NextRandomDouble() => _random?.NextDouble() ?? Random.Shared.NextDouble();

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int NextRandomInt(int maxValue) => _random?.Next(maxValue) ?? Random.Shared.Next(maxValue);


    /// <summary>
    /// Get the best move for the AI player with time awareness
    /// Dynamically adjusts search depth based on remaining time
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, long? timeRemainingMs, bool ponderingEnabled = true, bool parallelSearchEnabled = true)
    {
        return GetBestMove(board, player, timeRemainingMs, moveNumber: 0, ponderingEnabled: ponderingEnabled, parallelSearchEnabled: parallelSearchEnabled);
    }

    /// <summary>
    /// Get the best move for the AI player with full time awareness
    /// Dynamically adjusts search depth based on remaining time and game phase
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player to move</param>
    /// <param name="timeRemainingMs">Time remaining on clock in milliseconds (null for unlimited)</param>
    /// <param name="moveNumber">Current move number (1-indexed, 0 if unknown)</param>
    /// <param name="ponderingEnabled">Enable pondering (thinking on opponent's time)</param>
    /// <param name="parallelSearchEnabled">Enable Lazy SMP parallel search</param>
    /// <param name="incrementSeconds">Explicit increment in seconds (null to estimate from time remaining)</param>
    /// <param name="threadCount">Explicit thread count override (null to use default)</param>
    /// <param name="maxDepth">Maximum search depth (null for no depth limit)</param>
    /// <param name="maxNodes">Maximum nodes to search (null for no node limit)</param>
    /// <param name="maxTimeMs">Maximum time in ms (null to use time management)</param>
    /// <returns>Best move coordinates</returns>
    public (int x, int y) GetBestMove(Board board, Player player, long? timeRemainingMs, int moveNumber, bool ponderingEnabled = true, bool parallelSearchEnabled = true, int? incrementSeconds = null, int? threadCount = null, int? maxDepth = null, long? maxNodes = null, int? maxTimeMs = null)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

        // Initialize search statistics BEFORE any early returns
        // This ensures stats are clean even for instant moves (error rate, critical defense, VCF, etc.)
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _moveType = MoveType.Normal;  // Default, will be overridden by early exits
        _searchStopwatch.Restart();

        // Reset thread count and parallel diagnostics
        _lastThreadCount = threadCount ?? Math.Max(SHC.MinThreadCount, (Environment.ProcessorCount / 2) - 1);
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
                    return dx >= SHC.OpenRuleDistance || dy >= SHC.OpenRuleDistance;
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
                                if (dx >= SHC.OpenRuleDistance || dy >= SHC.OpenRuleDistance)
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
            var lastOppMove = QuickWinChecker.GetLastOpponentMove(board, player);
            if (lastOppMove.HasValue)
            {
                // Check if opponent played the predicted move
                var (ponderState, _) = _ponderer.HandleOpponentMove(lastOppMove.Value.x, lastOppMove.Value.y);

                if (ponderState == PonderState.PonderHit)
                {
                    // PONDER HIT - opponent played expected move!
                    // The ponder search was running during opponent's turn (free precomputation).
                    // CRITICAL FIX: Still check for immediate wins and threats before using ponder result.
                    // The ponder search might not have prioritized tactical moves correctly.

                    // First, check if we have an immediate winning move
                    // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
                    foreach (var (cx, cy) in candidates)
                    {
                        if (_threatDetector.IsWinningMove(board, cx, cy, player))
                        {
                            _ponderer.StopPondering();
                            _depthAchieved = 1;
                            _nodesSearched = 1;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateWin;
                            return (cx, cy);
                        }
                    }

                    // Second, check if opponent has an immediate winning threat we must block
                    // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
                    var ponderOppPlayer = player == Player.Red ? Player.Blue : Player.Red;
                    var ponderOpponentWinningSquares = new List<(int x, int y)>();
                    for (int x = 0; x < BoardSize; x++)
                    {
                        for (int y = 0; y < BoardSize; y++)
                        {
                            if (board.GetCell(x, y).Player == Player.None)
                            {
                                if (_threatDetector.IsWinningMove(board, x, y, ponderOppPlayer))
                                {
                                    ponderOpponentWinningSquares.Add((x, y));
                                }
                            }
                        }
                    }

                    // If there are immediate threats, must block - don't use ponder result
                    if (ponderOpponentWinningSquares.Count > 0)
                    {
                        // Fall through to normal blocking logic
                        // Don't use ponder result when immediate blocking is needed
                    }
                    else
                    {
                        // No immediate threats - safe to use ponder result
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
                                //Console.WriteLine($"[PONDER HIT] Used ponder result: depth={ponderResult.Depth}, nodes={ponderResult.NodesSearched:N0}, time={ponderResult.TimeSpentMs}ms");
                                return ponderMove;
                            }
                        }
                    }
                }
                // Ponder miss - fall through to normal search
                // The ponder search was already stopped by HandleOpponentMove on miss
            }
        }

        // CRITICAL OPTIMIZATION: Check for immediate winning moves BEFORE any expensive operations
        // This ensures we never waste time searching when a win is available in one move
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
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
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
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

        // CRITICAL: Also check for opponent's OPEN FOURS (StraightFour)
        // An open four is 4-in-a-row with an open end - opponent wins next turn if not blocked
        // This is NOT caught by IsWinningMove since it's not yet 5-in-a-row
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
        if (opponentWinningSquares.Count == 0)
        {
            var opponentThreats = _threatDetector.DetectThreats(board, oppPlayer);
            foreach (var threat in opponentThreats)
            {
                if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                {
                    // Add all gain squares (the squares that complete the 5-in-a-row)
                    foreach (var gain in threat.GainSquares)
                    {
                        if (board.GetCell(gain.x, gain.y).IsEmpty && !opponentWinningSquares.Contains(gain))
                        {
                            opponentWinningSquares.Add(gain);
                        }
                    }
                }
            }
        }

        // If opponent has immediate winning moves, we must respond
        // DESIGN PRINCIPLE: Per ENGINE_FEATURES.md, threat blocks are added to candidate list,
        // not returned immediately. Search evaluates offensive vs defensive options together.
        // The only early returns are for VERIFIED immediate wins.
        if (opponentWinningSquares.Count > 0)
        {
            // First check if we have our own winning move - always best to win immediately
            foreach (var (cx, cy) in candidates)
            {
                if (_threatDetector.IsWinningMove(board, cx, cy, player))
                {
                    _depthAchieved = 1;
                    _nodesSearched = opponentWinningSquares.Count + 1;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ImmediateWin;
                    return (cx, cy);
                }
            }

            // Add all blocking squares to candidates with highest priority
            // Search will evaluate which is best (considering our own threats, position, etc.)
            foreach (var block in opponentWinningSquares)
            {
                if (!candidates.Contains(block))
                {
                    candidates.Insert(0, block);
                }
            }

            // Filter to ONLY blocking moves - when opponent has winning threats, we MUST block
            // The search will find the best blocking move
            candidates = candidates.Where(c => opponentWinningSquares.Contains(c)).ToList();
            _logger.LogDebug("[AI DEFENSE] Filtering to {Count} blocking move(s) for search evaluation",
                candidates.Count);
            // Fall through to normal search with filtered candidates
        }

        // PROACTIVE DEFENSE: Check for opponent's open threes (3 in a row with both ends open)
        // An open three becomes an open four on the next move, which has 2 winning squares.
        // We should block open threes BEFORE they become open fours.
        // This is critical for Caro rules where sandwiched wins are blocked.
        // CHANGED: Don't immediately block - instead add to blocking candidates for search evaluation
        // This allows the AI to consider whether its own threats might be more urgent
        var openThreeBlocks = QuickWinChecker.FindOpenThreeBlocks(board, oppPlayer);

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

        // Calculate time allocation for chess-clock time control
        // Infer initial time and increment from the remaining time
        // This works for any time control: 3+2, 7+5, 15+10, etc.
        TimeAllocation timeAlloc;
        // Handle maxTimeMs (from "go movetime N") - use as absolute time budget
        if (maxTimeMs.HasValue)
        {
            timeRemainingMs = maxTimeMs.Value;
            timeAlloc = new TimeAllocation
            {
                SoftBoundMs = (long)(maxTimeMs.Value * SHC.SoftBoundRatio),
                HardBoundMs = maxTimeMs.Value,
                OptimalTimeMs = (long)(maxTimeMs.Value * SHC.OptimalBoundRatio)
            };
        }
        // CRITICAL FIX: For long time budgets, use direct time allocation without AdaptiveTimeManager
        // The adaptive manager under-allocates for long time budgets
        else if (timeRemainingMs.HasValue)
        {
            // Infer initial time from first few moves
            var inferredInitialMs = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : timeRemainingMs.Value;
            var initialTimeSeconds = (int)(inferredInitialMs / 1000);

            // Use explicit increment if provided, otherwise estimate
            int effectiveIncrementSeconds;
            if (incrementSeconds.HasValue)
            {
                effectiveIncrementSeconds = incrementSeconds.Value;
            }
            else if (initialTimeSeconds <= 120)
            {
                // Short time control - assume sudden death (no increment)
                effectiveIncrementSeconds = 0;
            }
            else
            {
                // Longer time controls - estimate increment based on common ratios
                effectiveIncrementSeconds = Math.Max(SHC.MinEffectiveIncrementSeconds, (int)Math.Round(initialTimeSeconds / SHC.InitialTimeToIncrementDivisor));
            }

            // Use AdaptiveTimeManager with PID-like controller for better time management
            // Automatically adjusts to any time control without hardcoded multipliers
            timeAlloc = _adaptiveTimeManager.CalculateMoveTime(
                timeRemainingMs.Value,
                moveNumber,
                candidates.Count,
                board,
                player,
                initialTimeSeconds,
                effectiveIncrementSeconds
            );
        }
        else
        {
            timeAlloc = TimeBudgetCalculator.GetDefaultTimeAllocation();
        }

        bool hasOpponentThreats = false;
        bool hasImmediateThreats = false;  // Only StraightFour and BrokenFour - require immediate response
        bool hasOpenFour = false;
        List<(int x, int y)> blockingSquares = new();
        List<(int x, int y)> priorityBlockingSquares = new();

        {
            // CRITICAL DEFENSE: Check for opponent threats BEFORE any early returns
            // This ensures we don't skip blocking in emergency mode
            // Note: oppPlayer is already defined above
            // CRITICAL FIX: Include BrokenThree threats - they become BrokenFour in one move!
            var threats = _threatDetector.DetectThreats(board, oppPlayer)
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenFour || t.Type == ThreatType.BrokenThree)
                .ToList();

            hasOpponentThreats = threats.Count > 0;

            // CRITICAL FIX: Only filter candidates for IMMEDIATE threats (StraightFour, BrokenFour)
            // StraightThree and BrokenThree are developing threats that don't require immediate response
            // The evaluation function will handle them through normal search
            hasImmediateThreats = threats.Any(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

            if (hasOpponentThreats)
            {
                var straightFourCount = threats.Count(t => t.Type == ThreatType.StraightFour);
                var straightThreeCount = threats.Count(t => t.Type == ThreatType.StraightThree);
                var brokenFourCount = threats.Count(t => t.Type == ThreatType.BrokenFour);
                var brokenThreeCount = threats.Count(t => t.Type == ThreatType.BrokenThree);

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

                // CRITICAL FIX: StraightThree and BrokenThree should be blocked, BUT only if we don't have
                // our own winning threats. If we can win immediately, that's better than blocking.
                // CRITICAL: A StraightThree becomes a StraightFour in ONE move, NOT two moves!
                // We must block three-threats BEFORE they become unstoppable open fours.
                // A BrokenThree becomes a StraightFour in 1 move if the gap is filled!
                // FIX: Handle three-threats even when there ARE four-threats, because:
                // 1. Three-threats in different directions can become additional four-threats
                // 2. Blocking a three-threat gain square might also block a four-threat
                // 3. We need to find the BEST block that addresses ALL threats
                if ((straightThreeCount > 0 || brokenThreeCount > 0))
                {
                    // First check if we have our own winning threats
                    var ourThreats = _threatDetector.DetectThreats(board, player);
                    var ourStraightFours = ourThreats.Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                    // If we have a winning threat (open four), play it instead of blocking
                    // CRITICAL: Only counter-attack if we have a GUARANTEED win (StraightFour/BrokenFour)
                    // OR if we have multiple StraightThrees (double threat - opponent can't block both!)
                    if (ourStraightFours.Count > 0)
                    {
                        // We have an open four - find and verify our winning move
                        foreach (var threat in ourStraightFours)
                        {
                            foreach (var gs in threat.GainSquares)
                            {
                                if (board.GetCell(gs.x, gs.y).IsEmpty && _threatDetector.IsWinningMove(board, gs.x, gs.y, player))
                                {
                                    _depthAchieved = 1;
                                    _nodesSearched = 1;
                                    _lastAllocatedTimeMs = 0;
                                    _moveType = MoveType.ImmediateWin;
                                    _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK with verified winning move at ({WX},{WY}) instead of blocking",
                                        player, gs.x, gs.y);
                                    return gs;
                                }
                            }
                        }
                    }

                    // DESIGN PRINCIPLE: Per ENGINE_FEATURES.md, threat blocks are added to candidate list,
                    // not returned immediately. Search evaluates offensive vs defensive options together.
                    // This maintains strategic initiative instead of reactive blocking.

                    // CRITICAL: Collect ALL gain squares from both three-threats AND four-threats
                    // When both exist, we need to find a block that addresses ALL threats
                    var threeThreats = threats
                        .Where(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree)
                        .ToList();

                    var fourThreats = threats
                        .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                        .ToList();

                    var allGainSquares = threats  // Include ALL threats, not just three-threats
                        .SelectMany(t => t.GainSquares)
                        .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                        .Distinct()
                        .ToList();

                    if (allGainSquares.Count > 0)
                    {
                        // Immediately block three-threats
                        // A StraightThree becomes an open four in ONE move. We must block NOW.
                        // Returning immediately bypasses search, guaranteeing the block.
                        {
                            // CRITICAL: Check if opponent has multiple independent three-threats
                            // This is a "double threat" situation - blocking one leaves the other
                            // which becomes a four-threat next turn. We MUST counter-attack.
                            var distinctThreeThreats = threeThreats
                                .Where(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree)
                                .GroupBy(t => t.Direction)  // Group by direction to find parallel threats
                                .Count(g => g.Any());

                            bool hasMultipleIndependentThreats = threeThreats.Count >= 2 &&
                                threeThreats.SelectMany(t => t.GainSquares).Distinct().Count() >= 3;

                            // If opponent has 2+ independent threats, blocking is futile
                            // We must create our own winning threat to counter
                            if (hasMultipleIndependentThreats)
                            {
                                _logger.LogDebug("[AI DEFENSE] ({Player}) CRITICAL: Opponent has {Count} independent three-threats - blocking is futile, must counter-attack!",
                                    player, threeThreats.Count);

                                // Try to find a move that creates our own winning threat
                                for (int x = 0; x < BoardSize; x++)
                                {
                                    for (int y = 0; y < BoardSize; y++)
                                    {
                                        if (!board.GetCell(x, y).IsEmpty) continue;

                                        var testBoard = board.PlaceStone(x, y, player);
                                        var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                                        var ourNewFourThreats = ourNewThreats.Where(t =>
                                            t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                                        // If we can create a four-threat (open four), that forces opponent to block
                                        // This changes the dynamic - they have to respond to us
                                        if (ourNewFourThreats.Count > 0)
                                        {
                                            // Check if this also blocks one of their threats (bonus!)
                                            bool alsoBlocks = threeThreats.Any(t => t.GainSquares.Contains((x, y)));

                                            _depthAchieved = 1;
                                            _nodesSearched = (x + 1) * BoardSize + y + 1;
                                            _lastAllocatedTimeMs = 0;
                                            _moveType = alsoBlocks ? MoveType.ImmediateBlock : MoveType.CounterAttack;
                                            _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK at ({X},{Y}) creates {Count} four-threat(s){AlsoBlocks}",
                                                player, x, y, ourNewFourThreats.Count, alsoBlocks ? " and blocks!" : "");
                                            return ValidateAndReturnBlockingMove(board, player, (x, y));
                                        }
                                    }
                                }

                                // No counter-attack available - fall through to best blocking strategy
                                _logger.LogDebug("[AI DEFENSE] ({Player}) No counter-attack found - must block best threat",
                                    player);
                            }

                            // Find the best blocking square - prioritize eliminating immediate threats
                            var bestBlock = allGainSquares.First();
                            int bestScore = int.MinValue;

                            foreach (var block in allGainSquares)
                            {
                                var testBoard = board.PlaceStone(block.x, block.y, player);
                                var ourThreatsAfter = _threatDetector.DetectThreats(testBoard, player);
                                var theirThreatsAfter = _threatDetector.DetectThreats(testBoard, oppPlayer);

                                // Count threats by type for weighted scoring
                                int theirFourThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);
                                int theirThreeThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree);
                                int ourFourThreats = ourThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

                                // CRITICAL: Check for immediate winning squares after this block
                                int theirWinningSquares = 0;
                                for (int wx = 0; wx < BoardSize; wx++)
                                {
                                    for (int wy = 0; wy < BoardSize; wy++)
                                    {
                                        if (testBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                                            theirWinningSquares++;
                                    }
                                }

                                // Score: heavily penalize blocks that leave immediate threats
                                // -10000 per winning square (CRITICAL - must block these!)
                                // -5000 per four-threat (URGENT - becomes winning next move)
                                // -500 per three-threat (important but not immediate)
                                // +8000 per our four-threat (STRONG COUNTER-ATTACK - forces opponent to respond!)
                                // -2000 BONUS penalty for multiple three-threats (can lead to double threat)
                                // CRITICAL: Counter-attacking is often better than just blocking!
                                int multipleThreePenalty = theirThreeThreats >= SHC.MultipleThreeThreshold ? -SHC.MultipleThreePenalty : 0;
                                int score = -theirWinningSquares * SHC.WinningSquarePenalty - theirFourThreats * SHC.FourThreatPenalty - theirThreeThreats * SHC.ThreeThreatPenalty + ourFourThreats * SHC.FourThreatBonus + multipleThreePenalty;

                                // Prefer central blocks as tiebreaker
                                int distToCenter = Math.Abs(block.x - SHC.CenterIndex) + Math.Abs(block.y - SHC.CenterIndex);
                                score -= distToCenter;

                                if (score > bestScore)
                                {
                                    bestScore = score;
                                    bestBlock = block;
                                }
                            }

                            // CRITICAL: If the best block still leaves us in a losing position,
                            // try counter-attacking instead - creating our own winning threat
                            // This is especially important when opponent has multiple developing threats
                            if (bestScore < -SHC.FourThreatPenalty) // Very negative = opponent still has winning squares
                            {
                                _logger.LogDebug("[AI DEFENSE] ({Player}) Best block score is {Score} - trying counter-attack instead",
                                    player, bestScore);

                                for (int x = 0; x < BoardSize; x++)
                                {
                                    for (int y = 0; y < BoardSize; y++)
                                    {
                                        if (!board.GetCell(x, y).IsEmpty) continue;

                                        var testBoard = board.PlaceStone(x, y, player);
                                        var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                                        var ourNewFourThreats = ourNewThreats.Where(t =>
                                            t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                                        // If we can create a four-threat that is also a winning move, take it!
                                        if (ourNewFourThreats.Count > 0)
                                        {
                                            foreach (var threat in ourNewFourThreats)
                                            {
                                                foreach (var gs in threat.GainSquares)
                                                {
                                                    if (testBoard.GetCell(gs.x, gs.y).IsEmpty &&
                                                        _threatDetector.IsWinningMove(testBoard, gs.x, gs.y, player))
                                                    {
                                                        // This counter-attack creates a verified winning position!
                                                        _depthAchieved = 1;
                                                        _nodesSearched = (x + 1) * BoardSize + y + 1;
                                                        _lastAllocatedTimeMs = 0;
                                                        _moveType = MoveType.CounterAttack;
                                                        _logger.LogDebug("[AI DEFENSE] ({Player}) DESPERATE COUNTER-ATTACK at ({X},{Y}) creates verified winning threat",
                                                            player, x, y);
                                                        return ValidateAndReturnBlockingMove(board, player, (x, y));
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                            }

                            _depthAchieved = 1;
                            _nodesSearched = allGainSquares.Count;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateBlock;
                            _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE three-threat block at ({BX},{BY}) - {Count} gain squares available (score: {Score})",
                                player, bestBlock.x, bestBlock.y, allGainSquares.Count, bestScore);
                            return ValidateAndReturnBlockingMove(board, player, bestBlock);
                        }
                    }
                }

                _logger.LogDebug("[AI DEFENSE] ({Player}) Opponent has {StraightFourCount} StraightFour, {StraightThreeCount} StraightThree, {BrokenFourCount} BrokenFour, {BrokenThreeCount} BrokenThree threat(s), blocking squares: {BlockingSquares}{OpenFourSuffix}",
                    player, straightFourCount, straightThreeCount, brokenFourCount, brokenThreeCount,
                    string.Join(", ", blockingSquares.Select(g => $"({g.x},{g.y})")),
                    hasOpenFour ? " [CRITICAL THREAT DETECTED]" : "");
            }
        }

        // Emergency mode - use TT move if available
        // BUT: If opponent has threats, blocking takes priority
        if (timeAlloc.IsEmergency && !hasOpponentThreats)
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

        // PROACTIVE ATTACK: When no opponent threats, create our own threats
        // This is critical for winning - we must attack, not just defend
        if (!hasOpponentThreats)
        {
            var ourThreats = _threatDetector.DetectThreats(board, player);

            // Priority 1: Create StraightFour/BrokenFour (immediate win)
            var ourFourThreats = ourThreats
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                .ToList();

            if (ourFourThreats.Count > 0)
            {
                // CRITICAL FIX: Verify the gain square actually wins using IsWinningMove
                // A StraightFour/BrokenFour threat means we have 4 stones, but we must verify
                // the gain square completes 5 in a row (not blocked, no overline, etc.)
                foreach (var threat in ourFourThreats)
                {
                    foreach (var gs in threat.GainSquares)
                    {
                        if (board.GetCell(gs.x, gs.y).IsEmpty && _threatDetector.IsWinningMove(board, gs.x, gs.y, player))
                        {
                            _depthAchieved = 1;
                            _nodesSearched = ourFourThreats.Count;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateWin;
                            _logger.LogDebug("[AI ATTACK] ({Player}) Playing verified winning move at ({WX},{WY})",
                                player, gs.x, gs.y);
                            return gs;
                        }
                    }
                }
            }

            // Priority 2: Extend existing StraightThree to create open four threat
            // SAFEGUARD: Validate move doesn't miss opponent winning squares
            var ourStraightThrees = ourThreats
                .Where(t => t.Type == ThreatType.StraightThree)
                .ToList();

            if (ourStraightThrees.Count > 0)
            {
                // Find the best StraightThree to extend (most open ends)
                var bestThree = ourStraightThrees
                    .OrderByDescending(t => t.GainSquares.Count(gs => board.GetCell(gs.x, gs.y).IsEmpty))
                    .First();

                var extendSquare = bestThree.GainSquares
                    .FirstOrDefault(gs => board.GetCell(gs.x, gs.y).IsEmpty);

                if (extendSquare != default)
                {
                    _depthAchieved = 1;
                    _nodesSearched = ourStraightThrees.Count;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ThreatCreation;
                    _logger.LogDebug("[AI ATTACK] ({Player}) Extending StraightThree at ({EX},{EY}) to create open four",
                        player, extendSquare.x, extendSquare.y);
                    return ValidateAndReturnBlockingMove(board, player, extendSquare);
                }
            }

            // Priority 3: Create new StraightThree by finding moves that create threats
            foreach (var candidate in candidates.Take(20))
            {
                if (!board.GetCell(candidate.x, candidate.y).IsEmpty)
                    continue;

                var testBoard = board.PlaceStone(candidate.x, candidate.y, player);
                var newThreats = _threatDetector.DetectThreats(testBoard, player);

                // Prioritize moves that create StraightThree
                if (newThreats.Any(t => t.Type == ThreatType.StraightThree))
                {
                    _depthAchieved = 1;
                    _nodesSearched = 20;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ThreatCreation;
                    _logger.LogDebug("[AI ATTACK] ({Player}) Creating StraightThree at ({TX},{TY})",
                        player, candidate.x, candidate.y);
                    return ValidateAndReturnBlockingMove(board, player, candidate);
                }
            }
        }

        // CRITICAL DEFENSE: Filter candidates to blocking/winning moves when opponent has IMMEDIATE threats
        // IMMEDIATE threats: StraightFour, BrokenFour (must be blocked now or lose)
        // DEVELOPING threats: StraightThree (can wait, evaluation will handle it)
        // Store original candidates in case filtering produces empty list
        var originalCandidates = candidates.ToList();
        if (hasOpponentThreats && hasImmediateThreats)
        {
            // CRITICAL FIX: For open fours, reserve minimum time to respond properly
            // An open four is a game-ending threat that requires proper calculation
            if (hasOpenFour)
            {
                const long minCriticalResponseTimeMs = 3000;  // Minimum 3 seconds for critical responses

                if (timeAlloc.SoftBoundMs < minCriticalResponseTimeMs)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) CRITICAL: Open four detected - reserving minimum time ({MinCriticalResponseTimeMs}ms)",
                        player, minCriticalResponseTimeMs);
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

            // FIX: Include our winning moves AND developing threats in candidate list
            // When opponent has threats, we should consider:
            // 1. Blocking their threats (blocking squares)
            // 2. Our immediate wins (StraightFour, BrokenFour)
            // 3. Our developing threats (StraightThree) - these can become winning threats
            var ourThreats = _threatDetector.DetectThreats(board, player);

            // Immediate winning squares
            var ourWinningSquares = ourThreats
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                .SelectMany(t => t.GainSquares)
                .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                .ToList();

            // Developing threat squares (StraightThree) - build our own threats
            var ourDevelopingSquares = ourThreats
                .Where(t => t.Type == ThreatType.StraightThree)
                .SelectMany(t => t.GainSquares)
                .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                .ToList();

            var blockingSet = new HashSet<(int x, int y)>(blockingSquares);
            var winningSet = new HashSet<(int x, int y)>(ourWinningSquares);
            var developingSet = new HashSet<(int x, int y)>(ourDevelopingSquares);

            // Include blocking squares, winning moves, AND developing moves
            // FIX: Only filter candidates when there are IMMEDIATE threats (StraightFour, BrokenFour)
            // For developing threats (StraightThree only), skip filtering and let search decide
            var filteredCandidates = candidates
                .Where(c => blockingSet.Contains(c) || winningSet.Contains(c) || developingSet.Contains(c))
                .ToList();

            if (filteredCandidates.Count > 0)
            {
                // Prioritize: winning > blocking > developing
                filteredCandidates = filteredCandidates
                    .OrderByDescending(c => winningSet.Contains(c) ? 2 : (blockingSet.Contains(c) ? 1 : 0))
                    .ToList();
                candidates = filteredCandidates;
                _logger.LogDebug("[AI DEFENSE] ({Player}) Filtered to {CandidateCount} move(s) ({WinningCount} winning, {BlockingCount} blocking, {DevelopingCount} developing)",
                    player, candidates.Count, winningSet.Count, blockingSet.Count, developingSet.Count);
            }
            else
            {
                // Fallback: use blocking, winning, and developing squares directly as candidates
                candidates = blockingSquares.Concat(ourWinningSquares).Concat(ourDevelopingSquares).Distinct().ToList();
                _logger.LogDebug("[AI DEFENSE] ({Player}) Using blocking/winning/developing squares directly as candidates",
                    player);
            }

            // CRITICAL FIX FOR GRANDMASTER: Immediately return best blocking move for four-threats
            // This bypasses search to guarantee we block correctly
            if (candidates.Count > 0)
            {
                // First check if we have an immediate winning move
                foreach (var winSquare in ourWinningSquares)
                {
                    if (_threatDetector.IsWinningMove(board, winSquare.x, winSquare.y, player))
                    {
                        _depthAchieved = 1;
                        _nodesSearched = 1;
                        _lastAllocatedTimeMs = 0;
                        _moveType = MoveType.ImmediateWin;
                        _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK with verified winning move at ({WX},{WY})",
                            player, winSquare.x, winSquare.y);
                        return winSquare;
                    }
                }

                // Find the best blocking square using the same scoring as three-threat blocking
                var bestBlock = candidates.First();
                int bestScore = int.MinValue;

                foreach (var block in candidates)
                {
                    if (!board.GetCell(block.x, block.y).IsEmpty)
                        continue;

                    var testBoard = board.PlaceStone(block.x, block.y, player);
                    var ourThreatsAfter = _threatDetector.DetectThreats(testBoard, player);
                    var theirThreatsAfter = _threatDetector.DetectThreats(testBoard, oppPlayer);

                    // Count threats by type for weighted scoring
                    int theirFourThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);
                    int theirThreeThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree);
                    int ourFourThreats = ourThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

                    // CRITICAL: Check for immediate winning squares after this block
                    int theirWinningSquares = 0;
                    for (int wx = 0; wx < BoardSize; wx++)
                    {
                        for (int wy = 0; wy < BoardSize; wy++)
                        {
                            if (testBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                                theirWinningSquares++;
                        }
                    }

                    // Score: heavily penalize blocks that leave immediate threats
                    // Counter-attack is valuable even when blocking four-threats!
                    // +8000 for four-threat counter-attack
                    int score = -theirWinningSquares * SHC.WinningSquarePenalty - theirFourThreats * SHC.FourThreatPenalty - theirThreeThreats * SHC.ThreeThreatPenalty + ourFourThreats * SHC.FourThreatBonus;

                    // Prefer central blocks as tiebreaker
                    int distToCenter = Math.Abs(block.x - SHC.CenterIndex) + Math.Abs(block.y - SHC.CenterIndex);
                    score -= distToCenter;

                    if (score > bestScore)
                    {
                        bestScore = score;
                        bestBlock = block;
                    }
                }

                // Only return immediately if the best block leaves no winning squares
                if (bestScore >= -4000)  // No winning squares left (-10000 each)
                {
                    _depthAchieved = 1;
                    _nodesSearched = candidates.Count;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ImmediateBlock;
                    _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE four-threat block at ({BX},{BY}) - score {Score}",
                        player, bestBlock.x, bestBlock.y, bestScore);
                    return ValidateAndReturnBlockingMove(board, player, bestBlock);
                }

                // CRITICAL: If the best block still leaves us in a losing position,
                // try counter-attacking instead - creating our own winning threat
                if (bestScore < -SHC.FourThreatPenalty)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) Four-threat best block score is {Score} - trying counter-attack",
                        player, bestScore);

                    for (int x = 0; x < BoardSize; x++)
                    {
                        for (int y = 0; y < BoardSize; y++)
                        {
                            if (!board.GetCell(x, y).IsEmpty) continue;

                            var testBoard = board.PlaceStone(x, y, player);
                            var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                            var ourNewFourThreats = ourNewThreats.Where(t =>
                                t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                            if (ourNewFourThreats.Count > 0)
                            {
                                foreach (var threat in ourNewFourThreats)
                                {
                                    foreach (var gs in threat.GainSquares)
                                    {
                                        if (testBoard.GetCell(gs.x, gs.y).IsEmpty &&
                                            _threatDetector.IsWinningMove(testBoard, gs.x, gs.y, player))
                                        {
                                            _depthAchieved = 1;
                                            _nodesSearched = (x + 1) * BoardSize + y + 1;
                                            _lastAllocatedTimeMs = 0;
                                            _moveType = MoveType.CounterAttack;
                                            _logger.LogDebug("[AI DEFENSE] ({Player}) DESPERATE COUNTER-ATTACK at ({X},{Y}) creates verified winning threat",
                                                player, x, y);
                                            return ValidateAndReturnBlockingMove(board, player, (x, y));
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // CRITICAL FIX: If threat filtering produced empty candidates, restore original candidates
            // This can happen when threat detection finds threats but gain squares are already occupied
            // or when our threat detection finds no counter-threats
            if (candidates.Count == 0)
            {
                _logger.LogDebug("[AI DEFENSE] ({Player}) Threat filtering produced empty candidates - restoring original {OriginalCount} candidates",
                    player, originalCandidates.Count);
                candidates = originalCandidates.ToList();
            }

            // CRITICAL FIX: For open fours (StraightFour with 2+ blocking squares),
            // we're in a lost position if we can't win immediately. Log this for debugging.
            if (hasOpenFour)
            {
                _logger.LogDebug("[AI DEFENSE] ({Player}) WARNING: Open four detected - opponent can win in 2 moves",
                    player);

                // Check if we have counter-threats
                var counterThreats = _threatDetector.DetectThreats(board, player);
                var ourStraightFours = counterThreats.Count(t => t.Type == ThreatType.StraightFour);
                var ourStraightThrees = counterThreats.Count(t => t.Type == ThreatType.StraightThree);

                if (ourStraightFours > 0)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) We have {OurStraightFours} StraightFour threat(s) - counter-attack instead of just blocking",
                        player, ourStraightFours);
                }
                else if (ourStraightThrees > 1)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) We have {OurStraightThrees} StraightThree threat(s) - creating counter-play",
                        player, ourStraightThrees);
                }
            }
        }
        // VCF Defense was causing Grandmaster+ to play too reactively, blocking opponent threats
        // instead of developing its own position. The evaluation function's defense
        // multiplier (2.2x for opponent threats) should be sufficient for defense.
        // Grandmaster's advantage comes from offensive VCF, not defensive VCF detection.

        // Try VCF (Victory by Continuous Four) search
        // VCF finds forced win sequences through continuous four threats.
        // VCF always enabled
        {
            var (vcfTimeLimit, vcfMaxDepth) = TimeBudgetCalculator.CalculateVCFTimeLimit(timeAlloc);

            // VCF-FIRST MODE: In emergency, use up to 80% of hard bound for VCF
            // CRITICAL: Even in emergency, VCF time scales with difficulty!
            // This prevents emergency mode from making all AIs equal
            if (timeAlloc.IsEmergency)
            {
                vcfTimeLimit = (int)Math.Min(timeAlloc.HardBoundMs * SHC.SoftBoundRatio, SHC.EmergencyVcfCapMs);
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
            // CRITICAL: Don't skip blocking even in emergency mode - but only for IMMEDIATE threats
            if (timeAlloc.IsEmergency)
            {
                // If opponent has IMMEDIATE threats (StraightFour, BrokenFour), MUST block
                // For developing threats (StraightThree), let search decide
                if (hasImmediateThreats && blockingSquares.Count > 0)
                {
                    // Return a blocking square immediately
                    _depthAchieved = 1;
                    _nodesSearched = 1;
                    return blockingSquares[0];
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

        // NPS is learned from actual search performance - no hardcoded targets

        // PARALLEL SEARCH: Use Lazy SMP when enabled
        if (parallelSearchEnabled)
        {
            int effectiveThreadCount = threadCount ?? Math.Max(SHC.MinThreadCount, (Environment.ProcessorCount / 2) - 1);
            _lastThreadCount = effectiveThreadCount;
            _tableHits = 0;
            _tableLookups = 0;

            var adjustedTimeAlloc = timeAlloc;

            var parallelResult = _parallelSearch.GetBestMoveWithStats(
                board,
                player,
                timeRemainingMs: timeRemainingMs,
                timeAlloc: adjustedTimeAlloc,
                moveNumber: moveNumber,
                fixedThreadCount: effectiveThreadCount,
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
            _lastFmcPercent = parallelResult.FirstMoveCutoffPercent;
            _lastEbf = parallelResult.EffectiveBranchingFactor;

            // Store PV and board for pondering prediction
            _lastPV = PV.FromSingleMove(parallelResult.X, parallelResult.Y, _depthAchieved, 0);
            _lastBoard = board;

            // Start pondering for opponent's response
            if (ponderingEnabled)
            {
                var opponent = player == Player.Red ? Player.Blue : Player.Red;
                var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
                var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(timeRemainingMs);

                if (ponderTimeMs > 0)
                {
                    _ponderer.StartPondering(
                        board,
                        opponent,
                        predictedOpponentMove,
                        player,
                        ponderTimeMs
                    );
                }
            }

            //Console.WriteLine($"[AI PARALLEL] Move: ({parallelResult.X}, {parallelResult.Y}), Depth: {_depthAchieved}, Nodes: {_nodesSearched:N0}, Threads: {parallelResult.ThreadCount}");

            // CRITICAL SAFEGUARD for parallel search path
            return ValidateAndReturnBlockingMove(board, player, (parallelResult.X, parallelResult.Y));
        }

        // TIME-BUDGET-BASED SEARCH: No hardcoded depths, scales with machine capability
        // Faster machines reach deeper depths naturally, slower machines stop earlier

        // Track thread count for diagnostics (even if using sequential search)
        _lastThreadCount = Math.Max(SHC.MinThreadCount, (Environment.ProcessorCount / 2) - 1);
        _lastParallelDiagnostics = null;
        _lastPonderingEnabled = ponderingEnabled;

        // NPS is learned from actual search performance - no hardcoded targets

        long adjustedSoftBoundMs = Math.Max(50, timeAlloc.SoftBoundMs);
        long adjustedHardBoundMs = Math.Max(adjustedSoftBoundMs, timeAlloc.HardBoundMs);

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
        int currentDepth = 1;

        // SAFEGUARD: Absolute max depth to prevent runaway values from TT bugs
        const int AbsoluteMaxDepth = SearchConstants.AbsoluteMaxDepth;
        const long MinNodesForValidIteration = 10;

        while (true)
        {
            if (currentDepth > AbsoluteMaxDepth)
            {
                break;
            }

            // Pre-iteration check: Total nodes must scale with depth
            // Prevents depth inflation from TT cache hits without real search
            // MUST match the same formula used in ParallelMinimaxSearch.SearchWithIterationTimeAware
            if (currentDepth > 10)
            {
                long minimumTotalNodesForDepth = (long)(currentDepth - 5) * (currentDepth - 5) * 200;
                if (_nodesSearched < minimumTotalNodesForDepth)
                {
                    // Not enough total nodes to justify this depth - stop now
                    break;
                }
            }

            // Max depth limit from "go depth N"
            if (maxDepth.HasValue && currentDepth > maxDepth.Value)
            {
                break;
            }

            // Max nodes limit from "go nodes N"
            if (maxNodes.HasValue && _nodesSearched >= maxNodes.Value)
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
                double estimatedNextTime = elapsed / 1000.0 * SHC.EffectiveBranchingFactorEstimate;
                if (remainingSeconds < estimatedNextTime * SHC.SoftBoundRatio)
                {
                    break;
                }
            }

            // Reset stopped flag for this depth
            _searchStopped = false;

            // Track nodes and time before this iteration to detect TT cache hits
            long nodesBeforeIteration = _nodesSearched;
            long ticksBeforeIteration = _searchStopwatch.ElapsedTicks;

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            long nodesSearchedThisIteration = _nodesSearched - nodesBeforeIteration;
            // Use ticks for high-resolution timing (ms has ~15ms resolution on Windows)
            long ticksThisIteration = _searchStopwatch.ElapsedTicks - ticksBeforeIteration;
            long timeThisIterationMs = (long)(ticksThisIteration * 1000.0 / System.Diagnostics.Stopwatch.Frequency);

            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
                _lastSearchScore = result.score;

                // Only update depth if this was a real search (not just TT cache hit)
                // TT hits return instantly with 0-1 nodes, which shouldn't count as "depth achieved"
                if (nodesSearchedThisIteration >= MinNodesForValidIteration)
                {
                    _depthAchieved = currentDepth; // Track deepest completed search
                }
            }

            // If search was stopped due to timeout, don't continue to next depth
            if (_searchStopped)
            {
                break;
            }

            // Only increment depth if meaningful search occurred
            // This prevents depth inflation from TT cache hits
            if (nodesSearchedThisIteration >= MinNodesForValidIteration)
            {
                currentDepth++;
            }
            else
            {
                // TT cache hit or instant return - no point searching deeper with cached results
                // Break to prevent depth inflation
                break;
            }
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


        // Store PV for pondering
        _lastPV = PV.FromSingleMove(bestMove.x, bestMove.y, _depthAchieved, 0);
        _lastBoard = board;
        _lastPlayer = player;

        // Start pondering for opponent's response
        if (ponderingEnabled)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
            var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(timeRemainingMs);

            if (ponderTimeMs > 0)
            {
                _ponderer.StartPondering(
                    board,
                    opponent,
                    predictedOpponentMove,
                    player,  // Pondering for us (next to move after opponent)
                    ponderTimeMs
                );
            }
        }

        // Publish search stats to channel
        PublishSearchStats(player, StatsType.MainSearch, _searchStopwatch.ElapsedMilliseconds);

        // CRITICAL SAFEGUARD: Final validation that the move blocks opponent's winning threats
        return ValidateAndReturnBlockingMove(board, player, bestMove);
    }

    /// <summary>
    /// SAFEGUARD: Final validation that the returned move blocks opponent's winning threats.
    /// This catches any edge cases where the blocking logic might be bypassed.
    /// </summary>
    private (int x, int y) ValidateAndReturnBlockingMove(Board board, Player player, (int x, int y) proposedMove)
    {
        var oppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var opponentWinningSquares = new List<(int x, int y)>();

        // CRITICAL: Validate that proposedMove is an empty square
        // If the proposed move is on an occupied square, we MUST find a valid move
        bool proposedMoveIsEmpty = board.GetCell(proposedMove.x, proposedMove.y).Player == Player.None;
        if (!proposedMoveIsEmpty)
        {
            _logger.LogWarning("[AI SAFEGUARD] CRITICAL: Proposed move ({X},{Y}) is occupied! Finding valid move...", proposedMove.x, proposedMove.y);
        }

        // Re-scan the full board for opponent winning moves (immediate 5-in-a-row)
        for (int fx = 0; fx < BoardSize; fx++)
        {
            for (int fy = 0; fy < BoardSize; fy++)
            {
                if (board.GetCell(fx, fy).Player == Player.None)
                {
                    if (_threatDetector.IsWinningMove(board, fx, fy, oppPlayer))
                    {
                        opponentWinningSquares.Add((fx, fy));
                    }
                }
            }
        }

        // CRITICAL: Also check for open fours (StraightFour with 2 winning squares)
        // An open four means opponent wins next move regardless of which square we block
        // We must detect these and block them before they're created
        var opponentThreats = _threatDetector.DetectThreats(board, oppPlayer);
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
            {
                // Add all gain squares from open fours to the blocking list
                foreach (var gs in threat.GainSquares)
                {
                    if (board.GetCell(gs.x, gs.y).IsEmpty && !opponentWinningSquares.Contains(gs))
                    {
                        opponentWinningSquares.Add(gs);
                    }
                }
            }
        }

        if (opponentWinningSquares.Count == 0)
        {
            // No opponent threats - return the proposed move IF it's empty
            if (proposedMoveIsEmpty)
            {
                return proposedMove;
            }
            // Proposed move is occupied - find any empty square
            return QuickWinChecker.FindAnyEmptySquare(board, proposedMove);
        }

        // If proposed move is occupied, we MUST find a blocking move - skip the validation
        if (!proposedMoveIsEmpty)
        {
            // Skip to the blocking logic below
            goto FindBlockingMove;
        }

        // Check if our proposed move blocks all threats
        var testBoard = board.PlaceStone(proposedMove.x, proposedMove.y, player);
        bool blocksAllThreats = true;

        // Check remaining winning squares
        foreach (var (wx, wy) in opponentWinningSquares)
        {
            if (wx == proposedMove.x && wy == proposedMove.y)
                continue; // This square is now occupied
            if (_threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
            {
                blocksAllThreats = false;
                break;
            }
        }

        // CRITICAL: Also check if opponent still has open fours after our move
        // An open four means opponent wins next move regardless of single block
        if (blocksAllThreats)
        {
            var remainingThreats = _threatDetector.DetectThreats(testBoard, oppPlayer);
            foreach (var threat in remainingThreats)
            {
                if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                {
                    // Opponent still has an open four - our block doesn't work
                    blocksAllThreats = false;
                    _logger.LogDebug("[AI SAFEGUARD] Block at ({X},{Y}) leaves opponent with {ThreatType}", proposedMove.x, proposedMove.y, threat.Type);
                    break;
                }
            }
        }

        if (blocksAllThreats)
        {
            // Proposed move is valid - it blocks all threats
            return proposedMove;
        }

    FindBlockingMove:

        // Our move doesn't block all threats - find one that does
        foreach (var (bx, by) in opponentWinningSquares)
        {
            var blockTestBoard = board.PlaceStone(bx, by, player);
            bool thisBlockWorks = true;

            // Check remaining winning squares
            foreach (var (wx, wy) in opponentWinningSquares)
            {
                if (wx == bx && wy == by)
                    continue;
                if (_threatDetector.IsWinningMove(blockTestBoard, wx, wy, oppPlayer))
                {
                    thisBlockWorks = false;
                    break;
                }
            }

            // CRITICAL: Also check if opponent still has open fours after this block
            if (thisBlockWorks)
            {
                var remainingThreats = _threatDetector.DetectThreats(blockTestBoard, oppPlayer);
                foreach (var threat in remainingThreats)
                {
                    if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                    {
                        // Opponent still has an open four - this block doesn't work
                        thisBlockWorks = false;
                        break;
                    }
                }
            }

            if (thisBlockWorks)
            {
                _logger.LogDebug("[AI SAFEGUARD] Forcing block at ({BX},{BY}) instead of ({X},{Y})", bx, by, proposedMove.x, proposedMove.y);
                _moveType = MoveType.ImmediateBlock;
                return (bx, by);
            }
        }

        // No single block works - opponent has multiple independent winning threats
        // Try to find a counter-attack move that creates our own winning threat
        // If we can force opponent to block, we gain a tempo and might survive
        var ourWinningMove = QuickWinChecker.FindOurWinningMove(board, player, _threatDetector);
        if (ourWinningMove.HasValue)
        {
            _logger.LogDebug("[AI SAFEGUARD] No single block works - counter-attacking with winning move at ({WX},{WY})", ourWinningMove.Value.x, ourWinningMove.Value.y);
            _moveType = MoveType.ImmediateWin;
            return ourWinningMove.Value;
        }

        // CRITICAL FIX: Find the block that minimizes remaining winning squares
        // For an open four with 2 winning squares, blocking one reduces winning squares to 1
        // This gives us a chance if opponent makes a mistake, or time to create our own threat
        var bestDelayingBlock = opponentWinningSquares[0];
        int minRemainingWinningSquares = int.MaxValue;

        foreach (var (bx, by) in opponentWinningSquares)
        {
            var blockTestBoard = board.PlaceStone(bx, by, player);
            int remainingWinningSquares = 0;

            // Count remaining winning squares after this block
            for (int wx = 0; wx < BoardSize; wx++)
            {
                for (int wy = 0; wy < BoardSize; wy++)
                {
                    if (blockTestBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(blockTestBoard, wx, wy, oppPlayer))
                        remainingWinningSquares++;
                }
            }

            if (remainingWinningSquares < minRemainingWinningSquares)
            {
                minRemainingWinningSquares = remainingWinningSquares;
                bestDelayingBlock = (bx, by);
            }
        }

        _logger.LogDebug("[AI SAFEGUARD] No single block works - best delaying block at ({BX},{BY}) leaves {Count} winning squares",
            bestDelayingBlock.x, bestDelayingBlock.y, minRemainingWinningSquares);
        _moveType = MoveType.ImmediateBlock;
        return bestDelayingBlock;
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
        return SHC.MaxAIMaxDepth;
    }

    /// <summary>
    // REMOVED: GetCriticalDefenseLevel - no longer used, threats handled by evaluation function

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
    /// - VCF check time scales with remaining time budget
    /// - Quick check: if opponent has no threats, skip VCF check entirely
    /// - Single VCF check (not nested) to detect opponent threats
    /// - Return first valid defensive move without re-checking VCF for each one
    /// - Skip VCF defense in emergency mode (prioritize speed over accuracy)
    /// </summary>
    private (int x, int y)? FindVCFDefense(Board board, Player player, TimeAllocation timeAlloc)
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
        var (vcfCheckTime, vcfMaxDepth) = TimeBudgetCalculator.CalculateVCFTimeLimit(timeAlloc);

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
                        _logger.LogDebug("[AI DEFENSE] ({Player}) Opponent has immediate win at ({X}, {Y}) - blocking!",
                            player, x, y);
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
        const int aspirationWindow = SHC.AspirationWindow;
        const int maxAspirationAttempts = SHC.MaxAspirationAttempts;

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
            var candidateScores = _moveOrderer.ScoreCandidatesForTiebreak(candidates, board, player, depth);

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
            if (found && cachedMove.HasValue)
            {
                // CRITICAL: Validate the cached move is actually legal
                // TT entries may be from different positions due to hash collisions or stale data
                var (cx, cy) = cachedMove.Value;
                if (cx >= 0 && cx < BoardSize && cy >= 0 && cy < BoardSize)
                {
                    var cell = board.GetCell(cx, cy);
                    if (cell.IsEmpty)
                    {
                        _tableHits++;
                        return (cx, cy, cachedScore);
                    }
                }
                // If cached move is invalid, fall through to normal search
            }

            // Reset best score for this attempt
            bestScore = int.MinValue;
            bestMove = candidates[0];
            bestTiebreaker = 0;

            // Order moves: Hash > Emergency > Threats > Killers > History > Positional
            var orderedMoves = _moveOrderer.OrderMoves(candidates, depth, board, player, cachedMove);

            // Pre-score ordered moves for tiebreaking
            var orderedTiebreakScores = _moveOrderer.ScoreCandidatesForTiebreak(orderedMoves, board, player, depth);

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
                    var randomBonus = NextRandomInt(SHC.RandomBonusRange);

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

    private void RecordKillerMove(int depth, int x, int y) { _heuristics.RecordKillerMove(depth, x, y); }

    /// <summary>
    /// Record a move that caused a cutoff in the history table
    /// Higher depth = more significant = larger bonus
    /// </summary>
    private void RecordHistoryMove(Player player, int x, int y, int depth) { _heuristics.RecordHistoryMove(player, x, y, depth); }

    /// <summary>
    /// Get the history score for a move
    /// </summary>
    private int GetHistoryScore(Player player, int x, int y) => _heuristics.GetHistoryScore(player, x, y);

    /// <summary>
    /// Clear history tables (call at start of new game)
    /// </summary>
    public void ClearHistory()
    {
        _heuristics.ClearHistory();
    }

    /// <summary>
    /// Clear search state for new position while preserving transposition table.
    /// Clears: history tables, killer moves, pondering state.
    /// Preserves: transposition table entries (memoization), adaptive time state.
    /// </summary>
    public void ClearSearchState()
    {
        ClearHistory();
        ResetPondering();

        // Clear killer moves (position-specific move ordering)
        _heuristics.ClearKillers();

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
        _parallelSearch.Clear();  // Also clear parallel search's TT
        ResetPondering();

        // Reset adaptive time manager state
        _adaptiveTimeManager.Reset();

        // Reset inferred initial time for adaptive thresholds
        // -1 means "unknown, will infer from first move"
        _inferredInitialTimeMs = -1;

        // Clear killer moves
        _heuristics.ClearKillers();

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
    /// Clear the transposition table to prevent position leakage between games.
    /// Use this for self-play scenarios where you want to reset search state
    /// without clearing all AI configuration.
    /// </summary>
    public void ClearTranspositionTable()
    {
        _transpositionTable.Clear();
        _parallelSearch.Clear();  // Also clear parallel search's TT
    }

    /// <summary>
    /// Resize transposition table. Clears and rebuilds both main and parallel search TTs.
    /// </summary>
    public void ResizeTranspositionTable(int newSizeMb)
    {
        _transpositionTable = new TranspositionTable(newSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(newSizeMb);
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
        var standPat = EvaluateBoard(board, aiPlayer);

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
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Generate tactical moves (only near existing stones)
        var tacticalMoves = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

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
        var orderedMoves = _moveOrderer.OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

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
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            return QuiesceCore(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING
        var isNullMoveEligible = (beta - alpha) <= 1;
        if (depth >= SearchConstants.NullMoveMinDepth && isNullMoveEligible && TacticalEvaluator.IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            int nullMoveDepth = depth - SearchConstants.NullMoveDepthReduction;
            if (nullMoveDepth > 0)
            {
                int nullMoveScore = MinimaxCore(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);
                if (nullMoveScore >= beta)
                {
                    return beta;
                }
            }
        }

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);
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
        var orderedMoves = _moveOrderer.OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = SHC.LMRFullDepthMoves;
        const int pvsEnabledDepth = SHC.PVSEnabledDepth;

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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
        var standPat = EvaluateBoard(board, aiPlayer);

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
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Generate tactical moves
        var tacticalMoves = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

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
        var orderedMoves = _moveOrderer.OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

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
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING: Skip a move to verify position is already good
        // Only apply in non-PV nodes with sufficient depth and safe position
        var isNullMoveEligible = (beta - alpha) <= 1;  // Not a PV node (narrow window)
        if (depth >= SearchConstants.NullMoveMinDepth && isNullMoveEligible && TacticalEvaluator.IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            // Make null move: skip turn, search with reduced depth
            // The reduced depth search is done from opponent's perspective (flipped min/max)
            int nullMoveDepth = depth - SearchConstants.NullMoveDepthReduction;

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

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);
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
        var vcfThresholdMs = timeRemainingPercent < SHC.VcfTimeRemainingThreshold
            ? SHC.VcfMinimumTimeMs
            : initialTime * SHC.VcfInitialTimeFraction;

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
        var orderedMoves = _moveOrderer.OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = SHC.LMRFullDepthMoves;
        const int pvsEnabledDepth = SHC.PVSEnabledDepth;

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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
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
    /// Check if there's a winner on the board using WinDetector
    /// This ensures Caro rules are enforced: exact 5-in-a-row, no sandwiched wins, no overlines
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner ? result.Winner : null;
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
        int probCutBeta = beta + SHC.ProbCutMargin;

        var score = Minimax(board, probCutDepth, probCutBeta - 1, probCutBeta, isMaximizing, aiPlayer, rootDepth);

        // If shallow search already exceeds beta, we're likely to cutoff
        return score >= probCutBeta;
    }

    #region Pondering Support

    /// <summary>
    /// Get the ponderer instance for external access
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
    public void StartPonderingNow(Board board, Player currentPlayerToMove, Player thisAIColor)
    {
        var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(null);
        if (ponderTimeMs > 0)
        {
            _ponderer.StartPondering(board, currentPlayerToMove, null, thisAIColor, ponderTimeMs);
        }
    }

    /// <summary>
    /// Start pondering after making a move (for opponent's response)
    /// This is a stateless version - all parameters passed explicitly
    /// </summary>
    public void StartPonderingAfterMove(Board board, Player opponentToMove, Player thisAIColor, PV? lastPV = null)
    {
        var predictedOpponentMove = lastPV?.GetPredictedOpponentMove() ?? _lastPV.GetPredictedOpponentMove();

        var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(null);
        if (ponderTimeMs > 0)
        {
            _ponderer.StartPondering(board, opponentToMove, predictedOpponentMove, thisAIColor, ponderTimeMs);
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
    public (int DepthAchieved, long NodesSearched, double NodesPerSecond, double TableHitRate, bool PonderingActive, int VCFDepthAchieved, long VCFNodesSearched, int ThreadCount, string? ParallelDiagnostics, double MasterTTPercent, double HelperAvgDepth, long AllocatedTimeMs, MoveType MoveType, int SearchScore, double FmcPercent, double Ebf) GetSearchStatistics()
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

        return (_depthAchieved, _nodesSearched, nps, hitRate, _lastPonderingEnabled, _vcfDepthAchieved, _vcfNodesSearched, _lastThreadCount, _lastParallelDiagnostics, masterTTPercent, helperAvgDepth, _lastAllocatedTimeMs, _moveType, _lastSearchScore, _lastFmcPercent, _lastEbf);
    }

    /// <summary>
    /// Publish search statistics to the stats channel
    /// Called automatically after each search
    /// </summary>
    public void PublishSearchStats(Player player, StatsType statsType, long moveTimeMs)
    {
        var (depthAchieved, nodesSearched, nps, hitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched, threadCount, _, masterTTPercent, helperAvgDepth, allocatedTimeMs, moveType, _, _, _) = GetSearchStatistics();

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
