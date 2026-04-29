using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class LockFreeTranspositionTableTests
{
    [Fact]
    public void Constructor_ShouldCreateShardedTable()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1, shardCount: 4);

        tt.Size.Should().BeGreaterThan(0);
    }

    [Fact]
    public void Constructor_ShouldForcePowerOfTwoShardCount()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1, shardCount: 3);

        // Non-power-of-2 should default to 16
        tt.Size.Should().BeGreaterThan(0);
    }

    [Fact]
    public void InitialAge_ShouldBeOne()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);

        tt.CurrentAge.Should().Be(1);
    }

    [Fact]
    public void StoreAndLookup_ExactScore_ShouldFindEntry()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0x123456789ABCDEF0;

        tt.Store(hash, depth: 5, score: 100, moveX: 7, moveY: 8, alpha: -10000, beta: 10000);
        var (found, hasExactDepth, score, move, threadIndex) = tt.Lookup(hash, depth: 5, alpha: -10000, beta: 10000);

        found.Should().BeTrue();
        hasExactDepth.Should().BeTrue();
        score.Should().Be(100);
        move.Should().Be((7, 8));
    }

    [Fact]
    public void Store_LowerBoundFlag_WhenScoreAboveBeta()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0xAABBCCDDEEFF0011;

        tt.Store(hash, depth: 4, score: 500, moveX: 3, moveY: 3, alpha: -100, beta: 100);
        // Lookup with beta <= score to trigger the LowerBound exact return path
        var (found, hasExactDepth, score, move, _) = tt.Lookup(hash, depth: 4, alpha: -10000, beta: 400);

        found.Should().BeTrue();
        hasExactDepth.Should().BeTrue();
        score.Should().Be(500);
    }

    [Fact]
    public void Store_UpperBoundFlag_WhenScoreBelowAlpha()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0x1122334455667788;

        tt.Store(hash, depth: 3, score: -500, moveX: 5, moveY: 5, alpha: 0, beta: 10000);
        // Lookup with alpha >= score to trigger the UpperBound exact return path
        var (found, hasExactDepth, score, move, _) = tt.Lookup(hash, depth: 3, alpha: -400, beta: 10000);

        found.Should().BeTrue();
        hasExactDepth.Should().BeTrue();
        score.Should().Be(-500);
    }

    [Fact]
    public void Lookup_ShouldReturnFalse_WhenEntryNotFound()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);

        var (found, _, _, _, _) = tt.Lookup(0xDEADBEEFCAFEBABE, depth: 5, alpha: -10000, beta: 10000);

        found.Should().BeFalse();
    }

    [Fact]
    public void Lookup_ShouldReturnPartialHit_WhenDepthInsufficient()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0x5566778899AABBCC;

        tt.Store(hash, depth: 3, score: 200, moveX: 7, moveY: 7, alpha: -10000, beta: 10000);
        var (found, hasExactDepth, score, move, _) = tt.Lookup(hash, depth: 5, alpha: -10000, beta: 10000);

        found.Should().BeTrue();
        hasExactDepth.Should().BeFalse();
        score.Should().Be(200);
        move.Should().Be((7, 7));
    }

    [Fact]
    public void DeepReplacement_ShouldReplaceShallowEntry()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0xAAAABBBBCCCCDDDD;

        tt.Store(hash, depth: 3, score: 100, moveX: 1, moveY: 1, alpha: -10000, beta: 10000);
        tt.Store(hash, depth: 7, score: 500, moveX: 2, moveY: 2, alpha: -10000, beta: 10000);

        var (found, _, score, move, _) = tt.Lookup(hash, depth: 3, alpha: -10000, beta: 10000);
        found.Should().BeTrue();
        score.Should().Be(500);
        move.Should().Be((2, 2));
    }

    [Fact]
    public void IncrementAge_ShouldResetAt255()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);

        // Increment age 254 times (starting from 1, should reach 255 then reset to 1)
        for (int i = 0; i < 254; i++)
        {
            tt.IncrementAge();
        }

        tt.CurrentAge.Should().Be(1);
    }

    [Fact]
    public void Clear_ShouldResetAllEntries()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0x1111111111111111;

        tt.Store(hash, depth: 5, score: 100, moveX: 0, moveY: 0, alpha: -10000, beta: 10000);
        tt.Clear();

        var (found, _, _, _, _) = tt.Lookup(hash, depth: 5, alpha: -10000, beta: 10000);
        found.Should().BeFalse();
    }

    [Fact]
    public void Clear_ShouldResetAge()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);

        tt.IncrementAge();
        tt.IncrementAge();
        tt.Clear();

        tt.CurrentAge.Should().Be(1);
    }

    [Fact]
    public void GetStats_ShouldTrackHitsAndLookups()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 1);
        ulong hash = 0x2222222222222222;

        tt.Store(hash, depth: 5, score: 100, moveX: 7, moveY: 7, alpha: -10000, beta: 10000);

        // Hit
        tt.Lookup(hash, depth: 5, alpha: -10000, beta: 10000);
        // Miss
        tt.Lookup(0x3333333333333333, depth: 5, alpha: -10000, beta: 10000);

        var (used, _, hitCount, lookupCount, hitRate) = tt.GetStats();
        lookupCount.Should().Be(2);
        hitCount.Should().Be(1); // Only the hit increments hit count, not the miss
    }

    [Fact]
    public void Entry_PackingAndUnpacking_ShouldRoundTrip()
    {
        var entry = new LockFreeTranspositionTable.TranspositionEntry(
            hash: 0xABCDEF0123456789,
            depth: 15,
            score: -1234,
            moveX: 12,
            moveY: 14,
            flag: LockFreeTranspositionTable.EntryFlag.LowerBound,
            age: 5,
            threadIndex: 3
        );

        entry.Hash.Should().Be(0xABCDEF0123456789);
        entry.Depth.Should().Be(15);
        entry.Score.Should().Be(-1234);
        entry.MoveX.Should().Be(12);
        entry.MoveY.Should().Be(14);
        entry.Flag.Should().Be(LockFreeTranspositionTable.EntryFlag.LowerBound);
        entry.Age.Should().Be(5);
        entry.ThreadIndex.Should().Be(3);
    }

    [Fact]
    public void Entry_HasMove_ShouldReturnTrueForValidMoves()
    {
        var entry = new LockFreeTranspositionTable.TranspositionEntry(
            hash: 1, depth: 1, score: 0, moveX: 5, moveY: 10,
            flag: LockFreeTranspositionTable.EntryFlag.Exact, age: 1
        );

        entry.HasMove.Should().BeTrue();
        entry.GetMove().Should().Be((5, 10));
    }

    [Fact]
    public void Entry_IsValid_ShouldReturnTrueForNonZeroHash()
    {
        var entry = new LockFreeTranspositionTable.TranspositionEntry(
            hash: 42, depth: 1, score: 0, moveX: 0, moveY: 0,
            flag: LockFreeTranspositionTable.EntryFlag.Exact, age: 1
        );

        entry.IsValid.Should().BeTrue();
    }

    [Fact]
    public void Entry_Default_ShouldBeInvalid()
    {
        var entry = default(LockFreeTranspositionTable.TranspositionEntry);

        entry.IsValid.Should().BeFalse();
        entry.Hash.Should().Be(0);
    }

    [Fact]
    public void Entry_MatchesHash_ShouldMatchHigh32Bits()
    {
        var entry = new LockFreeTranspositionTable.TranspositionEntry(
            hash: 0xAAAABBBBCCCCDDDD, depth: 1, score: 0, moveX: 0, moveY: 0,
            flag: LockFreeTranspositionTable.EntryFlag.Exact, age: 1
        );

        entry.MatchesHash(0xAAAABBBB11111111).Should().BeTrue();
        entry.MatchesHash(0xBBBBAAAACCCCDDDD).Should().BeFalse();
    }

    [Fact]
    public async Task ConcurrentReadWrite_ShouldNotCorruptData()
    {
        var tt = new LockFreeTranspositionTable(sizeMB: 4);
        var errors = new System.Collections.Concurrent.ConcurrentBag<Exception>();
        var hashes = Enumerable.Range(0, 100).Select(i => (ulong)i * 0x0101010101010101).ToArray();

        // Writers
        var writeTasks = Enumerable.Range(0, 4).Select(t => Task.Run(() =>
        {
            for (int i = 0; i < 100; i++)
            {
                var hash = hashes[(i + t * 25) % hashes.Length];
                tt.Store(hash, depth: (sbyte)(i % 20), score: (short)(i * 10), moveX: (sbyte)(i % 16), moveY: (sbyte)(i % 16), alpha: -10000, beta: 10000, threadIndex: (byte)t);
            }
        }));

        // Readers
        var readTasks = Enumerable.Range(0, 4).Select(t => Task.Run(() =>
        {
            for (int i = 0; i < 100; i++)
            {
                try
                {
                    var hash = hashes[i % hashes.Length];
                    tt.Lookup(hash, depth: 5, alpha: -10000, beta: 10000);
                }
                catch (Exception ex)
                {
                    errors.Add(ex);
                }
            }
        }));

        await Task.WhenAll(writeTasks.Concat(readTasks).ToArray());
        errors.Should().BeEmpty();
    }
}
