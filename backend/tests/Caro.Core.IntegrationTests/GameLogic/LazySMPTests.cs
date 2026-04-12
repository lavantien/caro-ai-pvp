using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.IntegrationTests.GameLogic;

[Trait("Category", "Integration")]
public class LazySMPTests
{
    private const int SmallTTSizeMb = 1;
    private const int DefaultTTSizeMb = 256;
    private const int SingleThread = 1;
    private const int TwoThreads = 2;
    private const int FourThreads = 4;
    private const int ShortTimeoutMs = 500;
    private const int MediumTimeoutMs = 1000;
    private const int StandardTimeoutMs = 2000;
    private const int CenterMoveLowerBound = 6;
    private const int CenterMoveUpperBound = 10;
    [Fact]
    public void GetBestMoveWithStats_EmptyBoard_ReturnsCenterMove()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: TwoThreads);
        var board = new Board();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: StandardTimeoutMs, fixedThreadCount: TwoThreads);

        result.X.Should().BeInRange(CenterMoveLowerBound, CenterMoveUpperBound);
        result.Y.Should().BeInRange(CenterMoveLowerBound, CenterMoveUpperBound);
        result.ThreadCount.Should().Be(TwoThreads);
    }

    [Fact]
    public void GetBestMoveWithStats_FixedSingleThread_ThreadCountIsOne()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: SingleThread);
        var board = new Board();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: MediumTimeoutMs, fixedThreadCount: SingleThread);

        result.ThreadCount.Should().Be(SingleThread);
    }

    [Fact]
    public void GetBestMoveWithStats_MultiThread_ThreadCountMatchesRequested()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: FourThreads);
        var board = new Board();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: StandardTimeoutMs, fixedThreadCount: FourThreads);

        result.ThreadCount.Should().Be(FourThreads);
    }

    [Fact]
    public void GetBestMoveWithStats_WinningPosition_FindsWinningMove()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: TwoThreads);
        var board = new Board();
        // 4 in a row, can win with 5th
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);
        // Blue stones elsewhere
        board = board.PlaceStone(0, 0, Player.Blue);
        board = board.PlaceStone(1, 0, Player.Blue);
        board = board.PlaceStone(2, 0, Player.Blue);

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: StandardTimeoutMs, fixedThreadCount: TwoThreads);

        // Should find the winning move
        var isWinningMove = (result.X == 6 && result.Y == 7) ||
                            (result.X == 11 && result.Y == 7);
        isWinningMove.Should().BeTrue("should find the winning 5th stone in the row");
    }

    [Fact]
    public void GetBestMoveWithStats_ReturnsValidStatistics()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: TwoThreads);
        var board = new Board();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: StandardTimeoutMs, fixedThreadCount: TwoThreads);

        result.DepthAchieved.Should().BeGreaterThan(0);
        result.NodesSearched.Should().BeGreaterThan(0);
        result.AllocatedTimeMs.Should().BeGreaterThan(0);
    }

    [Fact]
    public void GetBestMoveWithStats_RespectsTimeLimit()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: TwoThreads);
        var board = new Board();
        var sw = System.Diagnostics.Stopwatch.StartNew();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: ShortTimeoutMs, fixedThreadCount: TwoThreads);

        sw.Stop();
        // Should not exceed hard time bound by much
        sw.ElapsedMilliseconds.Should().BeLessThan(5000);
    }

    [Fact]
    public void GetBestMoveWithStats_MidGamePosition_ReturnsValidMove()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb, maxThreads: TwoThreads);
        var board = new Board();
        board = board.PlaceStone(8, 8, Player.Red);
        board = board.PlaceStone(9, 9, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);

        var result = search.GetBestMoveWithStats(
            board, Player.Blue, timeRemainingMs: StandardTimeoutMs, fixedThreadCount: TwoThreads);

        result.X.Should().BeInRange(0, 15);
        result.Y.Should().BeInRange(0, 15);
    }

    [Fact]
    public void GetBestMoveWithStats_DefaultThreads_UsesLazySMPFormula()
    {
        var search = new ParallelMinimaxSearch(sizeMB: SmallTTSizeMb);
        var board = new Board();

        var result = search.GetBestMoveWithStats(
            board, Player.Red, timeRemainingMs: StandardTimeoutMs);

        // Should use default Lazy SMP formula
        result.ThreadCount.Should().BeGreaterThan(0);
    }

    [Fact]
    public void ParallelSearchResult_RecordEquality_Works()
    {
        var r1 = new ParallelSearchResult(5, 5, 4, 1000, 2);
        var r2 = new ParallelSearchResult(5, 5, 4, 1000, 2);
        r1.Should().Be(r2);
    }

    [Fact]
    public void ParallelSearchResult_Properties_AreSet()
    {
        var result = new ParallelSearchResult(
            X: 7, Y: 8, DepthAchieved: 6, NodesSearched: 5000,
            ThreadCount: 4, ParallelDiagnostics: "test",
            AllocatedTimeMs: 1000, TableHits: 10, TableLookups: 100,
            Score: 500, FirstMoveCutoffPercent: 0.9, EffectiveBranchingFactor: 2.5);

        result.X.Should().Be(7);
        result.Y.Should().Be(8);
        result.DepthAchieved.Should().Be(6);
        result.NodesSearched.Should().Be(5000);
        result.ThreadCount.Should().Be(4);
        result.Score.Should().Be(500);
    }
}
