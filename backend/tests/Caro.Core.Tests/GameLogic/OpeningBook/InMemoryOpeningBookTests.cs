using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tests.Helpers;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Unit tests for InMemoryOpeningBook.
/// Tests fast lookup, symmetry handling, and move prioritization.
/// </summary>
public sealed class InMemoryOpeningBookTests : IDisposable
{
    private readonly MockOpeningBookStore _store;
    private readonly PositionCanonicalizer _canonicalizer;
    private readonly InMemoryOpeningBook _inMemoryBook;

    public InMemoryOpeningBookTests()
    {
        _store = new MockOpeningBookStore();
        _canonicalizer = new PositionCanonicalizer();
        _inMemoryBook = new InMemoryOpeningBook(_store, _canonicalizer);
    }

    public void Dispose()
    {
        _inMemoryBook.Dispose();
    }

    #region Constructor and Loading Tests

    [Fact]
    public void Constructor_LoadsEntriesFromStore()
    {
        // Arrange
        var board = new Board();
        var entry = CreateTestEntry(board, Player.Red);
        _store.StoreEntry(entry);

        // Act - Create new in-memory book
        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Assert
        book.Count.Should().Be(1);
    }

    [Fact]
    public void Count_ReturnsCorrectNumberOfEntries()
    {
        // Arrange
        var board1 = new Board();
        var board2 = board1.PlaceStone(7, 7, Player.Red);

        _store.StoreEntry(CreateTestEntry(board1, Player.Red));
        _store.StoreEntry(CreateTestEntry(board2, Player.Blue));

        // Act - Create new in-memory book
        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Assert
        book.Count.Should().Be(2);
    }

    #endregion

    #region Lookup Tests

    [Fact]
    public void Lookup_EmptyBook_ReturnsNull()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _inMemoryBook.Lookup(board, Player.Red);

