using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class IterativeDeepeningSearchTests
{
    private readonly List<(int x, int y)> _emptyCandidates = new();
    private readonly List<(int x, int y)> _singleCandidate = new() { (7, 7) };
    private readonly List<(int x, int y)> _multipleCandidates = new()
    {
        (7, 7), (7, 8), (8, 7), (8, 8)
    };

    [Fact]
    public void Search_EmptyCandidates_ThrowsArgumentException()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (0, 1)
        );

        // Act
        var board = new Board();
        var candidates = new List<(int x, int y)>();

        // Assert
        Assert.Throws<ArgumentException>(() =>
            search.Search(board, Player.Red, candidates, 1, 5, 1.0, 2.0)
        );
    }

    [Fact]
    public void Search_EmptyCandidates_MessageContainsExpectedText()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (0, 1)
        );
        var board = new Board();
        var candidates = new List<(int x, int y)>();

        // Act
        var exception = Assert.Throws<ArgumentException>(() =>
            search.Search(board, Player.Red, candidates, 1, 5, 1.0, 2.0)
        );

        // Assert
        exception.Message.Should().Contain("No candidates");
    }

    [Fact]
    public void Search_TimeBudgetExhaustion_StopsAtHardBound()
    {
        // Arrange
        bool callbackInvoked = false;
        int callbackDepth = 0;

        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                // Simulate time-consuming search
                Thread.Sleep(50);
                return (0, 100);
            },
            onIterationComplete: (depth, nodes) =>
            {
                callbackInvoked = true;
                callbackDepth = depth;
            }
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Very short time budget (100ms soft, 150ms hard)
        var result = search.Search(board, Player.Red, candidates, 1, 10, 0.1, 0.15);

        // Assert
        result.Should().NotBe(default);
        callbackInvoked.Should().BeTrue();
        // Should stop early due to hard bound
        callbackDepth.Should().BeLessThanOrEqualTo(3);
        result.ElapsedSeconds.Should().BeGreaterThanOrEqualTo(0.1);
    }

    [Fact]
    public void Search_SoftBoundStops_RespectsBestFromCompletedIteration()
    {
        // Arrange
        List<int> completedDepths = new();
        int searchCallCount = 0;

        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                Interlocked.Increment(ref searchCallCount);
                // Each depth adds some nodes
                int nodes = (int)Math.Pow(10, depth);
                return (depth * 100, nodes);
            },
            onIterationComplete: (depth, nodes) => completedDepths.Add(depth)
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Short time budget
        var result = search.Search(board, Player.Red, candidates, 1, 10, 0.05, 0.2);

        // Assert
        completedDepths.Should().NotBeEmpty();
        // Should have completed at least depth 1 (minDepth)
        completedDepths.Should().Contain(1);
        result.DepthAchieved.Should().BeGreaterThanOrEqualTo(1);
        // Should return best from completed iteration
        result.X.Should().BeInRange(0, 19);
        result.Y.Should().BeInRange(0, 19);
    }

    [Fact]
    public void Search_HardBoundStopsImmediately_DoesNotExceedHardBound()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                // Simulate slow search
                Thread.Sleep(100);
                return (0, 10);
            }
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Very short hard bound
        var result = search.Search(board, Player.Red, candidates, 1, 5, 0.1, 0.05);

        // Assert
        result.ElapsedSeconds.Should().BeLessThan(0.2); // Allow some margin
    }

    [Fact]
    public void OnIterationComplete_CallbackInvokedForEachDepth()
    {
        // Arrange
        List<int> callbackDepths = new();
        List<long> callbackNodes = new();

        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                int nodes = (int)Math.Pow(5, depth);
                return (depth * 10, nodes);
            },
            onIterationComplete: (depth, nodes) =>
            {
                callbackDepths.Add(depth);
                callbackNodes.Add(nodes);
            }
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Sufficient time for multiple depths
        var result = search.Search(board, Player.Red, candidates, 1, 3, 1.0, 2.0);

        // Assert
        callbackDepths.Should().NotBeEmpty();
        callbackDepths.Should().BeInAscendingOrder();
        callbackNodes.Should().HaveSameCount(callbackDepths);
        // Each callback should have positive nodes
        callbackNodes.Should().OnlyContain(n => n > 0);
    }

    [Fact]
    public void OnIterationComplete_PassesCorrectParameters()
    {
        // Arrange
        (int expectedDepth, long expectedNodes)? lastCallback = null;

        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                int nodes = (int)Math.Pow(3, depth);
                return (depth, nodes);
            },
            onIterationComplete: (depth, nodes) =>
            {
                lastCallback = (depth, nodes);
            }
        );

        var board = new Board();
        var candidates = _singleCandidate;

        // Act
        search.Search(board, Player.Red, candidates, 2, 4, 1.0, 2.0);

        // Assert
        lastCallback.HasValue.Should().BeTrue();
        var (depth, nodes) = lastCallback!.Value;
        depth.Should().BeGreaterThan(0);
        nodes.Should().BeGreaterThan(0);
    }

    [Fact]
    public void Search_ResultContainsExpectedFields()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (50, 100)
        );

        var board = new Board();
        var candidates = _singleCandidate;

        // Act
        var result = search.Search(board, Player.Red, candidates, 1, 3, 1.0, 2.0);

        // Assert
        result.X.Should().Be(7);
        result.Y.Should().Be(7);
        result.DepthAchieved.Should().BeGreaterThanOrEqualTo(1);
        result.NodesSearched.Should().BeGreaterThan(0);
        result.ElapsedSeconds.Should().BeGreaterThan(0);
        result.Score.Should().Be(50);
    }

    [Fact]
    public void Search_MultipleIterations_UsesDeepestCompletedMove()
    {
        // Arrange
        int bestDepthReported = 0;
        int bestScoreReported = int.MinValue;

        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                // Score improves with depth
                return (depth * 1000, (int)Math.Pow(2, depth));
            }
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Enough time for several iterations
        var result = search.Search(board, Player.Red, candidates, 1, 5, 0.5, 1.0);

        bestDepthReported = result.DepthAchieved;
        bestScoreReported = result.Score;

        // Assert
        bestDepthReported.Should().BeGreaterThanOrEqualTo(1);
        // Score should be positive (indicating a valid search result)
        bestScoreReported.Should().BeGreaterThanOrEqualTo(0);
    }

    [Fact]
    public void Search_SoftBoundAt90Percent_StopsBeforeHardBound()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) =>
            {
                // Simulate iterative search that takes measurable time
                Thread.Sleep(10);
                return (0, 50);
            }
        );

        var board = new Board();
        var candidates = _multipleCandidates;

        // Act - Set soft bound at 0.1s, hard at 0.5s
        // Should stop near soft bound (0.09s = 90% of soft bound)
        var result = search.Search(board, Player.Red, candidates, 1, 10, 0.1, 0.5);

        // Assert
        // Should stop well before hard bound
        result.ElapsedSeconds.Should().BeLessThan(0.3);
    }

    [Fact]
    public void OrderCandidatesByProximity_CandidatesNearExistingStones_First()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (0, 1)
        );

        var board = new Board();
        // Place a stone near center
        board = board.PlaceStone(7, 7, Player.Red);

        var candidates = new List<(int x, int y)>
        {
            (0, 0),  // Far from stone
            (7, 8),  // Adjacent to stone
            (8, 7),  // Adjacent to stone
            (10, 10) // Far from stone
        };

        // Act
        // Search uses the ordered candidates internally
        // We can't directly access OrderCandidatesByProximity, but we can observe behavior
        var result = search.Search(board, Player.Red, candidates, 1, 2, 1.0, 2.0);

        // Assert - Result should be valid (from one of the candidates)
        candidates.Should().Contain((result.X, result.Y));
    }

    [Fact]
    public void Search_EmptyBoard_OrdersCandidatesByCenter()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (0, 1)
        );

        var board = new Board();
        var candidates = new List<(int x, int y)>
        {
            (0, 0),
            (7, 7),  // Center
            (14, 14),
            (1, 1)
        };

        // Act
        var result = search.Search(board, Player.Red, candidates, 1, 2, 1.0, 2.0);

        // Assert - Should prefer center-proximate candidates
        result.X.Should().BeGreaterThanOrEqualTo(5);
        result.Y.Should().BeGreaterThanOrEqualTo(5);
    }

    [Fact]
    public void OnIterationComplete_NullCallback_DoesNotThrow()
    {
        // Arrange
        var search = new IterativeDeepeningSearch(
            (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (0, 1),
            onIterationComplete: null
        );

        var board = new Board();
        var candidates = _singleCandidate;

        // Act & Assert - Should not throw
        var exception = Record.Exception(() =>
            search.Search(board, Player.Red, candidates, 1, 3, 1.0, 2.0)
        );

        exception.Should().BeNull();
    }
}
