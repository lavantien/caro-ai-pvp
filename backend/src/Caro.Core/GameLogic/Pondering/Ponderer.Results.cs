using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using System.Diagnostics;

namespace Caro.Core.GameLogic.Pondering;

/// <summary>
/// Result and statistics methods for Ponderer partial class
/// </summary>
public sealed partial class Ponderer
{
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
    /// Check if we have a valid ponder hit result ready to use.
    /// This should be called BEFORE any other ponder operations to check state.
    /// </summary>
    public bool HasPonderHitResult
    {
        get
        {
            lock (_stateLock)
            {
                return _state == PonderState.PonderHit &&
                       _currentResult.BestMove.HasValue &&
                       _currentResult.Depth > 0;
            }
        }
    }

    /// <summary>
    /// Get the ponder result immediately on ponder hit.
    /// The ponder search runs during the opponent's turn, so by the time it's our turn,
    /// we should have a result. We do NOT wait - ponder time is "free" (precomputation).
    /// </summary>
    /// <returns>Current ponder result, or None if not a ponder hit</returns>
    public PonderResult GetPonderHitResult()
    {
        lock (_stateLock)
        {
            // Only valid for ponder hit state
            if (_state != PonderState.PonderHit)
                return PonderResult.None;

            // Return current result immediately - don't wait!
            // The ponder search has been running during opponent's turn.
            // Ponder time is "free" precomputation.
            return _currentResult;
        }
    }

    /// <summary>
    /// Get total pondering time to merge with main search time
    /// </summary>
    public long GetPonderTimeToMerge() => _totalPonderTimeMs;

    /// <summary>
    /// Get cancellation token for pondering
    /// </summary>
    public CancellationToken GetCancellationToken() => _cts?.Token ?? default;

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
                _ponderTask.Wait(TimeSpan.FromMilliseconds(TimeConstants.PonderDisposalTimeoutMs));
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
}
