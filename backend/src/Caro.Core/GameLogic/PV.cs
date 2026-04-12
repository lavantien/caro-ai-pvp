namespace Caro.Core.GameLogic;

/// <summary>
/// Principal Variation - sequence of best moves from current position
/// Used for pondering (thinking on opponent's time) prediction
/// </summary>
public readonly struct PV
{
    /// <summary>
    /// Sequence of moves in the principal variation
    /// Index 0 is the current player's best move
    /// Index 1 is the predicted opponent response (used for pondering)
    /// </summary>
    public readonly (int x, int y)[] Moves;

    /// <summary>
    /// Depth of search that produced this PV
    /// </summary>
    public int Depth { get; }

    /// <summary>
    /// Score of the PV from the current player's perspective
    /// </summary>
    public int Score { get; }

    public PV((int x, int y)[] moves, int depth, int score)
    {
        Moves = moves;
        Depth = depth;
        Score = score;
    }

    /// <summary>
    /// Get the predicted opponent move for pondering
    /// Returns the move at index 1 (opponent's response to our best move)
    /// </summary>
    public (int x, int y)? GetPredictedOpponentMove()
    {
        if (Moves.Length < 2)
            return null;

        // Index 1 is opponent's response (0 is our move)
        return Moves[1];
    }

    /// <summary>
    /// Get the best move for the current player
    /// </summary>
    public (int x, int y)? GetBestMove()
    {
        if (Moves.Length == 0)
            return null;

        return Moves[0];
    }

    /// <summary>
    /// Check if PV is empty
    /// </summary>
    public bool IsEmpty => Moves.Length == 0;

    /// <summary>
    /// Empty PV constant
    /// </summary>
    public static PV Empty => new(Array.Empty<(int, int)>(), 0, 0);

    /// <summary>
    /// Create a PV from a single move
    /// </summary>
    public static PV FromSingleMove(int x, int y, int depth, int score)
    {
        return new PV(new[] { (x, y) }, depth, score);
    }

    /// <summary>
    /// Create a PV from multiple moves
    /// </summary>
    public static PV FromMoves((int x, int y)[] moves, int depth, int score)
    {
        return new PV(moves, depth, score);
    }
}
