using Caro.Core.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

[Trait("Category", "Verification")]
public class QuickGrandmasterVsEasy
{
    [Fact]
    public void Grandmaster_BlocksFourInARow_Easy_DoesNot()
    {
        var board = new Board();

        // Blue has four in a row vertically at (3,4), (4,4), (5,4), (6,4)
        // Red blocked bottom at (7,4)
        // Red MUST block top at (2,4) or lose
        board.PlaceStone(3, 4, Player.Blue);
        board.PlaceStone(4, 4, Player.Blue);
        board.PlaceStone(5, 4, Player.Blue);
        board.PlaceStone(6, 4, Player.Blue);
        board.PlaceStone(7, 4, Player.Red);

        var ai = new MinimaxAI();

        // Grandmaster (D10) should block at (2, 4)
        var (gx, gy) = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        gx.Should().Be(2, "Grandmaster should block Blue's four in a row");
        gy.Should().Be(4, "Grandmaster should block Blue's four in a row");

        // Reset board
        board = new Board();
        board.PlaceStone(3, 4, Player.Blue);
        board.PlaceStone(4, 4, Player.Blue);
        board.PlaceStone(5, 4, Player.Blue);
        board.PlaceStone(6, 4, Player.Blue);
        board.PlaceStone(7, 4, Player.Red);

        // Easy (D2) should NOT automatically block - it plays based on its own evaluation
        var (ex, ey) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);
        // Easy might NOT block at (2, 4) since it doesn't have critical defense
        // This is expected behavior
        (ex, ey).Should().NotBe((2, 4), "Easy (D2) should not get free blocking move from critical defense logic");
    }
}
