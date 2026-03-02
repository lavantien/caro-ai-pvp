using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tests.Helpers;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Unit tests for SelfPlayGenerator.
/// Tests engine vs engine game generation.
/// </summary>
public sealed class SelfPlayGeneratorTests
{
    private readonly MockStagingBookStore _stagingStore;
    private readonly SelfPlayGenerator _generator;

    public SelfPlayGeneratorTests()
    {
        _stagingStore = new MockStagingBookStore();
        _generator = new SelfPlayGenerator(_stagingStore);
    }

    #region Constructor Tests

    [Fact]
    public void Constructor_InitializesCorrectly()
    {
        // Arrange & Act
        var generator = new SelfPlayGenerator(_stagingStore);

        // Assert
        generator.Should().NotBeNull();
    }

    #endregion

    #region GenerateGamesAsync Tests

    [Fact]
    public async Task GenerateGamesAsync_ZeroGames_ReturnsEmptySummary()
    {
        // Act
        var summary = await _generator.GenerateGamesAsync(0);

        // Assert
        summary.TotalGames.Should().Be(0);
        summary.RedWins.Should().Be(0);
        summary.BlueWins.Should().Be(0);
        summary.Draws.Should().Be(0);
    }

    [Fact]
    public async Task GenerateGamesAsync_SingleGame_ReturnsValidSummary()
    {
        // Act
        var summary = await _generator.GenerateGamesAsync(1);

        // Assert
        summary.TotalGames.Should().Be(1);
        summary.AverageMoves.Should().BeGreaterThan(0);
    }

    [Fact]
    public async Task GenerateGamesAsync_MultipleGames_ReturnsAggregatedSummary()
    {
        // Act
        var summary = await _generator.GenerateGamesAsync(3);

        // Assert
        summary.TotalGames.Should().Be(3);
        var totalResults = summary.RedWins + summary.BlueWins + summary.Draws;
        totalResults.Should().Be(3);
    }

    [Fact]
    public async Task GenerateGamesAsync_WithCancellationToken_CancelsGracefully()
    {
        // Arrange
        var cts = new CancellationTokenSource();

        // Cancel after a short delay to allow some games to complete
        cts.CancelAfter(TimeSpan.FromMilliseconds(500));

        // Act - run generator with cancellation token
        var summary = await _generator.GenerateGamesAsync(
            100,
            cancellationToken: cts.Token);

        // Assert - should complete without throwing, with fewer than 100 games
        // due to cancellation (exact count depends on timing)
        summary.TotalGames.Should().BeLessThan(100);
    }

    #endregion

    #region SelfPlaySummary Tests

    [Fact]
    public void SelfPlaySummary_CalculatesAverageMovesCorrectly()
    {
        // Arrange
        var summary = new SelfPlaySummary
        {
            RedWins = 2,
            BlueWins = 1,
            Draws = 1,
            TotalMoves = 150
        };

        // Act & Assert
        summary.TotalGames.Should().Be(4);
        summary.AverageMoves.Should().BeApproximately(37.5, 0.001); // 150/4
    }

    [Fact]
    public void SelfPlaySummary_DefaultValues_AreZero()
    {
        // Arrange & Act
        var summary = new SelfPlaySummary();

        // Assert
        summary.TotalGames.Should().Be(0);
        summary.AverageMoves.Should().Be(0);
    }

    #endregion

    #region Temperature Decay Tests (Expert Report Compliance)

    [Theory]
    [InlineData(0, 1.8)]    // First move (Red's first ply)
    [InlineData(5, 1.8)]    // Early game
    [InlineData(11, 1.8)]   // Last ply of high exploration
    [InlineData(12, 1.0)]   // First ply of medium exploration
    [InlineData(18, 1.0)]   // Mid-game
    [InlineData(23, 1.0)]   // Last ply of medium exploration
    [InlineData(24, 0.0)]   // First ply of optimal play
    [InlineData(30, 0.0)]   // Late game
    [InlineData(100, 0.0)]  // Very late game
    public void GetTemperature_ReturnsCorrectDecayPattern(int ply, double expectedTemp)
    {
        // Act
        var temp = SelfPlayGenerator.GetTemperature(ply);

        // Assert
        temp.Should().Be(expectedTemp);
    }

    [Fact]
    public void GetTemperature_HighExplorationForFirstSixMoves()
    {
        // Moves 1-6 correspond to plies 0-11 (each move = 2 plies)
        for (int ply = 0; ply < 12; ply++)
        {
            SelfPlayGenerator.GetTemperature(ply).Should().Be(1.8);
        }
    }

