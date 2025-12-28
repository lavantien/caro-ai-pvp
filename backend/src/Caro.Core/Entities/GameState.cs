namespace Caro.Core.Entities;

using Caro.Core.GameLogic;

public class GameState
{
    private const int InitialTimeMinutes = 3;
    private const int IncrementSeconds = 2;

    public Board Board { get; } = new Board();
    public Player CurrentPlayer { get; private set; } = Player.Red;
    public int MoveNumber { get; private set; } = 0;
    public bool IsGameOver { get; private set; } = false;
    public TimeSpan RedTimeRemaining { get; private set; } = TimeSpan.FromMinutes(InitialTimeMinutes);
    public TimeSpan BlueTimeRemaining { get; private set; } = TimeSpan.FromMinutes(InitialTimeMinutes);
    public Player Winner { get; private set; } = Player.None;
    public List<Position> WinningLine { get; private set; } = new();

    private readonly List<(int x, int y)> _moveHistory = new();

    public void RecordMove(Board board, int x, int y)
    {
        // Store the move in history before making it
        _moveHistory.Add((x, y));

        // Place stone with current player
        board.PlaceStone(x, y, CurrentPlayer);

        // Increment time for current player
        if (CurrentPlayer == Player.Red)
            RedTimeRemaining += TimeSpan.FromSeconds(IncrementSeconds);
        else
            BlueTimeRemaining += TimeSpan.FromSeconds(IncrementSeconds);

        MoveNumber++;
        CurrentPlayer = CurrentPlayer == Player.Red ? Player.Blue : Player.Red;
    }

    public void ApplyTimeIncrement()
    {
        // Time increment is now handled in RecordMove
        // This method is kept for backward compatibility but does nothing
    }

    public void EndGame(Player winner, List<Position>? winningLine = null)
    {
        IsGameOver = true;
        CurrentPlayer = Player.None;
        Winner = winner;
        WinningLine = winningLine ?? new List<Position>();
    }

    public void UndoMove(Board board)
    {
        if (IsGameOver)
            throw new InvalidOperationException("Cannot undo moves after game is over");

        if (MoveNumber == 0 || _moveHistory.Count == 0)
            throw new InvalidOperationException("No moves to undo");

        // Get the last move
        var (x, y) = _moveHistory[^1];
        _moveHistory.RemoveAt(_moveHistory.Count - 1);

        // Remove the stone from the board
        board.GetCell(x, y).Player = Player.None;

        // Restore the time increment
        // Odd MoveNumber: Red made the last move
        // Even MoveNumber: Blue made the last move
        var playerWhoMadeMove = MoveNumber % 2 == 1 ? Player.Red : Player.Blue;
        if (playerWhoMadeMove == Player.Red)
            RedTimeRemaining -= TimeSpan.FromSeconds(IncrementSeconds);
        else
            BlueTimeRemaining -= TimeSpan.FromSeconds(IncrementSeconds);

        // Decrement move number
        MoveNumber--;

        // Restore CurrentPlayer based on the new MoveNumber
        // Move 0: Red (initial state)
        // Move 1: Blue (Red just moved, now Blue's turn)
        // Move >= 2: Keep CurrentPlayer the same (no change needed)
        if (MoveNumber == 0)
            CurrentPlayer = Player.Red;
        else if (MoveNumber == 1)
            CurrentPlayer = Player.Blue;
        // MoveNumber >= 2: CurrentPlayer stays unchanged
    }

    public bool CanUndo()
    {
        return MoveNumber > 0 && !IsGameOver;
    }
}
