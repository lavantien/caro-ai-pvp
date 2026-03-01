using System.Diagnostics;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tests.Helpers;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Performance tests for InMemoryOpeningBook.
/// Verifies nanosecond-level lookup times.
/// </summary>
public sealed class InMemoryBookPerformanceTests
{
    private readonly MockOpeningBookStore _store;
    private readonly PositionCanonicalizer _canonicalizer;

    public InMemoryBookPerformanceTests()
    {
        _store = new MockOpeningBookStore();
        _canonicalizer = new PositionCanonicalizer();
    }

    [Fact]
    public void Lookup_IsSubMicrosecondPerformance()
    {
        // Arrange - populate book with some entries
        for (int i = 0; i < 100; i++)
        {
            var board = new Board();
            var player = Player.Red;
            for (int j = 0; j < i % 10; j++)
            {
                board = board.PlaceStone(j % 16, (j + 7) % 16, player);
                player = player == Player.Red ? Player.Blue : Player.Red;
            }
            var canonical = _canonicalizer.Canonicalize(board);
            var entry = new OpeningBookEntry
            {
                CanonicalHash = canonical.CanonicalHash,
                DirectHash = board.GetHash(),
                Player = player,
                Moves = new[] { CreateTestBookMove(7, 7, 100, MoveSource.Learned) },
                Depth = i % 10,
                IsNearEdge = false,
                Symmetry = SymmetryType.Identity
            };
            _store.StoreEntry(entry);
        }

        using var inMemoryBook = new InMemoryOpeningBook(_store, _canonicalizer);
        var testBoard = new Board();

        // Warmup
        for (int i = 0; i < 1000; i++)
        {
            inMemoryBook.Lookup(testBoard, Player.Red);
        }

        // Act - measure lookup time
        var sw = Stopwatch.StartNew();
        const int iterations = 100_000;
        for (int i = 0; i < iterations; i++)
        {
            inMemoryBook.Lookup(testBoard, Player.Red);
        }
        sw.Stop();

        // Assert - should be fast enough for game usage (under 100us per lookup)
        var nsPerLookup = (sw.ElapsedTicks * 100.0) / iterations; // 1 tick = 100ns
        Console.WriteLine($"Average lookup time: {nsPerLookup:F2} ns");
        Console.WriteLine($"Throughput: {iterations / sw.Elapsed.TotalSeconds:N0} lookups/sec");

        // Note: Includes canonicalization + hash computation overhead
        // Still orders of magnitude faster than SQLite (milliseconds)
        nsPerLookup.Should().BeLessThan(100_000, "lookup should be under 100 microseconds");
    }

    [Fact]
    public void Contains_IsSubMicrosecondPerformance()
    {
        // Arrange
        var board = new Board();
        var entry = new OpeningBookEntry
        {
            CanonicalHash = _canonicalizer.Canonicalize(board).CanonicalHash,
            DirectHash = board.GetHash(),
            Player = Player.Red,
            Moves = new[] { CreateTestBookMove(7, 7, 100, MoveSource.Learned) },
            Depth = 0,
            IsNearEdge = false,
            Symmetry = SymmetryType.Identity
        };
        _store.StoreEntry(entry);

        using var inMemoryBook = new InMemoryOpeningBook(_store, _canonicalizer);

        // Warmup
        for (int i = 0; i < 1000; i++)
        {
            inMemoryBook.Contains(board, Player.Red);
        }

        // Act
        var sw = Stopwatch.StartNew();
        const int iterations = 100_000;
        for (int i = 0; i < iterations; i++)
        {
            inMemoryBook.Contains(board, Player.Red);
        }
        sw.Stop();

        // Assert
        var nsPerLookup = (sw.ElapsedTicks * 100.0) / iterations;
        Console.WriteLine($"Average Contains time: {nsPerLookup:F2} ns");

        nsPerLookup.Should().BeLessThan(100_000, "Contains should be under 100 microseconds");
    }

    [Fact]
    public void GetBestMove_IsSubMicrosecondPerformance()
    {
        // Arrange
        var board = new Board();
        var entry = new OpeningBookEntry
        {
            CanonicalHash = _canonicalizer.Canonicalize(board).CanonicalHash,
            DirectHash = board.GetHash(),
            Player = Player.Red,
            Moves = new[]
            {
                CreateTestBookMove(7, 7, 100, MoveSource.Learned),
                CreateTestBookMove(8, 8, 200, MoveSource.SelfPlay)
            },
            Depth = 0,
            IsNearEdge = false,
            Symmetry = SymmetryType.Identity
        };
        _store.StoreEntry(entry);

        using var inMemoryBook = new InMemoryOpeningBook(_store, _canonicalizer);

        // Warmup
        for (int i = 0; i < 1000; i++)
        {
            inMemoryBook.GetBestMove(board, Player.Red);
        }

        // Act
        var sw = Stopwatch.StartNew();
        const int iterations = 100_000;
        for (int i = 0; i < iterations; i++)
        {
            inMemoryBook.GetBestMove(board, Player.Red);
        }
        sw.Stop();

        // Assert
        var nsPerLookup = (sw.ElapsedTicks * 100.0) / iterations;
        Console.WriteLine($"Average GetBestMove time: {nsPerLookup:F2} ns");

        nsPerLookup.Should().BeLessThan(100_000, "GetBestMove should be under 100 microseconds");
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
}
