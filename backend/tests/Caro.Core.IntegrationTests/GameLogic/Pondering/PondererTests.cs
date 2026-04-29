using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.Pondering;
using System.Threading;

namespace Caro.Core.IntegrationTests.GameLogic.Pondering;

[Trait("Category", "Slow")]
[Trait("Category", "Integration")]
public class PondererTests : IDisposable
{
    private const int PonderTimeoutMs = 5_000;
    private const int ShortSleepMs = 50;
    private const int LongSleepMs = 100;
    private const double ExpectedHitRate = 0.666;
    private const double HitRateTolerance = 0.01;

    private readonly Ponderer _ponderer;
    private readonly ParallelMinimaxSearch _parallelSearch;

    public PondererTests()
    {
        _parallelSearch = new ParallelMinimaxSearch();
        _ponderer = new Ponderer(_parallelSearch);
    }

    public void Dispose()
    {
        _ponderer.Dispose();
        _parallelSearch.Dispose();
    }

    [Fact]
    public void Constructor_InitialState_IsIdle()
    {
        // Act & Assert
        _ponderer.State.Should().Be(PonderState.Idle);
        _ponderer.PredictedMove.Should().BeNull();
    }

    [Fact]
    public void StartPondering_InitialState_ChangesToPondering()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        _ponderer.StartPondering(
            board,
            Player.Blue,
            (8, 8),
            Player.Red,
            PonderTimeoutMs
        );

