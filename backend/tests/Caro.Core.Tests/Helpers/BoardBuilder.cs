using Caro.Core.Domain.Entities;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Fluent builder for creating Board instances in tests.
/// Provides a clean, readable API for constructing test positions.
/// </summary>
public sealed class BoardBuilder
{
    private Board _board = new();

    /// <summary>
    /// Create a new builder with an empty board.
    /// </summary>
    public BoardBuilder()
    {
    }

    /// <summary>
    /// Create a new builder starting from an existing board.
    /// </summary>
    public BoardBuilder(Board board)
    {
        _board = board;
    }

    /// <summary>
    /// Place a stone at the given position for the specified player.
    /// </summary>
    public BoardBuilder PlaceStone(int x, int y, Player player)
    {
        _board = _board.PlaceStone(x, y, player);
        return this;
    }

    /// <summary>
    /// Place a stone at the given position for the specified player.
    /// </summary>
    public BoardBuilder PlaceStone(Position pos, Player player)
    {
        _board = _board.PlaceStone(pos, player);
        return this;
    }

    /// <summary>
    /// Place a Red stone at the given position.
    /// </summary>
    public BoardBuilder PlaceRed(int x, int y)
    {
        return PlaceStone(x, y, Player.Red);
    }

    /// <summary>
    /// Place a Red stone at the given position.
    /// </summary>
    public BoardBuilder PlaceRed(Position pos)
    {
        return PlaceStone(pos, Player.Red);
    }

    /// <summary>
    /// Place a Blue stone at the given position.
    /// </summary>
    public BoardBuilder PlaceBlue(int x, int y)
    {
        return PlaceStone(x, y, Player.Blue);
    }

    /// <summary>
    /// Place a Blue stone at the given position.
    /// </summary>
    public BoardBuilder PlaceBlue(Position pos)
    {
        return PlaceStone(pos, Player.Blue);
    }

    /// <summary>
    /// Place a line of stones in the specified direction.
    /// </summary>
    /// <param name="startX">Starting X coordinate</param>
    /// <param name="startY">Starting Y coordinate</param>
    /// <param name="dx">X direction (negative, zero, or positive)</param>
    /// <param name="dy">Y direction (negative, zero, or positive)</param>
    /// <param name="count">Number of stones to place</param>
    /// <param name="player">The player to place stones for</param>
    public BoardBuilder PlaceLine(int startX, int startY, int dx, int dy, int count, Player player)
    {
        for (int i = 0; i < count; i++)
        {
            int x = startX + (i * dx);
            int y = startY + (i * dy);
            _board = _board.PlaceStone(x, y, player);
        }
        return this;
    }

    /// <summary>
    /// Place a line of Red stones in the specified direction.
    /// </summary>
    public BoardBuilder PlaceRedLine(int startX, int startY, int dx, int dy, int count)
    {
        return PlaceLine(startX, startY, dx, dy, count, Player.Red);
    }

    /// <summary>
    /// Place a line of Blue stones in the specified direction.
    /// </summary>
    public BoardBuilder PlaceBlueLine(int startX, int startY, int dx, int dy, int count)
    {
        return PlaceLine(startX, startY, dx, dy, count, Player.Blue);
    }

    /// <summary>
    /// Place a horizontal line of stones.
    /// </summary>
    public BoardBuilder PlaceHorizontalLine(int startX, int y, int count, Player player)
    {
        return PlaceLine(startX, y, 1, 0, count, player);
    }

    /// <summary>
    /// Place a vertical line of stones.
    /// </summary>
    public BoardBuilder PlaceVerticalLine(int x, int startY, int count, Player player)
    {
        return PlaceLine(x, startY, 0, 1, count, player);
    }

    /// <summary>
    /// Place a diagonal line (top-left to bottom-right) of stones.
    /// </summary>
    public BoardBuilder PlaceDiagonalLine(int startX, int startY, int count, Player player)
    {
        return PlaceLine(startX, startY, 1, 1, count, player);
    }

    /// <summary>
    /// Place an anti-diagonal line (top-right to bottom-left) of stones.
    /// </summary>
    public BoardBuilder PlaceAntiDiagonalLine(int startX, int startY, int count, Player player)
    {
        return PlaceLine(startX, startY, -1, 1, count, player);
    }

    /// <summary>
    /// Build and return the configured board.
    /// </summary>
    public Board Build()
    {
        return _board;
    }
}
