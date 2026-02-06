using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using FluentAssertions;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace Caro.Core.MatchupTests.GameLogic.OpeningBook;

/// <summary>
/// Verify board mutation works correctly with opening book lookups.
/// Regression test for issue where RecordMove wasn't updating the board.
/// Now uses immutable GameState pattern.
/// </summary>
public class BoardMutationTests
{
    [Fact]
    public void WithMove_UpdatesBoard_StonesArePlaced()
    {
        // Arrange
        var game = GameState.CreateInitial();

        // Act - First move
        game = game.WithMove(9, 9);
        var board = game.Board;

        // Assert
        game.MoveNumber.Should().Be(1);
        game.CurrentPlayer.Should().Be(Player.Blue);
        board.GetOccupiedCells(Player.Red).Count().Should().Be(1);
        board.GetCell(9, 9).Player.Should().Be(Player.Red);

        // Act - Second move
        game = game.WithMove(9, 8);
        board = game.Board;

        // Assert
        game.MoveNumber.Should().Be(2);
        game.CurrentPlayer.Should().Be(Player.Red);
        board.GetOccupiedCells(Player.Blue).Count().Should().Be(1);
        board.GetCell(9, 8).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void OpeningBook_NonEmptyBoard_ReturnsDifferentMoves()
    {
        // Arrange
        var game = GameState.CreateInitial();

        var dbPath = Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "..", "..", "..", "..", "opening_book.db");
        var store = new SqliteOpeningBookStore(dbPath, NullLogger<SqliteOpeningBookStore>.Instance);
        store.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);
        var openingBook = new Caro.Core.GameLogic.OpeningBook(store, canonicalizer, lookupService);

        // Act & Assert - Empty board should return center (9,9)
        var emptyBoardMove = openingBook.GetBookMove(new Board(), Player.Red, AIDifficulty.Hard, null);
        emptyBoardMove.Should().NotBeNull();
        emptyBoardMove.Value.x.Should().Be(9);
        emptyBoardMove.Value.y.Should().Be(9);

        // Make first move
        game = game.WithMove(9, 9);
        var board = game.Board;

        // Second move should be different from (9,9) - blue's response
        var secondMove = openingBook.GetBookMove(board, Player.Blue, AIDifficulty.Hard, (9, 9));
        secondMove.Should().NotBeNull("Opening book should have a response to center move");
        secondMove.Value.Should().NotBe((9, 9), "Blue cannot play on Red's stone");

        // Make second move
        game = game.WithMove(secondMove.Value.x, secondMove.Value.y);
        board = game.Board;

        // Third move should be different from first two
        var thirdMove = openingBook.GetBookMove(board, Player.Red, AIDifficulty.Hard, (secondMove.Value.x, secondMove.Value.y));
        thirdMove.Should().NotBeNull("Opening book should have a continuation");
        thirdMove.Value.Should().NotBe((9, 9), "Cell (9,9) is occupied");
        thirdMove.Value.Should().NotBe((secondMove.Value.x, secondMove.Value.y), "Cell is occupied by Blue");
    }

    [Fact]
    public void Board_TotalStones_AfterMultipleMoves()
    {
        // Arrange
        var game = GameState.CreateInitial();

        // Act - Make several moves
        game = game.WithMove(9, 9);  // Red
        game = game.WithMove(9, 8);  // Blue
        game = game.WithMove(8, 8);  // Red
        game = game.WithMove(10, 10); // Blue

        var board = game.Board;

        // Assert
        board.GetOccupiedCells(Player.Red).Count().Should().Be(2);
        board.GetOccupiedCells(Player.Blue).Count().Should().Be(2);
        board.TotalStones().Should().Be(4);
    }
}
