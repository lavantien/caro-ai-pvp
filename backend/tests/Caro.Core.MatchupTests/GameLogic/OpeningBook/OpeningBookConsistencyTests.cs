using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using FluentAssertions;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.GameLogic.OpeningBook;

/// <summary>
/// Tests to verify Hard shows variety while Grandmaster stays deterministic.
/// </summary>
[Trait("Category", "Verification")]
[Trait("Category", "Integration")]
public class OpeningBookConsistencyTests
{
    private readonly ITestOutputHelper _output;

    public OpeningBookConsistencyTests(ITestOutputHelper output)
    {
        _output = output;
    }

    private static (MinimaxAI redAi, MinimaxAI blueAi, Caro.Core.GameLogic.OpeningBook book) CreateAIs(AIDifficulty redDifficulty, AIDifficulty blueDifficulty)
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

        return (redAi, blueAi, openingBook);
    }

    private static List<((int x, int y) move, Player player, int turn, bool usedBook)> PlayGame(
        MinimaxAI redAi,
        MinimaxAI blueAi,
        Caro.Core.GameLogic.OpeningBook book,
        AIDifficulty redDifficulty,
        AIDifficulty blueDifficulty,
        int maxMoves = 40)
    {
        var board = new Board();
        var moves = new List<((int x, int y) move, Player player, int turn, bool usedBook)>();
        (int x, int y)? lastOpponentMove = null;

        for (int turn = 0; turn < maxMoves; turn++)
        {
            var currentPlayer = turn % 2 == 0 ? Player.Red : Player.Blue;
            var currentDifficulty = currentPlayer == Player.Red ? redDifficulty : blueDifficulty;
            var currentAi = currentPlayer == Player.Red ? redAi : blueAi;

            // Check if in opening phase and get book move
            bool usedBook = false;
            (int x, int y)? bestMove = null;

            if (book.IsInOpeningPhase(board, currentDifficulty))
            {
                var bookMove = book.GetBookMove(board, currentPlayer, currentDifficulty, lastOpponentMove);
                if (bookMove.HasValue)
                {
                    bestMove = bookMove.Value;
                    usedBook = true;
                }
            }

            // Fall back to AI calculation if no book move
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

            moves.Add((bestMove.Value, currentPlayer, turn, usedBook));

            // Apply move to board (immutable pattern)
            board = board.PlaceStone(bestMove.Value.x, bestMove.Value.y, currentPlayer);
            lastOpponentMove = bestMove;
        }

        return moves;
    }

    [Fact]
    public void HardVsHard_ShowsVariety_GMvsGM_IsDeterministic()
    {
        _output.WriteLine("=== Testing Hard vs Hard (should show variety) ===");
        var hardGames = new List<List<((int x, int y) move, Player player, int turn, bool usedBook)>>();

        for (int game = 0; game < 5; game++)
        {
            var (redAi, blueAi, book) = CreateAIs(AIDifficulty.Hard, AIDifficulty.Hard);
            var moves = PlayGame(redAi, blueAi, book, AIDifficulty.Hard, AIDifficulty.Hard);
            hardGames.Add(moves);
            _output.WriteLine($"Hard vs Hard Game {game + 1} complete");
        }

        _output.WriteLine("\n=== Testing Grandmaster vs Grandmaster (should be deterministic) ===");
        var gmGames = new List<List<((int x, int y) move, Player player, int turn, bool usedBook)>>();

        for (int game = 0; game < 5; game++)
        {
            var (redAi, blueAi, book) = CreateAIs(AIDifficulty.Grandmaster, AIDifficulty.Grandmaster);
            var moves = PlayGame(redAi, blueAi, book, AIDifficulty.Grandmaster, AIDifficulty.Grandmaster);
            gmGames.Add(moves);
            _output.WriteLine($"GM vs GM Game {game + 1} complete");
        }

        // Check Hard vs Hard for variety
        _output.WriteLine("\n=== HARD vs HARD VARIETY CHECK ===");
        var hardVarietyTurns = new List<int>();
        for (int turn = 0; turn < 12; turn++)
        {
            var uniqueMoves = hardGames.Select(g => g[turn].move).Distinct().ToList();
            if (uniqueMoves.Count > 1)
            {
                hardVarietyTurns.Add(turn);
                _output.WriteLine($"Turn {turn} ({hardGames[0][turn].player}): {uniqueMoves.Count} different moves - {string.Join(", ", uniqueMoves.Select(m => $"({m.x},{m.y})"))}");
            }
        }

        // Check GM vs GM for determinism
        _output.WriteLine("\n=== GRANDMASTER vs GRANDMASTER CONSISTENCY CHECK ===");
        var firstGmGame = gmGames.First();
        var gmIsDeterministic = true;

        for (int gameIdx = 1; gameIdx < gmGames.Count; gameIdx++)
        {
            for (int turn = 0; turn < 12; turn++)
            {
                if (firstGmGame[turn].move != gmGames[gameIdx][turn].move)
                {
                    gmIsDeterministic = false;
                    _output.WriteLine($"Game {gameIdx + 1} Turn {turn}: MISMATCH - {firstGmGame[turn].move} vs {gmGames[gameIdx][turn].move}");
                }
            }
        }

        // ASSERT: Hard vs Hard should show variety
        hardVarietyTurns.Should().NotBeEmpty("Hard vs Hard should show variety across games when multiple moves have equal scores");

        // ASSERT: GM vs GM should be deterministic
        gmIsDeterministic.Should().BeTrue("Grandmaster vs Grandmaster should be deterministic, always picking highest priority move");
        if (gmIsDeterministic)
        {
            _output.WriteLine("All 5 GM vs GM games are IDENTICAL");
        }

        _output.WriteLine($"\nVERIFIED: Hard shows variety at {hardVarietyTurns.Count} turn(s), Grandmaster stays deterministic");
    }

    [Fact]
    public void GMvsGM_AllGamesIdentical()
    {
        // Run 5 games with Grandmaster vs Grandmaster to verify determinism
        var allGames = new List<List<((int x, int y) move, Player player, int turn, bool usedBook)>>();

        for (int game = 0; game < 5; game++)
        {
            var (redAi, blueAi, book) = CreateAIs(AIDifficulty.Grandmaster, AIDifficulty.Grandmaster);
            var moves = PlayGame(redAi, blueAi, book, AIDifficulty.Grandmaster, AIDifficulty.Grandmaster);
            allGames.Add(moves);
        }

        // All GM vs GM games should be IDENTICAL
        var firstGame = allGames.First();

        for (int gameIdx = 1; gameIdx < allGames.Count; gameIdx++)
        {
            var currentGame = allGames[gameIdx];

            _output.WriteLine($"\nComparing Game 1 vs Game {gameIdx + 1}");

            for (int turn = 0; turn < 12; turn++)
            {
                var move1 = firstGame[turn];
                var move2 = currentGame[turn];

                if (move1.move != move2.move || move1.usedBook != move2.usedBook)
                {
                    _output.WriteLine($"  Turn {turn} ({move1.player}): {move1.move}{(move1.usedBook ? "*" : "")} vs {move2.move}{(move2.usedBook ? "*" : "")}");
                }

                // GM vs GM should be identical
                move1.move.Should().Be(move2.move, $"Game {gameIdx + 1} turn {turn} should match game 1 (Grandmaster is deterministic)");
            }

            _output.WriteLine($"  Game {gameIdx + 1}: MATCHES Game 1 for first 12 moves");
        }

        _output.WriteLine("\nAll 5 GM vs GM games have IDENTICAL opening book sequences");
    }

    [Fact]
    public void OutputFullMoveSequence_SingleMatch()
    {
        var (redAi, blueAi, book) = CreateAIs(AIDifficulty.Hard, AIDifficulty.Grandmaster);
        var moves = PlayGame(redAi, blueAi, book, AIDifficulty.Hard, AIDifficulty.Grandmaster, maxMoves: 32);

        _output.WriteLine("\n=== FULL MOVE SEQUENCE (Hard=Red, GM=Blue) ===");
        _output.WriteLine("Turn | Player | Move    | In Book?");
        _output.WriteLine(new string('-', 40));

        foreach (var (move, player, turn, usedBook) in moves.Take(32))
        {
            var playerStr = player == Player.Red ? "Red" : "Blue";
            var bookStr = usedBook ? "YES *" : "NO";
            _output.WriteLine($"{turn,3} | {playerStr,-6} | ({move.x},{move.y}) | {bookStr}");
        }

        var redBookMoves = moves.Where(m => m.player == Player.Red && m.usedBook).Count();
        var blueBookMoves = moves.Where(m => m.player == Player.Blue && m.usedBook).Count();

        _output.WriteLine($"\nTotal book moves: Red={redBookMoves}, Blue={blueBookMoves}");
    }
}
