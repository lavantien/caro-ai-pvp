using System.Runtime.CompilerServices;
using System.Diagnostics;
using System.Threading.Channels;
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

    // Time-budget-based depth manager - scales depth with machine capability
    // Replaces hardcoded depths with iterative deepening based on NPS and time
    private readonly TimeBudgetDepthManager _depthManager = new();

    // Track initial time for adaptive depth thresholds
    // -1 means "unknown, will infer from first move"
    private long _inferredInitialTimeMs = -1;

    // Track last calculated depth for smoothing (prevent sudden drops)
    private int _lastCalculatedDepth = -1;

    // Track thread count used for last search (for diagnostics)
    private int _lastThreadCount = 1;

    // Track parallel diagnostics from last search
    private string? _lastParallelDiagnostics = null;

    // Parallel search for high difficulties (D7+)
    // Lazy SMP provides 4-8x speedup on multi-core systems
    private readonly ParallelMinimaxSearch _parallelSearch;

    // Search radius around existing stones (optimization)
    private const int SearchRadius = 2;

    // Killer heuristic: track best moves at each depth
    // No depth cap - array sized for maximum practical depth (19x19 board = 361 cells)
    private const int MaxKillerMoves = 2;
    private const int MaxKillerDepth = 512;  // Effectively unlimited for practical game play
    private readonly (int x, int y)[,] _killerMoves = new (int x, int y)[MaxKillerDepth, MaxKillerMoves];

    // History heuristic: track moves that cause cutoffs across all depths
    // Two tables: one for Red, one for Blue (each move can be good for different players)
    private readonly int[,] _historyRed = new int[19, 19];   // History scores for Red moves
    private readonly int[,] _historyBlue = new int[19, 19];  // History scores for Blue moves

    // Butterfly heuristic: track moves that cause beta cutoffs (complements history)
    private readonly int[,] _butterflyRed = new int[19, 19];
    private readonly int[,] _butterflyBlue = new int[19, 19];

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

    // Time control for search timeout
    private long _searchHardBoundMs;
    // Check time more frequently to catch timeout earlier (power of 2 for efficient masking)
    // 4096 = check every ~4K nodes. At 1M nodes/sec, this checks every ~4ms
    // This is much more frequent than the old 100K interval which only checked every ~100ms
    private const int TimeCheckInterval = 4096;
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
    /// <param name="difficulty">AI difficulty level (D1-D11)</param>
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

        // Check for ponder hit - if opponent played predicted move, we can use cached result
        if (ponderingEnabled && _ponderer.IsPondering && _lastPV.IsEmpty == false)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var (ponderState, ponderResult) = _ponderer.HandleOpponentMove(-1, -1); // Dummy, just to get state

            // For now, we handle ponder hit by checking if board matches our pondered position
            // The actual ponder hit detection is done externally via TournamentEngine
        }

        // Opening book for Hard, Grandmaster, and Experimental difficulties
        // Depth-filtered by difficulty:
        // - Hard: book moves up to depth 24 (12 moves per side, 24 plies)
        // - Grandmaster/Experimental: book moves up to depth 32 (16 moves per side, 32 plies)
        var lastOpponentMove = GetLastOpponentMove(board, player);
        var bookMove = _openingBook?.GetBookMove(board, player, difficulty, lastOpponentMove);
        if (bookMove.HasValue)
        {
            return bookMove.Value;
        }

        // Error rate simulation: Lower difficulties make random/suboptimal moves
        // Uses AdaptiveDepthCalculator.GetErrorRate() for consistent error rates
        // - Braindead: 50%, Easy: 15%, Medium: 5%, Hard: 1%, Grandmaster: 0%
        var errorRate = AdaptiveDepthCalculator.GetErrorRate(difficulty);
        if (errorRate > 0 && NextRandomDouble() < errorRate)
        {
            // Play a random valid move instead of searching
            // Report minimal stats to indicate instant move (not D0 which looks like a bug)
            _depthAchieved = 1;
            _nodesSearched = 1;
            var randomIndex = NextRandomInt(candidates.Count);
            return candidates[randomIndex];
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
                    SoftBoundMs = timeRemainingMs.Value - 1000,  // Leave 1s margin
                    HardBoundMs = timeRemainingMs.Value - 100,
                    OptimalTimeMs = (long)(timeRemainingMs.Value * 0.8)
                }
                : GetDefaultTimeAllocation(difficulty);
        }

        // CRITICAL DEFENSE: Check for opponent threats BEFORE any early returns
        // This ensures we don't skip blocking in emergency mode
        var oppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var threats = _threatDetector.DetectThreats(board, oppPlayer)
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree)
            .ToList();

        bool hasOpponentThreats = threats.Count > 0;
        bool hasOpenFour = false;  // Open four: 4 stones with 2 blocking squares

        List<(int x, int y)> blockingSquares = new();
        List<(int x, int y)> priorityBlockingSquares = new();  // For open fours

        if (hasOpponentThreats)
        {
            var straightFourCount = threats.Count(t => t.Type == ThreatType.StraightFour);
            var straightThreeCount = threats.Count(t => t.Type == ThreatType.StraightThree);

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

            _logger.LogDebug("[AI DEFENSE] {Difficulty} ({Player}) Opponent has {StraightFourCount} StraightFour, {StraightThreeCount} StraightThree threat(s), blocking squares: {BlockingSquares}{OpenFourSuffix}",
                difficulty, player, straightFourCount, straightThreeCount,
                string.Join(", ", blockingSquares.Select(g => $"({g.x},{g.y})")),
                hasOpenFour ? " [OPEN FOUR DETECTED]" : "");
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
        // VCF Defense was causing D11 to play too reactively, blocking opponent threats
        // instead of developing its own position. The evaluation function's defense
        // multiplier (2.2x for opponent threats) should be sufficient for defense.
        // D11's advantage comes from offensive VCF, not defensive VCF detection.
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
                Console.WriteLine($"[AI VCF] {difficulty} ({player}) Emergency mode: Using up to 80% of hard bound ({vcfTimeLimit}ms, cap: {emergencyVcfCap}ms) for VCF");
            }

            var vcfResult = _vcfSolver.SolveVCF(board, player, timeLimitMs: vcfTimeLimit, maxDepth: vcfMaxDepth);

            // Capture VCF statistics even if not a winning sequence
            _vcfDepthAchieved = vcfResult.DepthAchieved;
            _vcfNodesSearched = vcfResult.NodesSearched;

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                // VCF found a forced win sequence - use it immediately
                Console.WriteLine($"[AI VCF] {difficulty} ({player}) Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}, nodes: {vcfResult.NodesSearched}");
                return vcfResult.BestMove.Value;
            }

            // VCF-FIRST MODE: In emergency mode, if VCF didn't find a win, check opponent threats
            // CRITICAL: Don't skip blocking even in emergency mode
            if (timeAlloc.IsEmergency)
            {
                // If opponent has threats, MUST block - don't use TT fallback
                if (hasOpponentThreats && candidates.Count > 0)
                {
                    Console.WriteLine($"[AI VCF] {difficulty} ({player}) Emergency: No VCF found, but opponent has threats - using blocking move");
                    // Candidates are already filtered to blocking squares from earlier threat detection
                    _depthAchieved = 1;
                    _nodesSearched = 1;
                    return candidates[0];
                }

                // No opponent threats - safe to use TT move
                var ttMove = GetTranspositionTableMove(board, player, minDepth: 3);
                if (ttMove.HasValue)
                {
                    Console.WriteLine($"[AI VCF] {difficulty} ({player}) Emergency: No VCF found, using TT move as fallback");
                    _depthAchieved = 3;
                    _nodesSearched = 1;
                    return ttMove.Value;
                }

                // Last resort: return the first candidate (usually the center or near existing stones)
                Console.WriteLine($"[AI VCF] {difficulty} ({player}) Emergency: No TT move, using quick candidate selection");
                _depthAchieved = 1;
                _nodesSearched = 1;
                return candidates[0];
            }
        }

        // Get time multiplier for this difficulty (applies to both parallel and sequential search)
        // Braindead: 1%, Easy: 10%, Medium: 30%, Hard: 70%, Grandmaster: 100%
        double timeMultiplier = AdaptiveDepthCalculator.GetTimeMultiplier(difficulty);

        // CRITICAL FIX: Calibrate NPS estimate from difficulty settings before searching
        // This ensures we don't underestimate machine capability based on initial 100K default
        _depthManager.CalibrateNpsForDifficulty(difficulty);

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
            var adjustedTimeAlloc = new TimeAllocation
            {
                SoftBoundMs = Math.Max(1, (long)(timeAlloc.SoftBoundMs * timeMultiplier)),
                HardBoundMs = Math.Max(1, (long)(timeAlloc.HardBoundMs * timeMultiplier)),
                OptimalTimeMs = Math.Max(1, (long)(timeAlloc.OptimalTimeMs * timeMultiplier)),
                IsEmergency = timeAlloc.IsEmergency,
                Phase = timeAlloc.Phase
            };

            // DEBUG: Log time allocation for BookGeneration
            if (difficulty == AIDifficulty.BookGeneration)
            {
                Console.WriteLine($"[BookGeneration DEBUG] timeRemainingMs={timeRemainingMs}, timeMultiplier={timeMultiplier}");
                Console.WriteLine($"[BookGeneration DEBUG] adjustedTimeAlloc: Soft={adjustedTimeAlloc.SoftBoundMs}ms, Hard={adjustedTimeAlloc.HardBoundMs}ms, Optimal={adjustedTimeAlloc.OptimalTimeMs}ms");
            }

            var parallelResult = _parallelSearch.GetBestMoveWithStats(
                board,
                player,
                difficulty,
                timeRemainingMs: timeRemainingMs,
                timeAlloc: adjustedTimeAlloc,
                moveNumber: moveNumber,
                fixedThreadCount: threadCount,
                candidates: candidates);

            // Update statistics from parallel search
            _depthAchieved = parallelResult.DepthAchieved;
            _nodesSearched = parallelResult.NodesSearched;
            _lastParallelDiagnostics = parallelResult.ParallelDiagnostics;
            _lastAllocatedTimeMs = parallelResult.AllocatedTimeMs;
            _lastPonderingEnabled = ponderingEnabled;
            _tableHits = parallelResult.TableHits;
            _tableLookups = parallelResult.TableLookups;

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

        // CRITICAL FIX: Calibrate NPS estimate from difficulty settings before searching
        // This ensures we don't underestimate machine capability based on initial 100K default
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Apply time multiplier to the soft bound - lower difficulties use less time
        long adjustedSoftBoundMs = Math.Max(1, (long)(timeAlloc.SoftBoundMs * timeMultiplier));

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
                _depthAchieved = currentDepth; // Track deepest completed search

                // Update NPS estimate from this iteration
                double iterationElapsedSeconds = _searchStopwatch.Elapsed.TotalSeconds;
                if (iterationElapsedSeconds > 0)
                {
                    _depthManager.UpdateNpsEstimate(_nodesSearched, iterationElapsedSeconds);
                }
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
            Console.WriteLine($"[AI TT] {difficulty} ({player}) Hits: {_tableHits}/{_tableLookups} ({hitRate:F1}%)");
            var (used, usage) = _transpositionTable.GetStats();
            Console.WriteLine($"[AI TT] {difficulty} ({player}) Table usage: {used} entries ({usage:F2}%)");
            var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
            var nps = elapsedMs > 0 ? nodesSearched * 1000 / elapsedMs : 0;
            Console.WriteLine($"[AI STATS] {difficulty} ({player}) TimeMult: {timeMultiplier:P0}, Depth: {depthAchieved}, Nodes: {nodesSearched}, NPS: {nps:F0}");
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
    /// Calculate appropriate search depth based on time allocation and position complexity
    /// Uses adaptive percentage-based thresholds based on inferred initial time
    /// Ensures full depth is reachable at the start of the game
    ///
    /// CRITICAL: Must preserve AI strength ordering! D11 should always search deeper than D10,
    /// which searches deeper than D8, etc. The depth reduction must maintain relative ordering.
    /// </summary>
    /// <summary>
    /// Calculate appropriate search depth based on time allocation and position complexity
    /// Uses adaptive percentage-based thresholds based on inferred initial time
    /// Ensures full depth is reachable at the start of the game
    ///
    /// ADAPTIVE DEPTH CALCULATION:
    /// - Estimates nodes per ply based on branching factor
    /// - Uses NPS from previous searches to estimate server capability
    /// - Calculates how many plies can be completed in the allocated time
    /// - Reduces depth aggressively for short time controls
    ///
    /// CRITICAL: Must preserve AI strength ordering! D5 should always search deeper than D10,
    /// which searches deeper than D8, etc. The depth reduction must maintain relative ordering.
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, long? timeRemainingMs, int candidateCount)
    {
        // Infer initial time from the move number and remaining time
        // On early moves (when we haven't spent much time yet), use current remaining as initial
        // This works for any time control: 3+2, 7+5, 15+10, etc.
        if (timeRemainingMs.HasValue && timeAlloc.Phase == GamePhase.Opening)
        {
            // First move or unknown initial time: infer from current remaining
            if (_inferredInitialTimeMs < 0)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
            // Update inferred time if we're seeing a significantly different time control
            else if (Math.Abs(timeRemainingMs.Value - _inferredInitialTimeMs) > _inferredInitialTimeMs * 0.3)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
        }

        // Fallback for edge cases (shouldn't happen with proper time control)
        var effectiveInitialTimeMs = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : 420_000;

        // ADAPTIVE: Estimate maximum sustainable depth based on time control and NPS
        // Formula: max_depth = log(time_per_move * NPS / branching_factor) / log(branching_factor)
        // For Caro: branching factor ~35, NPS varies by server capability
        int adaptiveMaxDepth = CalculateAdaptiveMaxDepth(timeAlloc, effectiveInitialTimeMs);

        // Clamp base depth to what's sustainable for this time control
        // This is the key fix: don't even try for depth 9-11 on short time controls
        int sustainableBaseDepth = Math.Min(baseDepth, adaptiveMaxDepth);

        // Emergency mode - VERY aggressive depth reduction to avoid timeout
        // In time scramble, rely on VCF + TT move, not deep search
        // But still preserve relative strength: D5 should be deeper than D4 even in emergency
        if (timeAlloc.IsEmergency)
        {
            // Emergency depth with separation: D5->4, D4->3
            int emergencyDepth = sustainableBaseDepth switch
            {
                >= 11 => 6,  // D11: depth 6
                >= 10 => 5,  // D10: depth 5
                >= 9 => 5,   // D9: depth 5
                >= 8 => 4,   // D8: depth 4
                >= 7 => 4,   // D7: depth 4
                >= 6 => 3,   // D6: depth 3
                >= 5 => 4,   // D5 (Grandmaster): depth 4 - ALWAYS deeper than D4!
                _ => 3       // D4 and below: depth 3
            };
            return SmoothDepth(emergencyDepth);
        }

        // Per-move time allocation
        var softBoundSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Total time remaining
        var totalTimeRemainingSeconds = timeRemainingMs.HasValue ? timeRemainingMs.Value / 1000.0 : (double)effectiveInitialTimeMs / 1000.0;
        var initialTimeSeconds = (double)effectiveInitialTimeMs / 1000.0;

        // Calculate minimum depth to preserve AI strength ordering
        // CRITICAL: Create 1-ply separation between adjacent difficulties!
        // D5 searches at least 1 ply deeper than D4, which searches 1 ply deeper than D3, etc.
        int minDepthForStrength = sustainableBaseDepth switch
        {
            >= 11 => 8,  // D11: at least depth 8 (1 ply above D10)
            >= 10 => 7,  // D10: at least depth 7 (1 ply above D9)
            >= 9 => 6,   // D9: at least depth 6 (1 ply above D8)
            >= 8 => 5,   // D8: at least depth 5 (1 ply above D7)
            >= 7 => 4,   // D7: at least depth 4
            >= 6 => 4,   // D6: at least depth 4
            >= 5 => 4,   // D5 (Grandmaster): at least depth 4 - ALWAYS deeper than D4!
            >= 4 => 3,   // D4 (Hard): at least depth 3
            _ => 2        // D1-D3: at least depth 2
        };

        // Critical: less than 10% of initial time or very tight per-move limit
        int targetDepth;
        if (softBoundSeconds < 2 || totalTimeRemainingSeconds < initialTimeSeconds * 0.10)
        {
            targetDepth = Math.Max(minDepthForStrength, sustainableBaseDepth / 2);
        }
        // Low: 10-25% of initial time or moderate per-move limit
        // Check if soft bound is very small relative to initial time (< 1.5%)
        // This prevents triggering depth reduction when we have plenty of time (e.g., 5s out of 420s)
        else
        {
            double softBoundRatio = softBoundSeconds / initialTimeSeconds;
            if ((softBoundSeconds < 4 && softBoundRatio < 0.015) || totalTimeRemainingSeconds < initialTimeSeconds * 0.15)
            {
                if (candidateCount > 25) // Complex position
                {
                    targetDepth = Math.Max(minDepthForStrength, sustainableBaseDepth - 3);
                }
                else
                {
                    targetDepth = Math.Max(minDepthForStrength, sustainableBaseDepth - 1);
                }
            }
            // Moderate: 25-50% of initial time
            else if (totalTimeRemainingSeconds < initialTimeSeconds * 0.50)
            {
                if (candidateCount > 25) // Complex position with limited time
                {
                    targetDepth = Math.Max(minDepthForStrength, sustainableBaseDepth - 2);
                }
                else
                {
                    targetDepth = Math.Max(minDepthForStrength, sustainableBaseDepth - 1);
                }
            }
            // Good time availability (>50% remaining): use full depth
            else
            {
                targetDepth = sustainableBaseDepth;
            }
        }

        return SmoothDepth(targetDepth);
    }

    /// <summary>
    /// Calculate the maximum sustainable depth based on time control and server capability
    /// 
    /// This is the core adaptive function that prevents time exhaustion on short time controls.
    /// Formula estimates how many plies can be completed in the allocated time:
    /// - Branching factor for Caro: ~35 candidates in midgame
    /// - Time per move: soft bound from TimeManager
    /// - NPS: estimated from previous searches, with conservative defaults
    /// 
    /// Examples for 7+5 time control:
    /// - Opening (7 min / 40 moves = ~10s/move): depth 8-9 on fast server, 6-7 on slow
    /// - Middlegame (4 min / 30 moves = ~8s/move): depth 7-8 on fast server, 5-6 on slow
    /// - Endgame (2 min / 20 moves = ~6s/move): depth 6-7 on fast server, 4-5 on slow
    /// 
    /// Examples for 3+2 time control:
    /// - Opening (3 min / 40 moves = ~4.5s/move): depth 6-7 on fast server, 4-5 on slow
    /// - Middlegame (2 min / 30 moves = ~4s/move): depth 5-6 on fast server, 3-4 on slow
    /// </summary>
    private int CalculateAdaptiveMaxDepth(TimeAllocation timeAlloc, long initialTimeMs)
    {
        // Estimate NPS from previous search, or use conservative defaults
        // These are conservative estimates that should work on most servers
        double estimatedNps = EstimateNodesPerSecond();

        // Time available for this move (in seconds)
        double timeAvailableSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Branching factor for Caro (varies by phase)
        // Opening: ~25 candidates, Middlegame: ~35, Endgame: ~20
        double branchingFactor = timeAlloc.Phase switch
        {
            GamePhase.Opening => 25.0,
            GamePhase.EarlyMid => 35.0,
            GamePhase.LateMid => 35.0,
            GamePhase.Endgame => 20.0,
            _ => 30.0
        };

        // Calculate max depth using formula: max_depth = log(time * NPS / BF) / log(BF)
        // This estimates how many plies we can complete in the given time
        double maxPly = 1;
        if (estimatedNps > 0 && timeAvailableSeconds > 0 && branchingFactor > 1)
        {
            // Total nodes we can search in this time
            double totalNodes = timeAvailableSeconds * estimatedNps;

            // Solve for ply: BF^ply = totalNodes
            // ply = log(totalNodes) / log(BF)
            maxPly = Math.Log(totalNodes) / Math.Log(branchingFactor);
        }

        // Adjust for Caro ruleset complexity (5-in-a-row is more tactical than chess)
        // Add overhead for VCF, threat detection, evaluation
        int maxDepth = (int)(maxPly * 0.7); // 70% efficiency factor

        // Clamp to reasonable bounds (depth 3 minimum, depth 12 maximum)
        maxDepth = Math.Clamp(maxDepth, 3, 12);

        return maxDepth;
    }

    /// <summary>
    /// Estimate nodes per second based on previous search performance
    /// Uses conservative defaults if no history available
    /// 
    /// Returns NPS estimate (nodes per second)
    /// </summary>
    private double EstimateNodesPerSecond()
    {
        // Try to estimate from last search if available
        if (_searchStopwatch != null && _searchStopwatch.ElapsedMilliseconds > 0 && _nodesSearched > 0)
        {
            double lastNps = _nodesSearched * 1000.0 / _searchStopwatch.ElapsedMilliseconds;

            // Only use if we have a reasonable sample (at least 1000 nodes searched)
            if (_nodesSearched > 1000)
            {
                // Apply a safety factor (80%) to account for position variability
                return lastNps * 0.8;
            }
        }

        // Conservative defaults based on server capability tiers
        // These are safe minimums that most servers should exceed
        return _inferredInitialTimeMs < 180_000 ? 50_000  // Fast server for 3+2
             : _inferredInitialTimeMs < 300_000 ? 100_000 // Medium server for 5+3 or 7+5
             : 200_000;                             // Slow server or long time control
    }

    /// <summary>
    /// Apply depth smoothing to prevent sudden drops in search depth
    /// Gradually transition between depths to maintain move quality consistency
    /// </summary>
    private int SmoothDepth(int targetDepth)
    {
        // First move or game reset
        if (_lastCalculatedDepth < 0)
        {
            _lastCalculatedDepth = targetDepth;
            return targetDepth;
        }

        // Allow increases immediately (good positions deserve deeper search)
        if (targetDepth > _lastCalculatedDepth)
        {
            _lastCalculatedDepth = targetDepth;
            return targetDepth;
        }

        // For decreases, apply gradual smoothing (reduce by at most 1 per move)
        // This prevents sudden quality drops when time gets tight
        int smoothedDepth = Math.Max(targetDepth, _lastCalculatedDepth - 1);
        _lastCalculatedDepth = smoothedDepth;
        return smoothedDepth;
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
            if (x >= 0 && x < 19 && y >= 0 && y < 19)
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
    /// This is essential for D11 to prevent losing to lower difficulty VCF attacks
    ///
    /// OPTIMIZED: Uses fast threat detection + immediate defensive move selection
    /// - VCF check time scales with difficulty (D11 gets more time for defensive VCF)
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
            Console.WriteLine($"[AI VCF] {difficulty} ({player}) Opponent has VCF threat (depth: {opponentVCFResult.DepthAchieved}, nodes: {opponentVCFResult.NodesSearched})");

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
                        Console.WriteLine($"[AI VCF] {difficulty} ({player}) Using defensive move ({defense.x}, {defense.y})");
                        return defense;
                    }
                }

                // Fallback: use first defensive move even if not currently empty
                // (shouldn't happen, but handle gracefully)
                var fallback = defenses[0];
                if (fallback.x >= 0 && fallback.x < board.BoardSize &&
                    fallback.y >= 0 && fallback.y < board.BoardSize)
                {
                    Console.WriteLine($"[AI VCF] {difficulty} ({player}) Using fallback defense at ({fallback.x}, {fallback.y})");
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

    private (int x, int y) SearchWithDepth(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        // Aspiration window: try narrow search first, then wider if needed
        const int aspirationWindow = 50;  // Initial window size
        const int maxAspirationAttempts = 3;  // Max re-searches with wider windows

        var bestScore = int.MinValue;
        var bestMove = candidates[0];
        int bestTiebreaker = 0;  // Track tiebreaker score

        // Calculate board hash for transposition table
        var boardHash = _transpositionTable.CalculateHash(board);

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
                var newBoard = board.PlaceStone(x, y, player);
                var score = Minimax(newBoard, depth - 2, searchAlpha, searchBeta, false, player, depth);

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
                    return cachedMove.Value;
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
                // Make move
                var newBoard = board.PlaceStone(x, y, player);

                // Evaluate using minimax
                var score = Minimax(newBoard, depth - 1, alpha, beta, false, player, depth);

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
    /// Score candidates for tie-breaking when minimax scores are equal.
    /// Uses position heuristics similar to OrderMoves but without full sorting.
    /// Higher score = more desirable move.
    /// </summary>
    private int[] ScoreCandidatesForTiebreak(List<(int x, int y)> candidates, Board board, Player player, int depth)
    {
        int count = candidates.Count;
        var scores = new int[count];
        const int butterflySize = 19;  // Must match array declaration

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

            // Prefer center (9,9) for 19x19 board
            var distanceToCenter = Math.Abs(x - 9) + Math.Abs(y - 9);
            score += (18 - distanceToCenter) * 10;

            // Prefer moves near existing stones
            var nearby = 0;
            for (int dx = -1; dx <= 1; dx++)
            {
                for (int dy = -1; dy <= 1; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    if (nx >= 0 && nx < 19 && ny >= 0 && ny < 19)
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
                // Bounds check for butterfly tables (19x19)
                const int butterflySize = 19;
                var butterflyScore = (x >= 0 && x < butterflySize && y >= 0 && y < butterflySize)
                    ? (player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y])
                    : 0;
                score += Math.Min(300, butterflyScore / 100);

                var historyScore = GetHistoryScore(player, x, y);
                score += Math.Min(500, historyScore / 10);

                // PRIORITY #6: Positional Heuristics - center proximity, nearby stones
                // Prefer center (9,9) for 19x19 board
                var distanceToCenter = Math.Abs(x - 9) + Math.Abs(y - 9);
                score += (18 - distanceToCenter) * 10;

                // Prefer moves near existing stones
                var nearby = 0;
                for (int dx = -1; dx <= 1; dx++)
                {
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        if (dx == 0 && dy == 0) continue;
                        var nx = x + dx;
                        var ny = y + dy;
                        if (nx >= 0 && nx < 19 && ny >= 0 && ny < 19)
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;

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

        // Reset depth smoothing
        _lastCalculatedDepth = -1;

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
        // Time control: check periodically (every N nodes) to avoid timeout
        // Use a different offset to stagger checks between Minimax and Quiesce
        if ((_nodesSearched & (TimeCheckInterval - 1)) == (TimeCheckInterval / 2))
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
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check horizontal
                var count = 1;
                for (int dy = 1; dy <= 4 && y + dy < 19; dy++)
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
                for (int dx = 1; dx <= 4 && x + dx < 19; dx++)
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
                for (int i = 1; i <= 4 && x + i < 19 && y + i < 19; i++)
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
                for (int i = 1; i <= 4 && x + i < 19 && y - i >= 0; i++)
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (playerBitBoard.GetBit(nx, ny)) playerCount++;
                else if (!occupied.GetBit(nx, ny)) { playerOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) { oppOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
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
            for (int x = 0; x < 19; x++)
            {
                for (int y = 0; y < 19; y++)
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
    /// Get candidate moves (empty cells near existing stones)
    /// Zero-allocation implementation using stackalloc for tracking
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        const int boardSize = 19;
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
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: creates 4+ or open 3
            if (count >= 4) return true; // Potential winning move
            if (count == 3)
            {
                // Check if both ends are open
                bool leftOpen = x - dx >= 0 && x - dx < 19 && y - dy >= 0 && y - dy < 19
                               && !occupied.GetBit(x - dx, y - dy);
                bool rightOpen = x + dx * 3 >= 0 && x + dx * 3 < 19 && y + dy * 3 >= 0 && y + dy * 3 < 19
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count opponent stones in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: blocks opponent's 4 or open 3
            if (oppCount >= 4) return true; // Blocks winning threat
            if (oppCount == 3)
            {
                // Check if this blocks an open three
                var leftOpen = x - dx >= 0 && x - dx < 19 && y - dy >= 0 && y - dy < 19
                              && !occupied.GetBit(x - dx, y - dy);
                var rightOpen = x + dx * 3 >= 0 && x + dx * 3 < 19 && y + dy * 3 >= 0 && y + dy * 3 < 19
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
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
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 19 || ny < 0 || ny >= 19) break;
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
    /// Stop any active pondering and publish ponder stats
    /// OBSOLETE: Use StopPondering(Player forPlayer) for explicit player color
    /// </summary>
    [Obsolete("Use StopPondering(Player forPlayer) for explicit player color")]
    public void StopPondering()
    {
        _ponderer.StopPondering();
        // Can't publish stats without knowing player color - this method shouldn't be used
    }

    /// <summary>
    /// Stop any active pondering and publish ponder stats with explicit player color
    /// </summary>
    public void StopPondering(Player forPlayer)
    {
        _ponderer.StopPondering();
        PublishPonderStats(forPlayer);
    }

    /// <summary>
    /// Start pondering for opponent's response (called at start of opponent's turn)
    /// OBSOLETE: Use StartPonderingNow() with explicit parameters
    /// </summary>
    [Obsolete("Use StartPonderingNow(Board, Player currentPlayerToMove, AIDifficulty difficulty, Player thisAIColor)")]
    public void StartPonderingForOpponent(Board board, Player opponentToMove)
    {
        // This method should not be used - it relies on internal state
        throw new InvalidOperationException("Use StartPonderingNow with explicit parameters");
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
    public (int DepthAchieved, long NodesSearched, double NodesPerSecond, double TableHitRate, bool PonderingActive, int VCFDepthAchieved, long VCFNodesSearched, int ThreadCount, string? ParallelDiagnostics, double MasterTTPercent, double HelperAvgDepth, long AllocatedTimeMs) GetSearchStatistics()
    {
        double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
        var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
        double nps = elapsedMs > 0 ? (double)_nodesSearched * 1000 / elapsedMs : 0;

        // Parse % from master and helper avg depth from diagnostics string
        double masterTTPercent = 0;
        double helperAvgDepth = 0;

        if (!string.IsNullOrEmpty(_lastParallelDiagnostics))
        {
            // Parse "XX% from master" from TT part
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

        return (_depthAchieved, _nodesSearched, nps, hitRate, _lastPonderingEnabled, _vcfDepthAchieved, _vcfNodesSearched, _lastThreadCount, _lastParallelDiagnostics, masterTTPercent, helperAvgDepth, _lastAllocatedTimeMs);
    }

    /// <summary>
    /// Publish search statistics to the stats channel
    /// Called automatically after each search
    /// </summary>
    public void PublishSearchStats(Player player, StatsType statsType, long moveTimeMs)
    {
        var (depthAchieved, nodesSearched, nps, hitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched, threadCount, _, masterTTPercent, helperAvgDepth, allocatedTimeMs) = GetSearchStatistics();

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
            AllocatedTimeMs = allocatedTimeMs
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
