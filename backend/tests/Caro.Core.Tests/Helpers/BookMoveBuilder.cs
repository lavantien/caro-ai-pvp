using Caro.Core.Domain.Entities;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Fluent builder for creating BookMove test data.
/// Provides a clean, readable API for constructing test moves.
/// </summary>
public sealed class BookMoveBuilder
{
    private int _x;
    private int _y;
    private int _winRate = 50;
    private int _depthAchieved = 10;
    private long _nodesSearched = 1000;
    private int _score = 0;
    private bool _isForcing = false;
    private int _priority = 1;
    private bool _isVerified = true;

    /// <summary>
    /// Create a new builder with default values.
    /// </summary>
    public BookMoveBuilder()
    {
        _x = 9; // Center X
        _y = 9; // Center Y
    }

    /// <summary>
    /// Create a new builder with specified coordinates.
    /// </summary>
    public BookMoveBuilder(int x, int y)
    {
        _x = x;
        _y = y;
    }

    /// <summary>
    /// Set the coordinates for this move.
    /// </summary>
    public BookMoveBuilder WithCoordinates(int x, int y)
    {
        _x = x;
        _y = y;
        return this;
    }

    /// <summary>
    /// Set the win rate percentage (0-100).
    /// </summary>
    public BookMoveBuilder WithWinRate(int winRate)
    {
        _winRate = winRate;
        return this;
    }

    /// <summary>
    /// Set the search depth achieved (in plies).
    /// </summary>
    public BookMoveBuilder WithDepthAchieved(int depth)
    {
        _depthAchieved = depth;
        return this;
    }

    /// <summary>
    /// Set the number of nodes searched.
    /// </summary>
    public BookMoveBuilder WithNodesSearched(long nodes)
    {
        _nodesSearched = nodes;
        return this;
    }

    /// <summary>
    /// Set the evaluation score (in centipawns).
    /// </summary>
    public BookMoveBuilder WithScore(int score)
    {
        _score = score;
        return this;
    }

    /// <summary>
    /// Set whether this is a forcing move (VCF).
    /// </summary>
    public BookMoveBuilder WithForcing(bool isForcing)
    {
        _isForcing = isForcing;
        return this;
    }

    /// <summary>
    /// Set the move priority for selection.
    /// </summary>
    public BookMoveBuilder WithPriority(int priority)
    {
        _priority = priority;
        return this;
    }

    /// <summary>
    /// Set whether this move is verified.
    /// </summary>
    public BookMoveBuilder WithVerified(bool isVerified)
    {
        _isVerified = isVerified;
        return this;
    }

    /// <summary>
    /// Set this as a good move with positive score.
    /// </summary>
    public BookMoveBuilder AsGoodMove()
    {
        _score = 100;
        _winRate = 55;
        return this;
    }

    /// <summary>
    /// Set this as a bad move with negative score.
    /// </summary>
    public BookMoveBuilder AsBadMove()
    {
        _score = -100;
        _winRate = 45;
        return this;
    }

    /// <summary>
    /// Set this as a neutral/even move.
    /// </summary>
    public BookMoveBuilder AsEvenMove()
    {
        _score = 0;
        _winRate = 50;
        return this;
    }

    /// <summary>
    /// Build the BookMove with the configured values.
    /// </summary>
    public BookMove Build()
    {
        return new BookMove
        {
            RelativeX = _x,
            RelativeY = _y,
            WinRate = _winRate,
            DepthAchieved = _depthAchieved,
            NodesSearched = _nodesSearched,
            Score = _score,
            IsForcing = _isForcing,
            Priority = _priority,
            IsVerified = _isVerified
        };
    }

    /// <summary>
    /// Create a simple center move with default values.
    /// </summary>
    public static BookMove CenterMove() => new BookMoveBuilder(9, 9).Build();

    /// <summary>
    /// Create a move at specified coordinates with default values.
    /// </summary>
    public static BookMove At(int x, int y) => new BookMoveBuilder(x, y).Build();

    /// <summary>
    /// Create multiple moves at specified coordinates.
    /// </summary>
    public static BookMove[] Many(params (int x, int y)[] coordinates)
    {
        return coordinates.Select(c => At(c.x, c.y)).ToArray();
    }
}
