using Caro.Core.Entities;
using System.Diagnostics;
using System.Threading.Tasks;

namespace Caro.Core.GameLogic.Pondering;

/// <summary>
/// Manages pondering (thinking during opponent's turn) with state machine approach
/// Uses PV-based prediction with time merging on ponder hit
/// Uses Lazy SMP (parallel search) for effective pondering on multi-core systems
///
/// CONSTANT PONDERING for D7+:
/// - D7+ (VeryHard and above): Always ponders regardless of position
/// - D1-D6: Only ponders when there are immediate threats (VCF pre-check enabled)
/// </summary>
public sealed class Ponderer : IDisposable
{
    // Dependencies
    private readonly VCFPrecheck _vcfPrecheck = new();
    private readonly ParallelMinimaxSearch _parallelSearch = new();

    // State management - use lock for all access (removed volatile)
    private PonderState _state = PonderState.Idle;
    private readonly object _stateLock = new();

    // Pondering context
    private Board? _ponderBoard;
    private (int x, int y)? _predictedMove;
    private Player _ponderingForPlayer;  // Player we're pondering FOR (not TO MOVE)
    private Player _playerToMove;         // Player who is to move in pondered position
    private AIDifficulty _difficulty;
    private long _ponderStartTimeTicks;
    private long _maxPonderTimeMs;

    // Time tracking for merging
    private long _totalPonderTimeMs;
    private PonderResult _currentResult;

    // Cache last progress result for use when cancelled
    private (int x, int y)? _lastProgressMove;
    private int _lastProgressDepth;
    private int _lastProgressScore;

    // Cancellation - use lock for all access (removed volatile)
    private bool _shouldStop;
    private bool _allowFinalResultUpdate;  // Allow final result update even after stopping
    private CancellationTokenSource? _cts;
    private Task? _ponderTask;

    // Statistics
    private long _totalPonderHits;
    private long _totalPonderMisses;
    private long _totalPonderTimeMsAll;

    /// <summary>
    /// Current pondering state (thread-safe with lock)
    /// </summary>
    public PonderState State
    {
        get
        {
            lock (_stateLock)
            {
                return _state;
            }
        }
    }

    /// <summary>
    /// Predicted move being pondered
    /// </summary>
    public (int x, int y)? PredictedMove => _predictedMove;

    /// <summary>
    /// Statistics: Total ponder hits
    /// </summary>
    public long TotalPonderHits => _totalPonderHits;

    /// <summary>
    /// Statistics: Total ponder misses
    /// </summary>
    public long TotalPonderMisses => _totalPonderMisses;

    /// <summary>
    /// Statistics: Total time spent pondering in milliseconds
    /// </summary>
    public long TotalPonderTimeMs => _totalPonderTimeMsAll;

    /// <summary>
    /// Statistics: Ponder hit rate (0.0 to 1.0)
    /// </summary>
    public double PonderHitRate
    {
        get
        {
            var total = _totalPonderHits + _totalPonderMisses;
            return total > 0 ? (double)_totalPonderHits / total : 0.0;
        }
    }