        // Assert
        _ponderer.State.Should().Be(PonderState.Pondering);
        _ponderer.PredictedMove.Should().Be((8, 8));
    }

    [Fact]
    public void StartPondering_AlreadyPondering_DoesNotInterrupt()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act - Try to start pondering again
        _ponderer.StartPondering(board, Player.Blue, (9, 9), Player.Red, PonderTimeoutMs);

        // Assert - Should keep original predicted move
        _ponderer.PredictedMove.Should().Be((8, 8));
    }

    [Fact]
    public void StartPondering_CreatesBoardWithPredictedMove()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        _ponderer.StartPondering(
            board,
            Player.Blue,
            (8, 8),
            Player.Red,
            PonderTimeoutMs
        );

        // Assert
        var ponderBoard = _ponderer.GetPonderBoard();
        ponderBoard.Should().NotBeNull();
        ponderBoard!.GetCell(7, 7).Player.Should().Be(Player.Red); // Original move
        ponderBoard.GetCell(8, 8).Player.Should().Be(Player.Blue); // Predicted move
    }

    [Fact]
    public void StopPondering_WhilePondering_ChangesToCancelled()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.StopPondering();

        // Assert
        _ponderer.State.Should().Be(PonderState.Cancelled);
    }

    [Fact]
    public void StopPondering_NotPondering_DoesNothing()
    {
        // Arrange - Already idle
        _ponderer.Reset();

        // Act - Should not throw
        _ponderer.StopPondering();

        // Assert
        _ponderer.State.Should().Be(PonderState.Idle);
    }

    [Fact]
    public void HandleOpponentMove_PonderHit_ReturnsHitState()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act - Opponent plays predicted move
        var (state, result) = _ponderer.HandleOpponentMove(8, 8);

        // Assert
        state.Should().Be(PonderState.PonderHit);
        result.Should().NotBeNull();
        result.Value.PonderHit.Should().BeTrue();
    }

    [Fact]
    public void HandleOpponentMove_PonderMiss_ReturnsMissState()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act - Opponent plays different move
        var (state, result) = _ponderer.HandleOpponentMove(9, 9);

        // Assert
        state.Should().Be(PonderState.PonderMiss);
        result.Should().NotBeNull();
        result.Value.PonderHit.Should().BeFalse();
    }

    [Fact]
    public void HandleOpponentMove_NotPondering_ReturnsIdle()
    {
        // Arrange - Not pondering
        var board = new Board();

        // Act
        var (state, result) = _ponderer.HandleOpponentMove(7, 7);

        // Assert
        state.Should().Be(PonderState.Idle);
        result.Should().BeNull();
    }

    [Fact]
    public void HandleOpponentMove_PonderHit_IncrementsHitCount()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.HandleOpponentMove(8, 8);

        // Assert
        _ponderer.TotalPonderHits.Should().Be(1);
        _ponderer.TotalPonderMisses.Should().Be(0);
    }

    [Fact]
    public void HandleOpponentMove_PonderMiss_IncrementsMissCount()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.HandleOpponentMove(9, 9);

        // Assert
        _ponderer.TotalPonderHits.Should().Be(0);
        _ponderer.TotalPonderMisses.Should().Be(1);
    }

    [Fact]
    public void UpdatePonderResult_WhilePondering_UpdatesResult()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.UpdatePonderResult((10, 10), 5, 100, 1000);

        // Assert
        var result = _ponderer.GetCurrentResult();
        result.BestMove.Should().Be((10, 10));
        result.Depth.Should().Be(5);
        result.Score.Should().Be(100);
        result.NodesSearched.Should().Be(1000);
    }

    [Fact]
    public void UpdatePonderResult_NotPondering_DoesNotUpdate()
    {
        // Arrange - Not pondering
        var board = new Board();

        // Act - Should not throw
        _ponderer.UpdatePonderResult((10, 10), 5, 100, 1000);

        // Assert
        var result = _ponderer.GetCurrentResult();
        result.BestMove.Should().BeNull();
    }

    [Fact]
    public void ShouldStopPondering_Initially_ReturnsFalse()
    {
        // Arrange & Act & Assert
        _ponderer.ShouldStopPondering.Should().BeFalse();
    }

    [Fact]
    public void ShouldStopPondering_AfterStop_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.StopPondering();

        // Assert
        _ponderer.ShouldStopPondering.Should().BeTrue();
    }

    [Fact]
    public void IsPondering_WhilePondering_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act & Assert
        _ponderer.IsPondering.Should().BeTrue();
    }

    [Fact]
    public void IsPondering_NotPondering_ReturnsFalse()
    {
        // Arrange & Act & Assert
        _ponderer.IsPondering.Should().BeFalse();
    }

    [Fact]
    public void GetPlayerToMove_ReturnsCorrectPlayer()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act & Assert
        _ponderer.GetPlayerToMove().Should().Be(Player.Blue);
    }

    [Fact]
    public void GetPonderTimeToMerge_AfterPonderHit_ReturnsElapsedTime()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        Thread.Sleep(LongSleepMs); // Small delay
        _ponderer.HandleOpponentMove(8, 8);
        var timeToMerge = _ponderer.GetPonderTimeToMerge();

        // Assert
        timeToMerge.Should().BeGreaterThanOrEqualTo(100);
        timeToMerge.Should().BeLessThan(1000); // Should be around 100ms
    }

    [Fact]
    public void Reset_AfterPondering_ReturnsToIdle()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        _ponderer.Reset();

        // Assert
        _ponderer.State.Should().Be(PonderState.Idle);
        _ponderer.PredictedMove.Should().BeNull();
        _ponderer.GetPonderBoard().Should().BeNull();
    }

    [Fact]
    public void PonderHitRate_CalculatesCorrectly()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act - 2 hits, 1 miss
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);
        _ponderer.HandleOpponentMove(8, 8); // Hit
        _ponderer.Reset();

        _ponderer.StartPondering(board, Player.Blue, (9, 9), Player.Red, PonderTimeoutMs);
        _ponderer.HandleOpponentMove(10, 10); // Miss
        _ponderer.Reset();

        _ponderer.StartPondering(board, Player.Blue, (11, 11), Player.Red, PonderTimeoutMs);
        _ponderer.HandleOpponentMove(11, 11); // Hit
        _ponderer.Reset();

        // Assert
        _ponderer.TotalPonderHits.Should().Be(2);
        _ponderer.TotalPonderMisses.Should().Be(1);
        _ponderer.PonderHitRate.Should().BeApproximately(ExpectedHitRate, HitRateTolerance);
    }

    [Fact]
    public void PonderHitRate_NoPonders_ReturnsZero()
    {
        // Arrange & Act & Assert
        _ponderer.PonderHitRate.Should().Be(0.0);
    }

    [Fact]
    public void TotalPonderTime_AccumulatesCorrectly()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act - Multiple pondering sessions
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);
        Thread.Sleep(ShortSleepMs);
        _ponderer.HandleOpponentMove(8, 8);
        var firstTime = _ponderer.TotalPonderTimeMs;

        _ponderer.Reset();
        _ponderer.StartPondering(board, Player.Blue, (9, 9), Player.Red, PonderTimeoutMs);
        Thread.Sleep(ShortSleepMs);
        _ponderer.HandleOpponentMove(9, 9);
        var secondTime = _ponderer.TotalPonderTimeMs;

        // Assert
        secondTime.Should().BeGreaterThan(firstTime);
        secondTime.Should().BeGreaterThanOrEqualTo(100); // At least 50ms + 50ms
    }

    [Fact]
    public void GetStatistics_ReturnsFormattedString()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);
        _ponderer.HandleOpponentMove(8, 8);

        // Act
        var stats = _ponderer.GetStatistics();

        // Assert
        stats.Should().Contain("Pondering");
        stats.Should().Contain("1/1");
        stats.Should().Contain("hits");
    }

    [Fact]
    public void StartPondering_WithNullPrediction_HandlesGracefully()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act - No predicted move
        _ponderer.StartPondering(
            board,
            Player.Blue,
            null,
            Player.Red,
            PonderTimeoutMs
        );

        // Assert
        _ponderer.State.Should().Be(PonderState.Pondering);
        _ponderer.PredictedMove.Should().BeNull();
        var ponderBoard = _ponderer.GetPonderBoard();
        ponderBoard.Should().NotBeNull();
        // Predicted move not made, board should only have original move
        ponderBoard!.GetCell(7, 7).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void HandleOpponentMove_WithNullPrediction_AlwaysMiss()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, null, Player.Red, PonderTimeoutMs);

        // Act
        var (state, result) = _ponderer.HandleOpponentMove(8, 8);

        // Assert
        state.Should().Be(PonderState.PonderMiss);
        result.Should().NotBeNull();
        result.Value.PonderHit.Should().BeFalse();
    }

    [Fact]
    public void MultiplePonderingSessions_AccumulateStatistics()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act - 3 pondering sessions
        for (int i = 0; i < 3; i++)
        {
            var (px, py) = (8 + i, 8 + i);
            _ponderer.StartPondering(board, Player.Blue, (px, py), Player.Red, PonderTimeoutMs);
            _ponderer.HandleOpponentMove(px, py); // All hits
            _ponderer.Reset();
        }

        // Assert
        _ponderer.TotalPonderHits.Should().Be(3);
        _ponderer.TotalPonderMisses.Should().Be(0);
        _ponderer.PonderHitRate.Should().Be(1.0);
    }

    [Fact]
    public void GetCancellationToken_NotDisposed_ReturnsToken()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act
        var token = _ponderer.GetCancellationToken();

        // Assert
        token.CanBeCanceled.Should().BeTrue();
    }

    [Fact]
    public void Dispose_CleansUpResources()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        _ponderer.StartPondering(board, Player.Blue, (8, 8), Player.Red, PonderTimeoutMs);

        // Act - Should not throw
        _ponderer.Dispose();

        // Assert
        _ponderer.State.Should().Be(PonderState.Cancelled);
    }
}
