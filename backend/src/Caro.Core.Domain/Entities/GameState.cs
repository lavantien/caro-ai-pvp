using Caro.Core.Domain.ValueObjects;
using Caro.Core.Domain.Exceptions;

namespace Caro.Core.Domain.Entities;

/// <summary>
/// Immutable game state containing all information about a game.
/// All state transitions return new instances.
/// Equality is based on the board hash, current player, move number, game over status, winner, and time remaining.
/// </summary>
public sealed class GameState : IEquatable<GameState>
{
    private readonly ulong _boardHash;
    private readonly Board _board;
    private readonly Player _currentPlayer;
    private readonly int _moveNumber;
    private readonly bool _isGameOver;
    private readonly Player _winner;
    private readonly ReadOnlyMemory<Position> _winningLine;
    private readonly TimeSpan _redTimeRemaining;
    private readonly TimeSpan _blueTimeRemaining;
    private readonly ReadOnlyMemory<AnnotatedMove> _moveHistory;

    /// <summary>
    /// Private constructor for factory methods
    /// </summary>
    private GameState(
        ulong boardHash,
        Board board,
        Player currentPlayer,
        int moveNumber,
        bool isGameOver,
        Player winner,
        ReadOnlyMemory<Position> winningLine,
        TimeSpan redTimeRemaining,
        TimeSpan blueTimeRemaining,
        ReadOnlyMemory<AnnotatedMove> moveHistory)
    {
        _boardHash = boardHash;
        _board = board;
        _currentPlayer = currentPlayer;
        _moveNumber = moveNumber;
        _isGameOver = isGameOver;
        _winner = winner;
        _winningLine = winningLine;
        _redTimeRemaining = redTimeRemaining;
        _blueTimeRemaining = blueTimeRemaining;
        _moveHistory = moveHistory;
    }

    /// <summary>
    /// Board hash for equality comparison
    /// </summary>
    public ulong BoardHash => _boardHash;

    /// <summary>
    /// Current board state
    /// </summary>
    public Board Board => _board;

    /// <summary>
    /// Player whose turn it is
    /// </summary>
    public Player CurrentPlayer => _currentPlayer;

    /// <summary>
    /// Number of moves made so far
    /// </summary>
    public int MoveNumber => _moveNumber;

    /// <summary>
    /// Whether the game is over
    /// </summary>
    public bool IsGameOver => _isGameOver;

    /// <summary>
    /// The winner (None if game is ongoing or draw)
    /// </summary>
    public Player Winner => _winner;

    /// <summary>
    /// The winning line (5 in a row), if applicable
    /// </summary>
    public ReadOnlyMemory<Position> WinningLine => _winningLine;

    /// <summary>
    /// Time remaining for Red player
    /// </summary>
    public TimeSpan RedTimeRemaining => _redTimeRemaining;

    /// <summary>
    /// Time remaining for Blue player
    /// </summary>
    public TimeSpan BlueTimeRemaining => _blueTimeRemaining;

    /// <summary>
    /// Move history with annotations
    /// </summary>
    public ReadOnlyMemory<AnnotatedMove> MoveHistory => _moveHistory;

    /// <summary>
    /// Create an initial game state
    /// </summary>
    public static GameState CreateInitial(
        TimeSpan initialTime,
        TimeSpan increment)
    {
        var emptyBoard = Board.CreateEmpty();
        return new GameState(
            emptyBoard.Hash,
            emptyBoard,
            Player.Red,
            0,
            false,
            Player.None,
            ReadOnlyMemory<Position>.Empty,
            initialTime,
            initialTime,
            ReadOnlyMemory<AnnotatedMove>.Empty
        );
    }

    /// <summary>
    /// Create an initial game state with default time control (7min + 5sec)
    /// </summary>
    public static GameState CreateInitial() =>
        CreateInitial(TimeSpan.FromMinutes(7), TimeSpan.FromSeconds(5));

    /// <summary>
    /// Return a new game state with the move applied
    /// </summary>
    public GameState MakeMove(int x, int y, AIStats? stats = null)
    {
        if (IsGameOver)
            throw new GameOverException("Cannot make moves after game is over");

        if (!Board.IsEmpty(x, y))
            throw new InvalidMoveException($"Cell ({x}, {y}) is already occupied");

        // Create new board with the move
        var newBoard = Board.PlaceStone(x, y, CurrentPlayer);

        // Update time (add increment to current player)
        var newRedTime = RedTimeRemaining;
        var newBlueTime = BlueTimeRemaining;
        var increment = TimeSpan.FromSeconds(5); // Default increment

        if (CurrentPlayer == Player.Red)
            newRedTime = RedTimeRemaining.Add(increment);
        else
            newBlueTime = BlueTimeRemaining.Add(increment);

        // Create annotated move
        var annotatedMove = new AnnotatedMove
        {
            Move = new Move(x, y, CurrentPlayer),
            MoveNumber = MoveNumber + 1,
            TimeRemainingMs = CurrentPlayer == Player.Red ? newRedTime.Milliseconds : newBlueTime.Milliseconds,
            TimeTakenMs = 0, // Will be set by caller
            Stats = stats
        };

        // Update move history
        var newHistory = new List<AnnotatedMove>(MoveHistory.Length + 1);
        newHistory.AddRange(MoveHistory.Span);
        newHistory.Add(annotatedMove);

        // Switch player
        var newPlayer = CurrentPlayer.Opponent();

        return new GameState(
            newBoard.Hash,
            newBoard,
            newPlayer,
            MoveNumber + 1,
            false,
            Player.None,
            ReadOnlyMemory<Position>.Empty,
            newRedTime,
            newBlueTime,
            newHistory.ToArray()
        );
    }