    /// <summary>
    /// Start pondering based on predicted opponent move
    /// Spawns a background task that uses Lazy SMP to search with the predicted move
    ///
    /// CONSTANT PONDERING for D7+:
    /// - D7+ (VeryHard and above): Always ponders regardless of position
    /// - D1-D6: Only ponders when there are immediate threats (VCF pre-check enabled)
    /// </summary>
    /// <param name="currentBoard">Board state before opponent moves</param>
    /// <param name="opponentToMove">Player whose turn it is (opponent)</param>
    /// <param name="predictedOpponentMove">Predicted move from PV</param>
    /// <param name="ponderingForPlayer">Player who will move next (us)</param>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="maxPonderTimeMs">Maximum time to spend pondering</param>
    public void StartPondering(
        Board currentBoard,
        Player opponentToMove,
        (int x, int y)? predictedOpponentMove,
        Player ponderingForPlayer,
        AIDifficulty difficulty,
        long maxPonderTimeMs)
    {
        lock (_stateLock)
        {
            // Already pondering - don't interrupt
            if (_state == PonderState.Pondering)
                return;

            // Reset cancellation token
            _cts?.Dispose();
            _cts = new CancellationTokenSource();
            _shouldStop = false;

            // CONSTANT PONDERING for Medium (D3) and above: Skip VCF pre-check for higher difficulties
            // Medium+ always ponders regardless of position complexity
            // Braindead-Easy only ponder when there are immediate threats to save CPU
            bool isHighDifficulty = difficulty >= AIDifficulty.Medium;
            if (!isHighDifficulty)
            {
                // VCF pre-check - skip pondering if no immediate threats
                if (!_vcfPrecheck.IsOpeningPhase(currentBoard))
                {
                    var hasThreats = _vcfPrecheck.HasPotentialThreats(currentBoard, opponentToMove);
                    if (!hasThreats && _vcfPrecheck.GetThreatUrgency(currentBoard, opponentToMove) < 20)
                    {
                        _state = PonderState.Idle;
                        return;
                    }
                }
            }

            // Set up pondering state
            _state = PonderState.Pondering;
            _playerToMove = opponentToMove;
            _ponderingForPlayer = ponderingForPlayer;
            _predictedMove = predictedOpponentMove;
            _difficulty = difficulty;
            _ponderStartTimeTicks = Stopwatch.GetTimestamp();
            _maxPonderTimeMs = maxPonderTimeMs;

            // Reset cached progress
            _lastProgressMove = null;
            _lastProgressDepth = 0;
            _lastProgressScore = 0;

            // Clone board and make predicted move
            _ponderBoard = currentBoard.Clone();
            if (predictedOpponentMove.HasValue)
            {
                _ponderBoard.PlaceStone(
                    predictedOpponentMove.Value.x,
                    predictedOpponentMove.Value.y,
                    opponentToMove
                );
            }

            // Start with empty result
            _currentResult = PonderResult.None;
        }

        // Spawn background pondering task (outside lock to prevent deadlock)
        StartPonderingTask();
    }

    /// <summary>
    /// Start the background pondering task using Lazy SMP
    /// </summary>
    private void StartPonderingTask()
    {
        var ponderBoard = _ponderBoard;
        var playerToMove = _playerToMove;
        var difficulty = _difficulty;
        var maxTime = _maxPonderTimeMs;
        var token = _cts?.Token ?? default;

        if (ponderBoard == null || playerToMove == Player.None)
            return;

        // Store local references for the closure
        var localPonderBoard = ponderBoard.Clone();
        var localPonderingForPlayer = _ponderingForPlayer;

        _ponderTask = Task.Run(() =>
        {
            ((int x, int y)? bestMove, int depth, int score, long nodesSearched) result = (null, 0, 0, 0);
            try
            {
                // Use Lazy SMP parallel search for pondering
                result = _parallelSearch.PonderLazySMP(
                    localPonderBoard,
                    playerToMove,
                    difficulty,
                    maxTime,
                    token,
                    progressCallback: (move) =>
                    {
                        // Cache last progress for use when cancelled
                        lock (_stateLock)
                        {
                            _lastProgressMove = (move.Item1, move.Item2);
                            _lastProgressDepth = move.Item3;
                            _lastProgressScore = move.Item4;
                        }

                        // Update result as search progresses
                        if (_state == PonderState.Pondering && !ShouldStopPondering)
                        {
                            UpdatePonderResult((move.Item1, move.Item2), move.Item3, move.Item4, 0);
                        }
                    },
                    ponderingFor: localPonderingForPlayer
                );

                // Final update with complete result
                // Always update even if pondering was stopped - this captures the final depth achieved
                if (result.bestMove.HasValue)
                {
                    UpdatePonderResult(
                        result.bestMove.Value,
                        result.depth,
                        result.score,
                        result.nodesSearched
                    );
                }
            }
            catch (OperationCanceledException)
            {
                // Expected when pondering is cancelled - use cached progress result
                // Report at least 1 node to indicate pondering was active (even if we don't have exact count)
                lock (_stateLock)
                {
                    if (_lastProgressMove.HasValue)
                    {
                        UpdatePonderResult(
                            _lastProgressMove.Value,
                            _lastProgressDepth,
                            _lastProgressScore,
                            _currentResult.NodesSearched > 0 ? _currentResult.NodesSearched : 1
                        );
                    }
                    else if (_currentResult.Depth > 0)
                    {
                        // We have some result but no cached progress - use current result with min nodes
                        UpdatePonderResult(
                            _currentResult.BestMove ?? (0, 0),
                            _currentResult.Depth,
                            _currentResult.Score,
                            1
                        );
                    }
                }
            }
            catch (Exception)
            {
                // Ignore other exceptions during pondering
            }
        }, token);
    }

