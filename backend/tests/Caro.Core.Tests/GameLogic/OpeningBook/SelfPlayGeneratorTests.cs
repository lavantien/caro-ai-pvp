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
        cts.Cancel();

        // Act
        var summary = await _generator.GenerateGamesAsync(100, cancellationToken: cts.Token);

        // Assert
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
        summary.AverageMoves.Should().Be(37.5); // 150/4
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
}
