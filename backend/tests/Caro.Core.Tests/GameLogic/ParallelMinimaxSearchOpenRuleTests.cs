using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public class ParallelMinimaxSearchOpenRuleTests
{
    [Fact]
    public void GetBestMove_Move3_RedRespectsDynamicOpenRule()
    {
        // Arrange: First red move at (7,6), Blue responds at (7,7)
        // The Open Rule exclusion zone centered on (7,6) is: x in [5,9], y in [4,8]
        var board = new Board();
        board.PlaceStone(7, 6, Player.Red);   // Move #1: Red at (7,6)
        board.PlaceStone(7, 7, Player.Blue);  // Move #2: Blue at (7,7)

        var search = new ParallelMinimaxSearch();

        // Act: Get move #3 (Red's second move, Open Rule applies)
        var (x, y) = search.GetBestMove(board, Player.Red, AIDifficulty.Medium, null, null, moveNumber: 3);

        // Assert: AI should not select (7,4) or (7,5) or any position within 2 intersections of (7,6)
        // The exclusion zone is x in [5,9], y in [4,8] centered on first move (7,6)
        // Distance from (7,6): max(|x-7|, |y-6|) must be >= 3
        var dx = System.Math.Abs(x - 7);
        var dy = System.Math.Abs(y - 6);
        var distance = System.Math.Max(dx, dy);

        Assert.True(distance >= 3, $"AI selected ({x},{y}) which is only {distance} intersections from first move (7,6). Minimum required is 3.");
    }

    [Fact]
    public void GetBestMove_Move3_FirstMoveAtEdge_ExclusionZoneCorrect()
    {
        // Arrange: First red move near edge (3,3)
        // Exclusion zone: x in [1,5], y in [1,5]
        var board = new Board();
        board.PlaceStone(3, 3, Player.Red);   // Move #1: Red at (3,3)
        board.PlaceStone(7, 7, Player.Blue);  // Move #2: Blue at (7,7)

        var search = new ParallelMinimaxSearch();

        // Act: Get move #3
        var (x, y) = search.GetBestMove(board, Player.Red, AIDifficulty.Medium, null, null, moveNumber: 3);

        // Assert: Distance from (3,3) must be >= 3
        var dx = System.Math.Abs(x - 3);
        var dy = System.Math.Abs(y - 3);
        var distance = System.Math.Max(dx, dy);

        Assert.True(distance >= 3, $"AI selected ({x},{y}) which is only {distance} intersections from first move (3,3). Minimum required is 3.");
    }

    [Fact]
    public void GetBestMove_Move3_FirstMoveAtCorner_ExclusionZoneCorrect()
    {
        // Arrange: First red move near corner (1,1)
        // Exclusion zone: x in [-1,3], y in [-1,3] (clipped to board)
        // Valid positions on board must have x >= 4 or y >= 4
        var board = new Board();
        board.PlaceStone(1, 1, Player.Red);   // Move #1: Red at (1,1)
        board.PlaceStone(7, 7, Player.Blue);  // Move #2: Blue at (7,7)

        var search = new ParallelMinimaxSearch();

        // Act: Get move #3
        var (x, y) = search.GetBestMove(board, Player.Red, AIDifficulty.Medium, null, null, moveNumber: 3);

        // Assert: Distance from (1,1) must be >= 3
        var dx = System.Math.Abs(x - 1);
        var dy = System.Math.Abs(y - 1);
        var distance = System.Math.Max(dx, dy);

        Assert.True(distance >= 3, $"AI selected ({x},{y}) which is only {distance} intersections from first move (1,1). Minimum required is 3.");
    }

    [Fact]
    public void GetBestMove_Move4_OpenRuleDoesNotApply()
    {
        // Arrange: Three stones already on board (Open Rule only applies to move #3)
        var board = new Board();
        board.PlaceStone(7, 6, Player.Red);   // Move #1
        board.PlaceStone(7, 7, Player.Blue);  // Move #2
        board.PlaceStone(10, 6, Player.Red);  // Move #3 (valid, 3 intersections from (7,6))

        var search = new ParallelMinimaxSearch();

        // Act: Get move #4 (Blue's turn, Open Rule doesn't apply)
        var (x, y) = search.GetBestMove(board, Player.Blue, AIDifficulty.Medium, null, null, moveNumber: 4);

        // Assert: Any valid position on board is acceptable
        Assert.True(board.GetCell(x, y).Player == Player.None, "Selected position should be empty");
    }
}