    /// <summary>
    /// Signal that pondering should stop and capture final stats
    /// </summary>
    public void StopPondering()
    {
        PonderState currentState;
        lock (_stateLock)
        {
            currentState = _state;
            if (currentState != PonderState.Pondering)
                return;

            _shouldStop = true;
            _allowFinalResultUpdate = true;  // IMPORTANT: Set BEFORE state change to prevent race condition
            _parallelSearch.StopSearch();
            _cts?.Cancel();
            _state = PonderState.Cancelled;
        }

        // Wait for task to complete (outside lock) - longer timeout to allow final results
        try
        {
            _ponderTask?.Wait(TimeSpan.FromMilliseconds(500));
        }
        catch (AggregateException)
        {
            // Task was cancelled - this is expected
        }

        // Now capture final stats AFTER task completes
        long elapsedMs;
        long finalNodesSearched;
        int finalDepth;

        lock (_stateLock)
        {
            elapsedMs = Stopwatch.GetElapsedTime(_ponderStartTimeTicks).Milliseconds;
            // Use nodes from _currentResult (set by UpdatePonderResult during search)
            // GetRealNodesSearched() is unreliable as the shared counter may not be updated
            finalNodesSearched = _currentResult.NodesSearched;
            finalDepth = _currentResult.Depth;
            _allowFinalResultUpdate = false;  // Prevent further updates

            // Update result with final time spent (keep existing depth and nodes from search)
            _currentResult = new PonderResult
            {
                BestMove = _currentResult.BestMove,
                Depth = finalDepth,
                Score = _currentResult.Score,
                TimeSpentMs = elapsedMs,
                FinalState = PonderState.Cancelled,
                PonderHit = false,
                NodesSearched = finalNodesSearched
            };
        }

        // Debug logging - simplified format
        Console.WriteLine($"[PONDER {_ponderingForPlayer}] Stop: {finalNodesSearched:N0} nodes, {elapsedMs}ms, depth {finalDepth}");
    }

    /// <summary>
    /// Handle opponent's move - determine ponder hit or miss
    /// Call this when the opponent actually makes a move
    /// Waits for ponder task to complete before returning
    /// </summary>
    /// <param name="actualX">Actual opponent move X coordinate</param>
    /// <param name="actualY">Actual opponent move Y coordinate</param>
    /// <returns>Tuple of (new state, ponder result if available)</returns>
    public (PonderState state, PonderResult? result) HandleOpponentMove(int actualX, int actualY)
    {
        lock (_stateLock)
        {
            // Not pondering - nothing to do
            if (_state != PonderState.Pondering)
                return (_state, null);
        }

        // Wait for ponder task to complete (short timeout to avoid blocking)
        try
        {
            if (_ponderTask != null && !_ponderTask.IsCompleted)
            {
                _ponderTask.Wait(TimeSpan.FromMilliseconds(50));
            }
        }
        catch (AggregateException)
        {
            // Task was cancelled or threw - ignore
        }

        lock (_stateLock)
        {
            // Double-check state after waiting
            if (_state != PonderState.Pondering)
                return (_state, null);

            // Check if opponent played the predicted move
            var isHit = _predictedMove.HasValue &&
                        _predictedMove.Value.x == actualX &&
                        _predictedMove.Value.y == actualY;

            // Calculate time spent pondering so far
            var elapsedMs = Stopwatch.GetElapsedTime(_ponderStartTimeTicks).Milliseconds;
            _totalPonderTimeMs = elapsedMs;
            _totalPonderTimeMsAll += elapsedMs;

            if (isHit)
            {
                // PONDER HIT - opponent played expected move
                _state = PonderState.PonderHit;
                _totalPonderHits++;

                var hitResult = new PonderResult
                {
                    BestMove = _currentResult.BestMove,
                    Depth = _currentResult.Depth,
                    Score = _currentResult.Score,
                    TimeSpentMs = elapsedMs,
                    FinalState = PonderState.PonderHit,
                    PonderHit = true,
                    NodesSearched = _currentResult.NodesSearched
                };

                return (PonderState.PonderHit, hitResult);
            }
            else
            {
                // PONDER MISS - opponent played different move
                _state = PonderState.PonderMiss;
                _totalPonderMisses++;
                StopPondering();

                var missResult = new PonderResult
                {
                    BestMove = null,
                    Depth = 0,
                    Score = 0,
                    TimeSpentMs = elapsedMs,
                    FinalState = PonderState.PonderMiss,
                    PonderHit = false,
                    NodesSearched = _currentResult.NodesSearched
                };

                return (PonderState.PonderMiss, missResult);
            }
        }
    }