    [Fact]
    public void GetTemperature_MediumExplorationForMovesSevenToTwelve()
    {
        // Moves 7-12 correspond to plies 12-23
        for (int ply = 12; ply < 24; ply++)
        {
            SelfPlayGenerator.GetTemperature(ply).Should().Be(1.0);
        }
    }

    [Fact]
    public void GetTemperature_OptimalPlayAfterMoveTwelve()
    {
        // Move 13+ corresponds to ply 24+
        for (int ply = 24; ply < 50; ply++)
        {
            SelfPlayGenerator.GetTemperature(ply).Should().Be(0.0);
        }
    }

    #endregion

    #region Score Delta Threshold Tests (Expert Report Compliance)

    [Fact]
    public void ScoreDeltaThreshold_Is150Centipawns()
    {
        // Expert report recommends filtering moves >150cp worse
        SelfPlayGenerator.ScoreDeltaThreshold.Should().Be(150);
    }

    [Fact]
    public void SampleMove_WithTemperatureZero_SelectsBestMove()
    {
        // Arrange
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 100 },   // Best move
            new() { X = 1, Y = 1, Score = 50 },    // Second best
            new() { X = 2, Y = 2, Score = -100 },  // Poor move
            new() { X = 3, Y = 3, Score = -300 }   // Blunder
        };

        // Act - with temperature 0, should always select best move
        var selectionCounts = new Dictionary<(int, int), int>();
        for (int i = 0; i < 100; i++)
        {
            var move = _generator.SampleMove(candidates, 0.0);
            var key = (move.X, move.Y);
            selectionCounts[key] = selectionCounts.GetValueOrDefault(key, 0) + 1;
        }

        // Assert - best move (0,0) should be selected 100% of the time
        selectionCounts[(0, 0)].Should().Be(100);
    }

    [Fact]
    public void SampleMove_WithHighTemperature_DistributesSelections()
    {
        // Arrange
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 100 },   // Best move
            new() { X = 1, Y = 1, Score = 50 },    // Second best
            new() { X = 2, Y = 2, Score = 25 }     // Third best
        };

        // Act - with high temperature, selections should be distributed
        var selectionCounts = new Dictionary<(int, int), int>();
        for (int i = 0; i < 1000; i++)
        {
            var move = _generator.SampleMove(candidates, 1.8);
            var key = (move.X, move.Y);
            selectionCounts[key] = selectionCounts.GetValueOrDefault(key, 0) + 1;
        }

        // Assert - best move should be selected most often, but not exclusively
        selectionCounts[(0, 0)].Should().BeGreaterThan(selectionCounts[(2, 2)]);
        selectionCounts[(0, 0)].Should().BeLessThan(800); // Not 100%

        // All moves should have some selections due to high temperature
        selectionCounts[(0, 0)].Should().BeGreaterThan(0);
        selectionCounts[(1, 1)].Should().BeGreaterThan(0);
        selectionCounts[(2, 2)].Should().BeGreaterThan(0);
    }

    #endregion

    #region Dirichlet Noise Tests (Expert Report Compliance)

    [Fact]
    public void DirichletNoise_ParametersAreCorrect()
    {
        // Expert report recommends specific noise parameters
        SelfPlayGenerator.DirichletEpsilon.Should().Be(0.25);
        SelfPlayGenerator.DirichletAlpha.Should().Be(0.3);
    }

    [Fact]
    public void ApplyDirichletNoise_ModifiesScores()
    {
        // Arrange
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 100 },
            new() { X = 1, Y = 1, Score = 50 },
            new() { X = 2, Y = 2, Score = 25 }
        };
        var originalScores = candidates.Select(c => c.Score).ToList();

        // Act
        _generator.ApplyDirichletNoise(candidates);

        // Assert - at least some scores should be modified (probabilistic)
        // Since noise is applied, scores will likely change
        var anyChanged = candidates.Zip(originalScores)
            .Any(pair => pair.First.Score != pair.Second);
        anyChanged.Should().BeTrue("Dirichlet noise should modify scores");
    }

    [Fact]
    public void ApplyDirichletNoise_WithEmptyCandidates_DoesNotThrow()
    {
        // Arrange
        var candidates = new List<MoveCandidate>();

        // Act & Assert - should not throw
        var act = () => _generator.ApplyDirichletNoise(candidates);
        act.Should().NotThrow();
    }

    [Fact]
    public void GenerateDirichletNoise_ReturnsNormalizedDistribution()
    {
        // Act
        var noise = _generator.GenerateDirichletNoise(5, 0.3);

        // Assert
        noise.Length.Should().Be(5);
        noise.Sum().Should().BeApproximately(1.0, 0.001); // Should sum to 1
        noise.All(n => n >= 0).Should().BeTrue(); // All values non-negative
    }

    [Fact]
    public void GenerateDirichletNoise_WithDifferentCounts_ReturnsCorrectSize()
    {
        for (int count = 1; count <= 10; count++)
        {
            var noise = _generator.GenerateDirichletNoise(count, 0.3);
            noise.Length.Should().Be(count);
        }
    }

    #endregion

    #region Softmax Sampling Distribution Tests

    [Fact]
    public void SampleMove_WithSingleCandidate_ReturnsThatCandidate()
    {
        // Arrange
        var candidates = new List<MoveCandidate>
        {
            new() { X = 5, Y = 5, Score = 100 }
        };

        // Act
        var move = _generator.SampleMove(candidates, 1.0);

        // Assert
        move.X.Should().Be(5);
        move.Y.Should().Be(5);
    }

    [Fact]
    public void SampleMove_WithEmptyCandidates_ReturnsInvalidMove()
    {
        // Arrange
        var candidates = new List<MoveCandidate>();

        // Act
        var move = _generator.SampleMove(candidates, 1.0);

        // Assert
        move.X.Should().Be(-1);
        move.Y.Should().Be(-1);
    }

    [Fact]
    public void SampleMove_HigherScoresSelectedMoreOften()
    {
        // Arrange - large score difference to make distribution clear
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 1000 },  // Much better
            new() { X = 1, Y = 1, Score = 100 }    // Significantly worse
        };

        // Act - sample 1000 times
        var count0 = 0;
        var count1 = 0;
        for (int i = 0; i < 1000; i++)
        {
            var move = _generator.SampleMove(candidates, 1.0);
            if (move.X == 0 && move.Y == 0) count0++;
            if (move.X == 1 && move.Y == 1) count1++;
        }

        // Assert - higher scored move should be selected more often
        count0.Should().BeGreaterThan(count1);
        count0.Should().BeGreaterThan(900); // Should be very dominant
    }

    [Fact]
    public void SampleMove_ProbabilisticDistribution_MatchesSoftmaxExpectation()
    {
        // Arrange - use specific scores to compute expected probabilities
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 200 },
            new() { X = 1, Y = 1, Score = 100 },
            new() { X = 2, Y = 2, Score = 0 }
        };

        const int samples = 5000;
        var selectionCounts = new int[3];

        // Act
        for (int i = 0; i < samples; i++)
        {
            var move = _generator.SampleMove(candidates, 1.0);
            if (move.X == 0 && move.Y == 0) selectionCounts[0]++;
            else if (move.X == 1 && move.Y == 1) selectionCounts[1]++;
            else if (move.X == 2 && move.Y == 2) selectionCounts[2]++;
        }

        // Assert - order should match score ranking
        selectionCounts[0].Should().BeGreaterThan(selectionCounts[1]);
        selectionCounts[1].Should().BeGreaterThan(selectionCounts[2]);

        // Each should have at least some selections (probabilistic)
        selectionCounts[0].Should().BeGreaterThan(0);
        selectionCounts[1].Should().BeGreaterThan(0);
        selectionCounts[2].Should().BeGreaterThan(0);
    }

    #endregion

    #region Fallback Tests (Expert Report Compliance)

    [Fact]
    public void SampleMove_WithIdenticalScores_DistributesEvenly()
    {
        // Arrange - all moves have identical scores
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = 100 },
            new() { X = 1, Y = 1, Score = 100 },
            new() { X = 2, Y = 2, Score = 100 }
        };

        // Act
        var selectionCounts = new Dictionary<(int, int), int>();
        for (int i = 0; i < 1000; i++)
        {
            var move = _generator.SampleMove(candidates, 1.0);
            var key = (move.X, move.Y);
            selectionCounts[key] = selectionCounts.GetValueOrDefault(key, 0) + 1;
        }

        // Assert - with identical scores, distribution should be roughly uniform
        // Each should get approximately 333 selections (±100 for variance)
        selectionCounts[(0, 0)].Should().BeGreaterThan(200);
        selectionCounts[(1, 1)].Should().BeGreaterThan(200);
        selectionCounts[(2, 2)].Should().BeGreaterThan(200);
    }

    [Fact]
    public void SampleMove_WithNegativeScores_StillSelectsHighest()
    {
        // Arrange - all scores negative
        var candidates = new List<MoveCandidate>
        {
            new() { X = 0, Y = 0, Score = -50 },   // Best (least negative)
            new() { X = 1, Y = 1, Score = -100 },
            new() { X = 2, Y = 2, Score = -200 }   // Worst
        };

        // Act - with temperature 0
        var move = _generator.SampleMove(candidates, 0.0);

        // Assert - should select least negative (best) move
        move.X.Should().Be(0);
        move.Y.Should().Be(0);
    }

    #endregion
}
