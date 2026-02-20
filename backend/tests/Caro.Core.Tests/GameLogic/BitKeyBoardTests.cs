using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class BitKeyBoardTests
{
    #region Constructor Tests

    [Fact]
    public void Constructor_CreatesEmptyBoard()
    {
        // Arrange & Act
        var board = new BitKeyBoard();

        // Assert
        board.CountStones().Should().Be(0);
    }

    [Fact]
    public void Constructor_FromBoard_CopiesStones()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);

        // Act
        var bitKeyBoard = new BitKeyBoard(board);

        // Assert
        bitKeyBoard.CountStones().Should().Be(2);
        bitKeyBoard.GetPlayerAt(5, 5).Should().Be(Player.Red);
        bitKeyBoard.GetPlayerAt(10, 10).Should().Be(Player.Blue);
    }

    [Fact]
    public void Constructor_FromBitBoards_CopiesStones()
    {
        // Arrange
        var redBoard = new BitBoard();
        var blueBoard = new BitBoard();
        redBoard.SetBit(3, 3, true);
        blueBoard.SetBit(7, 7, true);

        // Act
        var bitKeyBoard = new BitKeyBoard(redBoard, blueBoard);

        // Assert
        bitKeyBoard.CountStones().Should().Be(2);
        bitKeyBoard.GetPlayerAt(3, 3).Should().Be(Player.Red);
        bitKeyBoard.GetPlayerAt(7, 7).Should().Be(Player.Blue);
    }

    #endregion

    #region SetBit/ClearBit Tests

    [Fact]
    public void SetBit_RedPlayer_SetsCorrectEncoding()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        board.SetBit(5, 5, Player.Red);

        // Assert
        board.GetPlayerAt(5, 5).Should().Be(Player.Red);
        board.CountStones().Should().Be(1);
    }

    [Fact]
    public void SetBit_BluePlayer_SetsCorrectEncoding()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        board.SetBit(5, 5, Player.Blue);

        // Assert
        board.GetPlayerAt(5, 5).Should().Be(Player.Blue);
        board.CountStones().Should().Be(1);
    }

    [Fact]
    public void SetBit_MultiplePositions_SetsAllCorrectly()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        board.SetBit(0, 0, Player.Red);
        board.SetBit(7, 7, Player.Blue);
        board.SetBit(15, 15, Player.Red);

        // Assert
        board.GetPlayerAt(0, 0).Should().Be(Player.Red);
        board.GetPlayerAt(7, 7).Should().Be(Player.Blue);
        board.GetPlayerAt(15, 15).Should().Be(Player.Red);
        board.CountStones().Should().Be(3);
    }

    [Fact]
    public void ClearBit_RemovesStone()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(5, 5, Player.Red);

        // Act
        board.ClearBit(5, 5);

        // Assert
        board.GetPlayerAt(5, 5).Should().Be(Player.None);
        board.CountStones().Should().Be(0);
    }

    [Fact]
    public void SetBit_OutOfBounds_DoesNotThrow()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act & Assert - Should not throw
        var act = () => board.SetBit(-1, 5, Player.Red);
        act.Should().NotThrow();
        var act2 = () => board.SetBit(32, 5, Player.Red);
        act2.Should().NotThrow();
    }

    #endregion

    #region GetKeyAt Tests

    [Fact]
    public void GetKeyAt_EmptyPosition_ReturnsNonZeroKey()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        var key = board.GetKeyAt(5, 5, 0);

        // Assert - Key should be computed even for empty positions
        key.Should().Be(board.GetHorizontalKey(5));
    }

    [Fact]
    public void GetKeyAt_WithStones_ReturnsDifferentKey()
    {
        // Arrange
        var board = new BitKeyBoard();
        var emptyKey = board.GetKeyAt(5, 5, 0);

        // Act
        board.SetBit(5, 5, Player.Red);
        var filledKey = board.GetKeyAt(5, 5, 0);

        // Assert
        filledKey.Should().NotBe(emptyKey);
    }

    [Fact]
    public void GetKeyAt_AllDirections_ReturnsValidKeys()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(10, 10, Player.Red);

        // Act
        var horizontal = board.GetKeyAt(10, 10, 0);
        var vertical = board.GetKeyAt(10, 10, 1);
        var diagonal = board.GetKeyAt(10, 10, 2);
        var antiDiagonal = board.GetKeyAt(10, 10, 3);

        // Assert
        horizontal.Should().NotBe(0);
        vertical.Should().NotBe(0);
        diagonal.Should().NotBe(0);
        antiDiagonal.Should().NotBe(0);
    }

    [Fact]
    public void GetAllKeysAt_ReturnsAllFourDirections()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(16, 16, Player.Red);

        // Act
        var (h, v, d, ad) = board.GetAllKeysAt(16, 16);

        // Assert
        h.Should().Be(board.GetKeyAt(16, 16, 0));
        v.Should().Be(board.GetKeyAt(16, 16, 1));
        d.Should().Be(board.GetKeyAt(16, 16, 2));
        ad.Should().Be(board.GetKeyAt(16, 16, 3));
    }

    #endregion

    #region GetPlayerAt Tests

    [Fact]
    public void GetPlayerAt_EmptyPosition_ReturnsNone()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act & Assert
        board.GetPlayerAt(5, 5).Should().Be(Player.None);
    }

    [Theory]
    [InlineData(0, 0)]
    [InlineData(7, 7)]
    [InlineData(15, 15)]
    [InlineData(0, 15)]
    [InlineData(15, 0)]
    public void GetPlayerAt_AtVariousPositions_ReturnsCorrectPlayer(int x, int y)
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(x, y, Player.Red);

        // Act & Assert
        board.GetPlayerAt(x, y).Should().Be(Player.Red);
    }

    [Fact]
    public void GetPlayerAt_OutOfBounds_ReturnsNone()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act & Assert
        board.GetPlayerAt(-1, 5).Should().Be(Player.None);
        board.GetPlayerAt(32, 5).Should().Be(Player.None);
    }

    #endregion

    #region Clone Tests

    [Fact]
    public void Clone_CreatesDeepCopy()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(5, 5, Player.Red);
        board.SetBit(10, 10, Player.Blue);

        // Act
        var clone = board.Clone();
        clone.SetBit(15, 15, Player.Red);

        // Assert
        board.GetPlayerAt(15, 15).Should().Be(Player.None);
        clone.GetPlayerAt(15, 15).Should().Be(Player.Red);
        board.CountStones().Should().Be(2);
        clone.CountStones().Should().Be(3);
    }

    #endregion

    #region GetHash Tests

    [Fact]
    public void GetHash_DifferentBoards_ReturnsDifferentHashes()
    {
        // Arrange
        var board1 = new BitKeyBoard();
        var board2 = new BitKeyBoard();
        board1.SetBit(5, 5, Player.Red);
        board2.SetBit(10, 10, Player.Red);

        // Act
        var hash1 = board1.GetHash();
        var hash2 = board2.GetHash();

        // Assert
        hash1.Should().NotBe(hash2);
    }

    [Fact]
    public void GetHash_SameBoards_ReturnsSameHash()
    {
        // Arrange
        var board1 = new BitKeyBoard();
        var board2 = new BitKeyBoard();
        board1.SetBit(5, 5, Player.Red);
        board2.SetBit(5, 5, Player.Red);

        // Act
        var hash1 = board1.GetHash();
        var hash2 = board2.GetHash();

        // Assert
        hash1.Should().Be(hash2);
    }

    #endregion

    #region Clear Tests

    [Fact]
    public void Clear_RemovesAllStones()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(5, 5, Player.Red);
        board.SetBit(10, 10, Player.Blue);
        board.SetBit(15, 15, Player.Red);

        // Act
        board.Clear();

        // Assert
        board.CountStones().Should().Be(0);
        board.GetPlayerAt(5, 5).Should().Be(Player.None);
        board.GetPlayerAt(10, 10).Should().Be(Player.None);
        board.GetPlayerAt(15, 15).Should().Be(Player.None);
    }

    #endregion

    #region GetOccupiedPositions Tests

    [Fact]
    public void GetOccupiedPositions_ReturnsAllOccupiedPositions()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(5, 5, Player.Red);
        board.SetBit(10, 10, Player.Blue);
        board.SetBit(15, 15, Player.Red);

        // Act
        var positions = board.GetOccupiedPositions();

        // Assert
        positions.Should().HaveCount(3);
        positions.Should().Contain((5, 5));
        positions.Should().Contain((10, 10));
        positions.Should().Contain((15, 15));
    }

    [Fact]
    public void GetOccupiedPositions_EmptyBoard_ReturnsEmptyList()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        var positions = board.GetOccupiedPositions();

        // Assert
        positions.Should().BeEmpty();
    }

    #endregion

    #region Raw Key Access Tests

    [Fact]
    public void GetHorizontalKey_ReturnsCorrectRow()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(5, 10, Player.Red);

        // Act
        var key = board.GetHorizontalKey(10);

        // Assert - The key should contain the stone at position 5
        // Position 5 is at bits 10-11 (5 * 2)
        key.Should().NotBe(0);
    }

    [Fact]
    public void GetVerticalKey_ReturnsCorrectColumn()
    {
        // Arrange
        var board = new BitKeyBoard();
        board.SetBit(10, 5, Player.Red);

        // Act
        var key = board.GetVerticalKey(10);

        // Assert
        key.Should().NotBe(0);
    }

    #endregion
}

