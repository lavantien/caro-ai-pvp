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
    ImmutableArray<Position> MoveHistory,
    string TimeControl,                     // Time control name (e.g., "7+5")
    long InitialTimeMs,                     // Initial time per player in milliseconds
    int IncrementSeconds,                   // Time increment per move in seconds
    string GameMode                         // "pvp", "pvai", "aivai"
)
{
    /// <summary>
    /// Create an initial game state.
    /// </summary>
    public static GameState CreateInitial(
        string timeControl = "7+5",
        long initialTimeMs = 420_000,
        int incrementSeconds = 5,
        string gameMode = "pvp") => new(
        new Board(),
        Player.Red,
        0,
        false,
        Player.None,
        ImmutableArray<Position>.Empty,
        ImmutableStack<Board>.Empty,
        ImmutableArray<Position>.Empty,
        timeControl,
        initialTimeMs,
        incrementSeconds,
        gameMode
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
        // Otherwise, switch to opponent (undo reverts the last move, giving the turn back
        // to the player who made that move)
        Player newCurrentPlayer = (MoveNumber - 1) == 0
            ? Player.Red
            : CurrentPlayer.Opponent();

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

}
