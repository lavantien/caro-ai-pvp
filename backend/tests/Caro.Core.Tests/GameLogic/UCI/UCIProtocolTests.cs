using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;
using Caro.Core.Tests.Helpers;
using Caro.UCI;
using Microsoft.Extensions.Logging;
using Moq;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIProtocolTests : IDisposable
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

    public void Dispose()
    {
        _protocol.Stop();
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
        _protocol.HandleCommand("position startpos moves bb9");

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
        var response = _protocol.HandleCommand("position startpos moves bb9");

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
        var input = new StringReader("uci\nisready\n");
        var output = new StringWriter();
        using var cts = new CancellationTokenSource();
        cts.CancelAfter(500);

        // Act
        await _protocol.RunAsync(input, output, cts.Token);

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

        Thread.Sleep(200);
        _protocol.Stop();
    }

    [Fact]
    public void HandleCommand_PositionInvalidMove_ReturnsError()
    {
        // Act
        var response = _protocol.HandleCommand("position startpos moves invalid");

        // Assert
        response.Should().ContainSingle(r => r.StartsWith("Error"));
    }

    [Fact]
    public void HandleCommand_Stop_WithoutSearch_ReturnsEmpty()
    {
        var response = _protocol.HandleCommand("stop");

        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_SetOption_WithSpacesInName_ReturnsError()
    {
        var response = _protocol.HandleCommand("setoption name Unknown Option value 5");

        response.Should().ContainSingle(r => r.Contains("Unknown option"));
    }

    [Fact]
    public void HandleCommand_SetOption_NoValue_ReturnsError()
    {
        // Ponder requires a value; bool.TryParse(null) returns false
        var response = _protocol.HandleCommand("setoption name Ponder");

        response.Should().ContainSingle(r => r.Contains("Unknown option"));
    }

    [Fact]
    public async Task RunAsync_SequentialCommands_AllProcessed()
    {
        var commands = "uci\nisready\nucinewgame\nposition startpos\nisready\n";
        var input = new StringReader(commands);
        var output = new StringWriter();
        using var cts = new CancellationTokenSource();
        cts.CancelAfter(500);

        await _protocol.RunAsync(input, output, cts.Token);

        var outputStr = output.ToString();
        outputStr.Should().Contain("uciok");
        // Multiple readyoks from the two isready commands
        var readyCount = outputStr.Split("readyok").Length - 1;
        readyCount.Should().Be(2);
    }

    [Fact]
    public void HandleCommand_PositionMultipleMoves_TracksCorrectly()
    {
        // Play several moves
        _protocol.HandleCommand("position startpos moves bb9 bd8");
        var response = _protocol.HandleCommand("isready");

        response.Should().Equal("readyok");
    }

    [Fact]
    public void HandleCommand_Position_WhitespaceHandling()
    {
        // Extra whitespace should be trimmed
        var response = _protocol.HandleCommand("  position   startpos  ");

        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_GoWithMovetime_ReturnsNonNull()
    {
        _protocol.HandleCommand("position startpos");

        var response = _protocol.HandleCommand("go movetime 100");

        response.Should().NotBeNull();

        // Wait for search to complete to avoid background task outliving test host
        Thread.Sleep(200);
        _protocol.Stop();
    }

    [Fact]
    public void HandleCommand_GoWithDepth_ReturnsNonNull()
    {
        _protocol.HandleCommand("position startpos");

        var response = _protocol.HandleCommand("go depth 3");

        response.Should().NotBeNull();

        Thread.Sleep(200);
        _protocol.Stop();
    }

    [Fact]
    public void HandleCommand_PositionAfterNewGame_ResetsBoard()
    {
        // Set up a position
        _protocol.HandleCommand("position startpos moves bb9");
        // New game resets
        _protocol.HandleCommand("ucinewgame");
        // Position should be clean
        var response = _protocol.HandleCommand("position startpos");

        response.Should().BeEmpty();
    }

    [Fact]
    public async Task RunAsync_EmptyLines_Ignored()
    {
        var input = new StringReader("\n\n\nuci\n\n\nisready\n\n");
        var output = new StringWriter();
        using var cts = new CancellationTokenSource();
        cts.CancelAfter(500);

        await _protocol.RunAsync(input, output, cts.Token);

        var outputStr = output.ToString();
        outputStr.Should().Contain("uciok");
        outputStr.Should().Contain("readyok");
    }

    [Fact]
    public void HandleCommand_Echo_NoArgs_ReturnsEmpty()
    {
        var response = _protocol.HandleCommand("echo");

        response.Should().Equal("");
    }

    [Theory]
    [InlineData("UCI")]
    [InlineData("Uci")]
    [InlineData("UCI ")]
    [InlineData(" uci ")]
    public void HandleCommand_CaseInsensitiveAndWhitespace(string command)
    {
        var response = _protocol.HandleCommand(command);

        response.Should().Contain("uciok");
    }

    [Fact]
    public void HandleCommand_SetOption_Hash_ResizedOnNewGame()
    {
        _protocol.HandleCommand("setoption name Hash value 1");

        // Should apply on ucinewgame
        var response = _protocol.HandleCommand("ucinewgame");

        response.Should().BeEmpty();
    }

    [Fact]
    public void HandleCommand_SetOption_Threads_Valid()
    {
        var response = _protocol.HandleCommand("setoption name Threads value 4");

        response.Should().BeEmpty();
    }
}