    /// <summary>
    /// Return a new game state with the move applied
    /// </summary>
    public GameState MakeMove(Move move, AIStats? stats = null) =>
        MakeMove(move.X, move.Y, stats);

    /// <summary>
    /// Return a new game state with the move applied
    /// </summary>
    public GameState MakeMove(Position position, AIStats? stats = null) =>
        MakeMove(position.X, position.Y, stats);

    /// <summary>
    /// Return a new game state ending the game
    /// </summary>
    public GameState EndGame(Player winner, ReadOnlyMemory<Position> winningLine)
    {
        return new GameState(
            Board.Hash,
            Board,
            Player.None,
            MoveNumber,
            true,
            winner,
            winningLine,
            RedTimeRemaining,
            BlueTimeRemaining,
            MoveHistory
        );
    }

    /// <summary>
    /// Return a new game state ending the game with a winner
    /// </summary>
    public GameState EndGame(Player winner) =>
        EndGame(winner, ReadOnlyMemory<Position>.Empty);

    /// <summary>
    /// Return a new game state ending the game (draw)
    /// </summary>
    public GameState EndGame() =>
        EndGame(Player.None, ReadOnlyMemory<Position>.Empty);

    /// <summary>
    /// Return a new game state with updated time for the current player
    /// </summary>
    public GameState WithTimeRemaining(TimeSpan timeRemaining)
    {
        if (CurrentPlayer == Player.Red)
        {
            return new GameState(
                Board.Hash,
                Board,
                CurrentPlayer,
                MoveNumber,
                IsGameOver,
                Winner,
                WinningLine,
                timeRemaining,
                BlueTimeRemaining,
                MoveHistory
            );
        }
        else
        {
            return new GameState(
                Board.Hash,
                Board,
                CurrentPlayer,
                MoveNumber,
                IsGameOver,
                Winner,
                WinningLine,
                RedTimeRemaining,
                timeRemaining,
                MoveHistory
            );
        }
    }

    /// <summary>
    /// Return a new game state with the last move undone
    /// </summary>
    public GameState UndoMove()
    {
        if (IsGameOver)
            throw new GameOverException("Cannot undo moves after game is over");

        if (MoveNumber == 0)
            throw new InvalidOperationException("No moves to undo");

        if (MoveHistory.Length == 0)
            throw new InvalidOperationException("No move history");

        // Get the last move
        var history = MoveHistory.Span;
        var lastMove = history[^1];

        // Remove the stone from the board
        var newBoard = Board.RemoveStone(lastMove.Move.X, lastMove.Move.Y);

        // Restore time (remove increment)
        var increment = TimeSpan.FromSeconds(5);
        var newRedTime = RedTimeRemaining;
        var newBlueTime = BlueTimeRemaining;

        // The player who made the move is the opponent of the current player
        var playerWhoMadeMove = CurrentPlayer.Opponent();
        if (playerWhoMadeMove == Player.Red)
            newRedTime = RedTimeRemaining.Subtract(increment);
        else
            newBlueTime = BlueTimeRemaining.Subtract(increment);

        // Update move history
        var newHistory = history[..^1].ToArray();

        // Restore current player
        var newPlayer = playerWhoMadeMove;

        return new GameState(
            newBoard.Hash,
            newBoard,
            newPlayer,
            MoveNumber - 1,
            false,
            Player.None,
            ReadOnlyMemory<Position>.Empty,
            newRedTime,
            newBlueTime,
            newHistory
        );
    }

    /// <summary>
    /// Check if undo is possible
    /// </summary>
    public bool CanUndo() => MoveNumber > 0 && !IsGameOver;

    /// <summary>
    /// Get the time remaining for the current player
    /// </summary>
    public TimeSpan GetCurrentPlayerTimeRemaining() =>
        CurrentPlayer == Player.Red ? RedTimeRemaining : BlueTimeRemaining;

    /// <summary>
    /// Override equality to only compare primary constructor parameters (not internal arrays)
    /// </summary>
    public bool Equals(GameState? other)
    {
        if (other is null) return false;
        if (ReferenceEquals(this, other)) return true;
        return BoardHash == other.BoardHash &&
               CurrentPlayer == other.CurrentPlayer &&
               MoveNumber == other.MoveNumber &&
               IsGameOver == other.IsGameOver &&
               Winner == other.Winner &&
               RedTimeRemaining == other.RedTimeRemaining &&
               BlueTimeRemaining == other.BlueTimeRemaining;
    }

    /// <summary>
    /// Override equality to only compare primary constructor parameters (not internal arrays)
    /// </summary>
    public override bool Equals(object? obj) => Equals(obj as GameState);

    /// <summary>
    /// Hash code based on primary constructor parameters
    /// </summary>
    public override int GetHashCode() => HashCode.Combine(
        BoardHash,
        CurrentPlayer,
        MoveNumber,
        IsGameOver,
        Winner,
        RedTimeRemaining,
        BlueTimeRemaining);
}
