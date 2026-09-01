using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class TranspositionTableTests
{
    [Fact]
    public void TTStoreAndLookup()
    {
        using TranspositionTable tt = new(1);
        TTEntry entry = new()
        {
            Hash = 0x1234567890ABCDEFUL,
            Score = 1500,
            Depth = 8,
            MoveX = 5,
            MoveY = 5,
            Type = TTEntryType.Exact,
            Age = 0,
        };
        tt.Store(entry);

        Assert.True(tt.Lookup(entry.Hash, out TTEntry got));
        Assert.Equal(entry.Score, got.Score);
        Assert.Equal(entry.Depth, got.Depth);
        Assert.Equal(entry.MoveX, got.MoveX);
        Assert.Equal(entry.MoveY, got.MoveY);
    }

    [Fact]
    public void TTMiss()
    {
        using TranspositionTable tt = new(1);
        Assert.False(tt.Lookup(0xDEADBEEFUL, out _));
    }

    [Fact]
    public void TTClear()
    {
        using TranspositionTable tt = new(1);
        tt.Store(new TTEntry { Hash = 0x1, Score = 100, Depth = 5, Type = TTEntryType.Exact });
        tt.Clear();
        Assert.False(tt.Lookup(0x1, out _));
    }

    [Fact]
    public void TTConcurrentAccess()
    {
        using TranspositionTable tt = new(4);
        Parallel.For(0, 100, n =>
        {
            tt.Store(new TTEntry { Hash = (ulong)n, Score = n, Depth = 5, Type = TTEntryType.Exact });
            tt.Lookup((ulong)n, out _);
        });
    }

    [Fact]
    public void TTDeepEntryNotTrampledByShallow()
    {
        using TranspositionTable tt = new(1);
        ulong hash = 0xDEADBEEFUL;
        tt.Store(new TTEntry { Hash = hash, Score = 9000, Depth = 10, MoveX = 5, MoveY = 5, Type = TTEntryType.Exact, Age = 0 });
        tt.Store(new TTEntry { Hash = hash, Score = 100, Depth = 2, MoveX = 3, MoveY = 3, Type = TTEntryType.Exact, Age = 0 });

        Assert.True(tt.Lookup(hash, out TTEntry got));
        Assert.Equal(9000, got.Score);
        Assert.Equal(10, got.Depth);
        Assert.Equal(5, got.MoveX);
    }

    [Fact]
    public void TTSameHashExactReplacesUpperAtSameDepth()
    {
        using TranspositionTable tt = new(1);
        ulong hash = 0xCAFEBABEUL;
        tt.Store(new TTEntry { Hash = hash, Score = 500, Depth = 5, Type = TTEntryType.UpperBound, Age = 0 });
        tt.Store(new TTEntry { Hash = hash, Score = 800, Depth = 5, Type = TTEntryType.Exact, Age = 0 });

        Assert.True(tt.Lookup(hash, out TTEntry got));
        Assert.Equal(800, got.Score);
        Assert.Equal(TTEntryType.Exact, got.Type);
    }

    [Fact]
    public void TTDeepEntrySurvivesMultipleShallowStores()
    {
        using TranspositionTable tt = new(1);
        ulong hash = 0x12345678UL;
        tt.Store(new TTEntry { Hash = hash, Score = 7000, Depth = 8, Type = TTEntryType.Exact, Age = 0 });

        for (byte d = 1; d <= 6; d++)
        {
            tt.Store(new TTEntry { Hash = hash, Score = d * 100, Depth = d, Type = TTEntryType.Exact, Age = 0 });
        }

        Assert.True(tt.Lookup(hash, out TTEntry got));
        Assert.Equal(7000, got.Score);
        Assert.Equal(8, got.Depth);
    }

    [Fact]
    public void TTDifferentHashPriorityApplied()
    {
        using TranspositionTable tt = new(1);

        tt.Store(new TTEntry { Hash = 0xAAA, Score = 5000, Depth = 10, Type = TTEntryType.Exact, Age = 0 });
        tt.Store(new TTEntry { Hash = 0xAAA, Score = 100, Depth = 3, Type = TTEntryType.Exact, Age = 0 });

        Assert.True(tt.Lookup(0xAAA, out TTEntry got));
        Assert.Equal(5000, got.Score);
        Assert.Equal(10, got.Depth);
    }

    [Fact]
    public void NullMoveTTDoesNotPoison()
    {
        SearchBoard sb = new(Board.NewBoard());
        using TranspositionTable tt = new(1);

        ulong parentHash = sb.Hash();
        tt.Store(new TTEntry
        {
            Hash = parentHash,
            Score = 5000,
            Depth = 8,
            Type = TTEntryType.Exact,
        });

        sb.MakeNullMove();
        ulong nullHash = sb.Hash();
        Assert.NotEqual(parentHash, nullHash);

        Assert.True(tt.Lookup(parentHash, out TTEntry entry));
        Assert.Equal(5000, entry.Score);

        Assert.False(tt.Lookup(nullHash, out _));
    }
}
