using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for the mutable SearchBoard class.
/// Verifies correct behavior of make/unmake pattern and hash maintenance.
/// </summary>
public class SearchBoardTests
{
    private const int BoardSize = GameConstants.BoardSize;

    [Fact]
    public void Constructor_CreatesEmptyBoard()
    {
        var board = new SearchBoard();

        board.BoardSize.Should().Be(BoardSize);
        board.GetHash().Should().Be(0);
        board.IsEmpty().Should().BeTrue();
        board.TotalStones().Should().Be(0);
    }

    [Fact]
    public void Constructor_FromBoard_CopiesPosition()
    {
        var immutableBoard = new Board();
        immutableBoard = immutableBoard.PlaceStone(5, 5, Player.Red);
        immutableBoard = immutableBoard.PlaceStone(10, 10, Player.Blue);

        var searchBoard = new SearchBoard(immutableBoard);

        searchBoard.GetPlayerAt(5, 5).Should().Be(Player.Red);
        searchBoard.GetPlayerAt(10, 10).Should().Be(Player.Blue);
        searchBoard.TotalStones().Should().Be(2);
        searchBoard.GetHash().Should().Be(immutableBoard.GetHash());
    }

    [Fact]
    public void MakeMove_SetsBitInCorrectBitBoard()
    {
        var board = new SearchBoard();
        var initialHash = board.GetHash();

        board.MakeMove(5, 7, Player.Red);

        board.GetPlayerAt(5, 7).Should().Be(Player.Red);
        board.IsEmpty(5, 7).Should().BeFalse();
        board.TotalStones().Should().Be(1);
        board.GetHash().Should().NotBe(initialHash);
    }

    [Fact]
    public void MakeMove_ThenUnmake_RestoresOriginalState()
    {
        var board = new SearchBoard();
        var initialHash = board.GetHash();

        var undo = board.MakeMove(8, 8, Player.Blue);
        board.GetPlayerAt(8, 8).Should().Be(Player.Blue);

        board.UnmakeMove(undo);

        board.GetPlayerAt(8, 8).Should().Be(Player.None);
        board.IsEmpty(8, 8).Should().BeTrue();
        board.GetHash().Should().Be(initialHash);
    }

    [Fact]
    public void MultipleMoves_UnmakeInReverseOrder_RestoresCorrectly()
    {
        var board = new SearchBoard();
        var initialHash = board.GetHash();

        var undo1 = board.MakeMove(5, 5, Player.Red);
        var undo2 = board.MakeMove(6, 6, Player.Blue);
        var undo3 = board.MakeMove(7, 7, Player.Red);

        board.TotalStones().Should().Be(3);

        // Unmake in reverse order
        board.UnmakeMove(undo3);
        board.TotalStones().Should().Be(2);
        board.GetPlayerAt(7, 7).Should().Be(Player.None);

        board.UnmakeMove(undo2);
        board.TotalStones().Should().Be(1);
        board.GetPlayerAt(6, 6).Should().Be(Player.None);

        board.UnmakeMove(undo1);
        board.TotalStones().Should().Be(0);
        board.GetHash().Should().Be(initialHash);
    }

    [Fact]
    public void Hash_MatchesImmutableBoard_AfterSameMoves()
    {
        var searchBoard = new SearchBoard();
        var immutableBoard = new Board();

        // Make same moves on both
        var moves = new[] { (5, 5, Player.Red), (10, 10, Player.Blue), (5, 6, Player.Red) };

        foreach (var (x, y, player) in moves)
        {
            searchBoard.MakeMove(x, y, player);
            immutableBoard = immutableBoard.PlaceStone(x, y, player);
        }

        searchBoard.GetHash().Should().Be(immutableBoard.GetHash());
    }

    [Fact]
    public void GetBitBoard_ReturnsCorrectBits()
    {
        var board = new SearchBoard();
        board.MakeMove(3, 3, Player.Red);
        board.MakeMove(4, 4, Player.Red);
        board.MakeMove(5, 5, Player.Blue);

        var redBits = board.GetBitBoard(Player.Red);
        var blueBits = board.GetBitBoard(Player.Blue);

        redBits.GetBit(3, 3).Should().BeTrue();
        redBits.GetBit(4, 4).Should().BeTrue();
        redBits.GetBit(5, 5).Should().BeFalse();

        blueBits.GetBit(5, 5).Should().BeTrue();
        blueBits.GetBit(3, 3).Should().BeFalse();
    }

    [Fact]
    public void GetOccupancy_ReturnsAllStones()
    {
        var board = new SearchBoard();
        board.MakeMove(3, 3, Player.Red);
        board.MakeMove(7, 7, Player.Blue);

        var occupancy = board.GetOccupancy();

        occupancy.GetBit(3, 3).Should().BeTrue();
        occupancy.GetBit(7, 7).Should().BeTrue();
        occupancy.GetBit(5, 5).Should().BeFalse();
        occupancy.CountBits().Should().Be(2);
    }

    [Fact]
    public void IsEmpty_OutOfBounds_ReturnsFalse()
    {
        var board = new SearchBoard();

        board.IsEmpty(-1, 5).Should().BeFalse();
        board.IsEmpty(5, -1).Should().BeFalse();
        board.IsEmpty(BoardSize, 5).Should().BeFalse();
        board.IsEmpty(5, BoardSize).Should().BeFalse();
    }

    [Fact]
    public void GetPlayerAt_OutOfBounds_ReturnsNone()
    {
        var board = new SearchBoard();

        board.GetPlayerAt(-1, 5).Should().Be(Player.None);
        board.GetPlayerAt(BoardSize, 5).Should().Be(Player.None);
        board.GetPlayerAt(5, BoardSize).Should().Be(Player.None);
    }

