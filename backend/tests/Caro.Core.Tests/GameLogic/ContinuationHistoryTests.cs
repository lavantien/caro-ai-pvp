using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public class ContinuationHistoryTests
{
    private const int BoardSize = GameConstants.BoardSize;

    [Fact]
    public void ContinuationHistory_InitializesToZero()
    {
        // Arrange & Act
        var history = new ContinuationHistory();

        // Assert - All values should start at 0 for all players and positions
        for (int player = 0; player < 2; player++)
        {
            for (int prev = 0; prev < BoardSize * BoardSize; prev++)
            {
                for (int curr = 0; curr < BoardSize * BoardSize; curr++)
                {
                    Assert.Equal(0, history.GetScore(player == 0 ? Player.Red : Player.Blue, prev, curr));
                }
            }
        }
    }

    [Fact]
    public void ContinuationHistory_Update_BoundsCorrectly()
    {
        // Arrange
        var history = new ContinuationHistory();
        const int MaxScore = 30000;

        // Act - Update with positive bonus (should not exceed MaxScore)
        history.Update(Player.Red, 0, 1, MaxScore);
        int score1 = history.GetScore(Player.Red, 0, 1);

        // Update with negative bonus (should not go below -MaxScore)
        history.Update(Player.Blue, 0, 1, -MaxScore);
        int score2 = history.GetScore(Player.Blue, 0, 1);

        // Assert
        Assert.Equal(MaxScore, score1);
        Assert.Equal(-MaxScore, score2);
    }

    [Fact]
    public void ContinuationHistory_MultipleUpdates_HandleOverflow()
    {
        // Arrange
        var history = new ContinuationHistory();
        const int MaxScore = 30000;

        // Act - Multiple updates that would overflow without bounding
        for (int i = 0; i < 10; i++)
        {
            history.Update(Player.Red, 0, 1, 10000);
        }

        int score = history.GetScore(Player.Red, 0, 1);

        // Assert - Score should be bounded, not overflow
        Assert.InRange(score, -MaxScore, MaxScore);

        // Multiple negative updates
        for (int i = 0; i < 10; i++)
        {
            history.Update(Player.Blue, 1, 2, -10000);
        }

        int scoreNegative = history.GetScore(Player.Blue, 1, 2);
        Assert.InRange(scoreNegative, -MaxScore, MaxScore);
    }

    [Fact]
    public void ContinuationHistory_Update_IncrementalValues()
    {
        // Arrange
        var history = new ContinuationHistory();

        // Act - Incremental updates
        history.Update(Player.Red, 0, 1, 100);
        int score1 = history.GetScore(Player.Red, 0, 1);

        history.Update(Player.Red, 0, 1, 50);
        int score2 = history.GetScore(Player.Red, 0, 1);

        // Assert - Score should increase
        Assert.Equal(100, score1);
        Assert.Equal(150, score2);
    }

    [Fact]
    public void ContinuationHistory_Clear_ResetsToZero()
    {
        // Arrange
        var history = new ContinuationHistory();
        history.Update(Player.Red, 0, 1, 5000);
        history.Update(Player.Blue, 5, 10, -3000);

        // Act
        history.Clear();

        // Assert - All values should be reset to 0
        Assert.Equal(0, history.GetScore(Player.Red, 0, 1));
        Assert.Equal(0, history.GetScore(Player.Blue, 5, 10));
    }

    [Fact]
    public void ContinuationHistory_Update_DecrementalValues()
    {
        // Arrange
        var history = new ContinuationHistory();

        // First set a positive value
        history.Update(Player.Red, 0, 1, 1000);
        int score1 = history.GetScore(Player.Red, 0, 1);

        // Then add a negative bonus (decrement)
        // Bounded formula: newValue = 1000 + (-200) - |1000 * -200| / 30000
        //                    = 1000 - 200 - 200000 / 30000
        //                    = 1000 - 200 - 6 = 794
        history.Update(Player.Red, 0, 1, -200);
        int score2 = history.GetScore(Player.Red, 0, 1);

        // Assert
        Assert.Equal(1000, score1);
        Assert.Equal(794, score2); // Bounded formula reduces by additional 6
    }

    [Fact]
    public void ContinuationHistory_DifferentPlayers_IndependentScores()
    {
        // Arrange
        var history = new ContinuationHistory();

        // Act
        history.Update(Player.Red, 0, 1, 1000);
        history.Update(Player.Blue, 0, 1, 2000);

        int redScore = history.GetScore(Player.Red, 0, 1);
        int blueScore = history.GetScore(Player.Blue, 0, 1);

        // Assert - Each player should have independent scores
        Assert.Equal(1000, redScore);
        Assert.Equal(2000, blueScore);
    }

    [Fact]
    public void ContinuationHistory_CellIndexing_CorrectMapping()
    {
        // Arrange
        var history = new ContinuationHistory();

        // Act - Test with various cell positions
        // Cell index = y * BoardSize + x (for 16x16 board, max is 15*16+15=255)
        int cell1 = 0 * GameConstants.BoardSize + 0;   // (0, 0)
        int cell2 = 1 * GameConstants.BoardSize + 1;   // (1, 1)
        int cell3 = 8 * GameConstants.BoardSize + 8;   // (8, 8) - center for 16x16

        history.Update(Player.Red, cell1, cell2, 500);
        history.Update(Player.Red, cell2, cell3, 1000);

        // Assert
        Assert.Equal(500, history.GetScore(Player.Red, cell1, cell2));
        Assert.Equal(1000, history.GetScore(Player.Red, cell2, cell3));
        Assert.Equal(0, history.GetScore(Player.Red, cell2, cell1)); // Reverse direction should be 0
    }

    [Fact]
    public void ContinuationHistory_BoundaryFormula_PreventsOverflow()
    {
        // Arrange
        var history = new ContinuationHistory();
        const int MaxScore = 30000;

        // Act - Set value near max, then add more
        history.Update(Player.Red, 0, 1, 25000);
        history.Update(Player.Red, 0, 1, 10000); // Would exceed 30000

        int score = history.GetScore(Player.Red, 0, 1);

        // Assert - Should be bounded by MaxScore
        Assert.InRange(score, -MaxScore, MaxScore);
    }

    [Fact]
    public void ContinuationHistory_BoundaryFormula_PreventsUnderflow()
    {
        // Arrange
        var history = new ContinuationHistory();
        const int MaxScore = 30000;

        // Act - Set value near min, then subtract more
        history.Update(Player.Blue, 0, 1, -25000);
        history.Update(Player.Blue, 0, 1, -10000); // Would go below -30000

        int score = history.GetScore(Player.Blue, 0, 1);

        // Assert - Should be bounded by -MaxScore
        Assert.InRange(score, -MaxScore, MaxScore);
    }
}
