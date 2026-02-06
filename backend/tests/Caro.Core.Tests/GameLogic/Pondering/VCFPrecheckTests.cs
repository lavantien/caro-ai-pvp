using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Pondering;

namespace Caro.Core.Tests.GameLogic.Pondering;

public class VCFPrecheckTests
{
    private readonly VCFPrecheck _precheck = new();

    [Fact]
    public void HasPotentialThreats_StraightFour_ReturnsTrue()
    {
        // Arrange - XXXX_ pattern
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Straight Four is always a winning threat
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_BrokenFour_ReturnsTrue()
    {
        // Arrange - XXX_X pattern
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(11, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Broken Four can create double attacks
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_MultipleStraightThrees_ReturnsTrue()
    {
        // Arrange - Two separate S3 threats
        var board = new Board();
        // First S3
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        // Second S3
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Multiple S3 can create double attack
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_SingleStraightThree_ReturnsTrue()
    {
        // Arrange - Single S3 without other threats
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Single S3 is considered a threat by the current implementation
        // The logic: anyThree && threeInRowCount >= 1 returns true when there's a StraightThree
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_EmptyBoard_ReturnsFalse()
    {
        // Arrange
        var board = new Board();

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert
        hasThreats.Should().BeFalse();
    }

    [Fact]
    public void HasPotentialThreats_ScatteredStones_ReturnsFalse()
    {
        // Arrange - Few scattered stones, no pattern
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(3, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - No significant threats
        hasThreats.Should().BeFalse();
    }

    [Fact]
    public void HasPotentialThreats_MidGameWithOneThreat_ReturnsTrue()
    {
        // Arrange - Mid-game position (>40 stones) with one threat
        var board = new Board();
        // Create ~45 stones scattered
        for (int i = 0; i < 45; i++)
        {
            var x = i % 15;
            var y = (i / 15) % 15;
            var player = i % 2 == 0 ? Player.Red : Player.Blue;
            board = board.PlaceStone(x, y, player);
        }
        // Add a threat
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Mid-game with threats is worth pondering
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_LateGameWithWeakThreat_ReturnsTrue()
    {
        // Arrange - Late-game position (>70 stones) with weak threat
        var board = new Board();
        // Create ~75 stones scattered
        for (int i = 0; i < 75; i++)
        {
            var x = i % 15;
            var y = (i / 15) % 15;
            var player = i % 2 == 0 ? Player.Red : Player.Blue;
            board = board.PlaceStone(x, y, player);
        }
        // Add a weak threat (two in a row)
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Late-game with any threat is worth pondering
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_OpponentHasStraightFour_ReturnsTrue()
    {
        // Arrange - Opponent has S4 threat
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(10, 7, Player.Blue);
        board = board.PlaceStone(5, 5, Player.Red); // Red has no threats

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Should ponder because opponent has forcing threat
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void HasPotentialThreats_OpponentHasStraightThree_ReturnsTrue()
    {
        // Arrange - Opponent has S3 threat
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(5, 5, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.Red);

        // Assert - Should ponder to respond to opponent threat
        hasThreats.Should().BeTrue();
    }

    [Fact]
    public void GetThreatUrgency_StraightFour_ReturnsHighValue()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert - S4 should have high urgency (50 points)
        urgency.Should().BeGreaterThanOrEqualTo(50);
    }

    [Fact]
    public void GetThreatUrgency_BrokenFour_ReturnsMediumValue()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(11, 7, Player.Red);

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert - B4 should have medium urgency (30 points)
        urgency.Should().BeGreaterThanOrEqualTo(30);
    }

    [Fact]
    public void GetThreatUrgency_StraightThree_ReturnsLowValue()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert - S3 should have low urgency (15 points)
        urgency.Should().BeGreaterThanOrEqualTo(15);
    }

    [Fact]
    public void GetThreatUrgency_MultipleThreats_SumsValues()
    {
        // Arrange - Two S4 threats
        var board = new Board();
        // First S4
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        // Second S4
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert - Should sum urgency (50 + 50 = 100)
        urgency.Should().Be(100);
    }

    [Fact]
    public void GetThreatUrgency_EmptyBoard_ReturnsZero()
    {
        // Arrange
        var board = new Board();

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert
        urgency.Should().Be(0);
    }

    [Fact]
    public void GetThreatUrgency_CapsAt100()
    {
        // Arrange - Multiple high-value threats
        var board = new Board();
        // Create many threats that would exceed 100
        for (int i = 0; i < 5; i++)
        {
            board = board.PlaceStone(i, 0, Player.Red);
            board = board.PlaceStone(i, 1, Player.Red);
            board = board.PlaceStone(i, 2, Player.Red);
            board = board.PlaceStone(i, 3, Player.Red);
        }

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.Red);

        // Assert - Should cap at 100
        urgency.Should().BeLessThanOrEqualTo(100);
    }

    [Fact]
    public void IsOpeningPhase_FewStones_ReturnsTrue()
    {
        // Arrange - 5 stones on board
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(9, 9, Player.Blue);
        board = board.PlaceStone(5, 5, Player.Red);

        // Act
        var isOpening = _precheck.IsOpeningPhase(board);

        // Assert
        isOpening.Should().BeTrue();
    }

    [Fact]
    public void IsOpeningPhase_TenStones_ReturnsFalse()
    {
        // Arrange - Exactly 10 stones
        var board = new Board();
        for (int i = 0; i < 10; i++)
        {
            board = board.PlaceStone(i, 0, i % 2 == 0 ? Player.Red : Player.Blue);
        }

        // Act
        var isOpening = _precheck.IsOpeningPhase(board);

        // Assert
        isOpening.Should().BeFalse();
    }

    [Fact]
    public void IsOpeningPhase_EmptyBoard_ReturnsTrue()
    {
        // Arrange
        var board = new Board();

        // Act
        var isOpening = _precheck.IsOpeningPhase(board);

        // Assert
        isOpening.Should().BeTrue();
    }

    [Fact]
    public void IsEndgamePhase_BoardOver70PercentFull_ReturnsTrue()
    {
        // Arrange - Board with >70% occupancy (more than 253 stones for 19x19 board)
        var board = new Board();
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 14; y++) // 14 * 19 = 266 stones (>70% of 361)
            {
                board = board.PlaceStone(x, y, x % 2 == 0 ? Player.Red : Player.Blue);
            }
        }

        // Act
        var isEndgame = _precheck.IsEndgamePhase(board);

        // Assert
        isEndgame.Should().BeTrue();
    }

    [Fact]
    public void IsEndgamePhase_HalfFull_ReturnsFalse()
    {
        // Arrange - Board with ~50% occupancy (19x19 board)
        var board = new Board();
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 10; y++) // 10 * 19 = 190 stones (~53%)
            {
                board = board.PlaceStone(x, y, x % 2 == 0 ? Player.Red : Player.Blue);
            }
        }

        // Act
        var isEndgame = _precheck.IsEndgamePhase(board);

        // Assert
        isEndgame.Should().BeFalse();
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_HighUrgency_ReturnsHighMultiplier()
    {
        // Arrange - High urgency position (not in opening phase)
        var board = new Board();
        // Add enough stones to avoid opening phase reduction
        for (int i = 0; i < 12; i++)
        {
            board = board.PlaceStone(i, 0, i % 2 == 0 ? Player.Blue : Player.Red);
        }
        // Add S4 threat for high urgency
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - High urgency should give multiplier > 1.0
        multiplier.Should().BeGreaterThan(1.0);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_LowUrgency_ReturnsLowMultiplier()
    {
        // Arrange - Low urgency position
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - Low urgency should give multiplier < 1.0
        multiplier.Should().BeLessThan(1.0);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_OpeningPhase_ReducesMultiplier()
    {
        // Arrange - Opening position with threat
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - Opening phase should reduce multiplier
        multiplier.Should().BeLessThan(1.0);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_EndgamePhase_IncreasesMultiplier()
    {
        // Arrange - Endgame position (>70% full for 19x19 board)
        var board = new Board();
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 14; y++) // 14 * 19 = 266 stones (>70% of 361)
            {
                board = board.PlaceStone(x, y, x % 2 == 0 ? Player.Red : Player.Blue);
            }
        }
        // Add a threat
        board = board.PlaceStone(5, 15, Player.Red);
        board = board.PlaceStone(6, 15, Player.Red);

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - Endgame should increase multiplier
        multiplier.Should().BeGreaterThan(0.5);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_ClampsAtMaximum()
    {
        // Arrange - Maximum urgency position
        var board = new Board();
        // Create multiple S4 threats
        for (int i = 0; i < 3; i++)
        {
            board = board.PlaceStone(i, 0, Player.Red);
            board = board.PlaceStone(i, 1, Player.Red);
            board = board.PlaceStone(i, 2, Player.Red);
            board = board.PlaceStone(i, 3, Player.Red);
        }

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - Should clamp at 2.0
        multiplier.Should().BeLessThanOrEqualTo(2.0);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_ClampsAtMinimum()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.Red);

        // Assert - Should clamp at 0.1
        multiplier.Should().BeGreaterThanOrEqualTo(0.1);
    }

    [Fact]
    public void HasPotentialThreats_PlayerNone_ReturnsFalse()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        var hasThreats = _precheck.HasPotentialThreats(board, Player.None);

        // Assert
        hasThreats.Should().BeFalse();
    }

    [Fact]
    public void GetThreatUrgency_PlayerNone_ReturnsZero()
    {
        // Arrange
        var board = new Board();

        // Act
        var urgency = _precheck.GetThreatUrgency(board, Player.None);

        // Assert
        urgency.Should().Be(0);
    }

    [Fact]
    public void CalculatePonderTimeMultiplier_PlayerNone_ReturnsLowValue()
    {
        // Arrange
        var board = new Board();

        // Act
        var multiplier = _precheck.CalculatePonderTimeMultiplier(board, Player.None);

        // Assert
        multiplier.Should().Be(0.5);
    }
}
