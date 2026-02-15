using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public sealed class AIDifficultyConfigTests
{
    [Fact]
    public void BookGeneration_HasParallelSearchEnabled()
    {
        // Arrange & Act
        var settings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.BookGeneration);

        // Assert - BookGeneration MUST have parallel search enabled for performance
        // This test guards against regression where parallel search was disabled
        settings.ParallelSearchEnabled.Should().BeTrue(
            "BookGeneration requires parallel search enabled to achieve high CPU utilization during book generation");
    }

    [Fact]
    public void BookGeneration_UsesProcessorQuarterThreads()
    {
        // Arrange & Act
        var settings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.BookGeneration);
        int expectedThreadCount = Math.Max(5, Environment.ProcessorCount / 4);

        // Assert - BookGeneration uses processorCount/4 threads for position-level parallelism
        settings.ThreadCount.Should().Be(expectedThreadCount,
            "BookGeneration should use processorCount/4 threads (min 5) for balanced parallelism");
    }

    [Fact]
    public void AllDifficulties_ExceptBookGeneration_DependOnConfig()
    {
        // Arrange & Act
        var allDifficulties = Enum.GetValues<AIDifficulty>();

        // Assert - All difficulty settings should be retrievable without error
        foreach (var difficulty in allDifficulties)
        {
            var settings = AIDifficultyConfig.Instance.GetSettings(difficulty);
            settings.Should().NotBeNull();
            settings.Difficulty.Should().Be(difficulty);
            settings.ThreadCount.Should().BePositive();
        }
    }

    [Fact]
    public void Grandmaster_UsesHalfProcessorThreads()
    {
        // Arrange & Act
        var settings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Grandmaster);
        // CRITICAL FIX: Grandmaster now uses max(5, (processorCount/2) - 1) to ensure it always has more threads than Hard (4)
        int expectedThreadCount = Math.Max(5, (Environment.ProcessorCount / 2) - 1);

        // Assert - Grandmaster uses (processorCount/2) - 1 formula with minimum 5 threads
        settings.ThreadCount.Should().Be(expectedThreadCount);
        settings.ThreadCount.Should().BeGreaterThanOrEqualTo(5, "Grandmaster must have at least 5 threads (more than Hard's 4)");
    }

    [Fact]
    public void Grandmaster_HasMoreThreadsThanHard()
    {
        // Arrange & Act
        var hardSettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Hard);
        var grandmasterSettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Grandmaster);

        // Assert - Grandmaster should ALWAYS have more threads than Hard
        // This is critical for ensuring GM is actually stronger than Hard
        grandmasterSettings.ThreadCount.Should().BeGreaterThan(hardSettings.ThreadCount,
            "Grandmaster must have more threads than Hard to ensure stronger play");
    }

    [Fact]
    public void EasyAndAbove_HaveParallelSearchEnabled()
    {
        // Arrange & Act
        var easySettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Easy);
        var mediumSettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Medium);
        var hardSettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Hard);
        var grandmasterSettings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Grandmaster);

        // Assert - Easy and above should have parallel search enabled
        easySettings.ParallelSearchEnabled.Should().BeTrue();
        mediumSettings.ParallelSearchEnabled.Should().BeTrue();
        hardSettings.ParallelSearchEnabled.Should().BeTrue();
        grandmasterSettings.ParallelSearchEnabled.Should().BeTrue();
    }

    [Fact]
    public void Braindead_HasParallelSearchDisabled()
    {
        // Arrange & Act
        var settings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Braindead);

        // Assert - Braindead should not use parallel search
        settings.ParallelSearchEnabled.Should().BeFalse();
    }
}
