namespace Caro.Core.Entities;

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

    public void RecordMove(Board board, int x, int y)
    {
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

    public void EndGame(Player winner)
    {
        IsGameOver = true;
        CurrentPlayer = Player.None;
    }
}
