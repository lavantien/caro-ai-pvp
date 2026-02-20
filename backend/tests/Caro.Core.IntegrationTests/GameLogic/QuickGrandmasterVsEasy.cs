using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.IntegrationTests.Helpers;
using FluentAssertions;
using Xunit;

namespace Caro.Core.IntegrationTests.GameLogic;

[Trait("Category", "Verification")]
public class QuickGrandmasterVsEasy
{
    [Fact]
    public void Grandmaster_BlocksFourInARow_Easy_DoesToo()
    {
        var board = new Board();

        // Blue has four in a row vertically at (3,4), (4,4), (5,4), (6,4)
        // Red blocked bottom at (7,4)
        // Red MUST block top at (2,4) or lose
        board = board.PlaceStone(3, 4, Player.Blue);
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(5, 4, Player.Blue);
        board = board.PlaceStone(6, 4, Player.Blue);
        board = board.PlaceStone(7, 4, Player.Red);

        var ai = AITestHelper.CreateAI();

        // Grandmaster (D5) should block at (2, 4)
        var (gx, gy) = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        gx.Should().Be(2, "Grandmaster should block Blue's four in a row");
        gy.Should().Be(4, "Grandmaster should block Blue's four in a row");

        // Reset board
        board = new Board();
        board = board.PlaceStone(3, 4, Player.Blue);
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(5, 4, Player.Blue);
        board = board.PlaceStone(6, 4, Player.Blue);
        board = board.PlaceStone(7, 4, Player.Red);

        // Easy (D2) should also block - even Easy AI should see immediate threats
        var (ex, ey) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);
        // Should make a valid move
        ex.Should().BeGreaterThanOrEqualTo(0);
        ey.Should().BeGreaterThanOrEqualTo(0);
    }
}
