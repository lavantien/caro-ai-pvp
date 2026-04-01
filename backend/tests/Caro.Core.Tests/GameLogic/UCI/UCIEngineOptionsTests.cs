using Caro.Core.Domain.Configuration;
using Caro.Core.GameLogic.UCI;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIEngineOptionsTests
{
    private const int DefaultTTSizeMb = SearchConstants.DefaultTTSizeMb;

    [Fact]
    public void GetOptionDeclarations_ReturnsAllOptions()
    {
        // Act
        var declarations = UCIEngineOptions.GetOptionDeclarations();

        // Assert
        declarations.Should().HaveCount(3);
        declarations.Should().Contain(d => d.Contains("Threads"));
        declarations.Should().Contain(d => d.Contains("Hash"));
        declarations.Should().Contain(d => d.Contains("Ponder"));
    }

    [Theory]
    [InlineData("Threads", "1", true, 1)]
    [InlineData("Threads", "16", true, 16)]
    [InlineData("Threads", "32", true, 32)]
    [InlineData("Threads", "0", false, 4)]
    [InlineData("Threads", "33", false, 4)]
    [InlineData("threads", "8", true, 8)]
    public void SetOption_Threads_ValidatesRange(string name, string value, bool expectedSuccess, int expectedValue)
    {
        // Arrange
        var options = new UCIEngineOptions();

        // Act
        var result = options.SetOption(name, value);

        // Assert
        result.Should().Be(expectedSuccess);
        if (expectedSuccess)
            options.Threads.Should().Be(expectedValue);
        else
            options.Threads.Should().Be(expectedValue);
    }

    [Theory]
    [InlineData("Hash", "32", true, 32)]
    [InlineData("Hash", "4096", true, 4096)]
    [InlineData("Hash", "31", false, DefaultTTSizeMb)]
    [InlineData("Hash", "4097", false, DefaultTTSizeMb)]
    public void SetOption_Hash_ValidatesRange(string name, string value, bool expectedSuccess, int expectedValue)
    {
        var options = new UCIEngineOptions();
        var result = options.SetOption(name, value);
        result.Should().Be(expectedSuccess);
        if (expectedSuccess)
            options.Hash.Should().Be(expectedValue);
        else
            options.Hash.Should().Be(expectedValue);
    }

    [Theory]
    [InlineData("Ponder", "true", true, true)]
    [InlineData("Ponder", "false", true, false)]
    [InlineData("Ponder", "invalid", false, false)]
    public void SetOption_Ponder_ValidatesBool(string name, string value, bool expectedSuccess, bool expectedValue)
    {
        var options = new UCIEngineOptions();
        var result = options.SetOption(name, value);
        result.Should().Be(expectedSuccess);
        if (expectedSuccess)
            options.Ponder.Should().Be(expectedValue);
    }

    [Fact]
    public void SetOption_UnknownOption_ReturnsFalse()
    {
        var options = new UCIEngineOptions();
        options.SetOption("UnknownOption", "1").Should().BeFalse();
    }

    [Fact]
    public void SetOption_NullName_ReturnsFalse()
    {
        var options = new UCIEngineOptions();
        options.SetOption(null!, "1").Should().BeFalse();
    }

    [Fact]
    public void SetOption_EmptyName_ReturnsFalse()
    {
        var options = new UCIEngineOptions();
        options.SetOption("", "1").Should().BeFalse();
    }

    [Fact]
    public void DefaultValues_AreCorrect()
    {
        var options = new UCIEngineOptions();
        options.Threads.Should().Be(4);
        options.Hash.Should().Be(DefaultTTSizeMb);
        options.Ponder.Should().BeFalse();
    }
}
