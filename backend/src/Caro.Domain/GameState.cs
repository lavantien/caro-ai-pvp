namespace Caro.Domain;

/// <summary>
/// Immutable game state. Transitions return a new GameState; error paths
/// throw the domain exception for the caller to map.
/// </summary>
public sealed class GameState
{
    public GameState(Board board, Player currentPlayer, int moveNumber, bool isGameOver,
        Player winner, Position[]? winningLine, string endReason,
        Board[] boardHistory, Position[] moveHistory,
        string timeControl, long initialTimeMs, int incrementSeconds, GameMode gameMode)
    {
        Board = board;
        CurrentPlayer = currentPlayer;
        MoveNumber = moveNumber;
        IsGameOver = isGameOver;
        Winner = winner;
        WinningLine = winningLine;
        EndReason = endReason;
        BoardHistory = boardHistory;
        MoveHistory = moveHistory;
        TimeControl = timeControl;
        InitialTimeMs = initialTimeMs;
        IncrementSeconds = incrementSeconds;
        GameMode = gameMode;
    }

    public Board Board { get; }
    public Player CurrentPlayer { get; }
    public int MoveNumber { get; }
    public bool IsGameOver { get; }
    public Player Winner { get; }
    public Position[]? WinningLine { get; }
    public string EndReason { get; }
    public Board[] BoardHistory { get; }
    public Position[] MoveHistory { get; }
    public string TimeControl { get; }
    public long InitialTimeMs { get; }
    public int IncrementSeconds { get; }
    public GameMode GameMode { get; }

    public static GameState NewGameState(GameMode mode, string timeControl, long initialTimeMs, int incrementSeconds) =>
        new(Board.NewBoard(), Player.Red, 0, isGameOver: false, Player.None,
            winningLine: null, endReason: string.Empty,
            boardHistory: [], moveHistory: [],
            timeControl, initialTimeMs, incrementSeconds, mode);

    public GameState WithMove(int x, int y)
    {
        if (IsGameOver)
        {
            throw new GameOverException();
        }
        if (CurrentPlayer == Player.Red && !OpenRule.IsValidSecondMove(Board, x, y))
        {
            throw new OpenRuleException();
        }
        Board newBoard = Board.PlaceStone(x, y, CurrentPlayer);

        Board[] history = new Board[BoardHistory.Length + 1];
        history[0] = Board;
        Array.Copy(BoardHistory, 0, history, 1, BoardHistory.Length);

        Position[] moveHistory = new Position[MoveHistory.Length + 1];
        Array.Copy(MoveHistory, moveHistory, MoveHistory.Length);
        moveHistory[MoveHistory.Length] = new Position(x, y);

        return new GameState(newBoard, CurrentPlayer.Opponent(), MoveNumber + 1,
            isGameOver: false, Player.None, winningLine: null, endReason: string.Empty,
            history, moveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode);
    }

    public GameState UndoMove()
    {
        if (IsGameOver)
        {
            throw new GameOverException();
        }
        if (BoardHistory.Length == 0)
        {
            throw new NoMovesException();
        }

        Board previousBoard = BoardHistory[0];
        Board[] newHistory = BoardHistory[1..];
        Position[] newMoveHistory = MoveHistory[..^1];

        Player newPlayer = CurrentPlayer.Opponent();
        if (MoveNumber - 1 == 0)
        {
            newPlayer = Player.Red;
        }

        return new GameState(previousBoard, newPlayer, MoveNumber - 1,
            isGameOver: false, Player.None, winningLine: null, endReason: string.Empty,
            newHistory, newMoveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode);
    }

    public bool CanUndo() => BoardHistory.Length > 0 && !IsGameOver;

    public GameState WithGameOver(Player winner, Position[]? line)
    {
        return new GameState(Board, Player.None, MoveNumber, isGameOver: true, winner, line,
            endReason: EndReasons.Win, BoardHistory, MoveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode);
    }

    /// <summary>Ends the game because a player ran out of clock time.</summary>
    public GameState WithTimeout(Player winner)
    {
        return new GameState(Board, Player.None, MoveNumber, isGameOver: true, winner,
            winningLine: null, endReason: EndReasons.Timeout,
            BoardHistory, MoveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode);
    }

    /// <summary>Ends the game without a winner (board full).</summary>
    public GameState WithDraw()
    {
        return new GameState(Board, Player.None, MoveNumber, isGameOver: true, Player.None,
            winningLine: null, endReason: EndReasons.Draw,
            BoardHistory, MoveHistory, TimeControl, InitialTimeMs, IncrementSeconds, GameMode);
    }
}
