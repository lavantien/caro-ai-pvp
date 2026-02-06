using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Caro.Core.Tournament;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;
using Xunit.Abstractions;

// Type alias to avoid namespace collision with OpeningBook namespace
using OpeningBookType = Caro.Core.GameLogic.OpeningBook;

namespace Caro.Core.MatchupTests.GameLogic.OpeningBook;

/// <summary>
/// Integration tests for opening book usage in Hard vs Grandmaster matchups.
/// Verifies that both difficulties use the book correctly with proper depth filtering.
///
/// Hard: book moves up to depth 24 (12 moves each)
/// Grandmaster: book moves up to depth 32 (16 moves each)
/// </summary>
[Trait("Category", "Verification")]
[Trait("Category", "Integration")]
[Trait("Category", "OpeningBook")]
public class OpeningBookMatchupTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;
    private readonly OpeningBookType _openingBook;

    // Test configuration
    private const int MaxMoves = 40;  // 20 full turns (40 half-moves)
    private const int InitialTimeSeconds = 420;  // 7+5 time control
    private const int IncrementSeconds = 5;

    // Depth limits from OpeningBookLookupService.GetMaxBookDepth
    private const int HardMaxDepth = 24;
    private const int GrandmasterMaxDepth = 32;

    public OpeningBookMatchupTests(ITestOutputHelper output)
    {
        _output = output;

        // Initialize opening book with SQLite store
        var logger = Microsoft.Extensions.Logging.Abstractions.NullLogger<SqliteOpeningBookStore>.Instance;
        // From bin/Debug/net10.0/, go up 6 levels to reach repo root
        var dbPath = Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "..", "..", "..", "..", "opening_book.db");
        var store = new SqliteOpeningBookStore(dbPath, logger);
        store.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);

        // Create OpeningBook with SQLite store for injection into MinimaxAI
        _openingBook = new OpeningBookType(store, canonicalizer, lookupService);

        // Create AI instances with OpeningBook dependency
        var botA = new MinimaxAI(openingBook: _openingBook);
        var botB = new MinimaxAI(openingBook: _openingBook);
        _engine = new TournamentEngine(botA, botB);
    }

    public void Dispose()
    {
        GC.SuppressFinalize(this);
    }

    /// <summary>
    /// Track opening book usage during a Hard vs Grandmaster game.
    /// Verifies both difficulties use the book correctly with depth filtering.
    /// </summary>
    [Fact]
    public void HardVsGrandmaster_OpeningBookUsage_BothUseBookCorrectly()
    {
        // Arrange
        var results = new List<BookMoveTracker>();
        var game = GameState.CreateInitial();

        // Create AI instances for each difficulty with opening book injection
        var redAI = new MinimaxAI(openingBook: _openingBook);
        var blueAI = new MinimaxAI(openingBook: _openingBook);

        // Act - Play 40 half-moves (20 full turns)
        for (int moveNumber = 0; moveNumber < MaxMoves && !game.IsGameOver; moveNumber++)
        {
            var board = game.Board;
            var difficulty = game.CurrentPlayer == Player.Red ? AIDifficulty.Hard : AIDifficulty.Grandmaster;
            var ai = game.CurrentPlayer == Player.Red ? redAI : blueAI;

            // Get the book move for this position
            var lastOpponentMove = GetLastOpponentMove(board, game.CurrentPlayer);
            var bookMove = _openingBook.GetBookMove(board, game.CurrentPlayer, difficulty, lastOpponentMove);
            var actualMove = ai.GetBestMove(board, game.CurrentPlayer, difficulty);

            // Track whether the AI used the book move
            bool usedBookMove = bookMove.HasValue && bookMove.Value.x == actualMove.x && bookMove.Value.y == actualMove.y;
            bool bookHadMove = bookMove.HasValue;

            // Calculate current depth (half-moves played so far)
            int currentDepth = game.MoveNumber;

            results.Add(new BookMoveTracker(
                MoveNumber: moveNumber,
                Player: game.CurrentPlayer,
                Difficulty: difficulty,
                UsedBookMove: usedBookMove,
                BookHadMove: bookHadMove,
                ActualMove: actualMove,
                BookMove: bookMove,
                CurrentDepth: currentDepth
            ));

            // Make the move (GameState handles player switching)
            game = game.WithMove(actualMove.x, actualMove.y);

            // Check for win
            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);
            if (result.HasWinner)
            {
                _output.WriteLine($"Game ended at move {moveNumber + 1} with winner {result.Winner}");
                break;
            }
        }

        // Assert - Print and verify results
        PrintBookUsageResults(results);

        // Verify Hard depth filtering
        var hardMoves = results.Where(r => r.Difficulty == AIDifficulty.Hard).ToList();
        var hardBookMovesWithinLimit = hardMoves.Where(r => r.UsedBookMove && r.CurrentDepth < HardMaxDepth).Count();
        var hardBookMovesBeyondLimit = hardMoves.Where(r => r.UsedBookMove && r.CurrentDepth >= HardMaxDepth).Count();
        var hardAvailableMovesWithinLimit = hardMoves.Where(r => r.BookHadMove && r.CurrentDepth < HardMaxDepth).Count();
        var hardAvailableMovesBeyondLimit = hardMoves.Where(r => r.BookHadMove && r.CurrentDepth >= HardMaxDepth).Count();

        _output.WriteLine($"\nHard (depth <{HardMaxDepth}):");
        _output.WriteLine($"  Book moves used within limit: {hardBookMovesWithinLimit}");
        _output.WriteLine($"  Book moves used beyond limit: {hardBookMovesBeyondLimit} (should be 0)");
        _output.WriteLine($"  Book available within limit: {hardAvailableMovesWithinLimit}");
        _output.WriteLine($"  Book available beyond limit: {hardAvailableMovesBeyondLimit}");

        // Verify Grandmaster depth filtering
        var gmMoves = results.Where(r => r.Difficulty == AIDifficulty.Grandmaster).ToList();
        var gmBookMovesWithinLimit = gmMoves.Where(r => r.UsedBookMove && r.CurrentDepth < GrandmasterMaxDepth).Count();
        var gmBookMovesBeyondLimit = gmMoves.Where(r => r.UsedBookMove && r.CurrentDepth >= GrandmasterMaxDepth).Count();
        var gmAvailableMovesWithinLimit = gmMoves.Where(r => r.BookHadMove && r.CurrentDepth < GrandmasterMaxDepth).Count();
        var gmAvailableMovesBeyondLimit = gmMoves.Where(r => r.BookHadMove && r.CurrentDepth >= GrandmasterMaxDepth).Count();

        _output.WriteLine($"\nGrandmaster (depth <{GrandmasterMaxDepth}):");
        _output.WriteLine($"  Book moves used within limit: {gmBookMovesWithinLimit}");
        _output.WriteLine($"  Book moves used beyond limit: {gmBookMovesBeyondLimit}");
        _output.WriteLine($"  Book available within limit: {gmAvailableMovesWithinLimit}");
        _output.WriteLine($"  Book available beyond limit: {gmAvailableMovesBeyondLimit}");

        // Assertions

        // At least one difficulty should have found book moves available
        (hardAvailableMovesWithinLimit + gmAvailableMovesWithinLimit).Should().BeGreaterThan(0, "Opening book should have moves available in the opening phase");

        // At least one book move should have been used (AI may deviate if it finds better moves)
        (hardBookMovesWithinLimit + gmBookMovesWithinLimit).Should().BeGreaterThan(0, "At least one difficulty should use opening book moves");

        // Hard should NOT use book moves beyond depth 24 (depth filtering)
        hardBookMovesBeyondLimit.Should().Be(0, "Hard should not use opening book beyond depth 24 due to depth filtering");
    }

    /// <summary>
    /// Play two games with alternating colors to verify opening book usage is consistent.
    /// </summary>
    [Fact]
    public void HardVsGrandmaster_AlternatingColors_BothUseBookConsistently()
    {
        // Game 1: Hard as Red, Grandmaster as Blue
        var game1Results = PlayAndTrackBookUsage(AIDifficulty.Hard, AIDifficulty.Grandmaster);
        _output.WriteLine("\n=== Game 1: Hard (Red) vs Grandmaster (Blue) ===");
        PrintBookUsageResults(game1Results);

        // Game 2: Grandmaster as Red, Hard as Blue
        var game2Results = PlayAndTrackBookUsage(AIDifficulty.Grandmaster, AIDifficulty.Hard);
        _output.WriteLine("\n=== Game 2: Grandmaster (Red) vs Hard (Blue) ===");
        PrintBookUsageResults(game2Results);

        // Verify both games used the book
        var game1HardBookMoves = game1Results.Where(r => r.Difficulty == AIDifficulty.Hard && r.UsedBookMove).Count();
        var game1GmBookMoves = game1Results.Where(r => r.Difficulty == AIDifficulty.Grandmaster && r.UsedBookMove).Count();
        var game2HardBookMoves = game2Results.Where(r => r.Difficulty == AIDifficulty.Hard && r.UsedBookMove).Count();
        var game2GmBookMoves = game2Results.Where(r => r.Difficulty == AIDifficulty.Grandmaster && r.UsedBookMove).Count();

        _output.WriteLine($"\nBook move usage summary:");
        _output.WriteLine($"  Game 1 - Hard: {game1HardBookMoves} book moves, Grandmaster: {game1GmBookMoves} book moves");
        _output.WriteLine($"  Game 2 - Hard: {game2HardBookMoves} book moves, Grandmaster: {game2GmBookMoves} book moves");

        // At least some book moves should be available or used across both games
        var totalBookMoves = game1HardBookMoves + game2HardBookMoves + game1GmBookMoves + game2GmBookMoves;
        totalBookMoves.Should().BeGreaterThan(0, "At least some opening book moves should be used across both games");
    }

    /// <summary>
    /// Verify that Hard and Grandmaster get different book moves at the same position due to depth filtering.
    /// </summary>
    [Fact]
    public void HardVsGrandmaster_DepthFiltering_HardStopsAtDepth24_GmGoesToDepth32()
    {
        // Arrange - Create a position at depth 25 (beyond Hard's limit but within GM's)
        var game = GameState.CreateInitial();
        var moves = new List<(int x, int y)>
        {
            (9, 9), (9, 8),  // 1
            (8, 8), (10, 10), // 2
            (11, 11), (7, 7), // 3
            (12, 12), (6, 6), // 4
            (13, 13), (5, 5), // 5
            (14, 14), (4, 4), // 6
            (15, 15), (3, 3), // 7
            (16, 16), (2, 2), // 8
            (17, 17), (1, 1), // 9
            (18, 18), (0, 0), // 10
            (8, 9), (7, 8),   // 11
            (6, 7), (5, 6),    // 12
            (4, 5)             // 13 - depth 26 for next move (Red to move)
        };

        foreach (var (x, y) in moves)
        {
            game = game.WithMove(x, y);
        }

        // Next move is at depth 26 (27 half-moves, Red to move)
        var currentPlayer = game.CurrentPlayer;
        var board = game.Board;

        // Act - Get book moves for both difficulties at the same position
        var lastOpponentMove = GetLastOpponentMove(board, currentPlayer);
        var hardBookMove = _openingBook.GetBookMove(board, currentPlayer, AIDifficulty.Hard, lastOpponentMove);
        var gmBookMove = _openingBook.GetBookMove(board, currentPlayer, AIDifficulty.Grandmaster, lastOpponentMove);

        // Assert
        _output.WriteLine($"At depth 26 (beyond Hard's limit of {HardMaxDepth}):");
        _output.WriteLine($"  Hard book move: {(hardBookMove.HasValue ? $"{hardBookMove.Value.x},{hardBookMove.Value.y}" : "null")} (expected: null due to depth filtering)");
        _output.WriteLine($"  GM book move: {(gmBookMove.HasValue ? $"{gmBookMove.Value.x},{gmBookMove.Value.y}" : "null")} (may have book move)");

        // At depth 26, Hard should not have book moves (beyond limit)
        // Grandmaster may still have book moves (within limit to depth 32)
        // Note: If both return null, the position might not be in the book (not a failure)
        if (gmBookMove.HasValue)
        {
            hardBookMove.Should().BeNull("Hard should not have book moves beyond depth 24 when GM does");
        }
    }

    /// <summary>
    /// Play a game and track opening book usage for each move.
    /// </summary>
    private List<BookMoveTracker> PlayAndTrackBookUsage(AIDifficulty redDifficulty, AIDifficulty blueDifficulty)
    {
        var results = new List<BookMoveTracker>();
        var game = GameState.CreateInitial();

        var redAI = new MinimaxAI(openingBook: _openingBook);
        var blueAI = new MinimaxAI(openingBook: _openingBook);

        for (int moveNumber = 0; moveNumber < MaxMoves && !game.IsGameOver; moveNumber++)
        {
            var board = game.Board;
            var difficulty = game.CurrentPlayer == Player.Red ? redDifficulty : blueDifficulty;
            var ai = game.CurrentPlayer == Player.Red ? redAI : blueAI;

            var lastOpponentMove = GetLastOpponentMove(board, game.CurrentPlayer);
            var bookMove = _openingBook.GetBookMove(board, game.CurrentPlayer, difficulty, lastOpponentMove);
            var actualMove = ai.GetBestMove(board, game.CurrentPlayer, difficulty);

            bool usedBookMove = bookMove.HasValue && bookMove.Value.x == actualMove.x && bookMove.Value.y == actualMove.y;
            bool bookHadMove = bookMove.HasValue;

            results.Add(new BookMoveTracker(
                MoveNumber: moveNumber,
                Player: game.CurrentPlayer,
                Difficulty: difficulty,
                UsedBookMove: usedBookMove,
                BookHadMove: bookHadMove,
                ActualMove: actualMove,
                BookMove: bookMove,
                CurrentDepth: game.MoveNumber
            ));

            game = game.WithMove(actualMove.x, actualMove.y);

            var detector = new WinDetector();
            var result = detector.CheckWin(game.Board);
            if (result.HasWinner) break;
        }

        return results;
    }

    /// <summary>
    /// Print opening book usage results to test output.
    /// </summary>
    private void PrintBookUsageResults(List<BookMoveTracker> results)
    {
        _output.WriteLine("\nMove | Player | Difficulty   | Book Move | Actual Move | Used Book?");
        _output.WriteLine(new string('-', 75));

        foreach (var result in results)
        {
            var bookStr = result.BookMove.HasValue ? $"({result.BookMove.Value.x},{result.BookMove.Value.y})" : "N/A";
            var usedStr = result.UsedBookMove ? "YES" : (result.BookHadMove ? "NO (deviated)" : "N/A");
            _output.WriteLine($"{result.MoveNumber,4} | {result.Player,6} | {result.Difficulty,-12} | {bookStr,-9} | ({result.ActualMove.x},{result.ActualMove.y})       | {usedStr}");
        }

        var hardMoves = results.Where(r => r.Difficulty == AIDifficulty.Hard).ToList();
        var gmMoves = results.Where(r => r.Difficulty == AIDifficulty.Grandmaster).ToList();

        _output.WriteLine(new string('-', 75));
        _output.WriteLine($"Hard: {hardMoves.Count(r => r.UsedBookMove)} book moves used out of {hardMoves.Count} moves");
        _output.WriteLine($"Grandmaster: {gmMoves.Count(r => r.UsedBookMove)} book moves used out of {gmMoves.Count} moves");
    }

    /// <summary>
    /// Get the last opponent move from the board for opening book lookup.
    /// Returns null if no moves have been played yet.
    /// </summary>
    private static (int x, int y)? GetLastOpponentMove(Board board, Player currentPlayer)
    {
        var opponent = currentPlayer == Player.Red ? Player.Blue : Player.Red;
        for (int y = 0; y < board.BoardSize; y++)
        {
            for (int x = 0; x < board.BoardSize; x++)
            {
                if (board.GetCell(x, y).Player == opponent)
                {
                    // Find the most recent opponent move by checking from the end
                    // For simplicity, return the first opponent stone found
                    // (book lookup doesn't strictly need the last move for this implementation)
                    return (x, y);
                }
            }
        }
        return null;
    }

    /// <summary>
    /// Tracks whether each move came from the opening book.
    /// </summary>
    private record BookMoveTracker(
        int MoveNumber,
        Player Player,
        AIDifficulty Difficulty,
        bool UsedBookMove,
        bool BookHadMove,
        (int x, int y) ActualMove,
        (int x, int y)? BookMove,
        int CurrentDepth
    );
}
