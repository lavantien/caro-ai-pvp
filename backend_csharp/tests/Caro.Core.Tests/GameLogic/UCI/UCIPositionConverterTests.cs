using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.UCI;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIPositionConverterTests
{
    [Fact]
    public void ParsePosition_Startpos_ReturnsEmptyBoard()
    {
        var board = UCIPositionConverter.ParsePosition("position startpos");
        board.Should().NotBeNull();
        board.Cells.Count(c => !c.IsEmpty).Should().Be(0);
    }

    [Fact]
    public void ParsePosition_SingleMove_HasOneStone()
    {
        var board = UCIPositionConverter.ParsePosition("position startpos moves cc9");
        board.Cells.Count(c => !c.IsEmpty).Should().Be(1);
    }

    [Fact]
    public void ParsePosition_MultipleMoves_AlternatesPlayers()
    {
        var board = UCIPositionConverter.ParsePosition("position startpos moves cc9 dd9 cc10");
        board.Cells.Count(c => !c.IsEmpty).Should().Be(3);
    }

    [Fact]
    public void ParsePosition_Empty_ThrowsArgumentException()
    {
        var act = () => UCIPositionConverter.ParsePosition("");
        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void ParsePosition_Null_ThrowsArgumentException()
    {
        var act = () => UCIPositionConverter.ParsePosition(null!);
        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void ParsePosition_InvalidPrefix_ThrowsArgumentException()
    {
        var act = () => UCIPositionConverter.ParsePosition("invalid command");
        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void ApplyMoves_EmptyArray_ReturnsOriginalBoard()
    {
        var board = new Board();
        var (result, nextPlayer) = UCIPositionConverter.ApplyMoves(board, []);
        result.Cells.Count(c => !c.IsEmpty).Should().Be(0);
        nextPlayer.Should().Be(Player.Red);
    }

    [Fact]
    public void ApplyMoves_SingleMove_ReturnsUpdatedBoard()
    {
        var board = new Board();
        var (result, nextPlayer) = UCIPositionConverter.ApplyMoves(board, ["cc9"]);
        result.Cells.Count(c => !c.IsEmpty).Should().Be(1);
        nextPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void BuildPositionCommand_NoMoves_ReturnsStartpos()
    {
        var result = UCIPositionConverter.BuildPositionCommand();
        result.Should().Be("position startpos");
    }

    [Fact]
    public void BuildPositionCommand_WithMoves_IncludesMoves()
    {
        var result = UCIPositionConverter.BuildPositionCommand("cc9", "dd9");
        result.Should().Be("position startpos moves cc9 dd9");
    }
}
