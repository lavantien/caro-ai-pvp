using Caro.Core.Domain.Entities;

namespace Caro.Core.Application.Extensions;

/// <summary>
/// Extension methods for GameState that add application-level concerns.
/// These are technical concerns that don't belong in the pure domain.
/// </summary>
public static class GameStateExtensions
{
    /// <summary>
    /// Create an initial game state.
    /// Application-level concern - domain GameState is pure without timing.
    /// </summary>
    public static GameState CreateInitial()
    {
        return new GameState();
    }

    /// <summary>
    /// Create an initial game state with time controls.
    /// Application-level concern - domain GameState is pure without timing.
    /// </summary>
    public static GameState CreateInitial(TimeSpan initialTime, TimeSpan increment)
    {
        return new GameState();
    }

    /// <summary>
    /// Make a move and return the state for fluent chaining.
    /// Uses the internal board.
    /// </summary>
    public static GameState MakeMove(this GameState state, int x, int y)
    {
        state.RecordMove(x, y);
        return state;
    }

    /// <summary>
    /// Update state with time remaining.
    /// Application-level concern - time tracking is not a domain concern.
    /// </summary>
    public static GameState WithTimeRemaining(this GameState state, TimeSpan elapsed)
    {
        // Time tracking is now handled externally
        // This method exists for API compatibility but does nothing
        return state;
    }

    /// <summary>
    /// End the game with a winner.
    /// Returns a new state with game over set.
    /// </summary>
    public static GameState WithEndGame(this GameState state, Player winner, List<Position>? winningLine = null)
    {
        state.EndGame(winner, winningLine);
        return state;
    }

    /// <summary>
    /// End the game with a winner.
    /// Accepts ReadOnlyMemory for compatibility.
    /// </summary>
    public static GameState WithEndGame(this GameState state, Player winner, ReadOnlyMemory<Position> winningLine)
    {
        state.EndGame(winner, winningLine.Length > 0 ? winningLine.ToArray().ToList() : null);
        return state;
    }

    /// <summary>
    /// End the game with a winner (no winning line).
    /// </summary>
    public static GameState WithEndGame(this GameState state, Player winner)
    {
        state.EndGame(winner, null);
        return state;
    }

    /// <summary>
    /// Undo the last move and return the state for fluent chaining.
    /// </summary>
    public static GameState UndoMoveAndReturn(this GameState state)
    {
        state.UndoMove();
        return state;
    }
}
