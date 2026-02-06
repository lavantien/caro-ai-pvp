using System.Collections.Immutable;

namespace Caro.Core.Domain.Entities;

/// <summary>
/// Pure domain representation of the game state.
/// Contains only domain logic - no technical concerns like timing or AI state.
/// Fully immutable - all operations return new instances.
/// </summary>
public sealed record GameState(
    Board Board,
    Player CurrentPlayer,
    int MoveNumber,
    bool IsGameOver,
    Player Winner,
    ImmutableArray<Position> WinningLine,
    ImmutableStack<Board> BoardHistory,
    ImmutableArray<Position> MoveHistory)
{
    /// <summary>
    /// Create an initial game state.
    /// </summary>
    public static GameState CreateInitial() => new(
        new Board(),
        Player.Red,
        0,
        false,
        Player.None,
        ImmutableArray<Position>.Empty,
        ImmutableStack<Board>.Empty,
        ImmutableArray<Position>.Empty
    );

    /// <summary>
    /// Record a move and return a new game state.
    /// </summary>
    public GameState WithMove(int x, int y)
    {
        if (IsGameOver)
            throw new InvalidOperationException("Cannot make moves after game is over");

        var newBoard = Board.PlaceStone(x, y, CurrentPlayer);
        var newHistory = BoardHistory.Push(Board);
        var newMoveHistory = MoveHistory.Add(new Position(x, y));

        return this with
        {
            Board = newBoard,
            CurrentPlayer = CurrentPlayer.Opponent(),
            MoveNumber = MoveNumber + 1,
            BoardHistory = newHistory,
            MoveHistory = newMoveHistory
        };
    }

    /// <summary>
    /// Undo the last move and return a new game state.
    /// </summary>
    public GameState UndoMove()
    {
        if (IsGameOver)
            throw new InvalidOperationException("Cannot undo moves after game is over");

        if (BoardHistory.IsEmpty)
            throw new InvalidOperationException("No moves to undo");

        var newHistory = BoardHistory.Pop(out var previousBoard);
        var newMoveHistory = MoveHistory.RemoveAt(MoveHistory.Length - 1);

        // Determine the new current player after undo
        // Special case: when undoing to MoveNumber 0, reset to Red (first player)
        // Otherwise, keep the same player (undo typically removes the opponent's last move)
        Player newCurrentPlayer = (MoveNumber - 1) == 0
            ? Player.Red
            : CurrentPlayer;

        return this with
        {
            Board = previousBoard,
            CurrentPlayer = newCurrentPlayer,
            MoveNumber = MoveNumber - 1,
            BoardHistory = newHistory,
            MoveHistory = newMoveHistory
        };
    }

    /// <summary>
    /// Check if undo is possible.
    /// </summary>
    public bool CanUndo() => !BoardHistory.IsEmpty && !IsGameOver;

    /// <summary>
    /// End the game and return a new game state.
    /// </summary>
    public GameState WithGameOver(Player winner, ImmutableArray<Position>? winningLine = null)
    {
        return this with
        {
            IsGameOver = true,
            CurrentPlayer = Player.None,
            Winner = winner,
            WinningLine = winningLine ?? ImmutableArray<Position>.Empty
        };
    }

    /// <summary>
    /// Legacy RecordMove for backward compatibility.
    /// OBSOLETE: Use WithMove() instead which returns a new state.
    /// </summary>
    [Obsolete("Use WithMove() instead, which returns a new GameState instance.")]
    public GameState RecordMove(int x, int y) => WithMove(x, y);

    /// <summary>
    /// Legacy RecordMove for backward compatibility with external board.
    /// OBSOLETE: Use WithMove() instead.
    /// </summary>
    [Obsolete("Use WithMove() instead, which uses the internal board.")]
    public GameState RecordMove(Board board, int x, int y)
    {
        // For external board usage, we ignore the provided board and use internal state
        return WithMove(x, y);
    }

    /// <summary>
    /// Legacy EndGame for backward compatibility.
    /// OBSOLETE: Use WithGameOver() instead.
    /// </summary>
    [Obsolete("Use WithGameOver() instead, which returns a new GameState instance.")]
    public GameState EndGame(Player winner, List<Position>? winningLine = null) =>
        WithGameOver(winner, winningLine?.ToImmutableArray());
}
