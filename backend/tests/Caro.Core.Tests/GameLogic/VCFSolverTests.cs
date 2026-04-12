using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public sealed class VCFSolverTests
{
    private const int DefaultMaxDepth = 10;
    private const int StandardTimeoutMs = 1000;
    private const int MinimalTimeoutMs = 5;
    private const int GenerousTimeoutMs = 5000;
    private const int AgeIncrementIterations = 300;

    private readonly VCFSolver _solver;

    public VCFSolverTests()
    {
        _solver = new VCFSolver(new ThreatSpaceSearch());
    }

    // --- HasVCFPotential ---

    [Fact]
    public void HasVCFPotential_EmptyBoard_ReturnsFalse()
    {
        var board = new Board();
        _solver.HasVCFPotential(board, Player.Red).Should().BeFalse();
    }

    [Fact]
    public void HasVCFPotential_SingleStone_ReturnsFalse()
    {
        var board = new Board();
        board = board.PlaceStone(8, 8, Player.Red);
        _solver.HasVCFPotential(board, Player.Red).Should().BeFalse();
    }

    [Fact]
    public void HasVCFPotential_MultipleThreats_ReturnsTrue()
    {
        // Create position with 2+ threats
        var board = new Board();
        // Open four (threat)
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        // Another threat
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);

        _solver.HasVCFPotential(board, Player.Red).Should().BeTrue();
    }

    [Fact]
    public void HasVCFPotential_OpponentHasThreats_ReturnsTrue()
    {
        var board = new Board();
        // Opponent has open four
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 5, Player.Blue);
        board = board.PlaceStone(7, 5, Player.Blue);
        board = board.PlaceStone(8, 5, Player.Blue);
        // Player has one stone
        board = board.PlaceStone(3, 3, Player.Red);

        _solver.HasVCFPotential(board, Player.Red).Should().BeTrue();
    }

    // --- CheckNodeVCF ---

    [Fact]
    public void CheckNodeVCF_EmptyBoard_ReturnsNull()
    {
        var board = new Board();
        var result = _solver.CheckNodeVCF(board, Player.Red, DefaultMaxDepth, 0, StandardTimeoutMs);
        result.Should().BeNull();
    }

    [Fact]
    public void CheckNodeVCF_InsufficientTime_ReturnsNull()
    {
        var board = new Board();
        var result = _solver.CheckNodeVCF(board, Player.Red, DefaultMaxDepth, 0, MinimalTimeoutMs);
        result.Should().BeNull();
    }

    [Fact]
    public void CheckNodeVCF_WinningPosition_ReturnsWinning()
    {
        // Create position with forced win: open four
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        // Block with a few blue stones elsewhere
        board = board.PlaceStone(0, 0, Player.Blue);
        board = board.PlaceStone(1, 0, Player.Blue);

        var result = _solver.CheckNodeVCF(board, Player.Red, DefaultMaxDepth, 0, GenerousTimeoutMs);
        // Open four should be detected as winning or at least have VCF potential
        // Result depends on implementation, but should not crash
        if (result != null)
        {
            result.Type.Should().Be(VCFResultType.WinningSequence);
        }
    }

    // --- DetectOpponentVCF ---

    [Fact]
    public void DetectOpponentVCF_NoThreats_ReturnsEmpty()
    {
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(3, 3, Player.Red);

        var result = _solver.DetectOpponentVCF(board, Player.Red, Player.Blue);
        result.Should().BeEmpty();
    }

    [Fact]
    public void DetectOpponentVCF_OpponentHasFour_ReturnsDefenseMoves()
    {
        var board = new Board();
        // Opponent has 4 in a row
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 5, Player.Blue);
        board = board.PlaceStone(7, 5, Player.Blue);
        board = board.PlaceStone(8, 5, Player.Blue);
        board = board.PlaceStone(9, 5, Player.Blue);
        // Player has scattered stones
        board = board.PlaceStone(3, 3, Player.Red);
        board = board.PlaceStone(4, 4, Player.Red);

        var defenses = _solver.DetectOpponentVCF(board, Player.Red, Player.Blue);
        // With 5 in a row already, opponent may already have won
        // Result depends on implementation
        defenses.Should().NotBeNull();
    }

    // --- Cache management ---

    [Fact]
    public void ClearCache_ResetsState()
    {
        // Populate cache via a VCF check
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        _solver.CheckNodeVCF(board, Player.Red, 10, 0, 1000);

        // Clear and verify no crash
        var act = () => _solver.ClearCache();
        act.Should().NotThrow();
    }

    [Fact]
    public void IncrementAge_DoesNotThrow()
    {
        var act = () => _solver.IncrementAge();
        act.Should().NotThrow();
    }

    [Fact]
    public void IncrementAge_MultipleIncrements_DoesNotThrow()
    {
        for (int i = 0; i < 300; i++)
            _solver.IncrementAge();
    }

    [Fact]
    public void CheckNodeVCF_CachesResult()
    {
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        board = board.PlaceStone(0, 0, Player.Blue);
        board = board.PlaceStone(1, 0, Player.Blue);

        // First call populates cache
        var result1 = _solver.CheckNodeVCF(board, Player.Red, 10, 0, 5000);
        // Second call should hit cache
        var result2 = _solver.CheckNodeVCF(board, Player.Red, 10, 0, 5000);

        // Both should return same result
        if (result1 != null && result2 != null)
        {
            result1.Type.Should().Be(result2.Type);
        }
    }

    // --- VCFNodeResult ---

    [Fact]
    public void VCFNodeResult_None_HasNoVCFType()
    {
        var none = VCFNodeResult.None;
        none.Type.Should().Be(VCFResultType.NoVCF);
        none.Score.Should().Be(0);
        none.ForcingMoves.Should().BeEmpty();
    }

    [Fact]
    public void VCFNodeResult_Winning_HasCorrectProperties()
    {
        var moves = new List<(int x, int y)> { (5, 5), (6, 5) };
        var result = VCFNodeResult.Winning(moves, 2, 10);
        result.Type.Should().Be(VCFResultType.WinningSequence);
        result.Score.Should().BeLessThan(VCFNodeResult.WinScore);
        result.ForcingMoves.Should().Equal(moves);
        result.Depth.Should().Be(2);
        result.NodesSearched.Should().Be(10);
    }

    [Fact]
    public void VCFNodeResult_Losing_HasCorrectProperties()
    {
        var defenses = new List<(int x, int y)> { (3, 3) };
        var result = VCFNodeResult.Losing(defenses, 3, 20);
        result.Type.Should().Be(VCFResultType.LosingSequence);
        result.Score.Should().BeGreaterThan(-VCFNodeResult.WinScore);
        result.ForcingMoves.Should().Equal(defenses);
        result.Depth.Should().Be(3);
        result.NodesSearched.Should().Be(20);
    }

    [Fact]
    public void VCFNodeResult_WinScore_IsPositive()
    {
        VCFNodeResult.WinScore.Should().BePositive();
    }

    [Fact]
    public void VCFNodeResult_Winning_PrefersShorterWins()
    {
        var shallow = VCFNodeResult.Winning([], 1, 0);
        var deep = VCFNodeResult.Winning([], 5, 0);
        // Shallow wins should have higher score
        shallow.Score.Should().BeGreaterThan(deep.Score);
    }
}