    /// <summary>
    /// Update the current pondering result (called by search during pondering)
    /// Uses lock to safely check state
    /// Only updates depth if a non-zero value is provided (preserves depth from previous updates)
    /// </summary>
    public void UpdatePonderResult((int x, int y) bestMove, int depth, int score, long nodesSearched)
    {
        lock (_stateLock)
        {
            // Allow update if pondering OR if we're allowing final result update (after stop)
            if (_state != PonderState.Pondering && !_allowFinalResultUpdate)
                return;

            _currentResult = new PonderResult
            {
                BestMove = bestMove,
                Depth = depth > 0 ? depth : _currentResult.Depth,
                Score = score,
                TimeSpentMs = Stopwatch.GetElapsedTime(_ponderStartTimeTicks).Milliseconds,
                FinalState = _state,
                PonderHit = false,
                NodesSearched = nodesSearched
            };
        }
    }

    /// <summary>
    /// Check if pondering should stop (called by search during pondering)
    /// Uses lock for thread-safe access to _shouldStop
    /// </summary>
    public bool ShouldStopPondering
    {
        get
        {
            lock (_stateLock)
            {
                return _shouldStop || _cts?.IsCancellationRequested == true;
            }
        }
    }

    /// <summary>
    /// Get the board being pondered (with predicted move already made)
    /// </summary>
    public Board? GetPonderBoard() => _ponderBoard;

    /// <summary>
    /// Get the player who is to move in the pondered position
    /// </summary>
    public Player GetPlayerToMove() => _playerToMove;

    /// <summary>
    /// Get the current ponder result
    /// </summary>
    public PonderResult GetCurrentResult() => _currentResult;

    /// <summary>
    /// Get total pondering time to merge with main search time
    /// </summary>
    public long GetPonderTimeToMerge() => _totalPonderTimeMs;

    /// <summary>
    /// Get cancellation token for pondering
    /// </summary>
    public CancellationToken GetCancellationToken() => _cts?.Token ?? default;

    /// <summary>
    /// Check if pondering is currently active (thread-safe)
    /// </summary>
    public bool IsPondering
    {
        get
        {
            lock (_stateLock)
            {
                return _state == PonderState.Pondering;
            }
        }
    }

    /// <summary>
    /// Reset state for new game or after ponder miss
    /// </summary>
    public void Reset()
    {
        lock (_stateLock)
        {
            StopPondering();
            _ponderBoard = null;
            _predictedMove = null;
            _totalPonderTimeMs = 0;
            _currentResult = PonderResult.None;
            _ponderTask = null;
            _state = PonderState.Idle;
        }
    }

    /// <summary>
    /// Get pondering statistics as a formatted string
    /// </summary>
    public string GetStatistics()
    {
        var total = _totalPonderHits + _totalPonderMisses;
        var hitRate = total > 0 ? (double)_totalPonderHits / total * 100 : 0;
        return $"Pondering: {_totalPonderHits}/{total} hits ({hitRate:F1}%), {_totalPonderTimeMsAll / 1000.0:F1}s total";
    }

    public void Dispose()
    {
        StopPondering();
        _cts?.Dispose();
        // Don't dispose the Task directly - it may still be running
        // The GC will clean it up once it completes
        _ponderTask = null;
        // Board is not disposable - just clear the reference
        _ponderBoard = null;
    }
}
