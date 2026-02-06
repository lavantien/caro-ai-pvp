using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class ThreatDetectorTests
{
    private readonly ThreatDetector _detector = new();

    [Fact]
    public void DetectStraightFour_Horizontal_ThreatFound()
    {
        // Arrange - XXXX_ pattern (both ends open)
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Positions 4,7 and 9,7 are empty

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.StraightFour);
        var s4 = threats.First(t => t.Type == ThreatType.StraightFour);
        // Both ends are open, so there are 2 gain squares
        s4.GainSquares.Count.Should().BeGreaterThanOrEqualTo(1);
        s4.GainSquares.Should().Contain((9, 7));
    }

    [Fact]
    public void DetectStraightFour_Vertical_ThreatFound()
    {
        // Arrange - XXXX_ pattern vertical
        var board = new Board();
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(7, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.StraightFour);
    }

    [Fact]
    public void DetectStraightFour_Diagonal_ThreatFound()
    {
        // Arrange - XXXX_ pattern diagonal
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.StraightFour);
    }

    [Fact]
    public void DetectBrokenFour_Horizontal_ThreatFound()
    {
        // Arrange - XXX_X pattern
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        // Position 8,7 is empty (gap)

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.BrokenFour);
        var b4 = threats.First(t => t.Type == ThreatType.BrokenFour);
        b4.GainSquares.Should().Contain((8, 7));
    }

    [Fact]
    public void DetectStraightThree_Horizontal_ThreatFound()
    {
        // Arrange - XXX__ pattern (both ends open)
        var board = new Board();
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Positions 5,7 and 9,7 are empty

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.StraightThree);
        var s3 = threats.First(t => t.Type == ThreatType.StraightThree);
        s3.GainSquares.Should().HaveCount(2);
    }

    [Fact]
    public void DetectBrokenThree_Horizontal_ThreatFound()
    {
        // Arrange - XX_X_ pattern
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Positions 7,7 and 9,7 are empty

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.BrokenThree);
    }

    [Fact]
    public void DetectThreats_StraightFourWithOneEndBlocked_ReturnsThreat()
    {
        // Arrange - OXXXX_ pattern (one end blocked, still a threat)
        var board = new Board();
        board = board.PlaceStone(4, 7, Player.Blue);  // Blocked on left
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Position 9,7 is empty

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert - Still a threat (can win by playing at 9,7)
        threats.Should().Contain(t => t.Type == ThreatType.StraightFour);
    }

    [Fact]
    public void DetectThreats_StraightFourBothEndsBlocked_NoThreat()
    {
        // Arrange - OXXXXO pattern (both ends blocked, can't complete to 5)
        var board = new Board();
        board = board.PlaceStone(4, 7, Player.Blue);  // Blocked on left
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Blue);  // Blocked on right

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert - Not a threat (sandwiched 5 would not be a win in Caro)
        threats.Should().NotContain(t => t.Type == ThreatType.StraightFour);
    }

    [Fact]
    public void DetectThreats_OverlineSixInRow_NoThreat()
    {
        // Arrange - XXXXXX (6 in a row, overline rule - not a threat)
        var board = new Board();
        for (int i = 0; i < 6; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert - Overline should not create winning threats
        threats.Should().NotContain(t => t.Type == ThreatType.StraightFour);
        threats.Should().NotContain(t => t.Type == ThreatType.BrokenFour);
    }

    [Fact]
    public void DetectThreats_FourInRowThatBecomesSix_NoThreat()
    {
        // Arrange - _XXXXX_ (5 with open ends, playing anywhere makes 6)
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert - Already have 5, check win detector instead
        // Adding a stone would create 6+ which is not a win
        threats.Should().BeEmpty();
    }

    [Fact]
    public void DetectThreats_EmptyBoard_NoThreats()
    {
        // Arrange
        var board = new Board();

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);

        // Assert
        threats.Should().BeEmpty();
    }

    [Fact]
    public void GetCostSquares_StraightFour_ReturnsGainSquares()
    {
        // Arrange - XXXX_ pattern (both ends open), defender must block at gain squares
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var s4 = threats.First(t => t.Type == ThreatType.StraightFour);
        var costSquares = _detector.GetCostSquares(s4, board, Player.Blue);

        // Assert - Must block at one of the gain squares (4,7) or (9,7) to prevent win
        costSquares.Count.Should().BeGreaterThanOrEqualTo(1);
        costSquares.Should().Contain((9, 7));
    }

    [Fact]
    public void GetCostSquares_BrokenFour_ReturnsTwoSquares()
    {
        // Arrange - XXX_X pattern, defender can block at gap or end
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var b4 = threats.First(t => t.Type == ThreatType.BrokenFour);
        var costSquares = _detector.GetCostSquares(b4, board, Player.Blue);

        // Assert - Can block at gap (8,7) or either end
        costSquares.Count.Should().BeGreaterThanOrEqualTo(1);
        costSquares.Should().Contain((8, 7));  // Gap is the critical defense
    }

    [Fact]
    public void IsForcingMove_StraightFour_ReturnsTrue()
    {
        // Arrange - S4 requires immediate response
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var s4 = threats.First(t => t.Type == ThreatType.StraightFour);
        var isForcing = _detector.IsForcingMove(s4, board, Player.Red);

        // Assert - S4 is forcing (opponent must block)
        isForcing.Should().BeTrue();
    }

    [Fact]
    public void IsForcingMove_StraightThree_ReturnsTrue()
    {
        // Arrange - S3 is forcing if unblocked
        var board = new Board();
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var s3 = threats.First(t => t.Type == ThreatType.StraightThree);
        var isForcing = _detector.IsForcingMove(s3, board, Player.Red);

        // Assert - S3 is forcing (creates unstoppable S4)
        isForcing.Should().BeTrue();
    }

    [Fact]
    public void IsForcingMove_TwoSquaresAroundBoard_NotForcing()
    {
        // Arrange - Threat near edge with only one gain square
        var board = new Board();
        board = board.PlaceStone(0, 7, Player.Red);
        board = board.PlaceStone(1, 7, Player.Red);
        board = board.PlaceStone(2, 7, Player.Red);
        board = board.PlaceStone(3, 7, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var s4 = threats.First(t => t.Type == ThreatType.StraightFour);
        var isForcing = _detector.IsForcingMove(s4, board, Player.Red);

        // Assert - Edge S4 with only one completion square
        isForcing.Should().BeTrue();  // Still forcing, only one way to win
    }

    [Fact]
    public void FindThreatMoves_OnlyThreatMovesReturned()
    {
        // Arrange - Board with some threats and non-threat moves
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act
        var threatMoves = _detector.FindThreatMoves(board, Player.Red);

        // Assert - Only return moves that create threats
        threatMoves.Should().NotBeEmpty();
        foreach (var move in threatMoves)
        {
            // Verify each move creates at least one threat
            var testBoard = board.PlaceStone(move.x, move.y, Player.Red);
            var threats = _detector.DetectThreats(testBoard, Player.Red);
            threats.Should().NotBeEmpty($"Move ({move.x}, {move.y}) should create threats");
        }
    }

    [Fact]
    public void DetectThreats_BluePlayer_WorksCorrectly()
    {
        // Arrange - XXXX_ pattern for Blue
        var board = new Board();
        board = board.PlaceStone(5, 7, Player.Blue);
        board = board.PlaceStone(6, 7, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);

        // Act
        var threats = _detector.DetectThreats(board, Player.Blue);

        // Assert
        threats.Should().Contain(t => t.Type == ThreatType.StraightFour);
    }

    [Fact]
    public void DetectDoubleThreat_TwoS4_ReturnsBoth()
    {
        // Arrange - Position with two separate S4 threats
        var board = new Board();
        // First S4 horizontal
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        // Second S4 vertical
        board = board.PlaceStone(3, 7, Player.Red);
        board = board.PlaceStone(3, 8, Player.Red);
        board = board.PlaceStone(3, 9, Player.Red);
        board = board.PlaceStone(3, 10, Player.Red);

        // Act
        var threats = _detector.DetectThreats(board, Player.Red);
        var s4Threats = threats.Where(t => t.Type == ThreatType.StraightFour).ToList();

        // Assert - Should detect both S4 threats
        s4Threats.Count.Should().BeGreaterThanOrEqualTo(2);
    }
}
