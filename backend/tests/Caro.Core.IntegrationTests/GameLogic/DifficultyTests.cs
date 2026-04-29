using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.IntegrationTests.Helpers;
using FluentAssertions;

namespace Caro.Core.IntegrationTests.GameLogic;

[Trait("Category", "Slow")]
[Trait("Category", "Integration")]
public class DifficultyTests
{
    [Fact]
    public void GetBestMove_L1Difficulty_ReturnsValidMove()
    {
        using var ai = AITestHelper.CreateAI();
        var board = new Board();
        var opts = new SearchOptions
        {
            TimeFraction = 0.05, UseVCF = false, ThreadCount = 1,
            PonderingEnabled = false, ParallelSearchEnabled = false,
        };
        var (x, y) = ai.GetBestMove(board, Player.Red, opts, CancellationToken.None);
        x.Should().BeInRange(0, 15);
        y.Should().BeInRange(0, 15);
    }

    [Fact]
    public void GetBestMove_L3Difficulty_ReturnsValidMove()
    {
        using var ai = AITestHelper.CreateAI();
        var board = new Board();
        var opts = new SearchOptions
        {
            TimeFraction = 0.40, UseVCF = true, ThreadCount = 2,
            PonderingEnabled = false, ParallelSearchEnabled = true,
        };
        var (x, y) = ai.GetBestMove(board, Player.Red, opts, CancellationToken.None);
        x.Should().BeInRange(0, 15);
        y.Should().BeInRange(0, 15);
    }

    [Fact]
    public void GetBestMove_L5Difficulty_ReturnsValidMove()
    {
        using var ai = AITestHelper.CreateAI();
        var board = new Board();
        var opts = new SearchOptions
        {
            TimeFraction = 1.0, UseVCF = true,
            ThreadCount = ThreadPoolConfig.MaxEngineThreads,
            PonderingEnabled = true, ParallelSearchEnabled = true,
        };
        var (x, y) = ai.GetBestMove(board, Player.Red, opts, CancellationToken.None);
        x.Should().BeInRange(0, 15);
        y.Should().BeInRange(0, 15);
    }

    [Fact]
    public void GetBestMove_LowTimeFraction_SearchesFaster()
    {
        using var ai = AITestHelper.CreateAI();
        var board = new Board()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue)
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(6, 6, Player.Blue);
        var opts = new SearchOptions
        {
            TimeRemainingMs = 420_000, IncrementSeconds = 5, MoveNumber = 4,
            TimeFraction = 0.05, UseVCF = false, ThreadCount = 1,
            PonderingEnabled = false, ParallelSearchEnabled = false,
        };
        ai.GetBestMove(board, Player.Red, opts, CancellationToken.None);
        var stats = ai.GetSearchStatistics();
        stats.AllocatedTimeMs.Should().BeLessThan(5000);
    }

    [Fact]
    public void GetBestMove_UseVCFFalse_StillFindsImmediateWins()
    {
        using var ai = AITestHelper.CreateAI();
        var board = new Board()
            .PlaceStone(7, 7, Player.Red).PlaceStone(7, 8, Player.Blue)
            .PlaceStone(8, 7, Player.Red).PlaceStone(8, 8, Player.Blue)
            .PlaceStone(9, 7, Player.Red).PlaceStone(9, 8, Player.Blue)
            .PlaceStone(10, 7, Player.Red).PlaceStone(10, 8, Player.Blue);
        var opts = new SearchOptions
        {
            UseVCF = false, ThreadCount = 1, ParallelSearchEnabled = false,
        };
        var (x, y) = ai.GetBestMove(board, Player.Red, opts, CancellationToken.None);
        new[] { 6, 11 }.Should().Contain(x);
        y.Should().Be(7);
    }
}
