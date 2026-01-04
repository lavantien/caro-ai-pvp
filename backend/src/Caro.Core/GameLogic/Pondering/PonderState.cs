namespace Caro.Core.GameLogic.Pondering;

/// <summary>
/// Pondering state machine - manages thinking during opponent's turn
/// State flow: IDLE -> PONDERING -> (PONDER_HIT | PONDER_MISS) -> IDLE
/// </summary>
public enum PonderState
{
    /// <summary>
    /// Not pondering - waiting for turn
    /// </summary>
    Idle,

    /// <summary>
    /// Currently thinking on opponent's time with predicted move
    /// </summary>
    Pondering,

    /// <summary>
    /// Opponent played the expected move - continue search with merged time
    /// </summary>
    PonderHit,

    /// <summary>
    /// Opponent played a different move - cancel and restart search
    /// </summary>
    PonderMiss,

    /// <summary>
    /// Pondering was cancelled (game ended, timeout, etc.)
    /// </summary>
    Cancelled
}

/// <summary>
/// Result of a pondering operation
/// </summary>
public readonly struct PonderResult
{
    /// <summary>
    /// Best move found during pondering (if search completed)
    /// </summary>
    public (int x, int y)? BestMove { get; init; }

    /// <summary>
    /// Depth of search that produced this result
    /// </summary>
    public int Depth { get; init; }

    /// <summary>
    /// Score of the best move from searching player's perspective
    /// </summary>
    public int Score { get; init; }

    /// <summary>
    /// Time spent pondering in milliseconds
    /// </summary>
    public long TimeSpentMs { get; init; }

    /// <summary>
    /// Final state when pondering ended
    /// </summary>
    public PonderState FinalState { get; init; }

    /// <summary>
    /// Whether the ponder hit (opponent played predicted move)
    /// </summary>
    public bool PonderHit { get; init; }

    /// <summary>
    /// Number of nodes searched during pondering (for statistics)
    /// </summary>
    public long NodesSearched { get; init; }

    /// <summary>
    /// Whether the result has a valid move
    /// </summary>
    public bool HasValidMove => BestMove.HasValue &&
                                 BestMove.Value.x >= 0 &&
                                 BestMove.Value.y >= 0;

    /// <summary>
    /// Create an empty/failed ponder result
    /// </summary>
    public static PonderResult None => new()
    {
        BestMove = null,
        Depth = 0,
        Score = 0,
        TimeSpentMs = 0,
        FinalState = PonderState.Idle,
        PonderHit = false,
        NodesSearched = 0
    };

    /// <summary>
    /// Create a cancelled ponder result
    /// </summary>
    public static PonderResult Cancelled(long timeSpentMs, long nodesSearched = 0) => new()
    {
        BestMove = null,
        Depth = 0,
        Score = 0,
        TimeSpentMs = timeSpentMs,
        FinalState = PonderState.Cancelled,
        PonderHit = false,
        NodesSearched = nodesSearched
    };
}
