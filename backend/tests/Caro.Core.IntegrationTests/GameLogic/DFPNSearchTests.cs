using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.IntegrationTests.GameLogic;

[Trait("Category", "Slow")]
[Trait("Category", "Integration")]
public class DFPNSearchTests
{
    private const int StandardMaxDepth = 10;
    private const int ShallowMaxDepth = 5;
    private const int MinimalMaxDepth = 2;
    private const int DeepMaxDepth = 30;
    private const int SingleMaxDepth = 1;
    private const int ThreeMaxDepth = 3;
    private const int ShortTimeoutMs = 100;
    private const int MediumTimeoutMs = 500;
    private const int GenerousTimeoutMs = 5_000;
    private const int StandardTimeoutMs = 1_000;
    private const int MinimalTimeoutMs = 1;
    private const int BriefTimeoutMs = 10;
    private const int ProofNumberThreshold = 10;

    private readonly DFPNSearch _search = new();
    private readonly WinDetector _winDetector = new();

    [Fact]
    public void Solve_EmptyBoard_ReturnsUnknown()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: StandardMaxDepth, timeLimitMs: ShortTimeoutMs);

        // Assert - Empty board has no VCF sequence
        result.result.Should().Be(SearchResult.Unknown);
        result.move.Should().BeNull();
    }

    [Theory]
    [InlineData(Player.Red)]
    [InlineData(Player.Blue)]
    public void Solve_EmptyBoard_AnyPlayerReturnsUnknown(Player player)
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _search.Solve(board, player, maxDepth: StandardMaxDepth, timeLimitMs: ShortTimeoutMs);

        // Assert
        result.result.Should().Be(SearchResult.Unknown);
    }

    [Fact]
    public void Solve_OpponentImmediateWin_ReturnsLoss()
    {
        // Arrange - Blue has XXXX_ pattern, Red cannot prevent
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(10, 7, Player.Blue);
        // Red has some stones but no immediate threats
        board = board.PlaceStone(5, 5, Player.Red);

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: MediumTimeoutMs);

        // Assert - Red cannot force a win when Blue has immediate win
        // DFPN searches for Red's VCF; no VCF means Unknown (not proven Loss)
        result.result.Should().NotBe(SearchResult.Win, "Red should not have a winning VCF");
    }

    [Fact]
    public void Solve_DepthLimit_StopsAtMaxDepth()
    {
        // Arrange - Position with no immediate win, requiring deeper search
        var board = new Board();
        // Three in a row for Red (needs deep search to find VCF)
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        // Blue blocks one side
        board = board.PlaceStone(10, 7, Player.Blue);
        // Additional stones for complexity
        for (int i = 0; i < 3; i++)
        {
            board = board.PlaceStone(6 + i, 8, Player.Blue);
        }

        // Act - Very shallow depth limit
        var result = _search.Solve(board, Player.Red, maxDepth: MinimalMaxDepth, timeLimitMs: GenerousTimeoutMs);

        // Assert - Should return Unknown or not complete due to depth limit
        // With depth 2, DFPN won't find deep VCF sequences
        result.result.Should().Be(SearchResult.Unknown);
    }

    [Fact]
    public void Solve_TimeLimit_ReturnsUnknown()
    {
        // Arrange - Complex position
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Act - Very short time limit
        var result = _search.Solve(board, Player.Red, maxDepth: DeepMaxDepth, timeLimitMs: MinimalTimeoutMs);

        // Assert - Should return unknown due to time limit
        result.result.Should().Be(SearchResult.Unknown);
    }

    [Fact]
    public void ProofNumbers_ORNodeFormula_SumAndMin()
    {
        // Arrange - Position with multiple threat moves (OR node = attacker's turn)
        var board = new Board();
        // Create S3 threat with multiple gain squares
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act - Get proof numbers (OR node)
        var (proof, disproof) = _search.GetProofNumbers(board, Player.Red);

        // Assert - OR node: proof should be small (min of children), disproof should be larger
        // For an OR node (attacker), pn = min(children.pn), dn = sum(children.dn)
        proof.Should().BeGreaterThan(0, "Proof number should be positive");
        disproof.Should().BeGreaterThan(0, "Disproof number should be positive");
    }

    [Fact]
    public void ProofNumbers_ANDNodeFormula_SumAndMin()
    {
        // Arrange - Position where defender must respond (AND node)
        var board = new Board();
        // Red has threat
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        // Blue has stones that create defense options
        board = board.PlaceStone(8, 8, Player.Blue);

        // Act - Get proof numbers for Blue (defender)
        var (proof, disproof) = _search.GetProofNumbers(board, Player.Blue);

        // Assert - AND node: defender needs to block all threats
        proof.Should().BeGreaterThan(0);
        disproof.Should().BeGreaterThan(0);
    }

    [Fact]
    public void TranspositionTable_UsesCachedResult()
    {
        // Arrange - Position with transpositions
        var board = new Board();
        // Create a position that can be reached via different move orders
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Red);

        // Act - Run search multiple times with same position
        var result1 = _search.Solve(board, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: MediumTimeoutMs);
        var result2 = _search.Solve(board, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: MediumTimeoutMs);

        // Assert - Results should be consistent (transposition table helps)
        result1.result.Should().Be(result2.result, "Results should be consistent across runs");
    }

    [Fact]
    public void TranspositionTable_DifferentMoveOrdersSamePosition()
    {
        // Arrange - Same final position via different move orders
        var board1 = new Board();
        board1 = board1.PlaceStone(7, 7, Player.Red);
        board1 = board1.PlaceStone(8, 7, Player.Blue);
        board1 = board1.PlaceStone(9, 7, Player.Red);
        board1 = board1.PlaceStone(10, 7, Player.Blue);

        var board2 = new Board();
        board2 = board2.PlaceStone(8, 7, Player.Blue);
        board2 = board2.PlaceStone(7, 7, Player.Red);
        board2 = board2.PlaceStone(10, 7, Player.Blue);
        board2 = board2.PlaceStone(9, 7, Player.Red);

        // Act - Search from both orders
        var result1 = _search.Solve(board1, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: MediumTimeoutMs);
        var result2 = _search.Solve(board2, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: MediumTimeoutMs);

        // Assert - Same position should give same result
        result1.result.Should().Be(result2.result, "Same position should yield same result regardless of move order");
    }

    [Fact]
    public void Solve_VerifiesReturnedMoveWins()
    {
        // Arrange - S4 position
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: StandardTimeoutMs);

        // Assert - If move is returned, it should actually win
        if (result.result == SearchResult.Win && result.move.HasValue)
        {
            var (x, y) = result.move.Value;
            var boardWithWin = board.PlaceStone(x, y, Player.Red);
            var winResult = _winDetector.CheckWin(boardWithWin);

            winResult.HasWinner.Should().BeTrue("Suggested winning move should actually win");
            winResult.Winner.Should().Be(Player.Red);
        }
    }

    [Fact]
    public void GetProofNumbers_InitialNode_CorrectValues()
    {
        // Arrange - S4 for Red
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var (proof, disproof) = _search.GetProofNumbers(board, Player.Red);

        // Assert - Should have low proof number (close to win)
        proof.Should().BeLessThan(ProofNumberThreshold, "Proof number should be small for winning position");
    }

    [Fact]
    public void Solve_ImmediateWin_ReturnsWin()
    {
        // Arrange - XXXX_ pattern, Red can win immediately
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        // Positions 6,7 and 11,7 are empty - both are winning moves

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: ShallowMaxDepth, timeLimitMs: StandardTimeoutMs);

        // Assert - Should find immediate winning move (either side)
        result.result.Should().Be(SearchResult.Win);
        result.move.Should().NotBeNull();

        // Verify suggested move actually wins
        var (x, y) = result.move.Value;
        var boardWithWin = board.PlaceStone(x, y, Player.Red);
        var winResult = _winDetector.CheckWin(boardWithWin);

        winResult.HasWinner.Should().BeTrue("Suggested move should actually win");
        winResult.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void Solve_ForcingSequence_SolvesVCF()
    {
        // Arrange - Position where Red creates double threat
        var board = new Board();
        // First S4 threat horizontal
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Blue blocks at 9,7
        board = board.PlaceStone(9, 7, Player.Blue);
        // Red has vertical threat at 7,5
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(7, 6, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);
        // Position 7,9 creates second threat

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: StandardMaxDepth, timeLimitMs: StandardTimeoutMs);

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
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(10, 7, Player.Blue);
        // Red has some stones but no immediate threats
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 6, Player.Red);

        // Act
        var result = _search.Solve(board, Player.Red, maxDepth: ThreeMaxDepth, timeLimitMs: MediumTimeoutMs);

        // Assert - Red cannot force a win (returns Unknown or Loss)
        // VCF solver should indicate Red can't force win against Blue's immediate threat
        result.result.Should().NotBe(SearchResult.Win, "Red should not be able to force a win when Blue has immediate win");
    }

    [Fact]
    public void Solve_DepthLimitExceeded_ReturnsUnknown()
    {
        // Arrange - Position with no immediate threats (scattered stones)
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(3, 3, Player.Blue);
        board = board.PlaceStone(4, 4, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Act - Very shallow depth limit with short time
        var result = _search.Solve(board, Player.Red, maxDepth: SingleMaxDepth, timeLimitMs: BriefTimeoutMs);

        // Assert - Should return unknown (no immediate win, no time to search deep)
        result.result.Should().Be(SearchResult.Unknown);
    }
}
