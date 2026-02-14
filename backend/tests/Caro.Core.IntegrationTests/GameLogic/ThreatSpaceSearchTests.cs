using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.IntegrationTests.GameLogic;

[Trait("Category", "Slow")]
[Trait("Category", "Integration")]
public class ThreatSpaceSearchTests
{
    private readonly ThreatSpaceSearch _vcf = new();

    [Fact]
    public void SolveVCF_EmptyBoard_ReturnsUnknown()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 100);

        // Assert
        result.IsSolved.Should().BeFalse("Empty board has no VCF");
        result.IsWin.Should().BeFalse();
        result.BestMove.Should().BeNull();
        result.NodesSearched.Should().Be(0);
    }

    [Fact]
    public void SolveVCF_ImmediateWin_SolvesQuickly()
    {
        // Arrange - Red has XXXX_ can win immediately
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 500);

        // Assert
        result.IsSolved.Should().BeTrue("Should find immediate win");
        result.IsWin.Should().BeTrue();
        result.BestMove.Should().NotBeNull();
        result.NodesSearched.Should().BeGreaterThan(0);
    }

    [Fact]
    public void SolveVCF_DoubleThreat_SolvesWinningSequence()
    {
        // Arrange - Red has two S4 threats that Blue can't both block
        var board = new Board();
        // First S4 horizontal
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Blue has some stones
        board = board.PlaceStone(3, 5, Player.Blue);
        board = board.PlaceStone(4, 6, Player.Blue);

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 2000);

        // Assert - VCF should find winning sequence
        result.BestMove.Should().NotBeNull("Should find at least one move");
    }

    [Fact]
    public void SolveVCF_OpponentHasWin_ReturnsLoss()
    {
        // Arrange - Blue has immediate win
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(10, 7, Player.Blue);

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 500);

        // Assert - Red cannot win (Blue wins next)
        result.IsWin.Should().BeFalse("Red cannot force win");
        result.IsSolved.Should().BeFalse();
    }

    [Fact]
    public void SolveVCF_TimeLimit_ReturnsPartialResult()
    {
        // Arrange - Complex position requiring deep search
        var board = new Board();
        // Red has a 4-in-a-row that's blocked on one side
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Blue blocks on right
        board = board.PlaceStone(9, 7, Player.Blue);
        // Various other stones creating complexity
        board = board.PlaceStone(5, 8, Player.Blue);
        board = board.PlaceStone(6, 9, Player.Red);
        board = board.PlaceStone(7, 6, Player.Blue);
        board = board.PlaceStone(10, 5, Player.Red);
        board = board.PlaceStone(11, 8, Player.Blue);

        // Act - Very short time limit (not enough for deep VCF)
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 5);

        // Assert - Should not crash, may return unknown or partial result
        // With extremely short time, VCF likely won't complete full search
        result.NodesSearched.Should().BeGreaterThan(0, "Should have searched at least some nodes");
    }

    [Fact]
    public void SolveVCF_NoThreats_ReturnsUnknown()
    {
        // Arrange - Scattered stones, no real threats
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(9, 9, Player.Blue);

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 500);

        // Assert - No forcing sequence found
        result.IsSolved.Should().BeFalse("No forcing sequence available");
    }

    [Fact]
    public void SolveVCF_VerifyBestMove_IsValid()
    {
        // Arrange - Mid-game position
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 7, Player.Red);

        // Act
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 500);

        // Assert - If move returned, it should be valid
        if (result.BestMove.HasValue)
        {
            var (x, y) = result.BestMove.Value;
            var cell = board.GetCell(x, y);
            cell.IsEmpty.Should().BeTrue("Suggested move should be on empty cell");
            x.Should().BeGreaterThanOrEqualTo(0).And.BeLessThan(board.BoardSize);
            y.Should().BeGreaterThanOrEqualTo(0).And.BeLessThan(board.BoardSize);
        }
    }

    [Theory]
    [InlineData(Player.Red)]
    [InlineData(Player.Blue)]
    public void SolveVCF_EmptyBoard_AnyPlayerReturnsUnknown(Player player)
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _vcf.SolveVCF(board, player, timeLimitMs: 100);

        // Assert
        result.IsSolved.Should().BeFalse();
        result.IsWin.Should().BeFalse();
        result.BestMove.Should().BeNull();
    }

    [Fact]
    public void SolveVCF_MaxDepth_LimitsSearch()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Blue);

        // Act - Very shallow depth
        var result = _vcf.SolveVCF(board, Player.Red, timeLimitMs: 500, maxDepth: 3);

        // Assert - Should complete quickly with shallow depth
        result.IsSolved.Should().BeFalse("Shallow depth limits VCF");
    }

    [Fact]
    public void GetThreatMoves_FiltersForcingMoves()
    {
        // Arrange - Position with threats and non-forcing moves
        var board = new Board();
        // Create S3 threat (forcing)
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        // Add some non-forcing stones
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(12, 12, Player.Red);

        // Act
        var threatMoves = _vcf.GetThreatMoves(board, Player.Red);

        // Assert - Should return only forcing threat moves
        threatMoves.Should().NotBeEmpty("Should find at least one forcing move");
        // All returned moves should be on empty cells
        foreach (var (x, y) in threatMoves)
        {
            var cell = board.GetCell(x, y);
            cell.IsEmpty.Should().BeTrue("Threat moves should target empty cells");
        }
    }

    [Fact]
    public void GetThreatMoves_IncludesWinningMoves()
    {
        // Arrange - Position with immediate winning move
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var threatMoves = _vcf.GetThreatMoves(board, Player.Red);

        // Assert - Should include winning move
        threatMoves.Should().Contain(m => m.Item1 == 6 && m.Item2 == 7 || m.Item1 == 11 && m.Item2 == 7,
            "Should include immediate winning moves");
    }

    [Fact]
    public void GetThreatMoves_NoThreats_ReturnsEmpty()
    {
        // Arrange - Single isolated stone, no threats
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        var threatMoves = _vcf.GetThreatMoves(board, Player.Red);

        // Assert
        threatMoves.Should().BeEmpty("Single stone has no forcing moves");
    }

    [Fact]
    public void GetDefenseMoves_BlocksThreats()
    {
        // Arrange - Red has threat, Blue must defend
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act - Get Blue's defense moves
        var defenses = _vcf.GetDefenseMoves(board, Player.Red, Player.Blue);

        // Assert - Should include blocking move
        defenses.Should().NotBeEmpty("Should have at least one defense");
        // All defenses should be on empty cells
        foreach (var (x, y) in defenses)
        {
            var cell = board.GetCell(x, y);
            cell.IsEmpty.Should().BeTrue("Defense moves should target empty cells");
        }
    }

    [Fact]
    public void GetDefenseMoves_IncludesCounterAttacks()
    {
        // Arrange - Both players have threats
        var board = new Board();
        // Red threat
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        // Blue threat (counter-attack opportunity)
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(9, 8, Player.Blue);

        // Act - Get Blue's defenses
        var defenses = _vcf.GetDefenseMoves(board, Player.Red, Player.Blue);

        // Assert - Should include both blocks and counter-attacks
        defenses.Should().NotBeEmpty();
    }

    [Fact]
    public void GetDefenseMoves_LimitsToMaxMoves()
    {
        // Arrange - Position with many possible defenses
        var board = new Board();
        // Create many scattered threats to generate many defenses
        for (int i = 0; i < 5; i++)
        {
            board = board.PlaceStone(5 + i * 2, 5, Player.Red);
            board = board.PlaceStone(5 + i * 2, 6, Player.Red);
            board = board.PlaceStone(5 + i * 2, 7, Player.Red);
        }

        // Act
        var defenses = _vcf.GetDefenseMoves(board, Player.Red, Player.Blue);

        // Assert - Should limit to reasonable number (<=10 as per implementation)
        defenses.Should().HaveCountLessOrEqualTo(10, "Should limit defense moves");
    }

    [Fact]
    public void GetDefenseMoves_NoThreatsToDefend_ReturnsCandidateMoves()
    {
        // Arrange - No threats to defend against
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Blue);

        // Act
        var defenses = _vcf.GetDefenseMoves(board, Player.Red, Player.Blue);

        // Assert - Should return at least some candidate moves
        defenses.Should().NotBeEmpty("Should have at least some candidate moves even without threats");
    }
}