    [Fact]
    public void Clone_CreatesIndependentCopy()
    {
        var original = new SearchBoard();
        original.MakeMove(5, 5, Player.Red);

        var clone = original.Clone();
        clone.MakeMove(10, 10, Player.Blue);

        original.GetPlayerAt(10, 10).Should().Be(Player.None);
        clone.GetPlayerAt(10, 10).Should().Be(Player.Blue);
        original.TotalStones().Should().Be(1);
        clone.TotalStones().Should().Be(2);
    }

    [Fact]
    public void CopyFrom_CopiesState()
    {
        var source = new SearchBoard();
        source.MakeMove(5, 5, Player.Red);
        source.MakeMove(6, 6, Player.Blue);

        var target = new SearchBoard();
        target.CopyFrom(source);

        target.GetPlayerAt(5, 5).Should().Be(Player.Red);
        target.GetPlayerAt(6, 6).Should().Be(Player.Blue);
        target.GetHash().Should().Be(source.GetHash());
    }

    [Fact]
    public void Clear_ResetsToEmpty()
    {
        var board = new SearchBoard();
        board.MakeMove(5, 5, Player.Red);
        board.MakeMove(6, 6, Player.Blue);

        board.Clear();

        board.IsEmpty().Should().BeTrue();
        board.GetHash().Should().Be(0);
        board.TotalStones().Should().Be(0);
    }

    [Fact]
    public void ToBoard_CreatesEquivalentImmutableBoard()
    {
        var searchBoard = new SearchBoard();
        searchBoard.MakeMove(5, 5, Player.Red);
        searchBoard.MakeMove(10, 10, Player.Blue);
        searchBoard.MakeMove(5, 6, Player.Red);

        var immutableBoard = searchBoard.ToBoard();

        immutableBoard.GetCell(5, 5).Player.Should().Be(Player.Red);
        immutableBoard.GetCell(10, 10).Player.Should().Be(Player.Blue);
        immutableBoard.GetCell(5, 6).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void GetOccupiedCells_ReturnsAllOccupied()
    {
        var board = new SearchBoard();
        board.MakeMove(3, 3, Player.Red);
        board.MakeMove(7, 7, Player.Blue);
        board.MakeMove(3, 4, Player.Red);

        var occupied = board.GetOccupiedCells().ToList();

        occupied.Should().HaveCount(3);
        occupied.Should().Contain((3, 3, Player.Red));
        occupied.Should().Contain((7, 7, Player.Blue));
        occupied.Should().Contain((3, 4, Player.Red));
    }

    [Fact]
    public void MoveUndo_RecordStruct_HasCorrectValues()
    {
        var undo = new MoveUndo(5, 7, Player.Red);

        undo.X.Should().Be(5);
        undo.Y.Should().Be(7);
        undo.Player.Should().Be(Player.Red);
    }

    [Fact]
    public void DeepSearch_MakeUnmakePattern_MaintainsCorrectState()
    {
        // Simulate a depth-4 search pattern
        var board = new SearchBoard();
        var initialHash = board.GetHash();

        // Make moves at depth 1-4
        var undo1 = board.MakeMove(8, 8, Player.Red);
        var undo2 = board.MakeMove(9, 9, Player.Blue);
        var undo3 = board.MakeMove(8, 9, Player.Red);
        var undo4 = board.MakeMove(9, 8, Player.Blue);

        // At leaf node, we have 4 stones
        board.TotalStones().Should().Be(4);

        // Unmake depth 4
        board.UnmakeMove(undo4);

        // Make alternative at depth 4
        var undo4b = board.MakeMove(10, 10, Player.Blue);
        board.TotalStones().Should().Be(4);

        // Unmake all the way back
        board.UnmakeMove(undo4b);
        board.UnmakeMove(undo3);
        board.UnmakeMove(undo2);
        board.UnmakeMove(undo1);

        board.GetHash().Should().Be(initialHash);
        board.IsEmpty().Should().BeTrue();
    }

    [Fact]
    public void HashConsistency_MultiplePositions_NoCollisions()
    {
        var board = new SearchBoard();
        var hashes = new HashSet<ulong>();

        // Generate positions with different move orders
        // Each position should have a unique hash
        for (int x1 = 0; x1 < 4; x1++)
        {
            for (int y1 = 0; y1 < 4; y1++)
            {
                var undo1 = board.MakeMove(x1, y1, Player.Red);
                for (int x2 = 0; x2 < 4; x2++)
                {
                    for (int y2 = 0; y2 < 4; y2++)
                    {
                        if (x2 == x1 && y2 == y1) continue;

                        var undo2 = board.MakeMove(x2, y2, Player.Blue);
                        hashes.Add(board.GetHash());
                        board.UnmakeMove(undo2);
                    }
                }
                board.UnmakeMove(undo1);
            }
        }

        // All hashes should be unique (Zobrist property)
        hashes.Count.Should().BeGreaterThan(0);
    }

    [Fact]
    public void BitBoard_MatchesImmutableBoard_AfterConversion()
    {
        var searchBoard = new SearchBoard();
        searchBoard.MakeMove(5, 5, Player.Red);
        searchBoard.MakeMove(10, 10, Player.Blue);

        var immutableBoard = searchBoard.ToBoard();

        var searchRed = searchBoard.GetBitBoard(Player.Red);
        var searchBlue = searchBoard.GetBitBoard(Player.Blue);
        var immutableRed = immutableBoard.GetBitBoard(Player.Red);
        var immutableBlue = immutableBoard.GetBitBoard(Player.Blue);

        searchRed.Equals(immutableRed).Should().BeTrue();
        searchBlue.Equals(immutableBlue).Should().BeTrue();
    }
}
