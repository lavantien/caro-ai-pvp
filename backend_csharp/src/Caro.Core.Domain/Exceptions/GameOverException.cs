namespace Caro.Core.Domain.Exceptions;

/// <summary>
/// Exception thrown when an operation is attempted on a game that is already over
/// </summary>
public sealed class GameOverException : InvalidOperationException
{
    /// <summary>
    /// The winner of the game (if applicable)
    /// </summary>
    public Entities.Player? Winner { get; }

    public GameOverException(string message) : base(message)
    {
    }

    public GameOverException(string message, Entities.Player winner) : base(message)
    {
        Winner = winner;
    }

    public GameOverException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
