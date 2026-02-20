using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for SearchBoardExtensions.
/// </summary>
public class SearchBoardExtensionsTests
{
    [Fact]
    public void GetCandidateMoves_OnEmptyBoard_ReturnsEmpty()
    {
        var board = new SearchBoard();

        var candidates = board.GetCandidateMoves();

        candidates.Should().BeEmpty("empty board has no stones to expand around");
    }

    [Fact]
    public void GetCandidateMoves_AfterFirstMove_ReturnsNearbyCells()
    {
        var board = new SearchBoard();
        board.MakeMove(8, 8, Player.Red);

        var candidates = board.GetCandidateMoves(radius: 2);

        // Should have cells within radius 2 of (8,8)
        candidates.Should().NotBeEmpty();
        candidates.Should().AllSatisfy(c =>
        {
            Math.Abs(c.x - 8).Should().BeLessThanOrEqualTo(2);
            Math.Abs(c.y - 8).Should().BeLessThanOrEqualTo(2);
        });
    }

    [Fact]
    public void HasWin_NoFiveInRow_ReturnsFalse()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(5, 6, Player.Red);

        board.HasWin(Player.Red).Should().BeFalse();
    }

    [Fact]
    public void HasWin_FiveInRow_ReturnsTrue()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(5, 6, Player.Red);
        board.MakeMove(5, 7, Player.Red);
        board.MakeMove(5, 8, Player.Red);
        board.MakeMove(5, 9, Player.Red);

        board.HasWin(Player.Red).Should().BeTrue();
    }

    [Fact]
    public void HasWin_FiveInColumn_ReturnsTrue()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Blue);
        board.MakeMove(6, 5, Player.Blue);
        board.MakeMove(7, 5, Player.Blue);
        board.MakeMove(8, 5, Player.Blue);
        board.MakeMove(9, 5, Player.Blue);

        board.HasWin(Player.Blue).Should().BeTrue();
    }

    [Fact]
    public void HasWin_FiveDiagonal_ReturnsTrue()
    {
        var board = new SearchBoard();
        // Diagonal \
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(6, 6, Player.Red);
        board.MakeMove(7, 7, Player.Red);
        board.MakeMove(8, 8, Player.Red);
        board.MakeMove(9, 9, Player.Red);

        board.HasWin(Player.Red).Should().BeTrue();
    }

    [Fact]
    public void HasWin_FiveAntiDiagonal_ReturnsTrue()
    {
        var board = new SearchBoard();
        // Diagonal /
        board.MakeMove(5, 9, Player.Blue);
        board.MakeMove(6, 8, Player.Blue);
        board.MakeMove(7, 7, Player.Blue);
        board.MakeMove(8, 6, Player.Blue);
        board.MakeMove(9, 5, Player.Blue);

        board.HasWin(Player.Blue).Should().BeTrue();
    }

    [Fact]
    public void IsWinningMove_WhenMoveWins_ReturnsTrue()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(5, 6, Player.Red);
        board.MakeMove(5, 7, Player.Red);
        board.MakeMove(5, 8, Player.Red);

        board.IsWinningMove(5, 9, Player.Red).Should().BeTrue();
        board.IsWinningMove(5, 4, Player.Red).Should().BeTrue();
    }

    [Fact]
    public void IsWinningMove_WhenNotWinning_ReturnsFalse()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(5, 6, Player.Red);

        board.IsWinningMove(5, 7, Player.Red).Should().BeFalse();
    }

    [Fact]
    public void IsWinningMove_DoesNotModifyBoard()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(5, 6, Player.Red);
        board.MakeMove(5, 7, Player.Red);
        board.MakeMove(5, 8, Player.Red);
        var hashBefore = board.GetHash();
        var stonesBefore = board.TotalStones();

        _ = board.IsWinningMove(5, 9, Player.Red);

        board.GetHash().Should().Be(hashBefore);
        board.TotalStones().Should().Be(stonesBefore);
    }

    [Fact]
    public void StoneCount_ReturnsCorrectCount()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(6, 6, Player.Red);
        board.MakeMove(7, 7, Player.Blue);

        board.StoneCount(Player.Red).Should().Be(2);
        board.StoneCount(Player.Blue).Should().Be(1);
        board.StoneCount(Player.None).Should().Be(0);
    }

    [Fact]
    public void GetOccupiedPositions_ReturnsCorrectPositions()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(7, 7, Player.Red);
        board.MakeMove(10, 10, Player.Blue);

        var redPositions = board.GetOccupiedPositions(Player.Red).ToList();
        var bluePositions = board.GetOccupiedPositions(Player.Blue).ToList();

        redPositions.Should().HaveCount(2);
        redPositions.Should().Contain((5, 5));
        redPositions.Should().Contain((7, 7));

        bluePositions.Should().HaveCount(1);
        bluePositions.Should().Contain((10, 10));
    }
}
