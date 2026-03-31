using Xunit;
using FluentAssertions;
using Caro.Core.IntegrationTests.Helpers;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.IntegrationTests.GameLogic;

/// <summary>
/// Tests for defensive AI behavior, specifically blocking semi-open fours
/// </summary>
public class DefensivePlayTests
{
    private const int DefaultTTSizeMb = 256;
    private const int ConsistencyRunCount = 10;
    private const int ConsistencyThreshold = 8;
    /// <summary>
    /// Test that AI blocks semi-open four (XXXX_ pattern where one end is blocked)
    /// </summary>
    [Fact]
    public void ParallelSearch_BlocksSemiOpenFour_Horizontal()
    {
        // Arrange: Create semi-open four for Red (XXXX_ pattern)
        // Red has stones at (7,7), (8,7), (9,7), (10,7)
        // Position (11,7) is open - this is the winning move
        // Position (6,7) is blocked by Blue
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue); // Blocks one end

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act: Get best move for Blue (must block at 11,7)
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Should block at (11, 7) to prevent Red from winning
        result.x.Should().Be(11, "Blue must block Red's semi-open four at (11, 7)");
        result.y.Should().Be(7, "Blue must block Red's semi-open four at (11, 7)");
    }

    [Fact]
    public void ParallelSearch_BlocksSemiOpenFour_Vertical()
    {
        // Arrange: Create vertical semi-open four
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);
        board = board.PlaceStone(7, 9, Player.Red);
        board = board.PlaceStone(7, 10, Player.Red);
        board = board.PlaceStone(7, 6, Player.Blue); // Blocks one end

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Should block at (7, 11)
        result.x.Should().Be(7, "Blue must block Red's vertical semi-open four");
        result.y.Should().Be(11, "Blue must block Red's vertical semi-open four");
    }

    [Fact]
    public void ParallelSearch_BlocksSemiOpenFour_Diagonal()
    {
        // Arrange: Create diagonal semi-open four
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Red);
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(10, 10, Player.Red);
        board = board.PlaceStone(6, 6, Player.Blue); // Blocks one end

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Should block at (11, 11)
        result.x.Should().Be(11, "Blue must block Red's diagonal semi-open four");
        result.y.Should().Be(11, "Blue must block Red's diagonal semi-open four");
    }

    /// <summary>
    /// Test Lazy SMP consistency - defensive moves should be consistent across multiple runs
    /// </summary>
    [Fact]
    public void LazyMP_ConsistentBlocking_SemiOpenFour()
    {
        // Arrange: Create semi-open four position
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);

        var moves = new List<(int x, int y)>();

        // Act: Run 10 times to test consistency
        for (int i = 0; i < ConsistencyRunCount; i++)
        {
            var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);
            var result = ai.GetBestMove(board, Player.Blue);
            moves.Add((result.x, result.y));
        }

        // Assert: All runs should produce the same defensive move
        var blockCount = moves.Count(m => m.x == 11 && m.y == 7);
        blockCount.Should().BeGreaterThanOrEqualTo(ConsistencyThreshold,
            "At least 80% of runs should block the semi-open four consistently");
    }

    /// <summary>
    /// Test that blocking takes priority over attacking when opponent has critical threat
    /// </summary>
    [Fact]
    public void ParallelSearch_DefenseOverAttack_PrefersBlockingThreat()
    {
        // Arrange:
        // Red has semi-open four at (7,7)-(10,7) with (11,7) open
        // Blue also has a potential three-in-row at (5,5)-(7,5)
        // Blue should prioritize blocking Red's threat over extending its own attack
        var board = new Board();
        // Red's semi-open four (critical threat)
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);

        // Blue's potential attack (less urgent)
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 5, Player.Blue);
        board = board.PlaceStone(7, 5, Player.Blue);

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Should block Red's threat at (11, 7) rather than extend Blue's attack
        result.x.Should().Be(11, "Should block opponent's threat first");
        result.y.Should().Be(7, "Should block opponent's threat first");
    }

    /// <summary>
    /// Test blocking of broken four (XXX_X pattern)
    /// </summary>
    [Fact]
    public void ParallelSearch_BlocksBrokenFour()
    {
        // Arrange: Create broken four (XXX_X) for Red
        // Red has stones at (7,7), (8,7), (9,7), (11,7) - gap at (10,7)
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(11, 7, Player.Red);

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act: Get best move for Blue
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Should block at (10, 7) or another critical position
        (result.x == 10 && result.y == 7 || result.x == 6 || result.x == 12)
            .Should().BeTrue("Blue should block Red's broken four threat");
    }

    /// <summary>
    /// Test MinimaxAI (not just ParallelMinimaxSearch) also blocks correctly
    /// </summary>
    [Fact]
    public void MinimaxAI_BlocksSemiOpenFour()
    {
        // Arrange: Create semi-open four
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);

        var ai = AITestHelper.CreateAI();

        // Act
        var (x, y) = ai.GetBestMove(board, Player.Blue, null);

        // Assert: Should block at (11, 7)
        x.Should().Be(11, "MinimaxAI should block semi-open four");
        y.Should().Be(7, "MinimaxAI should block semi-open four");
    }

    /// <summary>
    /// Test that immediate win takes priority over blocking
    /// If we can win, we should win, not block
    /// </summary>
    [Fact]
    public void ParallelSearch_WinsOverBlocks_PrefersWinningMove()
    {
        // Arrange:
        // Blue has four-in-row ready to win at (5,7)-(9,7), needs (10,7)
        // Red also has semi-open four at (12,7)-(15,7) but blocked on one end
        // Blue should take the winning move
        var board = new Board();
        // Blue's winning threat
        board = board.PlaceStone(6, 7, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);

        // Red's semi-open four (but Red is not to move)
        board = board.PlaceStone(11, 7, Player.Red);
        board = board.PlaceStone(12, 7, Player.Red);
        board = board.PlaceStone(13, 7, Player.Red);
        board = board.PlaceStone(14, 7, Player.Red);

        var ai = new ParallelMinimaxSearch(sizeMB: DefaultTTSizeMb);

        // Act: Blue to move
        var result = ai.GetBestMove(board, Player.Blue);

        // Assert: Blue should win at (10, 7) or (5, 7)
        (result.x == 10 || result.x == 5).Should().BeTrue("Blue should take winning move");
        result.y.Should().Be(7);
    }

    /// <summary>
    /// Test for the critical bug where the AI lost to a weaker opponent.
    /// Reproduces the exact game scenario: Blue has four in a row vertically at (3,4)-(6,4),
    /// Red blocked bottom at (7,4), so Red MUST block top at (2,4) or lose immediately.
    /// </summary>
    [Fact]
    public void MinimaxAI_BlocksFourInARow_Vertical_BugScenario()
    {
        // Arrange: Exact scenario from the bug report
        // Blue has four in a row vertically at (3,4), (4,4), (5,4), (6,4)
        // Red blocked bottom at (7,4)
        // Red MUST block top at (2,4) or Blue wins on next move
        var board = new Board();
        board = board.PlaceStone(3, 4, Player.Blue);
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(5, 4, Player.Blue);
        board = board.PlaceStone(6, 4, Player.Blue);
        board = board.PlaceStone(7, 4, Player.Red); // Red's previous block - wrong end!

        var ai = AITestHelper.CreateAI();

        // Act: Red to move - must block at (2, 4)
        var (x, y) = ai.GetBestMove(board, Player.Red, null);

        // Assert: Must block at (2, 4) to prevent immediate loss
        x.Should().Be(2, "Red must block Blue's four in a row at the top end");
        y.Should().Be(4, "Red must block Blue's vertical four in a row");
    }

    /// <summary>
    /// Additional test: Block at bottom when top is blocked
    /// </summary>
    [Fact]
    public void MinimaxAI_BlocksFourInARow_Vertical_BottomEnd()
    {
        // Arrange: Blue has four in a row at (3,4)-(6,4), top blocked by Red
        var board = new Board();
        board = board.PlaceStone(3, 4, Player.Blue);
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(5, 4, Player.Blue);
        board = board.PlaceStone(6, 4, Player.Blue);
        board = board.PlaceStone(2, 4, Player.Red); // Red blocked top

        var ai = AITestHelper.CreateAI();

        // Act: Red to move - must block at (7, 4)
        var (x, y) = ai.GetBestMove(board, Player.Red, null);

        // Assert: Must block at (7, 4)
        x.Should().Be(7, "Red must block Blue's four in a row at the bottom end");
        y.Should().Be(4, "Red must block Blue's vertical four in a row");
    }

    /// <summary>
    /// Test: Detect open three (three in a row with both ends open)
    /// </summary>
    [Fact]
    public void MinimaxAI_BlocksOpenThree_Vertical()
    {
        // Arrange: Blue has three in a row at (4,4)-(6,4), both ends open
        var board = new Board();
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(5, 4, Player.Blue);
        board = board.PlaceStone(6, 4, Player.Blue);

        var ai = AITestHelper.CreateAI();

        // Act: Red to move - should block at one end
        var (x, y) = ai.GetBestMove(board, Player.Red, null);

        // Assert: Should block at (3, 4) or (7, 4)
        ((x == 3 || x == 7) && y == 4).Should().BeTrue(
            "Red must block Blue's open three at either end");
    }
}
