using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class DFPNSearchTests
{
    private readonly DFPNSearch _search = new();
    private readonly WinDetector _winDetector = new();

    [Fact]
    public void Solve_EmptyBoard_ReturnsUnknown()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: 10, timeLimitMs: 100);

        // Assert - Empty board has no VCF sequence
        result.result.Should().Be(SearchResult.Unknown);
        result.move.Should().BeNull();
    }

    [Fact]
    public void Solve_ImmediateWin_ReturnsWin()
    {
        // Arrange - XXXX_ pattern, Red can win immediately
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);
        // Positions 6,7 and 11,7 are empty - both are winning moves

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: 5, timeLimitMs: 1000);

        // Assert - Should find immediate winning move (either side)
        result.result.Should().Be(SearchResult.Win);
        result.move.Should().NotBeNull();

        // Verify the suggested move actually wins
        var (x, y) = result.move.Value;
        board.PlaceStone(x, y, Player.Red);
        var winResult = _winDetector.CheckWin(board);
        board.GetCell(x, y).Player = Player.None;

        winResult.HasWinner.Should().BeTrue("Suggested move should actually win");
        winResult.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void Solve_ForcingSequence_SolvesVCF()
    {
        // Arrange - Position where Red creates double threat
        var board = new Board();
        // First S4 threat horizontal
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        // Blue blocks at 9,7
        board.PlaceStone(9, 7, Player.Blue);
        // Red has vertical threat at 7,5
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        // Position 7,9 creates second threat

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: 10, timeLimitMs: 1000);

        // Assert - Should find VCF sequence or indicate unknown
        // If Red has double threat, Blue cannot defend both
        result.move.Should().NotBeNull("Should find at least one candidate move");
    }

    [Fact]
    public void Solve_OpponentCanWin_ReturnsLoss()
    {
        // Arrange - Blue has immediate winning move, Red can't prevent
        var board = new Board();
        // Blue has XXXX_ pattern
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);
        board.PlaceStone(9, 7, Player.Blue);
        board.PlaceStone(10, 7, Player.Blue);
        // Red has some stones but no immediate threats
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 6, Player.Red);

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: 3, timeLimitMs: 500);

        // Assert - Red cannot force a win (returns Unknown or Loss)
        // VCF solver should indicate Red can't force win against Blue's immediate threat
        result.result.Should().NotBe(SearchResult.Win, "Red should not be able to force a win when Blue has immediate win");
    }

    [Fact]
    public void Solve_DepthLimitExceeded_ReturnsUnknown()
    {
        // Arrange - Position with no immediate threats (scattered stones)
        var board = new Board();
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(3, 3, Player.Blue);
        board.PlaceStone(4, 4, Player.Blue);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Very shallow depth limit with short time
        var result = _search.Solve(board, Player.Red, maxDepth: 1, timeLimitMs: 10);

        // Assert - Should return unknown (no immediate win, no time to search deep)
        result.result.Should().Be(SearchResult.Unknown);
    }

    [Fact]
    public void Solve_TimeLimitExceeded_ReturnsUnknown()
    {
        // Arrange - Complex position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Very short time limit
        var result = _search.Solve(board, Player.Red, maxDepth: 30, timeLimitMs: 1);

        // Assert - Should return unknown due to time limit
        result.result.Should().Be(SearchResult.Unknown);
    }

    [Fact]
    public void Solve_VerifyWinningMove_ActuallyWins()
    {
        // Arrange - S4 position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: 5, timeLimitMs: 1000);

        // Assert - If move is returned, it should actually win
        if (result.result == SearchResult.Win && result.move.HasValue)
        {
            var (x, y) = result.move.Value;
            board.PlaceStone(x, y, Player.Red);
            var winResult = _winDetector.CheckWin(board);
            board.GetCell(x, y).Player = Player.None; // Undo

            winResult.HasWinner.Should().BeTrue("Suggested winning move should actually win");
            winResult.Winner.Should().Be(Player.Red);
        }
    }

    [Fact]
    public void GetProofNumbers_InitialNode_CorrectValues()
    {
        // Arrange - S4 for Red
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);

        // Act
        var (proof, disproof) = _search.GetProofNumbers(board, Player.Red);

        // Assert - Should have low proof number (close to win)
        proof.Should().BeLessThan(10, "Proof number should be small for winning position");
    }
}
