using Caro.Core.Domain.Entities;
using System.Collections.Immutable;

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
        return GameState.CreateInitial();
    }

    /// <summary>
    /// Create an initial game state with time controls.
    /// Application-level concern - domain GameState is pure without timing.
    /// </summary>
    public static GameState CreateInitial(TimeSpan initialTime, TimeSpan increment)
    {
        return GameState.CreateInitial();
    }

    /// <summary>
    /// Make a move and return the state for fluent chaining.
    /// Uses the internal board.
    /// </summary>
    public static GameState MakeMove(this GameState state, int x, int y)
    {
        return state.WithMove(x, y);
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
        return state.WithGameOver(winner, winningLine?.ToImmutableArray());
    }

    /// <summary>
    /// End the game with a winner.
    /// Accepts ReadOnlyMemory for compatibility.
    /// </summary>
    public static GameState WithEndGame(this GameState state, Player winner, ReadOnlyMemory<Position> winningLine)
    {
        return state.WithGameOver(winner, winningLine.Length > 0 ? winningLine.ToArray().ToImmutableArray() : null);
    }

    /// <summary>
    /// End the game with a winner (no winning line).
    /// </summary>
    public static GameState WithEndGame(this GameState state, Player winner)
    {
        return state.WithGameOver(winner, null);
    }

    /// <summary>
    /// Undo the last move and return the state for fluent chaining.
    /// </summary>
    public static GameState UndoMoveAndReturn(this GameState state)
    {
        return state.UndoMove();
    }
}
