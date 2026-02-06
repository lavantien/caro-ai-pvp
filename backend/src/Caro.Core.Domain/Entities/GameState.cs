namespace Caro.Core.Domain.Entities;

/// <summary>
/// Pure domain representation of the game state.
/// Contains only domain logic - no technical concerns like timing or AI state.
/// Immutable operations where possible.
/// </summary>
public class GameState
{
    private Board _board = new();
    private readonly List<Board> _boardHistory = new();

    public Board Board => _board;
    public Player CurrentPlayer { get; private set; } = Player.Red;
    public int MoveNumber { get; private set; } = 0;
    public bool IsGameOver { get; private set; } = false;
    public Player Winner { get; private set; } = Player.None;
    public List<Position> WinningLine { get; private set; } = new();

    private readonly List<Position> _moveHistory = new();

    /// <summary>
    /// Get the move history as read-only list.
    /// </summary>
    public IReadOnlyList<Position> MoveHistory => _moveHistory;

    /// <summary>
    /// Record a move at the given position on an external board.
    /// For backward compatibility with code that manages Board separately.
    /// </summary>
    public void RecordMove(Board board, int x, int y)
    {
        if (IsGameOver)
            throw new InvalidOperationException("Cannot make moves after game is over");

        // Record move in history
        _moveHistory.Add(new Position(x, y));

        // Place stone on the provided board and update internal board
        _board = board.PlaceStone(x, y, CurrentPlayer);

        // Switch player
        MoveNumber++;
        CurrentPlayer = CurrentPlayer.Opponent();
    }

    /// <summary>
    /// Record a move at the given position on the internal board.
    /// </summary>
    public void RecordMove(int x, int y)
    {
        if (IsGameOver)
            throw new InvalidOperationException("Cannot make moves after game is over");

        // Save current board for undo
        _boardHistory.Add(_board);

        // Record move in history
        _moveHistory.Add(new Position(x, y));

        // Place stone on the board - returns new immutable board
        _board = _board.PlaceStone(x, y, CurrentPlayer);

        // Switch player
        MoveNumber++;
        CurrentPlayer = CurrentPlayer.Opponent();
    }

    /// <summary>
    /// End the game with a winner.
    /// </summary>
    public void EndGame(Player winner, List<Position>? winningLine = null)
    {
        IsGameOver = true;
        CurrentPlayer = Player.None;
        Winner = winner;
        WinningLine = winningLine ?? new List<Position>();
    }

    /// <summary>
    /// Undo the last move.
    /// </summary>
    public void UndoMove()
{
    if (IsGameOver)
        throw new InvalidOperationException("Cannot undo moves after game is over");

    if (MoveNumber == 0 || _moveHistory.Count == 0 || _boardHistory.Count == 0)
        throw new InvalidOperationException("No moves to undo");

    // Remove last move from history
    _moveHistory.RemoveAt(_moveHistory.Count - 1);

    // Restore previous board
    _board = _boardHistory[^1];
    _boardHistory.RemoveAt(_boardHistory.Count - 1);

    // Decrement move number
    MoveNumber--;

    // If back to initial state, reset to Red's turn
    // Otherwise, keep current player (same player can try a different move)
    if (MoveNumber == 0)
        CurrentPlayer = Player.Red;
}

    /// <summary>
    /// Check if undo is possible.
    /// </summary>
    public bool CanUndo() => MoveNumber > 0 && !IsGameOver && _boardHistory.Count > 0;

    /// <summary>
    /// Create a deep copy of the game state.
    /// </summary>
    public GameState Clone()
    {
        var clone = new GameState
        {
            _board = _board.Clone(),
            CurrentPlayer = CurrentPlayer,
            MoveNumber = MoveNumber,
            IsGameOver = IsGameOver,
            Winner = Winner,
            WinningLine = new List<Position>(WinningLine)
        };
        // Copy move history
        foreach (var move in _moveHistory)
            clone._moveHistory.Add(move);
        // Copy board history
        foreach (var board in _boardHistory)
            clone._boardHistory.Add(board.Clone());
        return clone;
    }
}
