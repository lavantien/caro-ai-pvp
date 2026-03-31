using Caro.Core.GameLogic.UCI;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIGoParametersTests
{
    [Fact]
    public void Parse_AllTimeParams_ParsesCorrectly()
    {
        // Arrange
        var args = "wtime 180000 btime 175000 winc 2000 binc 2000".Split(' ');

        // Act
        var result = UCIGoParameters.Parse(args);

        // Assert
        result.WhiteTimeMs.Should().Be(180000);
        result.BlackTimeMs.Should().Be(175000);
        result.WhiteIncrementMs.Should().Be(2000);
        result.BlackIncrementMs.Should().Be(2000);
    }

    [Fact]
    public void Parse_MoveTime_SetsCorrectly()
    {
        var args = "movetime 2000".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.MoveTimeMs.Should().Be(2000);
    }

    [Fact]
    public void Parse_Depth_SetsCorrectly()
    {
        var args = "depth 10".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.Depth.Should().Be(10);
    }

    [Fact]
    public void Parse_Nodes_SetsCorrectly()
    {
        var args = "nodes 1000000".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.Nodes.Should().Be(1000000);
    }

    [Fact]
    public void Parse_Infinite_SetsFlag()
    {
        var args = "infinite".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.Infinite.Should().BeTrue();
    }

    [Fact]
    public void Parse_Empty_ReturnsDefaults()
    {
        var result = UCIGoParameters.Parse([]);
        result.WhiteTimeMs.Should().BeNull();
        result.BlackTimeMs.Should().BeNull();
        result.WhiteIncrementMs.Should().BeNull();
        result.BlackIncrementMs.Should().BeNull();
        result.MoveTimeMs.Should().BeNull();
        result.Depth.Should().BeNull();
        result.Nodes.Should().BeNull();
        result.Infinite.Should().BeFalse();
    }

    [Fact]
    public void Parse_CombinedParams_AllSet()
    {
        var args = "wtime 60000 btime 60000 winc 1000 binc 1000 depth 5".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.WhiteTimeMs.Should().Be(60000);
        result.BlackTimeMs.Should().Be(60000);
        result.WhiteIncrementMs.Should().Be(1000);
        result.BlackIncrementMs.Should().Be(1000);
        result.Depth.Should().Be(5);
    }

    [Fact]
    public void Parse_InvalidValue_SkipsGracefully()
    {
        var args = "wtime abc movetime 1000".Split(' ');
        var result = UCIGoParameters.Parse(args);
        result.WhiteTimeMs.Should().BeNull();
        result.MoveTimeMs.Should().Be(1000);
    }
}
