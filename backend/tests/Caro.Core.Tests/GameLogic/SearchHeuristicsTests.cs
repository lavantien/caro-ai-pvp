using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;

namespace Caro.Core.Tests.GameLogic;

public class SearchHeuristicsTests
{
    [Fact]
    public void RecordKillerMove_ShouldStoreAtDepth()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordKillerMove(depth: 5, x: 7, y: 8);

        heuristics.IsKillerMove(5, 7, 8).Should().BeTrue();
    }

    [Fact]
    public void RecordKillerMove_ShouldShiftExistingMoves()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordKillerMove(depth: 3, x: 5, y: 5);
        heuristics.RecordKillerMove(depth: 3, x: 7, y: 8);

        // Most recent should be at slot 0
        heuristics.IsKillerMove(3, 7, 8).Should().BeTrue();
        // Previous should still be found
        heuristics.IsKillerMove(3, 5, 5).Should().BeTrue();
    }

    [Fact]
    public void IsKillerMove_ShouldReturnFalse_ForInvalidDepth()
    {
        var heuristics = new SearchHeuristics();

        heuristics.IsKillerMove(-1, 0, 0).Should().BeFalse();
        heuristics.IsKillerMove(SearchConstants.MaxKillerDepth, 0, 0).Should().BeFalse();
    }

    [Fact]
    public void GetKillerMoves_ShouldReturnMovesForDepth()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordKillerMove(depth: 2, x: 3, y: 4);
        heuristics.RecordKillerMove(depth: 2, x: 5, y: 6);

        var moves = heuristics.GetKillerMoves(2);
        moves.Should().HaveCount(SearchConstants.MaxKillerMoves);
        moves[0].Should().Be((5, 6)); // Most recent first
        moves[1].Should().Be((3, 4));
    }

    [Fact]
    public void RecordHistoryMove_RedPlayer_ShouldAccumulate()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 5);
        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 3);

        // depth^2 bonus: 5*5 + 3*3 = 25 + 9 = 34
        heuristics.GetHistoryScore(Player.Red, 7, 7).Should().Be(34);
    }

    [Fact]
    public void RecordHistoryMove_BluePlayer_ShouldBeSeparate()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 5);
        heuristics.RecordHistoryMove(Player.Blue, 7, 7, depth: 5);

        heuristics.GetHistoryScore(Player.Red, 7, 7).Should().Be(25);
        heuristics.GetHistoryScore(Player.Blue, 7, 7).Should().Be(25);
    }

    [Fact]
    public void GetHistoryScore_ShouldReturnZero_ForUnrecordedMoves()
    {
        var heuristics = new SearchHeuristics();

        heuristics.GetHistoryScore(Player.Red, 5, 5).Should().Be(0);
    }

    [Fact]
    public void ButterflyScore_ShouldAccumulateWithDoubleBonus()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 4);

        // butterfly bonus = depth^2 * 2 = 16 * 2 = 32
        heuristics.GetButterflyScore(Player.Red, 7, 7).Should().Be(32);
    }

    [Fact]
    public void ButterflyScore_ShouldBeSeparateForPlayers()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 3, 3, depth: 2);
        heuristics.RecordHistoryMove(Player.Blue, 3, 3, depth: 3);

        heuristics.GetButterflyScore(Player.Red, 3, 3).Should().Be(8);  // 2*2*2
        heuristics.GetButterflyScore(Player.Blue, 3, 3).Should().Be(18); // 3*3*2
    }

    [Fact]
    public void ClearHistory_ShouldResetHistoryAndButterflyTables()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 5);
        heuristics.RecordHistoryMove(Player.Blue, 7, 7, depth: 5);
        heuristics.ClearHistory();

        heuristics.GetHistoryScore(Player.Red, 7, 7).Should().Be(0);
        heuristics.GetHistoryScore(Player.Blue, 7, 7).Should().Be(0);
        heuristics.GetButterflyScore(Player.Red, 7, 7).Should().Be(0);
        heuristics.GetButterflyScore(Player.Blue, 7, 7).Should().Be(0);
    }

    [Fact]
    public void ClearKillers_ShouldResetAllKillerMoves()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordKillerMove(depth: 1, x: 5, y: 5);
        heuristics.RecordKillerMove(depth: 2, x: 7, y: 8);
        heuristics.ClearKillers();

        heuristics.IsKillerMove(1, 5, 5).Should().BeFalse();
        heuristics.IsKillerMove(2, 7, 8).Should().BeFalse();
    }

    [Fact]
    public void ClearHistory_ShouldNotAffectKillers()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordKillerMove(depth: 1, x: 5, y: 5);
        heuristics.ClearHistory();

        heuristics.IsKillerMove(1, 5, 5).Should().BeTrue();
    }

    [Fact]
    public void ClearKillers_ShouldNotAffectHistory()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 7, 7, depth: 5);
        heuristics.ClearKillers();

        heuristics.GetHistoryScore(Player.Red, 7, 7).Should().Be(25);
    }

    [Fact]
    public void RecordHistoryMove_ZeroDepth_ShouldAddZero()
    {
        var heuristics = new SearchHeuristics();

        heuristics.RecordHistoryMove(Player.Red, 5, 5, depth: 0);

        heuristics.GetHistoryScore(Player.Red, 5, 5).Should().Be(0);
        heuristics.GetButterflyScore(Player.Red, 5, 5).Should().Be(0);
    }

    [Fact]
    public void KillerMoves_MaxSlots_ShouldShiftOldMovesOut()
    {
        var heuristics = new SearchHeuristics();

        // Fill more than MaxKillerMoves slots at the same depth
        heuristics.RecordKillerMove(depth: 1, x: 1, y: 1);
        heuristics.RecordKillerMove(depth: 1, x: 2, y: 2);
        heuristics.RecordKillerMove(depth: 1, x: 3, y: 3);

        // Slot 0 should be most recent (3,3), slot 1 should be (2,2), oldest (1,1) evicted
        var moves = heuristics.GetKillerMoves(1);
        moves[0].Should().Be((3, 3));
        moves[1].Should().Be((2, 2));
        heuristics.IsKillerMove(1, 1, 1).Should().BeFalse();
    }
}
