using Caro.Core.Domain.Entities;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Static factory methods for creating common test position scenarios.
/// Provides pre-configured boards for standard testing patterns.
/// </summary>
public static class TestPositions
{
    /// <summary>
    /// Create a board with an open three in a row (XXX__) horizontally.
    /// Both ends are open, making it a forcing threat.
    /// </summary>
    public static Board OpenThreeHorizontal(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(6, 7, 1, 0, 3, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open three in a row (XXX__) horizontally.
    /// Both ends are open, making it a forcing threat.
    /// </summary>
    public static Board OpenThreeHorizontal(Player player, int startX, int y)
    {
        return new BoardBuilder()
            .PlaceLine(startX, y, 1, 0, 3, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open three in a row vertically.
    /// </summary>
    public static Board OpenThreeVertical(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(7, 6, 0, 1, 3, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open three in a row diagonally.
    /// </summary>
    public static Board OpenThreeDiagonal(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(6, 6, 1, 1, 3, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open four in a row (XXXX_) horizontally.
    /// One end is open (the other implicitly by board bounds).
    /// </summary>
    public static Board OpenFourHorizontal(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 7, 1, 0, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open four in a row (XXXX_) horizontally.
    /// </summary>
    public static Board OpenFourHorizontal(Player player, int startX, int y)
    {
        return new BoardBuilder()
            .PlaceLine(startX, y, 1, 0, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open four in a row vertically.
    /// </summary>
    public static Board OpenFourVertical(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(7, 5, 0, 1, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with an open four in a row diagonally.
    /// </summary>
    public static Board OpenFourDiagonal(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 5, 1, 1, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with both ends open for a four-in-a-row (_XXXX_).
    /// This is a very strong threat with two completion squares.
    /// </summary>
    public static Board OpenFourBothEnds(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(6, 7, 1, 0, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a broken four (XXX_X) pattern.
    /// The gap can be filled to create a straight four.
    /// </summary>
    public static Board BrokenFour(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 7, 1, 0, 3, player)
            .PlaceStone(9, 7, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a broken three (XX_X) pattern.
    /// Both the gap and the far end are open.
    /// </summary>
    public static Board BrokenThree(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 7, 1, 0, 2, player)
            .PlaceStone(8, 7, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a double threat position.
    /// Two separate straight fours that cannot both be defended.
    /// </summary>
    public static Board DoubleThreat(Player player)
    {
        return new BoardBuilder()
            // First S4 horizontal at y=5
            .PlaceLine(5, 5, 1, 0, 4, player)
            // Second S4 vertical at x=3
            .PlaceLine(3, 7, 0, 1, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a double threat using specified positions.
    /// </summary>
    public static Board DoubleThreat(
        Player player,
        int threat1X, int threat1Y, int threat1Dx, int threat1Dy,
        int threat2X, int threat2Y, int threat2Dx, int threat2Dy)
    {
        return new BoardBuilder()
            .PlaceLine(threat1X, threat1Y, threat1Dx, threat1Dy, 4, player)
            .PlaceLine(threat2X, threat2Y, threat2Dx, threat2Dy, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a VCF (Victory by Continuous Forcing) position.
    /// Contains multiple forcing moves that lead to inevitable victory.
    /// </summary>
    public static Board VCFPosition()
    {
        // Classic VCF setup: forcing move creates double threat
        return new BoardBuilder()
            // Main threat line
            .PlaceLine(5, 7, 1, 0, 3, Player.Red)
            // Second threat line intersecting
            .PlaceLine(7, 5, 0, 1, 3, Player.Red)
            // One more stone to create the double threat opportunity
            .PlaceStone(7, 7, Player.Red)
            .Build();
    }

    /// <summary>
    /// Create a board with a fork position (three or more forcing moves).
    /// </summary>
    public static Board ForkPosition(Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        return new BoardBuilder()
            // Create a position with multiple S3 threats
            .PlaceLine(5, 5, 1, 0, 3, player)
            .PlaceLine(5, 7, 1, 0, 3, player)
            .PlaceLine(7, 5, 0, 1, 3, player)
            // Block some responses to simulate real game state
            .PlaceStone(9, 5, opponent)
            .PlaceStone(9, 7, opponent)
            .Build();
    }

    /// <summary>
    /// Create an empty board.
    /// </summary>
    public static Board Empty()
    {
        return new Board();
    }

    /// <summary>
    /// Create a board with a single stone in the center.
    /// </summary>
    public static Board SingleCenterStone(Player player)
    {
        return new BoardBuilder()
            .PlaceStone(15, 15, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a five-in-a-row (winning position).
    /// </summary>
    public static Board FiveInRow(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 7, 1, 0, 5, player)
            .Build();
    }

    /// <summary>
    /// Create a board with six in a row (overline - not a win).
    /// </summary>
    public static Board Overline(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(5, 7, 1, 0, 6, player)
            .Build();
    }

    /// <summary>
    /// Create a board with a straight four that has one end blocked.
    /// Pattern: OXXXX_ (opponent blocks one end)
    /// </summary>
    public static Board StraightFourOneEndBlocked(Player attacker, Player blocker)
    {
        return new BoardBuilder()
            .PlaceStone(4, 7, blocker)
            .PlaceLine(5, 7, 1, 0, 4, attacker)
            .Build();
    }

    /// <summary>
    /// Create a board with a straight four that has both ends blocked.
    /// Pattern: OXXXXO (not a threat per Caro rules)
    /// </summary>
    public static Board StraightFourBothEndsBlocked(Player attacker, Player blocker)
    {
        return new BoardBuilder()
            .PlaceStone(4, 7, blocker)
            .PlaceLine(5, 7, 1, 0, 4, attacker)
            .PlaceStone(9, 7, blocker)
            .Build();
    }

    /// <summary>
    /// Create a board near the edge with limited space.
    /// </summary>
    public static Board EdgePosition(Player player)
    {
        return new BoardBuilder()
            .PlaceLine(0, 7, 1, 0, 4, player)
            .Build();
    }

    /// <summary>
    /// Create a position for testing search depth.
    /// Has multiple possible moves with varying complexity.
    /// </summary>
    public static Board SearchDepthPosition()
    {
        return new BoardBuilder()
            // Some stones placed to create a non-trivial position
            .PlaceStone(15, 15, Player.Red)
            .PlaceStone(16, 15, Player.Blue)
            .PlaceStone(15, 16, Player.Blue)
            .PlaceStone(14, 15, Player.Red)
            .PlaceStone(15, 14, Player.Red)
            .Build();
    }

    /// <summary>
    /// Create a mid-game position with several threats and responses.
    /// </summary>
    public static Board MidGamePosition()
    {
        return new BoardBuilder()
            // Red's attacks
            .PlaceLine(10, 10, 1, 0, 3, Player.Red)
            .PlaceLine(12, 12, 0, 1, 2, Player.Red)
            // Blue's defenses
            .PlaceStone(14, 10, Player.Blue)
            .PlaceStone(12, 14, Player.Blue)
            // Additional stones
            .PlaceStone(11, 11, Player.Blue)
            .PlaceStone(13, 13, Player.Red)
            .Build();
    }
}
