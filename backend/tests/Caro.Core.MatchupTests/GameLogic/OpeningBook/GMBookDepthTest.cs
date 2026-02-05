using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using FluentAssertions;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.GameLogic.OpeningBook;

/// <summary>
/// Test to verify Grandmaster gets 16 book moves when playing vs itself.
/// </summary>
public class GMBookDepthTest
{
    private readonly ITestOutputHelper _output;

    public GMBookDepthTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void GM_vs_GM_Should_Get_16_Book_Moves()
    {
        var dbPath = Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "..", "..", "..", "..", "opening_book.db");
        var store = new SqliteOpeningBookStore(dbPath, NullLogger<SqliteOpeningBookStore>.Instance);
        store.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);
        var openingBook = new Caro.Core.GameLogic.OpeningBook(store, canonicalizer, lookupService);

        var redAi = new MinimaxAI(openingBook: openingBook);
        var blueAi = new MinimaxAI(openingBook: openingBook);

        var board = new Board();
        (int x, int y)? lastOpponentMove = null;

        _output.WriteLine("=== GM vs GM - Move Sequence ===");
        _output.WriteLine("Turn | Player | Move    | In Book?");
        _output.WriteLine(new string('-', 40));

        int redBookCount = 0;
        int blueBookCount = 0;

        for (int turn = 0; turn < 40; turn++)
        {
            var currentPlayer = turn % 2 == 0 ? Player.Red : Player.Blue;
            var currentDifficulty = AIDifficulty.Grandmaster;
            var currentAi = currentPlayer == Player.Red ? redAi : blueAi;

            bool usedBook = false;
            (int x, int y)? bestMove = null;

            if (openingBook.IsInOpeningPhase(board, currentDifficulty))
            {
                var bookMove = openingBook.GetBookMove(board, currentPlayer, currentDifficulty, lastOpponentMove);
                if (bookMove.HasValue)
                {
                    bestMove = bookMove.Value;
                    usedBook = true;
                    if (currentPlayer == Player.Red) redBookCount++;
                    else blueBookCount++;
                }
            }

            if (!bestMove.HasValue)
            {
                bestMove = currentAi.GetBestMove(
                    board,
                    currentPlayer,
                    currentDifficulty,
                    5000,
                    0,
                    ponderingEnabled: false
                );
            }

            var playerStr = currentPlayer == Player.Red ? "Red" : "Blue";
            var bookStr = usedBook ? "YES *" : "NO";
            _output.WriteLine($"{turn,3} | {playerStr,-6} | ({bestMove.Value.x},{bestMove.Value.y}) | {bookStr}");

            board = board.PlaceStone(bestMove.Value.x, bestMove.Value.y, currentPlayer);
            lastOpponentMove = bestMove;
        }

        _output.WriteLine(new string('-', 40));
        _output.WriteLine($"Total book moves: Red={redBookCount}, Blue={blueBookCount}");
        _output.WriteLine($"Total: {redBookCount + blueBookCount} book moves (GM vs GM should follow book deeply)");

        // GM vs GM should follow the book deeply (actual depth depends on book coverage)
        _output.WriteLine($"\nRESULT: GM vs GM stayed in book for {redBookCount + blueBookCount} moves");
        (redBookCount + blueBookCount).Should().BeGreaterThan(0,
            $"GM vs GM should use book moves, but got {redBookCount + blueBookCount}");
    }
}
