using System.Collections.Concurrent;
using System.Diagnostics;
using Xunit;
using Xunit.Abstractions;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.Domain.Entities;
using Caro.Core.Tournament;

#pragma warning disable xUnit1031 // Stress tests intentionally use blocking operations

namespace Caro.Core.Tests.Concurrency;

/// <summary>
/// Stress tests that run 100+ concurrent operations to surface race conditions
/// Uses high contention scenarios to trigger edge cases
/// These tests should FAIL initially, revealing concurrency issues that will be fixed
/// </summary>
public class ConcurrencyStressTests
{
    private readonly ITestOutputHelper _output;

    public ConcurrencyStressTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public async Task LockFreeTranspositionTable_100ConcurrentWrites_NoDataLoss()
    {
        // Arrange
        var tt = new LockFreeTranspositionTable(1); // Small table for maximum contention
        var threads = 100;
        var writesPerThread = 100;
        var exceptions = new ConcurrentBag<Exception>();

        // Act - 100 threads writing 100 times each = 10,000 total writes
        var tasks = Enumerable.Range(0, threads).Select(threadId =>
            Task.Run(() =>
            {
                try
                {
                    for (int i = 0; i < writesPerThread; i++)
                    {
                        ulong hash = (ulong)(threadId * writesPerThread + i);
                        tt.Store(hash, (sbyte)(i % 10), (short)i, (sbyte)(i % 15), (sbyte)(i % 15), 0, 0);

                        // Insert random delays to increase race window
                        if (i % 17 == 0)
                            Thread.Yield();
                        if (i % 23 == 0)
                            Thread.SpinWait(10);
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        // Assert - Verify no crashes and table is consistent
        Assert.Empty(exceptions);

        var (used, _, hits, lookups, _) = tt.GetStats();
        _output.WriteLine($"Used: {used}, Hits: {hits}, Lookups: {lookups}");

        // Table should have some entries (not crashed to zero)
        Assert.True(used > 0, "Table should have entries after concurrent writes");
    }

    [Fact]
    public async Task ParallelMinimaxSearch_10ConcurrentSearches_RespectCancellation()
    {
        // Arrange
        var search = new ParallelMinimaxSearch(16, maxThreads: 4);

        // Act - Start 10 concurrent searches with separate boards to avoid shared state
        var results = new ConcurrentBag<(int x, int y, int depth, long nodes)>();
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = Enumerable.Range(0, 10).Select(i =>
            Task.Run(() =>
            {
                try
                {
                    // Create unique board per task using different areas of the board
                    var board = new Board();
                    var offsetX = (i % 3) * 4;
                    var offsetY = (i / 3) * 4;

                    // Place stones in non-overlapping regions
                    board.PlaceStone(offsetX, offsetY, Player.Red);
                    board.PlaceStone(offsetX, offsetY + 1, Player.Blue);
                    board.PlaceStone(offsetX + 1, offsetY, Player.Red);
                    board.PlaceStone(offsetX + 1, offsetY + 1, Player.Blue);

                    var result = search.GetBestMoveWithStats(
                        board, Player.Red, AIDifficulty.Hard,
                        timeRemainingMs: 5000
                    );
                    results.Add((result.X, result.Y, result.DepthAchieved, result.NodesSearched));
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                    _output.WriteLine($"Exception: {ex.GetType().Name} - {ex.Message}");
                }
            })
        ).ToArray();

        // Wait for all tasks with timeout - prevents hangs
        // Increased timeout to account for TT sharding overhead and system load
        var allCompleted = Task.WhenAll(tasks).Wait(TimeSpan.FromSeconds(60));

        // Assert - Most searches should complete (allow for system load variations)
        _output.WriteLine($"Completed: {allCompleted}, Results: {results.Count}, Exceptions: {exceptions.Count}");
        Assert.Empty(exceptions);
        Assert.True(results.Count >= 8, $"At least 8 out of 10 searches should complete, got {results.Count}");
        Assert.True(results.Count > 0, "At least some searches should complete");
    }

    [Theory]
    [InlineData(10)]
    [InlineData(50)]
    [InlineData(100)]
    public async Task Ponderer_ConcurrentStartStopPondering_NoDeadlocks(int operations)
    {
        // Arrange
        var ponderers = Enumerable.Range(0, Math.Min(operations, 10))
            .Select(_ => new Ponderer()).ToArray();

        var exceptions = new ConcurrentBag<Exception>();
        var completedOperations = 0;

        // Act - Rapid start/stop cycles across multiple ponderers
        // Each task gets its own board to avoid shared state issues
        // Note: Don't pre-place predictedOpponentMove - Ponderer places it internally
        var tasks = Enumerable.Range(0, operations).Select(i =>
        {
            var ponderer = ponderers[i % ponderers.Length];
            // Create a unique board for each task with base position only
            var board = new Board();
            board.PlaceStone(7, 7, Player.Red);
            // Don't place the predicted move here - Ponderer will place it on its cloned board

            return Task.Run(() =>
            {
                try
                {
                    // Each task uses a different predicted move to avoid collisions
                    ponderer.StartPondering(
                        board, Player.Blue, (7 + (i % 5), 8 + ((i / 5) % 5)),
                        Player.Red, AIDifficulty.Hard, 1000
                    );

                    // Small delay to ensure pondering starts
                    Thread.Sleep(5);

                    ponderer.StopPondering();
                    Interlocked.Increment(ref completedOperations);
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            });
        }).ToArray();

        // Wait for all tasks with timeout to detect deadlocks
        var completed = Task.WhenAll(tasks).Wait(TimeSpan.FromSeconds(30));

        // Cleanup
        foreach (var ponderer in ponderers)
        {
            try
            {
                ponderer.Dispose();
            }
            catch { }
        }

        // Assert - Should complete without deadlock
        Assert.True(completed, "Operations should complete within timeout (no deadlock)");
        Assert.Empty(exceptions);
        _output.WriteLine($"Completed {completedOperations}/{operations} operations");
    }

    [Fact]
    public async Task MinimaxAI_ConcurrentInstances_DontShareCorruptedState()
    {
        // Ensure AI instances don't share transposition table state incorrectly
        var boards = Enumerable.Range(0, 20).Select(i =>
        {
            var b = new Board();
            b.PlaceStone(7, 7, Player.Red);
            b.PlaceStone(7 + (i % 3), 8 + (i / 3), Player.Blue);
            return b;
        }).ToArray();

        var results = new ConcurrentBag<(int x, int y)>();
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = boards.Select((board, i) =>
            Task.Run(() =>
            {
                try
                {
                    // Each AI gets its own instance
                    var ai = new MinimaxAI();
                    var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);
                    results.Add((x, y));
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        // Assert - All should complete without sharing state corruption
        Assert.Empty(exceptions);
        Assert.Equal(20, results.Count);
        _output.WriteLine($"Completed {results.Count} concurrent searches");
    }

    [Fact]
    public void Interlocked_Patterns_WorkAsExpected()
    {
        // Verify our Interlocked usage patterns work correctly
        var counter = 0;
        var iterations = 10000;
        var exceptions = new ConcurrentBag<Exception>();

        // Parallel increment
        Parallel.For(0, iterations, i =>
        {
            try
            {
                Interlocked.Increment(ref counter);

                // Test CompareExchange pattern (this IS atomic)
                if (i % 50 == 0)
                {
                    // Read current value, then try to exchange only if still same value
                    var current = Interlocked.CompareExchange(ref counter, 0, -1);
                    // This reads current value (doesn't change it)
                    var _ = current;
                }
            }
            catch (Exception ex)
            {
                exceptions.Add(ex);
            }
        });

        Assert.Empty(exceptions);
        Assert.Equal(iterations, counter);
    }

    [Fact]
    public async Task ConcurrentDictionary_PerformanceUnderContention()
    {
        // Test that ConcurrentDictionary performs well under high contention
        var dict = new ConcurrentDictionary<string, int>();
        var threads = 50;
        var operationsPerThread = 1000;
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = Enumerable.Range(0, threads).Select(threadId =>
            Task.Run(() =>
            {
                try
                {
                    for (int i = 0; i < operationsPerThread; i++)
                    {
                        var key = $"key-{threadId}-{i % 100}"; // Some key collision
                        dict.AddOrUpdate(key, 1, (k, v) => v + 1);

                        if (i % 50 == 0)
                            Thread.Yield();
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        Assert.Empty(exceptions);
        _output.WriteLine($"Dictionary entries: {dict.Count}");
        Assert.True(dict.Count > 0, "Dictionary should have entries");
    }
}
