using Caro.Core.Domain.Entities;

namespace Caro.Core.Domain.Exceptions;

/// <summary>
/// Exception thrown when an invalid move is attempted
/// </summary>
public sealed class InvalidMoveException : InvalidOperationException
{
    /// <summary>
    /// X coordinate of the invalid move
    /// </summary>
    public int? X { get; }

    /// <summary>
    /// Y coordinate of the invalid move
    /// </summary>
    public int? Y { get; }

    /// <summary>
    /// Player who attempted the invalid move
    /// </summary>
    public Player? Player { get; }

    public InvalidMoveException(string message) : base(message)
    {
    }

    public InvalidMoveException(string message, int x, int y) : base(message)
    {
        X = x;
        Y = y;
    }

    public InvalidMoveException(string message, int x, int y, Player player) : base(message)
    {
        X = x;
        Y = y;
        Player = player;
    }

    public InvalidMoveException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
