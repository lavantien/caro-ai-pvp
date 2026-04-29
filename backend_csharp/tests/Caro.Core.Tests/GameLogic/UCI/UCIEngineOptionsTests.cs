using Caro.Core.Domain.Configuration;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIEngineOptionsTests
{
    private const int DefaultTTSizeMb = SearchConstants.DefaultTTSizeMb; // 64

    [Fact]
    public void GetOptionDeclarations_ReturnsAllOptions()
    {
        var declarations = UCIEngineOptions.GetOptionDeclarations();

        declarations.Should().HaveCount(4);
        declarations.Should().Contain(d => d.Contains("Threads"));
        declarations.Should().Contain(d => d.Contains("Hash"));
        declarations.Should().Contain(d => d.Contains("Ponder"));
        declarations.Should().Contain(d => d.Contains("Skill Level"));
    }

    [Fact]
    public void SetOption_Threads_AcceptsMin()
    {
        var options = new UCIEngineOptions();
        options.SetOption("Threads", "1").Should().BeTrue();
        options.Threads.Should().Be(1);
    }

    [Fact]
    public void SetOption_Threads_AcceptsMax()
    {
        int max = ThreadPoolConfig.MaxEngineThreads;
        var options = new UCIEngineOptions();
        options.SetOption("Threads", max.ToString()).Should().BeTrue();
        options.Threads.Should().Be(max);
    }

    [Fact]
    public void SetOption_Threads_RejectsZero()
    {
        int cap = ThreadPoolConfig.MaxEngineThreads;
        var options = new UCIEngineOptions();
        options.SetOption("Threads", "0").Should().BeFalse();
        options.Threads.Should().Be(cap);
    }

    [Fact]
    public void SetOption_Threads_RejectsAboveMax()
    {
        int cap = ThreadPoolConfig.MaxEngineThreads;
        var options = new UCIEngineOptions();
        options.SetOption("Threads", (cap + 1).ToString()).Should().BeFalse();
        options.Threads.Should().Be(cap);
    }

    [Fact]
    public void SetOption_Threads_CaseInsensitive()
    {
        var options = new UCIEngineOptions();
        options.SetOption("threads", "2").Should().BeTrue();
        options.Threads.Should().Be(2);
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
        options.Threads.Should().Be(ThreadPoolConfig.MaxEngineThreads);
        options.Hash.Should().Be(DefaultTTSizeMb);
        options.Ponder.Should().BeFalse();
    }
}