        // Assert
        result.Should().BeNull();
    }

    [Fact]
    public void Lookup_PositionNotInBook_ReturnsNull()
    {
        // Arrange
        var storedBoard = new Board();
        var entry = CreateTestEntry(storedBoard, Player.Red);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act - Look up a different position
        var differentBoard = storedBoard.PlaceStone(7, 7, Player.Red);
        var result = book.Lookup(differentBoard, Player.Blue);

        // Assert
        result.Should().BeNull();
    }

    [Fact]
    public void Lookup_PositionInBook_ReturnsMoves()
    {
        // Arrange
        var board = new Board();
        var moves = new[]
        {
            CreateTestBookMove(7, 7, 100, MoveSource.Learned)
        };
        var entry = CreateTestEntryWithMoves(board, Player.Red, moves);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act
        var result = book.Lookup(board, Player.Red);

        // Assert
        result.Should().NotBeNull();
        result.Should().HaveCount(1);
        result![0].RelativeX.Should().Be(7);
        result[0].RelativeY.Should().Be(7);
    }

    #endregion

    #region Contains Tests

    [Fact]
    public void Contains_PositionInBook_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        var entry = CreateTestEntry(board, Player.Red);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act
        var result = book.Contains(board, Player.Red);

        // Assert
        result.Should().BeTrue();
    }

    [Fact]
    public void Contains_PositionNotInBook_ReturnsFalse()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _inMemoryBook.Contains(board, Player.Red);

        // Assert
        result.Should().BeFalse();
    }

    #endregion

    #region GetBestMove Tests

    [Fact]
    public void GetBestMove_NoMoves_ReturnsNull()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _inMemoryBook.GetBestMove(board, Player.Red);

        // Assert
        result.Should().BeNull();
    }

    [Fact]
    public void GetBestMove_PrioritizesSolvedMoves()
    {
        // Arrange
        var board = new Board();
        var moves = new[]
        {
            CreateTestBookMove(8, 8, 200, MoveSource.Learned),
            CreateTestBookMove(7, 7, 100, MoveSource.Solved)
        };
        var entry = CreateTestEntryWithMoves(board, Player.Red, moves);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act
        var result = book.GetBestMove(board, Player.Red);

        // Assert
        result.Should().NotBeNull();
        result!.Source.Should().Be(MoveSource.Solved);  // Solved has highest priority
    }

    [Fact]
    public void GetBestMove_WhenSameSource_PrioritizesHigherScore()
    {
        // Arrange
        var board = new Board();
        var moves = new[]
        {
            CreateTestBookMove(8, 8, 200, MoveSource.Learned),
            CreateTestBookMove(7, 7, 100, MoveSource.Learned)
        };
        var entry = CreateTestEntryWithMoves(board, Player.Red, moves);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act
        var result = book.GetBestMove(board, Player.Red);

        // Assert
        result.Should().NotBeNull();
        result!.Score.Should().Be(200);  // Higher score wins when same source
    }

    #endregion

    #region AddEntry Tests

    [Fact]
    public void AddEntry_IncreasesCount()
    {
        // Arrange
        var initialCount = _inMemoryBook.Count;
        var entry = CreateTestEntry(new Board(), Player.Red);

        // Act
        _inMemoryBook.AddEntry(entry);

        // Assert
        _inMemoryBook.Count.Should().Be(initialCount + 1);
    }

    [Fact]
    public void AddEntry_EntryCanBeLookedUp()
    {
        // Arrange
        var board = new Board();
        var entry = CreateTestEntry(board, Player.Red);

        // Act
        _inMemoryBook.AddEntry(entry);

        // Assert
        _inMemoryBook.Contains(board, Player.Red).Should().BeTrue();
    }

    #endregion

    #region GetStatistics Tests

    [Fact]
    public void GetStatistics_ReturnsCorrectCounts()
    {
        // Arrange
        var moves = new[]
        {
            CreateTestBookMove(7, 7, 100, MoveSource.Solved),
            CreateTestBookMove(8, 8, 50, MoveSource.Learned),
            CreateTestBookMove(9, 9, 25, MoveSource.SelfPlay)
        };
        var entry = CreateTestEntryWithMoves(new Board(), Player.Red, moves);
        _store.StoreEntry(entry);

        using var book = new InMemoryOpeningBook(_store, _canonicalizer);

        // Act
        var stats = book.GetStatistics();

        // Assert
        stats.TotalPositions.Should().Be(1);
        stats.TotalMoves.Should().Be(3);
        stats.SolvedMoves.Should().Be(1);
        stats.LearnedMoves.Should().Be(1);
        stats.SelfPlayMoves.Should().Be(1);
    }

    #endregion

    #region Helper Methods

    private static OpeningBookEntry CreateTestEntry(Board board, Player player)
    {
        var moves = new[]
        {
            CreateTestBookMove(7, 7, 100, MoveSource.Learned)
        };
        return CreateTestEntryWithMoves(board, player, moves);
    }

    private static OpeningBookEntry CreateTestEntryWithMoves(Board board, Player player, BookMove[] moves)
    {
        var canonicalizer = new PositionCanonicalizer();
        var canonical = canonicalizer.Canonicalize(board);
        var directHash = board.GetHash();

        return new OpeningBookEntry
        {
            CanonicalHash = canonical.CanonicalHash,
            DirectHash = directHash,
            Player = player,
            Moves = moves,
            Depth = 0,
            IsNearEdge = false,
            Symmetry = SymmetryType.Identity
        };
    }

    private static BookMove CreateTestBookMove(int x, int y, int score, MoveSource source)
    {
        return new BookMove
        {
            RelativeX = x,
            RelativeY = y,
            WinRate = 50,
            DepthAchieved = 10,
            NodesSearched = 1000,
            Score = score,
            IsForcing = false,
            Priority = 0,
            IsVerified = true,
            Source = source
        };
    }

    #endregion
}
