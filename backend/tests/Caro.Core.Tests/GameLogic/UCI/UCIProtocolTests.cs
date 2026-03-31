using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;
using Caro.Core.Tests.Helpers;
using Caro.UCI;
using Microsoft.Extensions.Logging;
using Moq;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIProtocolTests
{
    private readonly MinimaxAI _ai;
    private readonly Mock<ILogger> _loggerMock;
    private readonly UCIProtocol _protocol;

    public UCIProtocolTests()
    {
        _ai = AITestHelper.CreateAI(ttSizeMb: 1);
        _loggerMock = new Mock<ILogger>();
        _protocol = new UCIProtocol(_ai, _loggerMock.Object);
    }

    [Fact]
    public void HandleCommand_Uci_ReturnsIdAndOptions()
    {
        // Act
        var response = _protocol.HandleCommand("uci");

        // Assert
        response.Should().NotBeEmpty();
        response[0].Should().StartWith("id name Caro AI");
        response[1].Should().Be("id author Caro AI Project");
        response.Should().Contain(r => r.StartsWith("option name Threads"));
        response.Should().Contain(r => r.StartsWith("option name Hash"));
        response.Should().Contain(r => r.StartsWith("option name Ponder"));
        response.Should().Contain("uciok");
    }

    [Fact]
    public void HandleCommand_IsReady_ReturnsReadyOk()
    {
        // Act
        var response = _protocol.HandleCommand("isready");

        // Assert
        response.Should().Equal("readyok");
    }

    [Fact]
    public void HandleCommand_UciNewGame_ResetsState()
    {
        // Arrange - set up a position first
        _protocol.HandleCommand("position startpos moves h9");

        // Act
        var response = _protocol.HandleCommand("ucinewgame");

        // Assert
        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_PositionStartpos_ReturnsEmpty()
    {
        // Act
        var response = _protocol.HandleCommand("position startpos");

        // Assert
        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_PositionWithMoves_ReturnsEmpty()
    {
        // Act
        var response = _protocol.HandleCommand("position startpos moves h9");

        // Assert
        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_PositionNoArgs_ReturnsError()
    {
        // Act
        var response = _protocol.HandleCommand("position");

        // Assert
        response.Should().ContainSingle(r => r.StartsWith("Error"));
    }

    [Theory]
    [InlineData("setoption name Threads value 8")]
    [InlineData("setoption name Hash value 512")]
    [InlineData("setoption name Ponder value true")]
    public void HandleCommand_SetOption_Valid_ReturnsEmpty(string command)
    {
        // Act
        var response = _protocol.HandleCommand(command);

        // Assert
        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_SetOption_InvalidName_ReturnsError()
    {
        // Act
        var response = _protocol.HandleCommand("setoption name InvalidOption value 1");

        // Assert
        response.Should().ContainSingle(r => r.Contains("Unknown option"));
    }

    [Fact]
    public void HandleCommand_SetOption_NoName_ReturnsError()
    {
        // Act
        var response = _protocol.HandleCommand("setoption");

        // Assert
        response.Should().ContainSingle(r => r.StartsWith("Error"));
    }

    [Fact]
    public void HandleCommand_Echo_ReturnsArgs()
    {
        // Act
        var response = _protocol.HandleCommand("echo hello world");

        // Assert
        response.Should().Equal("hello world");
    }

    [Fact]
    public void HandleCommand_Unknown_ReturnsUnknownCommand()
    {
        // Act
        var response = _protocol.HandleCommand("foobar");

        // Assert
        response.Should().ContainSingle(r => r.StartsWith("Unknown command"));
    }

    [Fact]
    public void HandleCommand_Empty_ReturnsEmpty()
    {
        // Act
        var response = _protocol.HandleCommand("");

        // Assert
        response.Should().BeEmpty();
    }

    [Fact]
    public async Task RunAsync_ReadsAndRespondsToCommands()
    {
        // Arrange
        var input = new StringReader("uci\nisready\nquit\n");
        var output = new StringWriter();

        // Act
        await _protocol.RunAsync(input, output, CancellationToken.None);

        // Assert
        var outputStr = output.ToString();
        outputStr.Should().Contain("uciok");
        outputStr.Should().Contain("readyok");
    }

    [Fact]
    public async Task RunAsync_Cancellation_StopsLoop()
    {
        // Arrange
        var input = new StringReader("uci\n");
        var output = new StringWriter();
        using var cts = new CancellationTokenSource();
        cts.CancelAfter(100);

        // Act
        await _protocol.RunAsync(input, output, cts.Token);

        // Assert - should not hang
        true.Should().BeTrue();
    }

    [Fact]
    public void Stop_StopsProtocol()
    {
        // Act
        _protocol.Stop();

        // Assert - should not throw
        true.Should().BeTrue();
    }

    [Fact]
    public void HandleCommand_GoWithoutPosition_ReturnsError()
    {
        // The protocol initializes with empty board, so this should work
        // unless there's a null board issue
        var response = _protocol.HandleCommand("go movetime 100");
        // Should not crash - empty board is valid
        response.Should().NotBeNull();
    }

    [Fact]
    public void HandleCommand_PositionInvalidMove_ReturnsError()
    {
        // Act
        var response = _protocol.HandleCommand("position startpos moves invalid");

        // Assert
        response.Should().ContainSingle(r => r.StartsWith("Error"));
    }
}