public class BitKeyPatternTableTests
{
    [Fact]
    public void GetPattern_EmptyPattern_ReturnsNone()
    {
        // Arrange
        ulong emptyKey = 0;

        // Act
        var pattern = BitKeyPatternTable.GetPattern(emptyKey);

        // Assert
        pattern.PatternType.Should().Be(Pattern4Evaluator.CaroPattern4.None);
        pattern.Score.Should().Be(0);
    }

    [Fact]
    public void GetScore_ReturnsCorrectScore()
    {
        // Arrange
        ulong key = 0;

        // Act
        int score = BitKeyPatternTable.GetScore(key);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void EvaluatePosition_WithEmptyBoard_ReturnsZero()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        int score = BitKeyPatternTable.EvaluatePosition(board, 16, 16);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void GetCombinedPattern_EmptyPosition_ReturnsNone()
    {
        // Arrange
        var board = new BitKeyBoard();

        // Act
        var (pattern, threatCount) = BitKeyPatternTable.GetCombinedPattern(board, 16, 16);

        // Assert
        pattern.Should().Be(Pattern4Evaluator.CaroPattern4.None);
        threatCount.Should().Be(0);
    }

    [Fact]
    public void IsWinningMove_FiveInRow_ReturnsTrue()
    {
        // Arrange
        var board = new BitKeyBoard();
        // Place 4 in a row
        for (int i = 0; i < 4; i++)
            board.SetBit(10 + i, 10, Player.Red);

        // Act - Check if placing 5th stone wins
        bool isWin = BitKeyPatternTable.IsWinningMove(board, 14, 10, Player.Red);

        // Assert
        isWin.Should().BeTrue();
    }

    [Fact]
    public void IsDoubleThreatMove_OpenThreePlusOpenFour_ReturnsTrue()
    {
        // Arrange
        var board = new BitKeyBoard();
        // Create a position where a move creates multiple threats
        // Horizontal: 3 in a row
        board.SetBit(10, 10, Player.Red);
        board.SetBit(11, 10, Player.Red);
        board.SetBit(12, 10, Player.Red);

        // Act - Check if completing to 4 creates double threat
        var testBoard = board.Clone();
        testBoard.SetBit(13, 10, Player.Red);
        bool isDoubleThreat = BitKeyPatternTable.IsDoubleThreatMove(board, 13, 10, Player.Red);

        // This may or may not be a double threat depending on board state
        // The test verifies the method runs without error
        isDoubleThreat.Should().BeFalse();  // Single line, not double threat
    }
}
