using Caro.Core.Domain.Entities;
using Caro.Core.Domain.ValueObjects;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.AI;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.AI;

public sealed class AIGameStateTests
{
    [Fact]
    public void Constructor_CreatesValidState()
    {
        // Act
        var state = new AIGameState(maxDepth: 10, tableSizeMB: 64);

        // Assert
        state.TranspositionTable.Should().NotBeNull();
        state.NodesSearched.Should().Be(0);
        state.TableHits.Should().Be(0);
        state.TableLookups.Should().Be(0);
        state.MaxDepthReached.Should().Be(0);
        state.Age.Should().Be(0);
        state.LastPV.Should().BeEmpty();
    }

    [Fact]
    public void KillerMoves_InitiallyReturnDefaultPosition()
    {
        // Arrange
        var state = new AIGameState();

        // Act
        var killer = state.GetKillerMove(5, 0);

        // Assert - Initially, killer moves are Position(0,0) (default struct value)
        killer.X.Should().Be(0);
        killer.Y.Should().Be(0);
    }

    [Fact]
    public void SetKillerMove_ThenGetKillerMove_ReturnsSameMove()
    {
        // Arrange
        var state = new AIGameState();
        var move = new Position(5, 7);

        // Act
        state.SetKillerMove(5, 0, move);
        var retrieved = state.GetKillerMove(5, 0);

        // Assert
        retrieved.X.Should().Be(5);
        retrieved.Y.Should().Be(7);
    }

    [Fact]
    public void GetKillerMove_InvalidDepth_ReturnsInvalidPosition()
    {
        // Arrange
        var state = new AIGameState();
        state.SetKillerMove(5, 0, new Position(3, 3));

        // Act
        var invalidDepth = state.GetKillerMove(100, 0);
        var negativeDepth = state.GetKillerMove(-1, 0);

        // Assert
        // Out of bounds returns Position(-1,-1) due to bounds checking
        invalidDepth.X.Should().Be(-1);
        negativeDepth.X.Should().Be(-1);
    }

    [Fact]
    public void GetKillerMove_InvalidSlot_ReturnsInvalidPosition()
    {
        // Arrange
        var state = new AIGameState();
        state.SetKillerMove(5, 0, new Position(3, 3));

        // Act
        var invalidSlot = state.GetKillerMove(5, 5);

        // Assert
        // Out of range slot returns Position(-1,-1)
        invalidSlot.X.Should().Be(-1);
    }

    [Fact]
    public void HistoryScore_InitiallyZero()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);

        // Act
        var score = state.GetHistoryScore(pos);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void UpdateHistoryScore_IncreasesScore()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);

        // Act
        state.UpdateHistoryScore(pos, depth: 3);
        var score = state.GetHistoryScore(pos);

        // Assert
        score.Should().Be(9); // 3 * 3 = depth^2
    }

    [Fact]
    public void UpdateHistoryScore_MultipleUpdates_Accumulate()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);

        // Act
        state.UpdateHistoryScore(pos, depth: 2);
        state.UpdateHistoryScore(pos, depth: 3);
        var score = state.GetHistoryScore(pos);

        // Assert
        score.Should().Be(13); // 2*2 + 3*3 = 4 + 9 = 13
    }

    [Fact]
    public void ButterflyScore_InitiallyZero()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);

        // Act
        var score = state.GetButterflyScore(pos);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void UpdateButterflyScore_ModifiesScore()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);

        // Act
        state.UpdateButterflyScore(pos, delta: 5);
        var score = state.GetButterflyScore(pos);

        // Assert
        score.Should().Be(5);
    }

    [Fact]
    public void UpdateButterflyScore_NegativeDelta_Works()
    {
        // Arrange
        var state = new AIGameState();
        var pos = new Position(5, 7);
        state.UpdateButterflyScore(pos, delta: 10);

        // Act
        state.UpdateButterflyScore(pos, delta: -3);
        var score = state.GetButterflyScore(pos);

        // Assert
        score.Should().Be(7);
    }

    [Fact]
    public void ResetStatistics_ZerosCounters()
    {
        // Arrange
        var state = new AIGameState();
        state.NodesSearched = 1000;
        state.TableHits = 500;
        state.TableLookups = 800;
        state.MaxDepthReached = 10;

        // Act
        state.ResetStatistics();

        // Assert
        state.NodesSearched.Should().Be(0);
        state.TableHits.Should().Be(0);
        state.TableLookups.Should().Be(0);
        state.MaxDepthReached.Should().Be(0);
    }

    [Fact]
    public void Clear_ClearsAllState()
    {
        // Arrange
        var state = new AIGameState();
        state.SetKillerMove(5, 0, new Position(3, 3));
        state.UpdateHistoryScore(new Position(5, 5), depth: 2);
        state.NodesSearched = 100;

        // Act
        state.Clear();

        // Assert
        state.NodesSearched.Should().Be(0);
        // Killer moves reset to Position(-1,-1)
        state.GetKillerMove(5, 0).X.Should().Be(-1);
        state.GetHistoryScore(new Position(5, 5)).Should().Be(0);
    }

    [Fact]
    public void GetHistoryScore_InvalidPosition_ReturnsZero()
    {
        // Arrange
        var state = new AIGameState();
        var invalidPos = new Position(-1, -1);

        // Act
        var score = state.GetHistoryScore(invalidPos);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void GetButterflyScore_InvalidPosition_ReturnsZero()
    {
        // Arrange
        var state = new AIGameState();
        var invalidPos = new Position(20, 20); // Outside board

        // Act
        var score = state.GetButterflyScore(invalidPos);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void Dispose_ClearsState()
    {
        // Arrange
        var state = new AIGameState();
        state.NodesSearched = 100;
        state.SetKillerMove(5, 0, new Position(3, 3));

        // Act
        state.Dispose();

        // Assert
        state.NodesSearched.Should().Be(0);
        // Killer moves reset to Position(-1,-1)
        state.GetKillerMove(5, 0).X.Should().Be(-1);
    }
}
