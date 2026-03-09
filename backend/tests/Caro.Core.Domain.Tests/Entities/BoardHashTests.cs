using Caro.Core.Domain.Entities;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.Entities;

/// <summary>
/// Tests for Zobrist hashing - ensures unique hashes for different positions.
/// Critical for opening book integrity: hash collisions cause position data corruption.
/// </summary>
public class BoardHashTests
{
    [Fact]
    public void EmptyBoard_HashIsZero()
    {
        // Arrange
        var board = new Board();

        // Act
        var hash = board.GetHash();

        // Assert
        hash.Should().Be(0UL, "empty board should have hash of 0");
    }

    [Fact]
    public void NonEmptyBoard_HashIsNonZero()
    {
        // Arrange
        var board = new Board().PlaceStone(8, 8, Player.Red);

        // Act
        var hash = board.GetHash();

        // Assert
        hash.Should().NotBe(0UL, "non-empty board should have non-zero hash");
    }

    [Fact]
    public void DifferentPositions_HaveDifferentHashes()
    {
        // Arrange - create several different positions
        var board1 = new Board().PlaceStone(8, 8, Player.Red);
        var board2 = new Board().PlaceStone(7, 7, Player.Red);
        var board3 = new Board().PlaceStone(8, 8, Player.Blue);

        // Act
        var hash1 = board1.GetHash();
        var hash2 = board2.GetHash();
        var hash3 = board3.GetHash();

        // Assert - all hashes should be different
        hash1.Should().NotBe(hash2, "Red at (8,8) should differ from Red at (7,7)");
        hash1.Should().NotBe(hash3, "Red at (8,8) should differ from Blue at (8,8)");
        hash2.Should().NotBe(hash3, "Red at (7,7) should differ from Blue at (8,8)");
    }

    [Fact]
    public void HashIsDeterministic_SamePositionSameHash()
    {
        // Arrange
        var board1 = new Board()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue)
            .PlaceStone(9, 9, Player.Red);

        var board2 = new Board()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue)
            .PlaceStone(9, 9, Player.Red);

        // Act
        var hash1 = board1.GetHash();
        var hash2 = board2.GetHash();

        // Assert
        hash1.Should().Be(hash2, "identical positions must have identical hashes");
    }

    [Fact]
    public void NoCollision_TwoStonesVsTwoDifferentStones()
    {
        // This is the critical test that fails with simple XOR hashing
        // Arrange - two positions with 2 stones each
        var board1 = new Board()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 1, Player.Blue);

        var board2 = new Board()
            .PlaceStone(2, 2, Player.Red)
            .PlaceStone(3, 3, Player.Blue);

        // Act
        var hash1 = board1.GetHash();
        var hash2 = board2.GetHash();

        // Assert - MUST be different to prevent opening book corruption
        hash1.Should().NotBe(hash2,
            "positions with different stone placements must have different hashes");
    }

    [Fact]
    public void NoCollision_AllSingleStonePositionsUnique()
    {
        // Arrange - create many different positions and verify no collisions
        var hashes = new HashSet<ulong>();
        var collisions = new List<string>();

        // Test all single-stone positions for Red (256 positions)
        for (int x = 0; x < 16; x++)
        {
            for (int y = 0; y < 16; y++)
            {
                var board = new Board().PlaceStone(x, y, Player.Red);
                var hash = board.GetHash();
                if (!hashes.Add(hash))
                {
                    collisions.Add($"Red({x},{y})");
                }
            }
        }

        // Test all single-stone positions for Blue (256 positions)
        for (int x = 0; x < 16; x++)
        {
            for (int y = 0; y < 16; y++)
            {
                var board = new Board().PlaceStone(x, y, Player.Blue);
                var hash = board.GetHash();
                if (!hashes.Add(hash))
                {
                    collisions.Add($"Blue({x},{y})");
                }
            }
        }

        // Assert - no collisions should occur (512 unique hashes expected)
        collisions.Should().BeEmpty(
            $"all 512 single-stone positions should have unique hashes. " +
            $"Collisions: {string.Join(", ", collisions.Take(10))}");
    }

    [Fact]
    public void HashDistribution_BitDistributionIsReasonable()
    {
        // Arrange - collect hashes for many positions
        var hashes = new List<ulong>();

        for (int x = 0; x < 16; x++)
        {
            for (int y = 0; y < 16; y++)
            {
                hashes.Add(new Board().PlaceStone(x, y, Player.Red).GetHash());
            }
        }

        // Act - count bit positions that vary
        var bitVariance = new int[64];
        foreach (var hash in hashes)
        {
            for (int bit = 0; bit < 64; bit++)
            {
                if ((hash & (1UL << bit)) != 0)
                    bitVariance[bit]++;
            }
        }

        // Assert - each bit should be set in roughly 50% of hashes (+/- 30%)
        // With proper Zobrist, each bit should be ~50% 0 and ~50% 1
        var minExpected = hashes.Count * 0.2;  // 20%
        var maxExpected = hashes.Count * 0.8;  // 80%

        var poorBits = new List<int>();
        for (int bit = 0; bit < 64; bit++)
        {
            if (bitVariance[bit] < minExpected || bitVariance[bit] > maxExpected)
            {
                poorBits.Add(bit);
            }
        }

        // Allow some bits to be skewed, but not too many
        poorBits.Count.Should().BeLessThan(20,
            $"most bits should have reasonable distribution. " +
            $"Poor bits: {string.Join(", ", poorBits.Take(10))}");
    }
}
